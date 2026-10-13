#!/usr/bin/env python3
"""Git checks and conservative finalization for scheduled upstream merges."""

import argparse
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile


def git(repo, *args, env=None, check=True):
    result = subprocess.run(
        ['git', '-C', str(repo), *args], env=env, capture_output=True, text=True,
    )
    if check and result.returncode:
        raise RuntimeError(f"git {' '.join(args)}: {result.stderr or result.stdout}")
    return result


def review(repo, exclusions=False):
    # Stage into a disposable index: failed checks must leave :2/:3 available
    # to the next Codex repair pass, including modify/delete conflicts.
    with tempfile.TemporaryDirectory(prefix='upstream-review-') as tmp:
        env = dict(os.environ, GIT_INDEX_FILE=str(Path(tmp) / 'index'))
        # Copy the current merge index, including newly tracked upstream files
        # under ignored paths. Rebuilding from HEAD would omit those files.
        index = git(repo, 'rev-parse', '--path-format=absolute', '--git-path', 'index').stdout.strip()
        shutil.copyfile(index, env['GIT_INDEX_FILE'])
        git(repo, 'add', '--all', env=env)
        git(repo, 'diff', '--cached', '--check', env=env)
        markers = git(repo, 'grep', '--cached', '-n', '-I', '-E',
                      r'^(<<<<<<<|>>>>>>>)( |$)', '--', '.', env=env, check=False)
        if markers.returncode != 1:
            raise RuntimeError(markers.stdout or markers.stderr or 'Conflict marker check failed')
        if exclusions:
            names = git(repo, 'ls-files', '-z', env=env).stdout.split('\0')
            forbidden = [name for name in names if re.search(
                r'(^|/)(shared.?pool|shared_account|shared-account|SharedPool|add-shared-account-pool)',
                name, re.IGNORECASE)]
            if forbidden:
                raise RuntimeError('Shared account pool paths remain: ' + ', '.join(forbidden))
            subprocess.run([sys.executable, str(repo / '.github/check-upstream-exclusions.py'),
                            str(repo)], env=env, check=True)


def unfinished(repo):
    for name in ('MERGE_HEAD', 'CHERRY_PICK_HEAD', 'REVERT_HEAD', 'REBASE_HEAD',
                 'rebase-merge', 'rebase-apply', 'sequencer', 'BISECT_START', 'index.lock'):
        path = git(repo, 'rev-parse', '--path-format=absolute', '--git-path', name).stdout.strip()
        if Path(path).exists():
            return True
    return False


def clean(repo):
    # Git-ignored build output may be removed with a clean disposable worktree.
    return not git(repo, 'status', '--porcelain=v1', '--untracked-files=all').stdout and not unfinished(repo)


def worktrees(repo):
    rows = []
    for block in git(repo, 'worktree', 'list', '--porcelain', '-z').stdout.split('\0\0'):
        row = dict(field.partition(' ')[::2] for field in block.split('\0') if field)
        if 'worktree' in row:
            rows.append(row)
    return rows


def finalize(args):
    repo = args.repo.resolve()
    root = args.worktree_root.resolve()
    ref = f'refs/remotes/{args.remote}/{args.target}'
    # Never decide deletions from a stale tracking ref, even after a push.
    git(repo, 'fetch', '--no-tags', args.remote,
        f'refs/heads/{args.target}:{ref}')
    remote_head = git(repo, 'rev-parse', '--verify', ref).stdout.strip()
    branch = git(repo, 'symbolic-ref', '--quiet', '--short', 'HEAD', check=False).stdout.strip()
    # Ignored build output does not prevent fast-forward; Git protects files
    # that would actually be overwritten. No force/reset/stash is used.
    if (branch == args.target and not unfinished(repo)
            and not git(repo, 'status', '--porcelain=v1', '--untracked-files=all').stdout
            and git(repo, 'merge-base', '--is-ancestor', 'HEAD', remote_head, check=False).returncode == 0):
        git(repo, 'merge', '--ff-only', '--no-overwrite-ignore', remote_head)
        print(f'Primary {args.target} synchronized to {remote_head}', flush=True)
    else:
        print('Preserved primary worktree: wrong branch, local changes, unfinished operation or divergent history', flush=True)
    if args.sync_only:
        return
    prefix = 'refs/heads/' + args.branch_prefix
    for row in worktrees(repo):
        path = Path(row['worktree']).resolve()
        branch_ref = row.get('branch', '')
        if not branch_ref.startswith(prefix) or path == repo or root not in path.parents:
            continue
        if ('locked' in row or 'prunable' in row or not path.is_dir()
                or not clean(path)
                or git(repo, 'merge-base', '--is-ancestor', row['HEAD'], remote_head, check=False).returncode):
            print(f'Preserved candidate: {path} ({branch_ref})', flush=True)
            continue
        # No --force: Git rechecks locks, dirty files and submodules itself.
        git(repo, 'worktree', 'remove', str(path))
        print(f'Removed merged worktree: {path}', flush=True)
    # Also collect historical branches whose worktree was already removed.
    # Registered/locked/missing worktrees still protect their branches.
    attached = {row.get('branch') for row in worktrees(repo)}
    rows = git(repo, 'for-each-ref', '--format=%(refname) %(objectname)', prefix + '*').stdout.splitlines()
    for row in rows:
        branch_ref, head = row.split()
        if branch_ref in attached or not branch_ref.startswith(prefix):
            continue
        if git(repo, 'merge-base', '--is-ancestor', head, remote_head, check=False).returncode:
            print(f'Preserved unmerged branch: {branch_ref}', flush=True)
            continue
        # Compare-and-delete after ancestry and attachment checks; never -D.
        git(repo, 'update-ref', '-d', branch_ref, head)
        print(f'Removed merged branch: {branch_ref}', flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest='command', required=True)
    check = commands.add_parser('review')
    check.add_argument('--repo', type=Path, required=True)
    check.add_argument('--exclusions', action='store_true')
    finish = commands.add_parser('finalize')
    finish.add_argument('--repo', type=Path, required=True)
    finish.add_argument('--remote', required=True)
    finish.add_argument('--target', required=True)
    finish.add_argument('--worktree-root', type=Path, required=True)
    finish.add_argument('--branch-prefix', required=True)
    finish.add_argument('--sync-only', action='store_true')
    args = parser.parse_args()
    try:
        if args.command == 'review':
            review(args.repo.resolve(), args.exclusions)
        else:
            if not args.branch_prefix.startswith('automation/') or args.branch_prefix == 'automation/':
                raise RuntimeError('An explicit automation branch prefix is required')
            finalize(args)
    except (RuntimeError, OSError, subprocess.CalledProcessError) as exc:
        print(f'Upstream Git check/finalization failed: {exc}', file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
