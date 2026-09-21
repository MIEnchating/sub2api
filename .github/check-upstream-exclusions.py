#!/usr/bin/env python3
"""Reject retired upstream features while preserving shipped database migrations."""
import argparse
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


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('root', type=Path, nargs='?', default=Path('.'))
    args = parser.parse_args()
    root = args.root.resolve()
    rules = json.loads((root / '.github/upstream-exclusions.json').read_text())
    names = subprocess.check_output(
        ['git', '-C', str(root), 'ls-files', '-z', '--cached', '--others', '--exclude-standard'],
    ).decode().split('\0')
    failures = [error for name in sorted(set(names)) if name for error in check_path(root, name, rules)]
    if failures:
        print('\n'.join(failures))
        return 1
    print('上游功能排除检查通过')
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
