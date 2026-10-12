import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[1]


class GoToolchainTest(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory(prefix='sync-go-test-')
        self.addCleanup(temp.cleanup)
        self.root = Path(temp.name)
        (self.root / 'backend').mkdir()
        self.mod = self.root / 'backend/go.mod'
        self.mod.write_text('module example.test/backend\n\ngo 1.27.2\n')
        self.bin = self.root / 'bin'
        self.bin.mkdir()
        docker = self.bin / 'docker'
        docker.write_text('#!/usr/bin/env python3\nimport json, sys\nprint(json.dumps(sys.argv[1:]))\n')
        docker.chmod(0o755)
        self.env = dict(os.environ, PATH=str(self.bin) + os.pathsep + os.environ['PATH'],
                        WORKTREE=str(self.root), TOOLS_DIR=str(self.root / 'tools'), GO_IMAGE='')
        source = (ROOT / 'scripts/upstream-sync-validation.sh').read_text()
        self.functions = '\n'.join(
            name + '() {' + source.split(name + '() {', 1)[1].split('\n}\n', 1)[0] + '\n}'
            for name in ('backend_go_image', 'docker_go', 'check_backend_lint', 'check_backend_security'))

    def run_shell(self, command, **env):
        return subprocess.run(['bash', '-ec', self.functions + '\n' + command],
                              env=self.env | env, text=True, capture_output=True)

    def test_default_tracks_candidate_and_repair_in_same_process(self):
        result = self.run_shell('backend_go_image\n'
                                'printf "go 1.28.1\\n" > "$WORKTREE/backend/go.mod"\n'
                                'backend_go_image')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.splitlines(), ['golang:1.27.2', 'golang:1.28.1'])

    def test_explicit_image_is_preserved(self):
        result = self.run_shell('backend_go_image', GO_IMAGE='registry.example/go:1.27.3')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.strip(), 'registry.example/go:1.27.3')

    def test_invalid_or_missing_module_never_launches_container(self):
        for value in ('', 'module example.test/backend\n', 'go latest\n',
                      'go 1.27.2\ngo 1.28.0\n', 'go 1.27.2;echo unsafe\n', None):
            with self.subTest(value=value):
                if value is None:
                    self.mod.unlink()
                else:
                    self.mod.write_text(value)
                result = self.run_shell('docker_go go test ./...')
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(result.stdout, '')

    def container_args(self, command):
        result = self.run_shell(command)
        self.assertEqual(result.returncode, 0, result.stderr)
        args = json.loads(result.stdout)
        self.assertIn('GOTOOLCHAIN=local', args)
        self.assertEqual(args[args.index('-w') + 1:args.index('-w') + 3],
                         ['/src/backend', 'golang:1.27.2'])
        self.assertIn(str(self.root / 'tools') + ':/tools', args)
        return args

    def test_all_failed_checks_use_candidate_image(self):
        for command in ('docker_go go test ./...', 'docker_go go test -tags=unit ./...',
                        'docker_go go test -tags=integration ./...', 'docker_go go vet ./...',
                        'check_backend_lint', 'check_backend_security',
                        'docker_go /bin/bash -ec "CGO_ENABLED=0 go build ./cmd/server"'):
            with self.subTest(command=command):
                self.container_args(command)

    def test_integration_tests_can_reach_ephemeral_testcontainers(self):
        args = self.container_args('docker_go go test -tags=integration ./...')
        self.assertIn('/var/run/docker.sock:/var/run/docker.sock', args)
        self.assertIn('TESTCONTAINERS_HOST_OVERRIDE=127.0.0.1', args)
        self.assertEqual(args[args.index('--network') + 1], 'host')
        self.assertNotIn('/var/run/docker.sock:/var/run/docker.sock',
                         self.container_args('docker_go go test ./...'))

    def test_tools_rebuild_after_go_upgrade_and_do_not_use_legacy_cache(self):
        go = self.bin / 'go'
        go.write_text('''#!/bin/bash
set -eu
if [[ "$1" == env ]]; then
  case "$2" in
    GOVERSION) echo "$FAKE_GO_VERSION" ;;
    GOOS) echo linux ;;
    GOARCH) echo amd64 ;;
  esac
else
  echo "$FAKE_GO_VERSION $*" >> "$INSTALL_LOG"
  [[ "${FAIL_INSTALL:-false}" != true ]] || exit 23
  tool="${2##*/}"
  tool="${tool%@*}"
  printf '#!/bin/sh\\necho checked\\n' > "$GOBIN/$tool"
  chmod +x "$GOBIN/$tool"
fi
''')
        go.chmod(0o755)
        for check, tool in (('check_backend_lint', 'golangci-lint'),
                            ('check_backend_security', 'govulncheck')):
            with self.subTest(check=check):
                cache = self.root / check
                cache.mkdir()
                legacy = cache / tool
                legacy.write_text('#!/bin/sh\nexit 99\n')
                legacy.chmod(0o755)
                script = self.container_args(check)[-1].replace('/tools/', str(cache) + '/')
                log = cache / 'install.log'
                for version, installs in (('go1.27.2', 1), ('go1.27.2', 1), ('go1.28.1', 2)):
                    result = subprocess.run(['bash', '-ec', script], text=True, capture_output=True,
                                            env=self.env | {'FAKE_GO_VERSION': version, 'INSTALL_LOG': str(log)})
                    self.assertEqual(result.returncode, 0, result.stderr)
                    self.assertEqual(result.stdout.strip(), 'checked')
                    self.assertEqual(len(log.read_text().splitlines()), installs)
                if tool == 'golangci-lint':
                    self.assertIn('@v2.14.0', log.read_text())
                result = subprocess.run(['bash', '-ec', script], text=True, capture_output=True,
                                        env=self.env | {'FAKE_GO_VERSION': 'go1.29.0',
                                                        'INSTALL_LOG': str(log), 'FAIL_INSTALL': 'true'})
                self.assertEqual(result.returncode, 23)
                self.assertNotIn('checked', result.stdout)


if __name__ == '__main__':
    unittest.main()
