#!/usr/bin/env python3
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
"""Provision the approved Linux Hugo into task-local storage, never system paths."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import tarfile
import tempfile
from urllib.parse import urlsplit
from urllib.request import build_opener, HTTPRedirectHandler

ROOT = Path(__file__).resolve().parents[2]
MAX_DOWNLOAD = 128 * 1024 * 1024
MAX_MEMBER = 128 * 1024 * 1024
MAX_TOTAL = 256 * 1024 * 1024
MAX_MEMBERS = 8


def require(condition, message):
    if not condition:
        raise ValueError(message)


def sha(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def cached(binary, pin):
    if not binary.exists() and not binary.is_symlink():
        return None
    require(binary.is_file() and not binary.is_symlink(), 'cached executable must be a regular file')
    require(sha(binary) == pin['executable_sha256'], 'cached executable checksum mismatch; remove the poisoned cache explicitly')
    require(os.access(binary, os.X_OK), 'cached executable is not executable')
    return binary


def install(archive, pin, binary):
    require(archive.is_file() and not archive.is_symlink(), 'archive must be a regular file')
    require(archive.stat().st_size <= MAX_DOWNLOAD, 'archive exceeds download limit')
    require(sha(archive) == pin['archive_sha256'], 'archive checksum mismatch')
    executable, names, total = None, set(), 0
    # Stream and bound decompressed entries; never call extract/extractall.
    with tarfile.open(archive, 'r|gz') as tar:
        for member in tar:
            total += member.size
            require(len(names) < MAX_MEMBERS and 0 <= member.size <= MAX_MEMBER and total <= MAX_TOTAL, 'archive limits exceeded')
            require(member.name in ('hugo', 'LICENSE', 'README.md') and member.name not in names, 'unexpected or duplicate archive path')
            require(member.isfile(), 'archive member must be regular, not a link or special file')
            names.add(member.name)
            if member.name == 'hugo':
                with tar.extractfile(member) as stream:
                    executable = stream.read(MAX_MEMBER + 1)
                require(len(executable) == member.size, 'truncated executable')
    require(executable is not None, 'archive lacks hugo')
    require(hashlib.sha256(executable).hexdigest() == pin['executable_sha256'], 'executable checksum mismatch')
    binary.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='verified-', dir=binary.parent) as tmp:
        candidate = Path(tmp) / 'hugo'
        candidate.write_bytes(executable)
        candidate.chmod(0o755)
        os.replace(candidate, binary)
    return binary


def approved_url(url):
    parsed = urlsplit(url)
    return parsed.scheme == 'https' and parsed.hostname in ('github.com', 'release-assets.githubusercontent.com') and not parsed.username and parsed.port in (None, 443)


class ApprovedRedirects(HTTPRedirectHandler):
    def redirect_request(self, request, fp, code, msg, headers, newurl):
        require(approved_url(newurl), 'unapproved download redirect')
        return super().redirect_request(request, fp, code, msg, headers, newurl)


def download(url, destination):
    require(approved_url(url), 'unapproved download URL')
    with build_opener(ApprovedRedirects()).open(url, timeout=30) as response:
        with destination.open('wb') as stream:
            total = 0
            while chunk := response.read(1024 * 1024):
                total += len(chunk)
                require(total <= MAX_DOWNLOAD, 'download exceeds limit')
                stream.write(chunk)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--archive', type=Path, help='Use an already downloaded archive; the same pinned checks apply')
    args = parser.parse_args()
    require(platform.system() == 'Linux' and platform.machine() in ('x86_64', 'amd64'), 'provisioning is qualified only for Linux AMD64')
    pins = json.loads((ROOT / 'website/quality/dependencies.json').read_text())
    pin = pins['hugo']['platforms']['linux/amd64']
    cache = ROOT / '.tmp/website-tools/linux-amd64'
    for path in (ROOT / '.tmp', cache.parent, cache):
        require(not path.is_symlink(), 'cache directory must not be a symlink')
    cache.mkdir(parents=True, exist_ok=True)
    binary = cache / 'hugo'
    if not cached(binary, pin):
        if args.archive:
            install(args.archive, pin, binary)
        else:
            with tempfile.TemporaryDirectory(prefix='download-', dir=cache) as tmp:
                archive = Path(tmp) / 'hugo.tar.gz'
                download(pin['archive_url'], archive)
                install(archive, pin, binary)
    print(binary)


if __name__ == '__main__':
    try:
        main()
    except (ValueError, OSError, tarfile.TarError) as error:
        raise SystemExit(f'website Hugo provisioning: {error}') from None
