# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
"""TEST ONLY: transport uses disposable absolute filesystem bare repositories."""
import importlib.util
from pathlib import Path
import subprocess
import tempfile
import os
import shutil
import unittest
import publication
import publisher
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('site_tools', Path(__file__).with_name('site.py'))
site = importlib.util.module_from_spec(spec)
spec.loader.exec_module(site)


class TransactionTests(unittest.TestCase):
    def fixture(self, root):
        public = root / 'public'
        public.mkdir()
        (public / '.nojekyll').touch()
        (public / 'index.html').write_text('Fixture A, no application sources')
        remote = root / 'remote.git'
        subprocess.check_call(['git', 'init', '--quiet', '--bare', str(remote)])
        return public, remote

    def push(self, candidate, remote, lease):
        # Only substitute the network destination at the process boundary. The
        # operational publisher constructs every argument and environment value.
        remote = publication.local_repository(remote)
        original = subprocess.run
        def local_transport(args, **kwargs):
            if 'push' in args:
                self.assertIn('https://github.com/draconianneocoder-cpu/GoPMgr.git', args)
                self.assertIn(candidate['commit'] + ':refs/heads/gh-pages', args)
                self.assertIn('--force-with-lease=refs/heads/gh-pages:' + lease, args)
                self.assertNotIn('fixture-secret', ' '.join(args))
                self.assertEqual(kwargs['env']['GIT_TERMINAL_PROMPT'], '0')
                self.assertEqual(kwargs['env']['GIT_CONFIG_GLOBAL'], '/dev/null')
                self.assertEqual(kwargs['env']['GIT_CONFIG_KEY_0'], 'http.https://github.com/.extraHeader')
                self.assertEqual(kwargs['env']['GIT_CONFIG_VALUE_0'], 'Authorization: Basic eC1hY2Nlc3MtdG9rZW46Zml4dHVyZS1zZWNyZXQ=')
                self.assertIn('http.followRedirects=false', args)
                self.assertNotIn('GIT_TRACE', kwargs['env'])
                self.assertNotIn('HTTPS_PROXY', kwargs['env'])
                self.assertNotIn('GIT_SSL_NO_VERIFY', kwargs['env'])
                args = [str(remote) if arg == publisher.DESTINATION else arg for arg in args]
            return original(args, **kwargs)
        with publisher.transport_repository(Path(candidate['repository']), candidate['commit']) as transport:
            publisher.verify_tree(transport, candidate['commit'], site.inventory(Path(candidate['public'])))
            with patch.dict(os.environ, {'GIT_TRACE':'1', 'HTTPS_PROXY':'http://invalid.example', 'GIT_SSL_NO_VERIFY':'1'}), \
                    patch('publisher.subprocess.run', side_effect=local_transport):
                try:
                    publisher.transfer(transport, candidate['commit'], lease or None, 'fixture-secret')
                    return 0
                except ValueError:
                    return 1

    def test_bootstrap_update_stale_tip_and_rollback(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            public, remote = self.fixture(root)
            (public / 'stale.txt').write_text('previous artifact only')
            first = publication.stage(public, site.inventory(public), site.inventory)
            first['public'] = str(public)
            self.assertEqual(self.push(first, remote, ''), 0)
            original = (public / 'index.html').read_text()
            (public / 'index.html').write_text('Fixture B')
            (public / 'stale.txt').unlink()
            second = publication.stage(public, site.inventory(public), site.inventory, remote, first['commit'])
            second['public'] = str(public)
            self.assertEqual(publication.git(Path(second['repository']), 'rev-parse', second['commit'] + '^'), first['commit'])
            self.assertEqual(self.push(second, remote, first['commit']), 0)
            self.assertNotIn('stale.txt', publication.git(remote, 'ls-tree', '-r', '--name-only', 'gh-pages').splitlines())
            with self.assertRaisesRegex(ValueError, 'stale'):
                publication.stage(public, site.inventory(public), site.inventory, remote, first['commit'])
            # A concurrent update cannot be overwritten with an old lease.
            with self.assertRaisesRegex(ValueError, 'stale'):
                publication.validate_transaction(first['repository'], remote, first['commit'])
            # Even a matching lease cannot authorize an old/non-forward tree.
            with self.assertRaisesRegex(ValueError, 'single direct parent'):
                publication.validate_transaction(first['repository'], remote, second['commit'])
            self.assertEqual(publication.git(remote, 'rev-parse', 'gh-pages'), second['commit'])
            # An unchanged no-op needs no remote mutation.
            self.assertEqual(self.push(second, remote, first['commit']), 0)
            # Rollback uses a new forward commit with earlier accepted bytes.
            (public / 'index.html').write_text(original)
            (public / 'stale.txt').write_text('previous artifact only')
            rollback = publication.stage(public, site.inventory(public), site.inventory, remote, second['commit'])
            rollback['public'] = str(public)
            other = root / 'other'
            shutil.copytree(public, other)
            (other / 'index.html').write_text('Concurrent publisher')
            competitor = publication.stage(other, site.inventory(other), site.inventory, remote, second['commit'])
            competitor['public'] = str(other)
            self.assertEqual(self.push(competitor, remote, second['commit']), 0)
            self.assertEqual(self.push(rollback, remote, second['commit']), 1)
            self.assertEqual(publication.git(remote, 'rev-parse', 'gh-pages'), competitor['commit'])
            rollback = publication.stage(public, site.inventory(public), site.inventory, remote, competitor['commit'])
            rollback['public'] = str(public)
            self.assertEqual(self.push(rollback, remote, competitor['commit']), 0)
            self.assertEqual(publication.git(remote, 'show', 'gh-pages:index.html'), original)
            self.assertEqual(set(publication.git(remote, 'ls-tree', '-r', '--name-only', 'gh-pages').splitlines()), {'.nojekyll', 'index.html', 'stale.txt'})

    def test_network_and_scp_paths_rejected(self):
        for value in ('https://github.com/owner/repo', 'git@github.com:owner/repo', 'file:///tmp/repo', '../repo'):
            with self.subTest(path=value), self.assertRaises(ValueError):
                publication.local_repository(value)

    def test_mutated_artifact_rejected(self):
        with tempfile.TemporaryDirectory() as tmp:
            public, _ = self.fixture(Path(tmp))
            expected = site.inventory(public)
            (public / 'unexpected.txt').write_text('unreviewed')
            with self.assertRaisesRegex(ValueError, 'differs'):
                publication.stage(public, expected, site.inventory)


if __name__ == '__main__':
    unittest.main()
