# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
import hashlib
import io
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch

import provision_hugo as provision


class ProvisionTests(unittest.TestCase):
    def archive(self, root, members):
        path = root / 'fixture.tar.gz'
        with tarfile.open(path, 'w:gz') as tar:
            for name, data, kind in members:
                info = tarfile.TarInfo(name)
                info.type = kind
                info.size = len(data) if kind == tarfile.REGTYPE else 0
                tar.addfile(info, io.BytesIO(data) if info.size else None)
        pin = {'archive_sha256': hashlib.sha256(path.read_bytes()).hexdigest(),
               'executable_sha256': hashlib.sha256(b'verified fixture').hexdigest()}
        return path, pin

    def test_verified_install_and_cache(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            archive, pin = self.archive(root, [('hugo', b'verified fixture', tarfile.REGTYPE)])
            binary = provision.install(archive, pin, root / 'bin/hugo')
            self.assertEqual(binary.read_bytes(), b'verified fixture')
            self.assertEqual(binary.stat().st_mode & 0o777, 0o755)
            self.assertEqual(provision.cached(binary, pin), binary)
            binary.write_bytes(b'poisoned cache')
            with self.assertRaisesRegex(ValueError, 'cached executable'):
                provision.cached(binary, pin)

    def test_archive_faults_never_install(self):
        cases = {
            'duplicate': [('hugo', b'verified fixture', tarfile.REGTYPE)] * 2,
            'traversal': [('../hugo', b'verified fixture', tarfile.REGTYPE)],
            'symlink': [('hugo', b'', tarfile.SYMTYPE)],
            'hardlink': [('hugo', b'', tarfile.LNKTYPE)],
            'missing': [('README.md', b'doc', tarfile.REGTYPE)],
            'binary digest': [('hugo', b'unverified executable', tarfile.REGTYPE)],
            'absolute': [('/hugo', b'verified fixture', tarfile.REGTYPE)],
        }
        for label, members in cases.items():
            with self.subTest(label=label), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                archive, pin = self.archive(root, members)
                with self.assertRaises(ValueError):
                    provision.install(archive, pin, root / 'bin/hugo')
                self.assertFalse((root / 'bin/hugo').exists())

    def test_archive_digest_checked_before_parse(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            archive, pin = self.archive(root, [('hugo', b'verified fixture', tarfile.REGTYPE)])
            archive.write_bytes(b'corrupt or truncated download')
            with patch.object(provision.tarfile, 'open', side_effect=AssertionError('parsed before verification')):
                with self.assertRaisesRegex(ValueError, 'archive checksum'):
                    provision.install(archive, pin, root / 'hugo')

    def test_limits_and_cache_symlink(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            archive, pin = self.archive(root, [('hugo', b'verified fixture', tarfile.REGTYPE)])
            with patch.object(provision, 'MAX_MEMBER', 2):
                with self.assertRaisesRegex(ValueError, 'archive limits'):
                    provision.install(archive, pin, root / 'hugo')
            with patch.object(provision, 'MAX_MEMBERS', 0):
                with self.assertRaisesRegex(ValueError, 'archive limits'):
                    provision.install(archive, pin, root / 'hugo')
            (root / 'hugo').symlink_to(archive)
            with self.assertRaisesRegex(ValueError, 'regular file'):
                provision.cached(root / 'hugo', pin)

    def test_redirect_policy(self):
        self.assertTrue(provision.approved_url('https://release-assets.githubusercontent.com/a'))
        for url in ('http://github.com/a', 'https://example.org/a', 'https://github.com.evil/a'):
            self.assertFalse(provision.approved_url(url))
    def test_download_limit_and_redirect_handler(self):
        from types import SimpleNamespace
        from urllib.request import Request
        with tempfile.TemporaryDirectory() as tmp:
            response = io.BytesIO(b'oversized response')
            opener = SimpleNamespace(open=lambda *args, **kwargs: response)
            with patch.object(provision, 'build_opener', return_value=opener), patch.object(provision, 'MAX_DOWNLOAD', 2):
                with self.assertRaisesRegex(ValueError, 'download exceeds limit'):
                    provision.download('https://github.com/fixture', Path(tmp) / 'download')
            self.assertTrue(response.closed)
        with self.assertRaisesRegex(ValueError, 'unapproved download redirect'):
            provision.ApprovedRedirects().redirect_request(Request('https://github.com/fixture'), None, 302, '', {}, 'http://example.org/fixture')



class CIOrchestrationTests(unittest.TestCase):
    def test_exact_report_and_failure_propagation(self):
        import shutil
        import subprocess
        for failure in ('none', 'build', 'check', 'json'):
            with self.subTest(failure=failure), tempfile.TemporaryDirectory() as tmp:
                tools = Path(tmp) / 'website/tools'
                tools.mkdir(parents=True)
                shutil.copyfile(Path(__file__).with_name('ci.sh'), tools / 'ci.sh')
                (tools / 'provision_hugo.py').write_text('print("/fixture only/hugo")\n')
                build = 'exit 7\n' if failure == 'build' else 'printf "not-json"\n' if failure == 'json' else "printf '%s\\n' '{\"report\":\"/exact output/report.json\"}'\n"
                (tools / 'build.sh').write_text('#!/bin/bash\n' + build)
                check = 'exit 9\n' if failure == 'check' else 'test "$1" = --report && test "$2" = "/exact output/report.json"\n'
                (tools / 'check.sh').write_text('#!/bin/bash\n' + check)
                result = subprocess.run(['bash', str(tools / 'ci.sh')], capture_output=True)
                self.assertEqual(result.returncode == 0, failure == 'none')


if __name__ == '__main__':
    unittest.main()
