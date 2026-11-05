# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
"""Exercise the Make licensing contract without mutating the checkout."""
import json
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
MAKE = shutil.which('make')
FIND = shutil.which('find')


class LicenseCheckTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(prefix='license contract ')
        self.addCleanup(self.directory.cleanup)
        self.base = Path(self.directory.name)
        self.root = self.base / 'repository'
        self.bin = self.base / 'bin'
        self.root.mkdir()
        self.bin.mkdir()
        shutil.copyfile(ROOT / 'Makefile', self.root / 'Makefile')
        for name in ('.DS_Store', 'nested/.DS_Store', '.git/index', 'keep.txt'):
            path = self.root / name
            path.parent.mkdir(exist_ok=True)
            path.write_bytes(b'preserve these bytes\x00\xff')
        (self.bin / 'find').symlink_to(FIND)
        self.trace = self.base / 'reuse-arguments.json'
        self.env = {'PATH': str(self.bin), 'TRACE': str(self.trace), 'REUSE_EXIT': '0'}
        self.install_reuse()

    def install_reuse(self):
        path = self.bin / 'reuse'
        path.write_text('#!' + sys.executable + '\n'
                        'import json, os, sys\n'
                        'from pathlib import Path\n'
                        'Path(os.environ["TRACE"]).write_text(json.dumps(sys.argv[1:]))\n'
                        'print("validator marker", file=sys.stderr)\n'
                        'sys.exit(int(os.environ["REUSE_EXIT"]))\n')
        path.chmod(0o755)

    def snapshot(self):
        return {str(p.relative_to(self.root)): p.read_bytes()
                for p in self.root.rglob('*') if p.is_file()}

    def run_target(self, target='license-check'):
        return subprocess.run([MAKE, target], cwd=self.root, env=self.env,
                              capture_output=True, text=True, timeout=30)

    def test_success_preserves_all_files_and_runs_lint(self):
        before = self.snapshot()
        result = self.run_target()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(json.loads(self.trace.read_text()), ['lint'])
        self.assertEqual(self.snapshot(), before)

    def test_validator_failure_preserves_files_and_recovery_runs_again(self):
        before = self.snapshot()
        self.env['REUSE_EXIT'] = '73'
        result = self.run_target()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('validator marker', result.stderr)
        self.assertEqual(json.loads(self.trace.read_text()), ['lint'])
        self.assertEqual(self.snapshot(), before)
        self.trace.unlink()
        self.env['REUSE_EXIT'] = '0'
        result = self.run_target()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(json.loads(self.trace.read_text()), ['lint'])
        self.assertEqual(self.snapshot(), before)

    def test_missing_validator_fails_without_cleanup(self):
        (self.bin / 'reuse').unlink()
        before = self.snapshot()
        result = self.run_target()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('reuse', result.stderr)
        self.assertFalse(self.trace.exists())
        self.assertEqual(self.snapshot(), before)

    def test_broken_validator_fails_without_cleanup(self):
        path = self.bin / 'reuse'
        path.write_text('#!/nonexistent/license-validator-python\n')
        before = self.snapshot()
        result = self.run_target()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('reuse', result.stderr)
        self.assertFalse(self.trace.exists())
        self.assertEqual(self.snapshot(), before)

    def test_existing_target_filename_does_not_skip_validation(self):
        (self.root / 'license-check').write_text('a file must not skip the check')
        self.env['REUSE_EXIT'] = '73'
        result = self.run_target()
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(json.loads(self.trace.read_text()), ['lint'])

    def test_cleanup_is_explicit_and_preserves_other_files(self):
        (self.root / 'clean-finder-metadata').write_text('must not skip maintenance')
        before = self.snapshot()
        result = self.run_target('clean-finder-metadata')
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        expected = {name: data for name, data in before.items()
                    if Path(name).name != '.DS_Store'}
        self.assertEqual(self.snapshot(), expected)
        self.assertFalse(self.trace.exists())

    def test_explicit_cleanup_failure_is_not_hidden(self):
        (self.bin / 'find').unlink()
        path = self.bin / 'find'
        path.write_text('#!/bin/sh\necho cleanup-failure-marker >&2\nexit 71\n')
        path.chmod(0o755)
        before = self.snapshot()
        result = self.run_target('clean-finder-metadata')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('cleanup-failure-marker', result.stderr)
        self.assertEqual(self.snapshot(), before)
        self.assertFalse(self.trace.exists())


if __name__ == '__main__':
    unittest.main()
