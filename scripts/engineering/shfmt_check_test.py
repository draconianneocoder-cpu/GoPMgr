# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
"""Real formatter checks for drift, failures and prohibited source mutation."""
import os
import pathlib
import shutil
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

ROOT = pathlib.Path(__file__).resolve().parents[2]
CHECK = ROOT / 'scripts/engineering/shfmt_check.py'
BINARY = ROOT / '.tmp/engineering/bin/shfmt'


class ShfmtCheckTests(unittest.TestCase):
    def setUp(self):
        directory = tempfile.TemporaryDirectory(prefix='p3 format ')
        self.addCleanup(directory.cleanup)
        self.root = pathlib.Path(directory.name)
        subprocess.run(['/usr/bin/git', 'init', '-q', str(self.root)], check=True)

    def track(self, name, body):
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(body)
        subprocess.run(['/usr/bin/git', '-C', str(self.root), 'add', '--', name], check=True)
        return path

    def check(self, *args):
        return subprocess.run([sys.executable, '-B', str(CHECK), '--root', str(self.root),
                               '--shfmt', str(BINARY), *args], capture_output=True, text=True)

    def test_valid_script_passes_without_source_or_index_writes(self):
        path = self.track('scripts/good.sh', '#!/bin/bash\nprintf "%s\\n" "safe value"\n')
        before = path.read_bytes(), (self.root / '.git/index').read_bytes()
        result = self.check()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('1 tracked scripts; 0 need formatting', result.stdout)
        self.assertEqual(before, (path.read_bytes(), (self.root / '.git/index').read_bytes()))

    def test_drift_fails_without_rewriting_source_or_index(self):
        path = self.track('scripts/drift.sh', '#!/bin/bash\nif true;then\n  echo ok\nfi\n')
        before = path.read_bytes(), (self.root / '.git/index').read_bytes()
        result = self.check()
        self.assertEqual(result.returncode, 1, result.stderr)
        self.assertIn('scripts/drift.sh', result.stdout)
        self.assertIn('1 need formatting', result.stdout)
        self.assertEqual(before, (path.read_bytes(), (self.root / '.git/index').read_bytes()))

    def test_invalid_shell_is_an_error_not_clean_input(self):
        self.track('scripts/bad.sh', '#!/bin/bash\nif then\n')
        result = self.check()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('scripts/bad.sh', result.stderr)
        self.assertIn('shfmt failed', result.stderr)

    def test_posix_dialect_rejects_bash_array(self):
        self.track('scripts/portable.sh', '#!/bin/sh\nvalues=(one two)\n')
        result = self.check()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('bash', result.stderr)

    def test_ambient_editorconfig_cannot_ignore_or_restyle_scripts(self):
        path = self.track('scripts/drift.sh', '#!/bin/bash\nif true; then\n    echo ok\nfi\n')
        (self.root / '.editorconfig').write_text('root = true\n[*]\nignore = true\nindent_style = space\nindent_size = 4\n')
        result = self.check()
        self.assertEqual(result.returncode, 1, result.stderr)
        self.assertIn('1 need formatting', result.stdout)
        self.assertIn('    echo ok', path.read_text())

    def test_new_extensionless_odd_path_is_reported_without_execution(self):
        self.track('-new runner\nname', '#!/usr/bin/env -S bash -eu\ntouch SHOULD_NOT_EXIST\nif true;then echo ok;fi\n')
        result = self.check()
        self.assertEqual(result.returncode, 1, result.stderr)
        self.assertIn('"-new runner\\nname"', result.stdout)
        self.assertFalse((self.root / 'SHOULD_NOT_EXIST').exists())

    def test_excluded_and_untracked_sources_are_not_parsed(self):
        self.track('good.sh', '#!/bin/bash\necho ok\n')
        self.track('vendor/bad.sh', '#!/bin/bash\nif then\n')
        self.track('frontend/dist/generated.sh', '#!/bin/bash\nif then\n')
        (self.root / 'untracked.sh').write_text('#!/bin/bash\nif then\n')
        result = self.check()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('1 tracked scripts', result.stdout)

    def test_substituted_binary_is_rejected_before_execution(self):
        self.track('good.sh', '#!/bin/bash\necho ok\n')
        fake = self.root / 'fake-shfmt'
        fake.write_text('#!/bin/sh\ntouch "' + str(self.root / 'EXECUTED') + '"\n')
        fake.chmod(0o755)
        result = self.check('--shfmt', str(fake))
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('executable checksum differs', result.stderr)
        self.assertFalse((self.root / 'EXECUTED').exists())

    def test_missing_binary_fails_visibly(self):
        self.track('good.sh', '#!/bin/bash\necho ok\n')
        result = self.check('--shfmt', str(self.root / 'absent'))
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('absent', result.stderr)

    def test_symlink_is_rejected(self):
        target = self.track('vendor/library.sh', '#!/bin/bash\necho ok\n')
        (self.root / 'linked.sh').symlink_to(target)
        subprocess.run(['/usr/bin/git', '-C', str(self.root), 'add', 'linked.sh'], check=True)
        result = self.check()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('symlink', result.stderr)

    def test_empty_inventory_is_not_a_clean_check(self):
        self.track('README', 'no shell scripts\n')
        result = self.check()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('no tracked first-party shell scripts', result.stderr)

    def test_make_target_fails_on_drift_and_passes_after_restoration(self):
        path = self.track('scripts/seed.sh', '#!/bin/bash\necho ok\n')
        for name in ['Makefile', 'mise.toml', 'mise.lock', 'tools/engineering/artifacts.json',
                     'scripts/engineering/provision.py', 'scripts/engineering/shellcheck.py',
                     'scripts/engineering/shfmt_check.py']:
            target = self.root / name
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(ROOT / name, target)
        env = dict(os.environ, PATH=str(BINARY.parent) + ':' + os.environ['PATH'])
        for text, expected in [('#!/bin/bash\necho ok\n', 0),
                               ('#!/bin/bash\nif true;then echo ok;fi\n', 2),
                               ('#!/bin/bash\necho ok\n', 0)]:
            path.write_text(text)
            before = path.read_bytes(), (self.root / '.git/index').read_bytes()
            result = subprocess.run(['make', 'shfmt-check'], cwd=self.root, env=env,
                                    capture_output=True, text=True)
            self.assertEqual(result.returncode, expected, result.stdout + result.stderr)
            self.assertEqual(before, (path.read_bytes(), (self.root / '.git/index').read_bytes()))
            if expected:
                self.assertIn('1 need formatting', result.stdout)

    def test_timeout_identifies_the_source_that_failed(self):
        self.track('scripts/timed-out.sh', '#!/bin/bash\necho ok\n')
        import shfmt_check
        run = subprocess.run

        def time_out_formatting(command, **kwargs):
            if '-ln=bash' in command:
                raise subprocess.TimeoutExpired(command, 30)
            return run(command, **kwargs)

        with mock.patch.object(sys, 'argv', [str(CHECK), '--root', str(self.root), '--shfmt', str(BINARY)]), \
                mock.patch.object(subprocess, 'run', side_effect=time_out_formatting):
            with self.assertRaises(SystemExit) as failure:
                shfmt_check.main()
        self.assertIn('scripts/timed-out.sh', str(failure.exception))
        self.assertIn('timed out', str(failure.exception))

    def test_multiple_drift_files_are_all_reported(self):
        for name in ['a.sh', 'b.sh', 'c.sh']:
            self.track(name, '#!/bin/bash\nif true;then echo ok;fi\n')
        result = self.check()
        self.assertEqual(result.returncode, 1, result.stderr)
        self.assertIn('3 tracked scripts; 3 need formatting', result.stdout)
        for name in ['a.sh', 'b.sh', 'c.sh']:
            self.assertIn('"' + name + '": needs formatting', result.stdout)

    def test_heredoc_and_literal_output_survive_formatting(self):
        original = b"#!/bin/bash\nif true;then\n  cat <<'DATA'\n  $literal `text` %s\nDATA\nfi\nprintf '%s\\n' 'a  b'\n"
        formatted = subprocess.run([str(BINARY), '-i=0', '-ln=bash'], input=original, capture_output=True, check=True).stdout
        self.assertNotEqual(original, formatted)
        path = self.track('heredoc.sh', formatted.decode())
        result = self.check()
        self.assertEqual(result.returncode, 0, result.stderr)
        for data in [original, path.read_bytes()]:
            out = subprocess.run(['/bin/bash'], input=data, capture_output=True, check=True).stdout
            self.assertEqual(out, b'  $literal `text` %s\na  b\n')


if __name__ == '__main__':
    unittest.main()
