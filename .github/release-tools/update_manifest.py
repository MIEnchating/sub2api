#!/usr/bin/env python3
"""Bind web updates to images pushed from the exact release commit."""
import argparse
from datetime import date
import json
from pathlib import Path
import re


def generate(version, revision, metadata_dir, simple=False):
    if not re.fullmatch(r'[0-9]{4}\.(0?[1-9]|1[0-2])\.(0?[1-9]|[12][0-9]|3[01])', version):
        raise ValueError('web updates require a fork date release version')
    date(*map(int, version.split('.')))
    if not re.fullmatch(r'[a-f0-9]{40}', revision):
        raise ValueError('invalid release revision')
    images = {}
    for arch in (['amd64'] if simple else ['amd64', 'arm64']):
        data = json.loads((Path(metadata_dir) / f'image-{arch}.json').read_text())
        digest = data.get('containerimage.digest', '')
        if not re.fullmatch(r'sha256:[a-f0-9]{64}', digest):
            raise ValueError(f'missing pushed image digest for {arch}')
        images[arch] = f'ghcr.io/mienchating/sub2api@{digest}'
    return dict(schema_version=1, version=version, revision=revision, images=images)


if __name__ == '__main__':
    p = argparse.ArgumentParser(description=__doc__)
    for arg in ('version', 'revision', 'metadata-dir', 'output'):
        p.add_argument('--' + arg, required=True)
    p.add_argument('--simple', action='store_true')
    a = p.parse_args()
    Path(a.output).write_text(json.dumps(generate(a.version, a.revision, a.metadata_dir, a.simple), indent=2) + '\n')
