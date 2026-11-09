# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
"""Behavioral qualification for the tracked, read-only ShellCheck boundary."""
import json
import os
import pathlib
import shutil
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

ROOT = pathlib.Path(__file__).resolve().parents[2]
CHECK = ROOT / 'scripts/engineering/shellcheck.py'
BINARY = ROOT / '.tmp/engineering/bin/shellcheck'


class ShellCheckTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(prefix='p2 shell inventory ')
        self.addCleanup(self.directory.cleanup)
        self.root = pathlib.Path(self.directory.name)
        subprocess.run(['/usr/bin/git', 'init', '-q', str(self.root)], check=True)

    def track(self, name, body):
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(body)
        subprocess.run(['/usr/bin/git', '-C', str(self.root), 'add', '--', name], check=True)
        return path

    def check(self, *args):
        return subprocess.run([sys.executable, '-B', str(CHECK), '--root', str(self.root),
                               '--shellcheck', str(BINARY), *args], capture_output=True, text=True)

    def test_accepts_valid_tracked_bash_without_mutating_source_or_index(self):
        path = self.track('scripts/good.sh', '#!/bin/bash\nprintf "%s\\n" "safe value"\n')
        source, index = path.read_bytes(), (self.root / '.git/index').read_bytes()
        result = self.check()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(path.read_bytes(), source)
        self.assertEqual((self.root / '.git/index').read_bytes(), index)

    def test_inventory_includes_odd_paths_and_extensionless_scripts(self):
        self.track('scripts/new runner\nname', '#!/usr/bin/env bash\necho safe\n')
        self.track('tools/portable.sh', '#!/bin/sh\necho safe\n')
        self.track('vendor/copied.sh', '#!/bin/bash\necho $1\n')
        self.track('frontend/dist/generated.sh', '#!/bin/bash\necho $1\n')
        (self.root / 'scripts/untracked.sh').write_text('#!/bin/bash\necho $1\n')
        result = self.check('--inventory')
        self.assertEqual(result.returncode, 0, result.stderr)
        inventory = json.loads(result.stdout)
        self.assertEqual(inventory['files'], [
            {'path': 'scripts/new runner\nname', 'shell': 'bash'},
            {'path': 'tools/portable.sh', 'shell': 'sh'},
        ])
        self.assertEqual({x['path'] for x in inventory['excluded']},
                         {'vendor/copied.sh', 'frontend/dist/generated.sh'})

    def test_env_split_string_shebang_is_not_silently_omitted(self):
        self.track('scripts/new runner', '#!/usr/bin/env -S bash -eu\nprintf "%s\n" safe\n')
        result = self.check('--inventory')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(json.loads(result.stdout)['files'],
                         [{'path': 'scripts/new runner', 'shell': 'bash'}])

    def test_newly_tracked_unsafe_extensionless_script_blocks(self):
        self.track('scripts/new runner', '#!/bin/bash\necho $1\n')
        result = self.check()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('SC2086', result.stdout)

    def test_shell_dialect_is_not_forced_to_bash(self):
        self.track('scripts/portable.sh', '#!/bin/sh\nvalues=(one two)\ntrue "${values[@]}"\n')
        result = self.check()
        self.assertNotEqual(result.returncode, 0)
        self.assertRegex(result.stdout, 'SC(3030|3054)')

    def test_unsupported_shell_does_not_disappear_from_inventory(self):
        self.track('scripts/unsupported.sh', '#!/bin/zsh\necho safe\n')
        result = self.check('--inventory')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('unsupported shell', result.stderr)

    def test_unlisted_external_source_is_not_followed(self):
        self.track('scripts/main.sh', '#!/bin/bash\nsource vendor/library.sh\n')
        self.track('vendor/library.sh', '#!/bin/bash\necho $1\n')
        result = self.check()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('SC1091', result.stdout)
        self.assertNotIn('SC2086', result.stdout)

    def test_unreviewed_source_is_not_authorized_by_incidental_batch_membership(self):
        self.track('scripts/main.sh', '#!/bin/bash\nsource scripts/other.sh\nfixture_output\n')
        self.track('scripts/other.sh', '#!/bin/bash\nfixture_output() { echo safe; }\n')
        result = self.check()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('SC1091', result.stdout)

    def test_reviewed_tracked_library_is_followed(self):
        self.track('scripts/main.sh', '#!/bin/bash\nsource scripts/package-version-lib.sh\nprintf "%s\\n" "$value"\n')
        self.track('scripts/package-version-lib.sh', '#!/bin/bash\nexport value="safe value"\n')
        result = self.check()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        import shellcheck as scanner
        inventory = scanner.tracked_inventory(self.root.resolve())
        with mock.patch.object(scanner, 'LIBRARIES', set()):
            findings = scanner.scan(self.root.resolve(), BINARY, inventory)
        self.assertIn(1091, {finding['code'] for finding in findings})
        self.assertEqual(scanner.scan(self.root.resolve(), BINARY, inventory), [])

    def test_symlink_cannot_smuggle_excluded_source_into_files(self):
        target = self.track('vendor/library.sh', '#!/bin/bash\necho $1\n')
        path = self.root / 'scripts/linked.sh'
        path.parent.mkdir()
        path.symlink_to(target)
        subprocess.run(['/usr/bin/git', '-C', str(self.root), 'add', '--', 'scripts/linked.sh'], check=True)
        result = self.check('--inventory')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('symlink', result.stderr)

    def test_ambient_configuration_cannot_disable_findings(self):
        self.track('scripts/bad.sh', '#!/bin/bash\necho $1\n')
        self.track('.shellcheckrc', 'disable=SC2086\n')
        result = self.check()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('SC2086', result.stdout)

    def test_missing_or_unverified_checker_is_not_success(self):
        self.track('scripts/good.sh', '#!/bin/bash\necho safe\n')
        marker = self.root / 'executed'
        fake = self.root / 'fake'
        fake.write_text(f'#!/bin/sh\ntouch "{marker}"\necho 0.11.0\n')
        fake.chmod(0o755)
        for binary in [fake, self.root / 'absent']:
            result = self.check('--shellcheck', str(binary))
            self.assertNotEqual(result.returncode, 0)
        self.assertFalse(marker.exists())

    def test_make_target_rejects_seeded_fault_and_accepts_restored_fixture(self):
        self.track('scripts/seed.sh', '#!/bin/bash\necho "$1"\n')
        for name in ['Makefile', 'mise.toml', 'mise.lock', 'tools/engineering/artifacts.json',
                     'scripts/engineering/provision.py', 'scripts/engineering/shellcheck.py']:
            target = self.root / name
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(ROOT / name, target)
        env = dict(os.environ, PATH=str(BINARY.parent) + ':' + os.environ['PATH'])
        for text, expected in [('#!/bin/bash\necho "$1"\n', 0),
                               ('#!/bin/bash\necho $1\n', 2),
                               ('#!/bin/bash\necho "$1"\n', 0)]:
            (self.root / 'scripts/seed.sh').write_text(text)
            result = subprocess.run(['make', 'shellcheck'], cwd=self.root, env=env,
                                    capture_output=True, text=True)
            self.assertEqual(result.returncode, expected, result.stdout + result.stderr)
            if expected:
                self.assertIn('SC2086', result.stdout)

    def test_many_scripts_preserve_all_findings_and_deduplicate_libraries(self):
        import shellcheck as scanner
        expected_paths = {'scripts/package-version-lib.sh'}
        for index in range(8):
            name = f'scripts/{index}-' + 'long-name-' * 10 + '.sh'
            self.track(name, '#!/bin/bash\necho $1\n')
            expected_paths.add(name)
        self.track('scripts/package-version-lib.sh', '#!/bin/bash\necho $1\n')
        inventory = scanner.tracked_inventory(self.root.resolve())
        findings = scanner.scan(self.root.resolve(), BINARY, inventory)
        self.assertEqual({finding['file'] for finding in findings}, expected_paths)
        self.assertEqual(len(findings), 9)
        self.assertEqual({finding['code'] for finding in findings}, {2086})

    def test_inline_external_sources_does_not_escape_files_boundary(self):
        self.track('scripts/main.sh', '#!/bin/bash\n# shellcheck external-sources=true\nsource vendor/library.sh\n')
        self.track('vendor/library.sh', '#!/bin/bash\necho $1\n')
        result = self.check()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('SC1144', result.stdout)
        self.assertNotIn('SC2086', result.stdout)

    def test_analyzer_error_is_not_an_empty_success(self):
        import shellcheck as scanner
        with self.assertRaisesRegex(ValueError, 'ShellCheck failed'):
            scanner.scan(self.root, BINARY, {'files': [{'path': 'absent.sh', 'shell': 'bash'}]})

    def test_git_inventory_failure_retains_the_actual_diagnostic(self):
        (self.root / '.git').rename(self.root / 'not-git')
        result = self.check()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('not a git repository', result.stderr)

    def test_empty_inventory_is_not_success(self):
        result = self.check()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('no tracked first-party shell scripts', result.stderr)


if __name__ == '__main__':
    unittest.main()
