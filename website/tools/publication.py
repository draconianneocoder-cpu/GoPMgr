# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
"""Output-only commit preparation. Transport is intentionally absent."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile


def git(repo, *args, data=None):
    # Ignore ambient Git redirection, identities, filters and credential settings.
    env = {k: v for k, v in os.environ.items() if not k.startswith('GIT_')}
    env.update(GIT_CONFIG_NOSYSTEM='1', GIT_CONFIG_GLOBAL=os.devnull,
               GIT_AUTHOR_NAME='GoPMgr website preparation',
               GIT_AUTHOR_EMAIL='website-preparation@invalid',
               GIT_COMMITTER_NAME='GoPMgr website preparation',
               GIT_COMMITTER_EMAIL='website-preparation@invalid')
    return subprocess.check_output(['git', '-c', 'core.hooksPath=' + os.devnull,
                                    '-C', str(repo), *args], input=data, env=env).decode().strip()


def local_repository(path):
    value = str(path)
    if '://' in value or ':' in value or not Path(path).is_absolute():
        raise ValueError('only absolute filesystem repositories are allowed')
    resolved = Path(path).resolve(strict=True)
    if not resolved.is_dir():
        raise ValueError('repository must be a directory')
    git(resolved, 'rev-parse', '--git-dir')
    return resolved


def stage(public, expected, inventory, parent_repository=None, expected_tip=None):
    """Create isolated commit objects without touching a source index or any remote.

    expected is the verified byte inventory, not a digest supplied by the output.
    A parent, when specified, is fetched only from an absolute local repository.
    No source metadata, acceptance report, or .git directory enters the tree.
    """
    if inventory(public) != expected:
        raise ValueError('artifact differs before staging')
    if any(item['mode'] != '100644' for item in expected.values()):
        raise ValueError('publication files must have mode 100644')
    target = Path(tempfile.mkdtemp(prefix='website-candidate-', dir=public.parent.parent))
    git(target, 'init', '--quiet')
    parents = []
    if parent_repository is not None:
        remote = local_repository(parent_repository)
        observed = git(remote, 'rev-parse', '--verify', 'refs/heads/gh-pages')
        if observed != expected_tip:
            raise ValueError('stale gh-pages tip; candidate aborted')
        git(target, '-c', 'protocol.file.allow=always', 'fetch', '--quiet', '--no-tags', str(remote), observed)
        parents = ['-p', observed]
    elif expected_tip is not None:
        raise ValueError('expected tip requires a local parent repository')
    records = []
    for name in sorted(expected):
        if any(c in name for c in ('\x00', '\n', '\r', '\t')) or name.startswith('.git/') or name == '.git':
            raise ValueError('unsafe output path')
        blob = git(target, 'hash-object', '-w', '--stdin', '--no-filters', data=(public / name).read_bytes())
        records.append(f'100644 {blob}\t{name}\0'.encode())
    git(target, 'update-index', '-z', '--index-info', data=b''.join(records))
    tree = git(target, 'write-tree')
    commit = git(target, 'commit-tree', tree, *parents, data=b'Prepare exact GoPMgr website artifact locally\n')
    git(target, 'update-ref', 'refs/heads/gh-pages', commit)
    # Read committed blobs back: Git attributes and filters cannot change bytes.
    for name, item in expected.items():
        data = subprocess.check_output(['git', '-C', str(target), 'show', f'{commit}:{name}'])
        if hashlib.sha256(data).hexdigest() != item['sha256']:
            raise ValueError('committed artifact differs')
    if inventory(public) != expected:
        raise ValueError('artifact changed during staging')
    receipt = {'repository': str(target), 'commit': commit, 'tree': tree,
               'expected_parent': expected_tip, 'publication_authorized': False,
               'note': 'Local candidate only. No push performed; acceptance remains separate.'}
    (target / 'candidate-receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
    return receipt


def validate_transaction(candidate_repository, parent_repository, expected_tip):
    """Read-only preflight. A matching lease alone never authorizes a rewrite."""
    candidate = local_repository(candidate_repository)
    remote = local_repository(parent_repository)
    observed = git(remote, 'rev-parse', '--verify', 'refs/heads/gh-pages')
    if observed != expected_tip:
        raise ValueError('stale gh-pages tip')
    commit = git(candidate, 'rev-parse', '--verify', 'refs/heads/gh-pages')
    parents = git(candidate, 'show', '-s', '--format=%P', commit).split()
    if parents != [expected_tip]:
        raise ValueError('candidate must have the observed tip as its single direct parent')
    return commit
