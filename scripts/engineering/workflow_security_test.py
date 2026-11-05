# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
"""Real offline zizmor tests for workflow trust boundaries and failure policy."""
import contextlib
import io
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
CHECK = ROOT / 'scripts/engineering/workflow_security.py'
BINARY = ROOT / '.tmp/engineering/bin/zizmor'
CHECKOUT = 'actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1'
GOOD = """name: Fixture
on: push
permissions:
  contents: read
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: CHECKOUT
        with:
          persist-credentials: false
      - run: echo ok
      - uses: actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9
        with:
          name: fixture
          path: artifact
""".replace('CHECKOUT', CHECKOUT)


class WorkflowSecurityTests(unittest.TestCase):
    def setUp(self):
        directory = tempfile.TemporaryDirectory(prefix='p5 workflow ')
        self.addCleanup(directory.cleanup)
        self.root = pathlib.Path(directory.name)
        subprocess.run(['/usr/bin/git', 'init', '-q', str(self.root)], check=True)

    def track(self, name, text):
        p = self.root / name
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(text)
        subprocess.run(['/usr/bin/git', '-C', str(self.root), 'add', '--', name], check=True)
        return p

    def check(self, *args, env=None):
        return subprocess.run([sys.executable, '-B', str(CHECK), '--root', str(self.root),
                               '--zizmor', str(BINARY), *args], capture_output=True, text=True, env=env)

    def test_clean_workflow_passes_without_source_or_index_writes(self):
        p = self.track('.github/workflows/good.yml', GOOD)
        before = p.read_bytes(), (self.root / '.git/index').read_bytes()
        r = self.check()
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertIn('1 tracked workflows; 0 findings', r.stdout)
        self.assertEqual(before, (p.read_bytes(), (self.root / '.git/index').read_bytes()))

    def test_injection_is_detected(self):
        self.track('.github/workflows/injection.yml', GOOD.replace('on: push', 'on: pull_request').replace('echo ok', 'echo "${{ github.event.pull_request.title }}"'))
        r = self.check()
        self.assertEqual(r.returncode, 1, r.stderr)
        self.assertIn('template-injection', r.stdout)

    def test_inherited_permissions_are_detected(self):
        self.track('.github/workflows/permissions.yml', GOOD.replace('permissions:\n  contents: read\n', ''))
        r = self.check()
        self.assertEqual(r.returncode, 1, r.stderr)
        self.assertIn('excessive-permissions', r.stdout)

    def test_omitted_checkout_credential_setting_is_detected(self):
        self.track('.github/workflows/credentials.yml', GOOD.replace('        with:\n          persist-credentials: false\n', ''))
        r = self.check()
        self.assertEqual(r.returncode, 1, r.stderr)
        self.assertIn('artipacked', r.stdout)

    def test_explicit_true_is_outside_pinned_regular_persona_detection(self):
        # Characterize the upstream limitation, not an endorsement of this input.
        self.track('.github/workflows/explicit.yml', GOOD.replace('persist-credentials: false', 'persist-credentials: true'))
        r = self.check()
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertIn('0 findings', r.stdout)

    def test_release_cache_restore_is_detected(self):
        self.track('.github/workflows/release.yml', GOOD.replace('on: push', 'on:\n  push:\n    tags: ["v*"]').replace('      - run: echo ok', '      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e\n        with:\n          go-version: "1.27.3"\n          cache: true'))
        r = self.check()
        self.assertEqual(r.returncode, 1, r.stderr)
        self.assertIn('cache-poisoning', r.stdout)

    def test_malformed_input_cannot_be_skipped(self):
        self.track('.github/workflows/broken.yml', 'on: [push\n')
        r = self.check()
        self.assertNotEqual(r.returncode, 0)
        self.assertNotIn('0 findings', r.stdout)
        self.assertIn('zizmor', r.stderr)

    def test_nonworkflow_yaml_is_not_silently_skipped(self):
        self.track('.github/workflows/unknown.yml', 'unexpected: value\n')
        r = self.check()
        self.assertNotEqual(r.returncode, 0)
        self.assertNotIn('0 findings', r.stdout)

    def test_ambient_ignores_and_tokens_cannot_change_policy(self):
        self.track('.github/workflows/bad.yml', GOOD.replace('        with:\n          persist-credentials: false\n', '        # zizmor: ignore[artipacked]\n'))
        config = self.root / 'zizmor.yml'
        config.write_text('rules:\n  artipacked:\n    disable: true\n')
        env = dict(os.environ, ZIZMOR_CONFIG=str(config), GH_TOKEN='fixture-not-a-real-token',
                   GITHUB_TOKEN='fixture-not-a-real-token', ZIZMOR_MIN_SEVERITY='high')
        r = self.check(env=env)
        self.assertEqual(r.returncode, 1, r.stderr)
        self.assertIn('artipacked', r.stdout)

    def test_all_tracked_workflows_are_checked_and_exclusions_preserved(self):
        self.track('.github/workflows/a.yml', GOOD)
        self.track('.github/workflows/z.yaml', GOOD.replace('        with:\n          persist-credentials: false\n', ''))
        self.track('vendor/.github/workflows/excluded.yml', 'bad: [')
        (self.root / '.github/workflows/untracked.yml').write_text('bad: [')
        r = self.check()
        self.assertEqual(r.returncode, 1, r.stderr)
        self.assertIn('z.yaml', r.stdout)
        self.assertIn('2 tracked workflows', r.stdout)

    def test_inline_code_is_not_executed(self):
        self.track('.github/workflows/good.yml', GOOD.replace('echo ok', 'touch SHOULD_NOT_EXIST'))
        r = self.check()
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertFalse((self.root / 'SHOULD_NOT_EXIST').exists())

    def test_substituted_binary_is_rejected_before_execution(self):
        self.track('.github/workflows/good.yml', GOOD)
        marker = self.root / 'EXECUTED'
        fake = self.root / 'fake'
        fake.write_text('#!/bin/sh\ntouch "' + str(marker) + '"\n')
        fake.chmod(0o755)
        r = self.check('--zizmor', str(fake))
        self.assertNotEqual(r.returncode, 0)
        self.assertIn('checksum differs', r.stderr)
        self.assertFalse(marker.exists())

    def test_missing_binary_is_not_a_pass(self):
        self.track('.github/workflows/good.yml', GOOD)
        r = self.check('--zizmor', str(self.root / 'missing'))
        self.assertNotEqual(r.returncode, 0)
        self.assertIn('missing', r.stderr)

    def test_empty_inventory_fails(self):
        self.track('README', 'no workflows\n')
        r = self.check()
        self.assertNotEqual(r.returncode, 0)
        self.assertIn('no tracked workflows', r.stderr)

    def test_symlink_input_is_rejected(self):
        p = self.track('vendor/copied.yml', GOOD)
        linked = self.root / '.github/workflows/linked.yml'
        linked.parent.mkdir(parents=True)
        linked.symlink_to(p)
        subprocess.run(['/usr/bin/git', '-C', str(self.root), 'add', '.github/workflows/linked.yml'], check=True)
        r = self.check()
        self.assertNotEqual(r.returncode, 0)
        self.assertIn('symlink', r.stderr)

    def test_make_target_fails_on_injection_and_recovers_without_writes(self):
        p = self.track('.github/workflows/good.yml', GOOD)
        for name in ['Makefile', 'mise.toml', 'mise.lock', 'tools/engineering/artifacts.json',
                     'scripts/engineering/provision.py', 'scripts/engineering/shellcheck.py',
                     'scripts/engineering/workflow_lint.py', 'scripts/engineering/workflow_security.py']:
            target = self.root / name
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(ROOT / name, target)
        bad = GOOD.replace('on: push', 'on: pull_request').replace('echo ok', 'echo "${{ github.event.pull_request.title }}"')
        env = dict(os.environ, PATH=str(BINARY.parent) + ':' + os.environ['PATH'])
        for text, expected in [(GOOD, 0), (bad, 2), (GOOD, 0)]:
            p.write_text(text)
            before = p.read_bytes(), (self.root / '.git/index').read_bytes()
            r = subprocess.run(['make', 'workflow-security'], cwd=self.root, env=env, capture_output=True, text=True)
            self.assertEqual(r.returncode, expected, r.stdout + r.stderr)
            self.assertEqual(before, (p.read_bytes(), (self.root / '.git/index').read_bytes()))
            if expected:
                self.assertIn('template-injection', r.stdout)

    def test_finding_data_fails_even_with_zero_process_exit(self):
        p = self.track('.github/workflows/good.yml', GOOD)
        import workflow_security
        payload = [{'ident': 'template-injection', 'ignored': False,
                    'determinations': {'severity': 'High', 'confidence': 'High'},
                    'locations': [{'symbolic': {'key': {'Local': {'verbatim_path': str(p.resolve())}}}}]}]
        run = subprocess.run

        def zero_exit_with_finding(command, **kwargs):
            if '--format=json' in command:
                return subprocess.CompletedProcess(command, 0, json.dumps(payload), '')
            return run(command, **kwargs)

        output = io.StringIO()
        with contextlib.redirect_stdout(output), \
                mock.patch.object(sys, 'argv', [str(CHECK), '--root', str(self.root), '--zizmor', str(BINARY)]), \
                mock.patch.object(subprocess, 'run', side_effect=zero_exit_with_finding):
            with self.assertRaises(SystemExit) as error:
                workflow_security.main()
        self.assertEqual(error.exception.code, 1)
        self.assertIn('template-injection', output.getvalue())

    def test_invalid_scanner_output_is_not_a_pass(self):
        self.track('.github/workflows/good.yml', GOOD)
        import workflow_security
        run = subprocess.run
        for code, output in [(0, 'not JSON'), (0, '{}'), (14, '[]'), (0, '[{}]'), (1, '[]')]:
            with self.subTest(code=code, output=output):
                def bad_output(command, **kwargs):
                    if '--format=json' in command:
                        return subprocess.CompletedProcess(command, code, output, '')
                    return run(command, **kwargs)
                with mock.patch.object(sys, 'argv', [str(CHECK), '--root', str(self.root), '--zizmor', str(BINARY)]), \
                        mock.patch.object(subprocess, 'run', side_effect=bad_output):
                    with self.assertRaises(SystemExit) as error:
                        workflow_security.main()
                    self.assertNotEqual(error.exception.code, 0)


if __name__ == '__main__':
    unittest.main()
