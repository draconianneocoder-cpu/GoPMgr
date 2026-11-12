# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
"""TEST ONLY: transport uses disposable absolute filesystem bare repositories."""
import importlib.util
from pathlib import Path
import subprocess
import tempfile
import unittest
import publication

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
        # Validate the destination again at the actual transport boundary.
        remote = publication.local_repository(remote)
        if lease:
            publication.validate_transaction(candidate['repository'], remote, lease)
        return subprocess.run(['git', '-C', candidate['repository'],
                               '-c', 'core.hooksPath=/dev/null', 'push',
                               '--force-with-lease=refs/heads/gh-pages:' + lease,
                               str(remote), 'refs/heads/gh-pages:refs/heads/gh-pages'],
                              capture_output=True)

    def test_bootstrap_update_stale_tip_and_rollback(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            public, remote = self.fixture(root)
            (public / 'stale.txt').write_text('previous artifact only')
            first = publication.stage(public, site.inventory(public), site.inventory)
            self.assertEqual(self.push(first, remote, '').returncode, 0)
            original = (public / 'index.html').read_text()
            (public / 'index.html').write_text('Fixture B')
            (public / 'stale.txt').unlink()
            second = publication.stage(public, site.inventory(public), site.inventory, remote, first['commit'])
            self.assertEqual(publication.git(Path(second['repository']), 'rev-parse', second['commit'] + '^'), first['commit'])
            self.assertEqual(self.push(second, remote, first['commit']).returncode, 0)
            self.assertNotIn('stale.txt', publication.git(remote, 'ls-tree', '-r', '--name-only', 'gh-pages').splitlines())
            with self.assertRaisesRegex(ValueError, 'stale'):
                publication.stage(public, site.inventory(public), site.inventory, remote, first['commit'])
            # A concurrent update cannot be overwritten with an old lease.
            with self.assertRaisesRegex(ValueError, 'stale'):
                self.push(first, remote, first['commit'])
            # Even a matching lease cannot authorize an old/non-forward tree.
            with self.assertRaisesRegex(ValueError, 'single direct parent'):
                self.push(first, remote, second['commit'])
            self.assertEqual(publication.git(remote, 'rev-parse', 'gh-pages'), second['commit'])
            # Rollback uses a new forward commit with earlier accepted bytes.
            (public / 'index.html').write_text(original)
            (public / 'stale.txt').write_text('previous artifact only')
            rollback = publication.stage(public, site.inventory(public), site.inventory, remote, second['commit'])
            self.assertEqual(self.push(rollback, remote, second['commit']).returncode, 0)
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
