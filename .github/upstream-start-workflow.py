#!/usr/bin/env python3
"""Recover a missing workflow run without changing source history or release tags."""
import argparse
import json
import re
import subprocess
import sys


class PlatformDisabled(RuntimeError):
    pass


def run(*args, stdin=None):
    result = subprocess.run(args, input=stdin, capture_output=True, text=True, timeout=60)
    if result.returncode:
        message = result.stderr.strip() or result.stdout.strip()
        if 'Actions has been disabled for this repository' in message:
            raise PlatformDisabled(message)
        raise RuntimeError(message or f'{args[0]} exited with {result.returncode}')
    return result.stdout


def start(args):
    workflows = json.loads(run('gh', 'api', f'repos/{args.repo}/actions/workflows?per_page=100'))
    matches = [w for w in workflows['workflows'] if w['name'] == args.workflow]
    if len(matches) != 1:
        raise RuntimeError(f'Cannot uniquely identify workflow {args.workflow}')
    workflow = matches[0]
    # A manual dispatch selects a branch/tag. Verify it still names the exact
    # candidate so a concurrent push cannot silently validate another commit.
    refs = run('git', '-C', args.directory, 'ls-remote', args.remote,
               f'refs/heads/{args.ref}', f'refs/tags/{args.ref}', f'refs/tags/{args.ref}^{{}}')
    refs = dict(line.split()[::-1] for line in refs.splitlines())
    head = refs.get(f'refs/heads/{args.ref}')
    tag = refs.get(f'refs/tags/{args.ref}^{{}}', refs.get(f'refs/tags/{args.ref}'))
    if (head and tag) or (head or tag) != args.commit:
        raise RuntimeError('Dispatch ref is ambiguous, missing or no longer points to the candidate')
    source = run('git', '-C', args.directory, 'show', f'{args.commit}:{workflow["path"]}')
    if not re.search(r'^  workflow_dispatch:', source, re.MULTILINE):
        raise RuntimeError(f'{args.workflow} at {args.commit} does not support workflow_dispatch')
    if workflow['state'] == 'disabled_inactivity':
        run('gh', 'api', '--method', 'PUT', f'repos/{args.repo}/actions/workflows/{workflow["id"]}/enable')
    elif workflow['state'] != 'active':
        raise RuntimeError(f'Workflow state is {workflow["state"]}; requires repository owner review')
    payload = {'ref': args.ref}
    if args.workflow == 'Release':
        payload['inputs'] = {'tag': args.ref, 'dry_run': False, 'simple_release': False}
    run('gh', 'api', '--method', 'POST',
        f'repos/{args.repo}/actions/workflows/{workflow["id"]}/dispatches',
        '--input', '-', stdin=json.dumps(payload))
    print(f'Dispatched {args.workflow} for {args.ref} ({args.commit}); still awaiting successful checks')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ('repo', 'directory', 'remote', 'workflow', 'commit', 'ref'):
        parser.add_argument('--' + name, required=True)
    args = parser.parse_args()
    try:
        start(args)
    except PlatformDisabled as exc:
        print(f'GitHub Actions 平台拒绝启动：{exc}。仓库设置显示 enabled=true 也不代表可以执行；'
              f'请仓库所有者检查 https://github.com/{args.repo}/actions 的禁用提示，'
              '若无法自行解除则联系 GitHub Support。代码修复不能解除平台限制。', file=sys.stderr)
        return 3
    except (RuntimeError, ValueError, KeyError, OSError, subprocess.TimeoutExpired) as exc:
        print(f'Workflow startup recovery failed: {exc}', file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
