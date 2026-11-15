#!/usr/bin/env python3
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
"""Artifact-bound publisher policy. A local owner grant is not created by this tool."""
from datetime import datetime, timezone
import argparse
import base64
from contextlib import contextmanager
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile

REPOSITORY = 'draconianneocoder-cpu/GoPMgr'
REPOSITORY_ID = 1277267642
OWNER_ID = 284033065
DESTINATION = 'https://github.com/' + REPOSITORY + '.git'
REF = 'refs/heads/gh-pages'
BINDING_FIELDS = {'commit', 'tree', 'expected_tip', 'source_digest',
                  'output_digest', 'acceptance_sha256'}


def require(condition, message):
    if not condition:
        raise ValueError(message)


def validate_grant(grant, binding, actor, repository):
    require(all(isinstance(value, dict) for value in (grant, binding, actor, repository)), 'invalid identity or grant record')
    require(isinstance(repository.get('owner'), dict) and isinstance(repository.get('permissions'), dict), 'invalid repository identity')
    fields = BINDING_FIELDS | {'schema_version', 'approval_id', 'status', 'expires_at',
                              'repository', 'repository_id', 'ref', 'approved_by', 'publisher_id'}
    require(set(grant) == fields and grant['schema_version'] == 1, 'unknown grant schema')
    require(grant['status'] == 'approved', 'grant is not approved')
    require(isinstance(grant['approval_id'], str) and grant['approval_id'].strip(), 'missing approval ID')
    require(grant['repository'] == REPOSITORY and grant['repository_id'] == REPOSITORY_ID
            and grant['ref'] == REF, 'wrong grant destination')
    require(repository.get('id') == REPOSITORY_ID and repository.get('full_name') == REPOSITORY
            and repository.get('owner', {}).get('id') == OWNER_ID, 'wrong authenticated destination')
    require(repository.get('permissions', {}).get('push') is True, 'publisher lacks push permission')
    require(type(grant['approved_by']) is int and grant['approved_by'] == OWNER_ID, 'wrong approving owner')
    require(type(grant['publisher_id']) is int and actor.get('id') == grant['publisher_id'], 'wrong authenticated publisher')
    expiry = datetime.fromisoformat(grant['expires_at'])
    require(expiry.tzinfo is not None, 'grant expiry needs timezone')
    require(expiry > datetime.now(timezone.utc), 'grant expired')
    require(set(binding) == BINDING_FIELDS, 'unknown candidate binding')
    for field in BINDING_FIELDS:
        require(grant[field] == binding[field], 'grant differs: ' + field)


# Authenticated network transport never uses candidate-local Git configuration.


def process_environment():
    blocked = ('GIT_', 'GH_', 'GITHUB_', 'CURL_', 'SSL_')
    proxy = {'http_proxy', 'https_proxy', 'all_proxy', 'no_proxy'}
    env = {k: v for k, v in os.environ.items()
           if not k.startswith(blocked) and k.lower() not in proxy}
    env.update(GIT_CONFIG_NOSYSTEM='1', GIT_CONFIG_GLOBAL=os.devnull,
               GIT_TERMINAL_PROMPT='0', GIT_NO_REPLACE_OBJECTS='1', GH_PROMPT_DISABLED='1')
    return env


def git_bytes(repository, args, token=None):
    env = process_environment()
    options = ['core.hooksPath=' + os.devnull, 'credential.helper=',
               'http.followRedirects=false', 'http.sslVerify=true', 'http.proxy=',
               'gc.auto=0', 'maintenance.auto=false']
    if token is not None:
        require(isinstance(token, str) and token and all(32 < ord(c) < 127 for c in token),
                'invalid credential')
        header = base64.b64encode(('x-access-token:' + token).encode()).decode()
        env.update(GIT_CONFIG_COUNT='1',
                   GIT_CONFIG_KEY_0='http.https://github.com/.extraHeader',
                   GIT_CONFIG_VALUE_0='Authorization: Basic ' + header)
    command = ['git', '--no-replace-objects', '-C', str(repository)]
    for option in options:
        command += ['-c', option]
    try:
        result = subprocess.run(command + args, env=env, capture_output=True, timeout=60)
    except (OSError, subprocess.TimeoutExpired):
        raise ValueError('Git operation unavailable or timed out; inspect remote state before retrying') from None
    require(result.returncode == 0, 'Git operation failed; inspect remote state before retrying')
    return result.stdout


def object_id(value):
    require(isinstance(value, str) and re.fullmatch('[0-9a-f]{40}', value), 'invalid immutable Git object ID')
    return value


def verify_tree(repository, commit, expected):
    object_id(commit)
    actual = {}
    for record in git_bytes(repository, ['ls-tree', '-rz', '--full-tree', commit]).split(b'\0'):
        if not record:
            continue
        metadata, name = record.split(b'\t', 1)
        mode, kind, blob = metadata.decode('ascii').split()
        name = name.decode('utf-8')
        require(mode == '100644' and kind == 'blob' and name not in actual, 'unsafe or duplicate Git entry')
        require(name in expected, 'unexpected Git payload path')
        data = git_bytes(repository, ['cat-file', 'blob', object_id(blob)])
        actual[name] = dict(mode=mode, size=len(data), sha256=hashlib.sha256(data).hexdigest())
    require(actual == expected, 'Git payload differs from accepted inventory')


def verify_parent(repository, commit, expected_tip):
    object_id(commit)
    if expected_tip is not None:
        object_id(expected_tip)
    headers = git_bytes(repository, ['cat-file', 'commit', commit]).split(b'\n\n', 1)[0]
    parents = [line.removeprefix(b'parent ').decode('ascii')
               for line in headers.split(b'\n') if line.startswith(b'parent ')]
    require(parents == ([expected_tip] if expected_tip else []), 'candidate must have expected sole parent or initial root')


@contextmanager
def transport_repository(candidate, commit):
    import publication
    candidate = publication.repository_path(candidate)
    git_bytes(candidate, ['rev-parse', '--git-dir'])
    object_id(commit)
    with tempfile.TemporaryDirectory(prefix='website-transport-') as temporary:
        target = Path(temporary)
        git_bytes(target, ['init', '--bare', '--quiet'])
        git_bytes(target, ['-c', 'protocol.file.allow=always', 'fetch', '--quiet',
                           '--no-tags', '--no-recurse-submodules', str(candidate), commit])
        yield target


def transfer(repository, commit, expected_tip, token):
    verify_parent(repository, commit, expected_tip)
    git_bytes(repository, ['push', '--porcelain', '--force-with-lease=' + REF + ':' + (expected_tip or ''),
                           DESTINATION, object_id(commit) + ':' + REF], token)


def credential():
    try:
        result = subprocess.run(['gh', 'auth', 'token', '--hostname', 'github.com', '--no-refresh'],
                                env=process_environment(), capture_output=True, timeout=30)
    except (OSError, subprocess.TimeoutExpired):
        raise ValueError('GitHub credential unavailable') from None
    require(result.returncode == 0, 'GitHub credential unavailable')
    token = result.stdout.decode('ascii').strip()
    require(token and all(32 < ord(c) < 127 for c in token), 'invalid GitHub credential')
    return token


def github_identity(token):
    env = process_environment()
    env['GH_TOKEN'] = token
    values = []
    for endpoint in ('user', 'repos/' + REPOSITORY):
        try:
            response = subprocess.run(['gh', 'api', '--hostname', 'github.com', endpoint],
                                      env=env, capture_output=True, timeout=30)
            require(response.returncode == 0, 'GitHub identity lookup failed')
            values.append(json.loads(response.stdout))
        except (OSError, subprocess.TimeoutExpired, json.JSONDecodeError):
            raise ValueError('GitHub identity lookup failed') from None
    return values


def remote_tip(repository, token):
    output = git_bytes(repository, ['ls-remote', '--refs', DESTINATION, REF], token)
    if output == b'':
        return None
    match = re.fullmatch(rb'([0-9a-f]{40})\trefs/heads/gh-pages\n?', output)
    require(match is not None, 'malformed remote ref response')
    return match.group(1).decode('ascii')


def read_json(data):
    def unique(pairs):
        result = {}
        for key, value in pairs:
            require(key not in result, 'duplicate JSON field')
            result[key] = value
        return result
    value = json.loads(data, object_pairs_hook=unique)
    require(isinstance(value, dict), 'JSON record must be an object')
    return value


# The stdlib module named site must not hide the companion site's verifier.
_spec = importlib.util.spec_from_file_location('website_site_verifier', Path(__file__).with_name('site.py'))
site = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(site)


def transaction(report_path, acceptance_path, candidate, grant_path, execute=False):
    import publication
    candidate = publication.repository_path(candidate)
    git_bytes(candidate, ['rev-parse', '--git-dir'])
    report = site.check_report(report_path)
    acceptance_bytes = acceptance_path.read_bytes()
    site.acceptance_check(report, read_json(acceptance_bytes))
    commit = object_id(git_bytes(candidate, ['rev-parse', '--verify', REF]).decode().strip())
    tree = object_id(git_bytes(candidate, ['rev-parse', commit + '^{tree}']).decode().strip())
    grant_bytes = grant_path.read_bytes()
    grant = read_json(grant_bytes)
    binding = dict(commit=commit, tree=tree, expected_tip=grant.get('expected_tip'),
                   source_digest=report['source']['digest'], output_digest=report['output_digest'],
                   acceptance_sha256=hashlib.sha256(acceptance_bytes).hexdigest())
    verify_tree(candidate, commit, report['output'])
    verify_parent(candidate, commit, binding['expected_tip'])
    token = credential()
    actor, repository = github_identity(token)
    validate_grant(grant, binding, actor, repository)
    with transport_repository(candidate, commit) as transport:
        verify_tree(transport, commit, report['output'])
        verify_parent(transport, commit, binding['expected_tip'])
        require(remote_tip(transport, token) == binding['expected_tip'], 'remote tip changed')
        # Re-read local approval and reviewed bytes after network/object work.
        current = site.check_report(report_path)
        require(current['source']['digest'] == binding['source_digest']
                and current['output_digest'] == binding['output_digest'], 'reviewed source/output changed')
        require(acceptance_path.read_bytes() == acceptance_bytes, 'acceptance changed')
        require(grant_path.read_bytes() == grant_bytes, 'grant changed before transport')
        actor, repository = github_identity(token)
        validate_grant(read_json(grant_bytes), binding, actor, repository)
        result = dict(repository=REPOSITORY, ref=REF, **binding, authorized_preflight=True, published=False)
        if execute:
            # Final point-in-time check. Revocation cannot atomically cancel an
            # update that has already entered the remote transaction.
            require(remote_tip(transport, token) == binding['expected_tip'], 'remote tip changed')
            require(grant_path.read_bytes() == grant_bytes, 'grant changed before transport')
            validate_grant(read_json(grant_bytes), binding, actor, repository)
            transfer(transport, commit, binding['expected_tip'], token)
            require(remote_tip(transport, token) == commit, 'published tip unconfirmed; inspect remote state')
            result['published'] = True
        return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('command', choices=('preflight', 'publish'), nargs='?', default='preflight')
    parser.add_argument('--report', type=Path, required=True)
    parser.add_argument('--acceptance', type=Path, required=True)
    parser.add_argument('--candidate', type=Path, required=True)
    parser.add_argument('--grant', type=Path, required=True)
    args = parser.parse_args()
    print(json.dumps(transaction(args.report, args.acceptance, args.candidate, args.grant,
                                 execute=args.command == 'publish'), indent=2))


def cli():
    try:
        main()
        return 0
    except (ValueError, OSError, KeyError, TypeError, UnicodeError):
        # Do not echo subprocess output, credentials, user-supplied JSON or paths.
        print('website publisher: refused or unconfirmed; review local approval and remote state', file=sys.stderr)
        return 1


if __name__ == '__main__':
    raise SystemExit(cli())
