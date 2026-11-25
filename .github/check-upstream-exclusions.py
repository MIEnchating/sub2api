#!/usr/bin/env python3
"""Reject retired upstream features while preserving shipped database migrations."""
import argparse
from collections import Counter
import os
import json
from pathlib import Path
import re
import subprocess


def check_path(root: Path, relative: str, rules: dict) -> list[str]:
    path = root / relative
    if not path.is_file():
        return []
    failures = []
    for rule in rules['features']:
        if relative in rule.get('preserved_paths', []):
            continue
        if re.search(rule['path_pattern'], relative, re.IGNORECASE):
            failures.append(f"{relative}: {rule['name']} 专属文件不得重新引入")
            continue
        if not relative.startswith(tuple(rule.get('content_roots', []))):
            continue
        # Shared tests may assert that retired routes/fields are absent.
        if relative.endswith(('_test.go', '.spec.ts')):
            continue
        pattern = rule.get('content_pattern')
        if pattern:
            for number, line in enumerate(path.read_text(encoding='utf-8', errors='replace').splitlines(), 1):
                if re.search(pattern, line, re.IGNORECASE):
                    failures.append(f"{relative}:{number}: {rule['name']} 实现或入口不得重新引入")
    return failures



def reference_contents(root, ref, name):
    result = subprocess.run(
        ['git', '-C', str(root), 'show', f'{ref}:{name}'], capture_output=True,
    )
    if result.returncode:
        raise RuntimeError(f'无法读取主上游文件 {ref}:{name}')
    return result.stdout


def primary_batch_markers(name, source, marker):
    """Compare native references while allowing unrelated generated fields."""
    lines = [line.strip() for line in source.splitlines()]
    if name == 'backend/internal/repository/regional_funds_runtime.go':
        # Regional funds cannot settle a separately enabled queue. This startup
        # refusal adds no image execution path and must match as a whole block.
        guard = [
            'if cfg.BatchImage.QueueEnabled {',
            'return nil, errors.New("batch image queue must be drained and disabled before coordinated funds")',
            '}',
        ]
        start = next((i for i in range(len(lines)) if lines[i:i + len(guard)] == guard), None)
        if start is not None:
            del lines[start:start + len(guard)]

    markers = Counter()
    for line in lines:
        if not marker.search(line):
            continue
        if name == 'backend/cmd/server/wire_gen.go':
            # Wire prints one argument list for all providers. Compare image
            # inputs in order without treating unrelated providers as changes.
            call = re.fullmatch(
                r'(handlers := handler\.ProvideHandlers|v := provideCleanup)\(([\w, ]+)\)',
                line,
            )
            if call is not None:
                args = [arg.strip() for arg in call[2].split(',')]
                image_args = [arg for arg in args if marker.search(arg) or arg.startswith('asyncImage')]
                markers[f'{call[1]}({", ".join(image_args)})'] += 1
                continue
        if name == 'backend/ent/client.go' and re.fullmatch(r'(?:(?:c\.)?[A-Z]\w*,\s*)+', line):
            refs = [part.strip() for part in line.rstrip(',').split(',') if marker.search(part)]
            if all(re.fullmatch(r'(?:c\.)?BatchImage(?:Event|Item|Job)', part) for part in refs):
                markers.update('ent client list: ' + part for part in refs)
                continue
        if name == 'backend/ent/group.go' and re.fullmatch(r'case group\.Field\w+(?:, group\.Field\w+)*:', line):
            refs = [field for field in line[5:-1].split(', ') if marker.search(field)]
            if refs == ['group.FieldAllowBatchImageGeneration']:
                markers.update(['ent group case: group.FieldAllowBatchImageGeneration'])
                continue
        if name == 'backend/internal/service/api_key_auth_cache_impl.go':
            mapping = re.fullmatch(
                r'AllowBatchImageGeneration:\s*(apiKey\.Group|group|snapshot\.Group|snapshot)\.AllowBatchImageGeneration,',
                line,
            )
            if mapping is not None:
                direction = 'from group' if mapping[1] in {'apiKey.Group', 'group'} else 'from snapshot'
                markers.update(['AllowBatchImageGeneration: ' + direction])
                continue
        markers[line] += 1
    return markers


def check_primary_batch_image(root, names, ref, rule):
    """Keep the primary's batch image code and reject additions from forks."""
    ref = subprocess.check_output(
        ['git', '-C', str(root), 'rev-parse', '--verify', '--end-of-options', f'{ref}^{{commit}}'],
        text=True,
    ).strip()
    official = set(subprocess.check_output(
        ['git', '-C', str(root), 'ls-tree', '-r', '--name-only', '-z', ref],
        text=True,
    ).split('\0'))
    dedicated = re.compile(rule['path_pattern'], re.IGNORECASE)
    marker = re.compile(rule['content_pattern'], re.IGNORECASE)
    roots = tuple(rule['content_roots'])
    preserved = set(rule['preserved_paths'])
    # Select mixed files in the primary tree without reading every binary or
    # running a Git subprocess for every ordinary source file in the project.
    hits = subprocess.run(
        ['git', '-C', str(root), 'grep', '-z', '-I', '-i', '-l', '-E',
         'batch[_-]?image|/images/batches|/key-image-batches|批量生图|批量图片生成',
         ref, '--', *roots], capture_output=True, text=True,
    )
    if hits.returncode not in (0, 1):
        raise RuntimeError(hits.stderr or '无法读取主上游的原生批量生图标识')
    official_mixed = {path.split(':', 1)[1] for path in hits.stdout.split('\0') if path}
    failures = []
    for name in sorted((names | official_mixed | {p for p in official if dedicated.search(p)}) - preserved):
        path = root / name
        if dedicated.search(name):
            if name in official:
                if not path.is_file() or path.read_bytes() != reference_contents(root, ref, name):
                    failures.append(f'{name}: 必须保留主上游原生批量生图文件')
            elif path.is_file():
                failures.append(f'{name}: 不得引入 fork 的批量生图专属文件')
            continue
        if not name.startswith(roots) or name.endswith(('_test.go', '.spec.ts')):
            continue
        actual = path.read_text(encoding='utf-8', errors='replace') if path.is_file() else ''
        actual_markers = primary_batch_markers(name, actual, marker)
        expected_markers = Counter()
        if name in official_mixed:
            source = reference_contents(root, ref, name).decode('utf-8', errors='replace')
            expected_markers = primary_batch_markers(name, source, marker)
        if actual_markers != expected_markers:
            failures.append(f'{name}: 批量生图接线偏离主上游，需逐段核对来源')
    return failures


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('root', type=Path, nargs='?', default=Path('.'))
    parser.add_argument('--primary', default=os.environ.get('SUB2API_PRIMARY_IMAGE_REF'),
                        help='primary upstream commit used for native batch image checks')
    args = parser.parse_args()
    root = args.root.resolve()
    rules = json.loads((root / '.github/upstream-exclusions.json').read_text())
    names = subprocess.check_output(
        ['git', '-C', str(root), 'ls-files', '-z', '--cached', '--others', '--exclude-standard'],
    ).decode().split('\0')
    names = set(names) - {''}
    failures = []
    if args.primary:
        primary_rule, = [rule for rule in rules['features'] if rule['name'] == '批量生图']
        failures.extend(check_primary_batch_image(root, names, args.primary, primary_rule))
        rules['features'].remove(primary_rule)
        rules['features'].append(rules['fork_image_generation'])
    failures.extend(error for name in sorted(names) for error in check_path(root, name, rules))
    if failures:
        print('\n'.join(failures))
        return 1
    print('上游功能排除检查通过')
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
