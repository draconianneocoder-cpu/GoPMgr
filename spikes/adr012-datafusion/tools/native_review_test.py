# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

import argparse
import hashlib
import io
import json
from pathlib import Path
import subprocess
import tarfile
import tempfile
import unittest
from unittest import mock

import native_review


def digest(data):
    return hashlib.sha256(data).hexdigest()


class NativeReviewTests(unittest.TestCase):
    def fixture(self):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        root = Path(temp.name)
        package = root / 'source'
        package.mkdir()
        files = {'Cargo.toml': b'[package]\n', 'LICENSE': b'Copyright Fixture\nLicense terms retained verbatim.\n',
                 'code.c': b'// SPDX-License-' b'Identifier: 0BSD\nint example;\n', 'include.h': b'/* fixture */\n', 'license-check.sh': b'#!/bin/sh\n# source tool, not a notice text\n'}
        cache = root / 'cache'
        cache.mkdir()
        archive = cache / 'example-1.0.0.crate'
        with tarfile.open(archive, 'w:gz') as output:
            for name, data in files.items():
                (package / name).write_bytes(data)
                info = tarfile.TarInfo('example-1.0.0/' + name)
                info.size = len(data)
                output.addfile(info, io.BytesIO(data))
        obj = root / '0123456789abcdef-code.o'
        subprocess.run(['cc', '-x', 'c', '-c', '-o', str(obj), '-'], input=b'int fixture;\n', check=True, capture_output=True)
        lib = root / 'libexample.a'
        subprocess.run(['ar', '-rc', str(lib), str(obj)], check=True, capture_output=True)
        members = subprocess.check_output(['ar', '-t', str(lib)], text=True).splitlines()
        worker = root / 'worker'
        worker.write_bytes(b'synthetic worker')
        metadata = {'packages': [{'id': 'native', 'name': 'example', 'version': '1.0.0', 'source': native_review.CRATES_IO,
                                 'license': 'MIT OR Apache-2.0', 'manifest_path': str(package / 'Cargo.toml')},
                                {'id': 'worker', 'name': 'df0-worker', 'version': '0.1.0'}]}
        native = {'archives': [{'package': 'example', 'version': '1.0.0', 'archive': lib.name,
                                'sha256': digest(lib.read_bytes()), 'object_members': members}]}
        events = [{'reason': 'compiler-artifact', 'package_id': 'worker', 'target': {'name': 'df0-worker', 'kind': ['bin']},
                   'profile': {'test': False}, 'executable': str(worker)},
                  {'reason': 'build-script-executed', 'package_id': 'native', 'linked_libs': ['static=example'],
                   'linked_paths': ['native=' + str(root)]}, {'reason': 'build-finished', 'success': True}]
        args = argparse.Namespace(crate_cache=cache, worker=worker, output_dir=root / 'result')
        for name, data in [('metadata', metadata), ('native_inputs', native), ('worker_evidence', {'worker_sha256': digest(worker.read_bytes())})]:
            path = root / (name + '.json')
            path.write_text(json.dumps(data))
            setattr(args, name, path)
        args.build_log = root / 'build.jsonl'
        args.build_log.write_text(''.join(json.dumps(e) + '\n' for e in events))
        args.lock = root / 'Cargo.lock'
        args.lock.write_text('[[package]]\nname="example"\nversion="1.0.0"\nsource="' + native_review.CRATES_IO + '"\nchecksum="' + digest(archive.read_bytes()) + '"\n')
        return args, package, files, events

    def test_locked_sources_notices_and_candidate_mapping(self):
        args, package, files, _ = self.fixture()
        report, notices = native_review.generate(args)
        self.assertEqual((report, notices), native_review.generate(args))
        entry = report['packages'][0]
        self.assertEqual(entry['object_source_candidates'][0]['candidate_source'], 'code.c')
        self.assertIn(files['LICENSE'], notices)
        self.assertTrue(notices.endswith(files['LICENSE']))
        self.assertNotIn(files['license-check.sh'], notices)
        self.assertEqual(entry['source_files'][0]['path'], 'Cargo.toml')
        self.assertIsNone(entry['source_files'][0]['spdx_declarations'])
        self.assertEqual(next(f for f in entry['source_files'] if f['path'] == 'code.c')['spdx_declarations'], ['0BSD'])
        self.assertEqual(next(f for f in entry['source_files'] if f['path'] == 'include.h')['spdx_declarations'], [])
        self.assertNotIn(str(package).encode(), json.dumps(report).encode())
        self.assertEqual(report['notices_sha256'], digest(notices))
        native_review.write_bundle(args)
        self.assertEqual(json.loads((args.output_dir / 'source-review.json').read_bytes()), report)
        self.assertEqual((args.output_dir / 'NATIVE-REVIEW-NOTICES.txt').read_bytes(), notices)

    def test_source_and_archive_failures_leave_no_output(self):
        for mode, message in [('changed', 'cached source differs'), ('missing', 'source file'), ('extra', 'unexpected cached'),
                              ('symlink', 'nonregular'), ('checksum', 'crate checksum'), ('native-hash', 'native archive hash'),
                              ('worker-hash', 'worker hash'), ('wrong-worker-path', 'worker executable'), ('build-failed', 'successful build'), ('unaccounted', 'unaccounted native'), ('trailing', 'event after'), ('incomplete-build', 'completed successful build'), ('no-notices', 'no native package notice'), ('ambiguous', 'candidate source')]:
            with self.subTest(mode=mode):
                args, package, _, events = self.fixture()
                if mode == 'changed':
                    (package / 'LICENSE').write_bytes(b'changed')
                elif mode == 'missing':
                    (package / 'LICENSE').unlink()
                elif mode == 'extra':
                    (package / 'untracked.c').write_bytes(b'int untracked;')
                elif mode == 'symlink':
                    target = package.parent / 'elsewhere'
                    target.write_bytes((package / 'LICENSE').read_bytes())
                    (package / 'LICENSE').unlink()
                    (package / 'LICENSE').symlink_to(target)
                elif mode == 'checksum':
                    args.lock.write_text(args.lock.read_text().replace('checksum="', 'checksum="0'))
                elif mode == 'native-hash':
                    data = json.loads(args.native_inputs.read_bytes())
                    data['archives'][0]['sha256'] = '0' * 64
                    args.native_inputs.write_text(json.dumps(data))
                elif mode == 'worker-hash':
                    args.worker.write_bytes(b'changed worker')
                elif mode == 'wrong-worker-path':
                    other = args.worker.parent / 'other-worker'
                    other.write_bytes(args.worker.read_bytes())
                    args.worker = other
                elif mode == 'incomplete-build':
                    events.pop()
                    args.build_log.write_text(''.join(json.dumps(e) + '\n' for e in events))
                elif mode in {'unaccounted', 'trailing'}:
                    if mode == 'unaccounted':
                        events[1]['linked_libs'].append('static=other')
                    else:
                        events.append({'reason': 'compiler-message'})
                    args.build_log.write_text(''.join(json.dumps(e) + '\n' for e in events))
                elif mode == 'build-failed':
                    events[-1]['success'] = False
                    args.build_log.write_text(''.join(json.dumps(e) + '\n' for e in events))
                elif mode in {'ambiguous', 'no-notices'}:
                    # Repack valid locked bytes so these cases reach attribution or
                    # notice presence rather than failing source integrity first.
                    crate = next(args.crate_cache.glob('*.crate'))
                    with tarfile.open(crate) as original:
                        items = [(m.name, original.extractfile(m).read()) for m in original.getmembers()]
                    if mode == 'ambiguous':
                        data = b'int second;'
                        (package / 'code.s').write_bytes(data)
                        items.append(('example-1.0.0/code.s', data))
                    else:
                        items = [(name, value) for name, value in items if not name.endswith('/LICENSE')]
                        (package / 'LICENSE').unlink()
                    with tarfile.open(crate, 'w:gz') as output:
                        for name, value in items:
                            info = tarfile.TarInfo(name)
                            info.size = len(value)
                            output.addfile(info, io.BytesIO(value))
                    text = args.lock.read_text()
                    start = text.index('checksum=')
                    args.lock.write_text(text[:start] + 'checksum="' + digest(crate.read_bytes()) + '"\n')
                with self.assertRaisesRegex(ValueError, message):
                    native_review.write_bundle(args)
                self.assertFalse(args.output_dir.exists())

    def test_invalid_archive_members_are_not_extracted(self):
        for mode in ['traversal', 'link', 'duplicate']:
            with self.subTest(mode=mode):
                args, _, _, _ = self.fixture()
                crate = next(args.crate_cache.glob('*.crate'))
                with tarfile.open(crate) as original:
                    items = [(m.name, original.extractfile(m).read()) for m in original.getmembers()]
                with tarfile.open(crate, 'w:gz') as output:
                    for name, value in items:
                        info = tarfile.TarInfo(name)
                        info.size = len(value)
                        output.addfile(info, io.BytesIO(value))
                    info = tarfile.TarInfo({'traversal': 'example-1.0.0/../escaped', 'link': 'example-1.0.0/link', 'duplicate': 'example-1.0.0/LICENSE'}[mode])
                    if mode == 'link':
                        info.type = tarfile.SYMTYPE
                        info.linkname = '../outside'
                        output.addfile(info)
                    else:
                        data = b'bad'
                        info.size = len(data)
                        output.addfile(info, io.BytesIO(data))
                text = args.lock.read_text()
                start = text.index('checksum=')
                args.lock.write_text(text[:start] + 'checksum="' + digest(crate.read_bytes()) + '"\n')
                with self.assertRaisesRegex(ValueError, 'crate member'):
                    native_review.write_bundle(args)
                self.assertFalse(args.output_dir.exists())
                self.assertFalse((args.output_dir.parent / 'escaped').exists())

    def test_interrupted_output_is_not_published(self):
        args, _, _, _ = self.fixture()
        with mock.patch.object(Path, 'write_bytes', side_effect=OSError('injected write failure')):
            with self.assertRaisesRegex(OSError, 'injected write failure'):
                native_review.write_bundle(args)
        self.assertFalse(args.output_dir.exists())
        self.assertFalse(list(args.output_dir.parent.glob('df0-native-review-*')))

    def test_existing_output_is_preserved(self):
        args, _, _, _ = self.fixture()
        args.output_dir.mkdir()
        marker = args.output_dir / 'keep'
        marker.write_bytes(b'previous work')
        with self.assertRaises(FileExistsError):
            native_review.write_bundle(args)
        self.assertEqual(marker.read_bytes(), b'previous work')


if __name__ == '__main__':
    unittest.main()
