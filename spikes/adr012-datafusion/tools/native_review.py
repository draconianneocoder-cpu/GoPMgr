#!/usr/bin/env python3
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

"""Offline native-source and notice review inputs for the isolated DF0 worker."""

import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import subprocess
import tarfile
import tempfile
import tomllib

CRATES_IO = 'registry+https://github.com/rust-lang/crates.io-index'
SOURCE_SUFFIXES = {'.c', '.h', '.s', '.S', '.inc'}


def sha256(data):
    return hashlib.sha256(data).hexdigest()


def regular(path, root):
    relative = path.relative_to(root)
    cursor = root
    for part in relative.parts:
        cursor /= part
        if cursor.is_symlink():
            raise ValueError('nonregular source file: ' + str(relative))
    if not path.is_file():
        raise ValueError('missing or nonregular source file: ' + str(relative))
    return path.read_bytes()


def package_sources(package, checksum, cache):
    prefix = package['name'] + '-' + package['version']
    archives = sorted(cache.glob('**/' + prefix + '.crate'))
    if len(archives) != 1 or sha256(archives[0].read_bytes()) != checksum:
        raise ValueError('missing, ambiguous or mismatched crate checksum: ' + prefix)
    root = Path(package['manifest_path']).parent.resolve()
    sources, notices, seen = [], [], set()
    with tarfile.open(archives[0]) as archive:
        for member in archive:
            parts = PurePosixPath(member.name).parts
            if not parts or parts[0] != prefix or '..' in parts or '\\' in member.name or member.name.startswith('/'):
                raise ValueError('invalid crate member path')
            if member.isdir():
                continue
            if not member.isfile() or len(parts) < 2:
                raise ValueError('nonregular crate member')
            name = '/'.join(parts[1:])
            if name in seen:
                raise ValueError('duplicate crate member: ' + name)
            seen.add(name)
            data = archive.extractfile(member).read()
            if regular(root / name, root) != data:
                raise ValueError('cached source differs from locked crate: ' + name)
            declarations = None
            if Path(name).suffix in SOURCE_SUFFIXES:
                # Split the literal so REUSE does not parse it as this file's metadata.
                declarations = sorted(set(m.strip().removesuffix('*/').strip() for m in re.findall(
                    r'SPDX-License-' r'Identifier:([^\r\n]+)', data.decode('utf-8', errors='replace'))))
            sources.append({'path': name, 'sha256': sha256(data), 'spdx_declarations': declarations})
            if (any(part in {'LICENSES', 'LICENCES', 'NOTICES'} for part in parts[1:-1])
                    or re.match(r'^(LICENSE|LICENCE|NOTICE|COPYING)(?:$|[._-])', parts[-1])
                    or parts[-1].lower() in {base + suffix for base in ['license', 'licence', 'notice', 'copying'] for suffix in ['', '.txt', '.md']}):
                data.decode('utf-8')  # A review text bundle must remain text.
                notices.append((name, data))
    actual = {p.relative_to(root).as_posix() for p in root.rglob('*') if not p.is_dir() or p.is_symlink()}
    # Cargo adds these extraction records; they are not published source inputs.
    extra = actual - seen - {'.cargo-ok', '.cargo-checksum.json'}
    if extra:
        raise ValueError('unexpected cached source files: ' + ', '.join(sorted(extra)))
    if not notices:
        raise ValueError('no native package notice texts: ' + prefix)
    return sorted(sources, key=lambda f: f['path']), sorted(notices)


def build_archives(args, metadata, expected):
    packages = {p['id']: p for p in metadata['packages']}
    found, worker_seen, finished = {}, False, False
    for line in args.build_log.read_text().splitlines():
        event = json.loads(line)
        if finished:
            raise ValueError('event after successful build completion')
        reason = event.get('reason')
        if reason == 'build-finished':
            finished = event.get('success') is True
            if not finished:
                raise ValueError('successful build required')
        elif reason == 'compiler-artifact' and event.get('executable'):
            p = packages.get(event['package_id'], {})
            if p.get('name') == 'df0-worker' and event['target']['name'] == 'df0-worker' and 'bin' in event['target']['kind'] and not event['profile']['test']:
                if worker_seen or Path(event['executable']).resolve() != args.worker.resolve():
                    raise ValueError('ambiguous or mismatched worker executable')
                worker_seen = True
        elif reason == 'build-script-executed':
            p = packages[event['package_id']]
            for lib in event['linked_libs']:
                if not re.fullmatch(r'static=[A-Za-z0-9_]+', lib):
                    raise ValueError('unsupported native link directive')
                name = 'lib' + lib[7:] + '.a'
                key = (p['name'], p['version'], name)
                if key not in expected:
                    raise ValueError('unaccounted native archive input')
                paths = sorted({Path(s[7:]) / name for s in event['linked_paths'] if s.startswith('native=') and (Path(s[7:]) / name).exists()})
                if len(paths) != 1 or paths[0].is_symlink() or sha256(paths[0].read_bytes()) != expected[key]['sha256']:
                    raise ValueError('missing, ambiguous or mismatched native archive hash')
                members = subprocess.check_output(['ar', '-t', str(paths[0])], text=True).splitlines()
                if members != expected[key]['object_members']:
                    raise ValueError('native archive members changed')
                if key in found and found[key] != members:
                    raise ValueError('conflicting native archive observations')
                found[key] = members
        elif reason not in {'compiler-artifact', 'compiler-message'}:
            raise ValueError('unknown Cargo event')
    if not finished or not worker_seen or found.keys() != expected.keys():
        raise ValueError('completed successful build and all native inputs required')
    return found


def generate(args):
    metadata = json.loads(args.metadata.read_bytes())
    native = json.loads(args.native_inputs.read_bytes())
    worker_hash = sha256(args.worker.read_bytes())
    if worker_hash != json.loads(args.worker_evidence.read_bytes())['worker_sha256']:
        raise ValueError('worker hash mismatch')
    expected = {(p['package'], p['version'], p['archive']): p for p in native['archives']}
    if not expected or len(expected) != len(native['archives']):
        raise ValueError('empty or duplicate native archive inventory')
    observed = build_archives(args, metadata, expected)
    locked = tomllib.loads(args.lock.read_text())['package']
    report = {'scope': 'Native build-input source/notice review only. Candidate filename matches are not compiler, preprocessor or final-linker proof; no license choice or distribution clearance.',
              'worker_sha256': worker_hash, 'input_sha256': {name: sha256(getattr(args, name).read_bytes()) for name in ['metadata', 'lock', 'native_inputs', 'build_log', 'worker_evidence']},
              'generator_sha256': sha256(Path(__file__).read_bytes()), 'packages': []}
    bundle = bytearray(b'DF0 NATIVE NOTICE REVIEW INPUT\n\nNot an installed-package notice set or distribution clearance.\nAll discovered texts are retained without a license choice. Each text keeps its own terms and scope; it is not relicensed as CC0.\n\n')
    for key, members in sorted(observed.items()):
        name, version, archive = key
        packages = [p for p in metadata['packages'] if p['name'] == name and p['version'] == version and p.get('source') == CRATES_IO]
        checksums = [p['checksum'] for p in locked if p['name'] == name and p['version'] == version and p.get('source') == CRATES_IO]
        if len(packages) != 1 or len(checksums) != 1:
            raise ValueError('unique locked registry package required')
        package = packages[0]
        files, notices = package_sources(package, checksums[0], args.crate_cache)
        candidates = []
        for member in members:
            if member == '__.SYMDEF SORTED':
                continue
            match = re.fullmatch(r'[0-9a-f]+-(.+)\.o', member)
            possible = [f['path'] for f in files if match and Path(f['path']).suffix in {'.c', '.s', '.S'} and Path(f['path']).stem == match[1]]
            if len(possible) != 1:
                raise ValueError('missing or ambiguous candidate source: ' + member)
            candidates.append({'object_member': member, 'candidate_source': possible[0]})
        if not candidates:
            raise ValueError('native archive has no source candidates')
        entries = []
        for path, data in notices:
            entries.append({'path': path, 'sha256': sha256(data), 'bytes': len(data)})
            bundle.extend(('===== ' + name + '@' + version + '/' + path + ' =====\n').encode())
            bundle.extend(data)
            bundle.extend(b'\n\n')
        report['packages'].append({'name': name, 'version': version, 'declared_crate_license': package.get('license'),
                                   'published_crate_sha256': checksums[0], 'archive': archive, 'archive_sha256': expected[key]['sha256'],
                                   'object_source_candidates': candidates, 'source_files': files, 'notice_files': entries})
    # Keep separators between sections, preserving the final upstream EOF.
    bundle = bundle[:-2]
    report['notices_sha256'] = sha256(bundle)
    return report, bytes(bundle)


def write_bundle(args):
    if args.output_dir.exists() or args.output_dir.is_symlink():
        raise FileExistsError('output directory already exists')
    report, notices = generate(args)
    with tempfile.TemporaryDirectory(prefix='df0-native-review-', dir=args.output_dir.parent) as temp:
        stage = Path(temp) / 'bundle'
        stage.mkdir()
        (stage / 'source-review.json').write_text(json.dumps(report, indent=2) + '\n')
        (stage / 'NATIVE-REVIEW-NOTICES.txt').write_bytes(notices)
        os.rename(stage, args.output_dir)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ['metadata', 'lock', 'build-log', 'native-inputs', 'worker', 'worker-evidence', 'crate-cache', 'output-dir']:
        parser.add_argument('--' + name, required=True, type=Path)
    write_bundle(parser.parse_args())


if __name__ == '__main__':
    try:
        main()
    except (OSError, ValueError, KeyError, tarfile.TarError, subprocess.CalledProcessError) as error:
        raise SystemExit('native_review: ' + str(error)) from error
