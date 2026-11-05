# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
"""Real actionlint boundary checks over disposable tracked workflows."""
import os
import pathlib
import shutil
import subprocess
import sys
import tempfile
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[2]
CHECK = ROOT / 'scripts/engineering/workflow_lint.py'
BINARY = ROOT / '.tmp/engineering/bin/actionlint'
GOOD = 'name: Fixture\non: push\njobs:\n  test:\n    runs-on: ubuntu-26.04\n    steps:\n      - run: echo ok\n'


class WorkflowLintTests(unittest.TestCase):
    def setUp(self):
        directory = tempfile.TemporaryDirectory(prefix='p4 workflow ')
        self.addCleanup(directory.cleanup)
        self.root = pathlib.Path(directory.name)
        subprocess.run(['/usr/bin/git', 'init', '-q', str(self.root)], check=True)

    def track(self, name, text):
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text)
        subprocess.run(['/usr/bin/git', '-C', str(self.root), 'add', '--', name], check=True)
        return path

    def check(self, *args, env=None):
        return subprocess.run([sys.executable, '-B', str(CHECK), '--root', str(self.root),
                               '--actionlint', str(BINARY), *args], capture_output=True, text=True, env=env)

    def test_valid_workflow_and_exact_extra_runner_label_pass_without_writes(self):
        p = self.track('.github/workflows/good.yml', GOOD)
        before = p.read_bytes(), (self.root / '.git/index').read_bytes()
        r = self.check()
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertIn('1 tracked workflows; clean', r.stdout)
        self.assertEqual(before, (p.read_bytes(), (self.root / '.git/index').read_bytes()))

    def test_unknown_runner_is_not_covered_by_the_exact_label_allowance(self):
        self.track('.github/workflows/bad.yml', GOOD.replace('ubuntu-26.04', 'ubuntu-26.typo'))
        r = self.check()
        self.assertEqual(r.returncode, 1, r.stderr)
        self.assertIn('runner-label', r.stdout)
        self.assertIn('ubuntu-26.typo', r.stdout)

    def test_invalid_expression_is_rejected(self):
        self.track('.github/workflows/expression.yml', GOOD.replace('echo ok', 'echo ${{ github.missing_field }}'))
        r = self.check()
        self.assertEqual(r.returncode, 1, r.stderr)
        self.assertIn('missing_field', r.stdout)

    def test_undeclared_needs_output_is_rejected(self):
        self.track('.github/workflows/outputs.yml', """on: push
jobs:
  build:
    runs-on: ubuntu-latest
    outputs:
      known: value
    steps:
      - run: echo ok
  test:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - run: echo ${{ needs.build.outputs.missing }}
""")
        r = self.check()
        self.assertEqual(r.returncode, 1, r.stderr)
        self.assertIn('missing', r.stdout)

    def test_missing_needs_job_is_rejected(self):
        self.track('.github/workflows/needs.yml', GOOD.replace('    runs-on:', '    needs: absent\n    runs-on:'))
        r = self.check()
        self.assertEqual(r.returncode, 1, r.stderr)
        self.assertIn('absent', r.stdout)

    def test_malformed_yaml_is_a_failure(self):
        self.track('.github/workflows/broken.yml', 'on: [push\n')
        r = self.check()
        self.assertNotEqual(r.returncode, 0)
        self.assertIn('broken.yml', r.stdout + r.stderr)

    def test_tracked_yaml_and_reusable_workflows_are_not_omitted(self):
        self.track('.github/workflows/main.yml', GOOD)
        self.track('.github/workflows/reusable.yaml', GOOD.replace('on: push', 'on: workflow_call'))
        self.track('vendor/.github/workflows/excluded.yml', 'invalid yaml: [')
        (self.root / '.github/workflows/untracked.yml').write_text('invalid yaml: [')
        r = self.check()
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertIn('2 tracked workflows; clean', r.stdout)

    def test_self_hosted_runner_example_is_valid_without_execution(self):
        self.track('.github/workflows/custom.yml', GOOD.replace('runs-on: ubuntu-26.04',
                   'runs-on: [self-hosted, linux, x64, ubuntu-26.04]'))
        r = self.check()
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)

    def reusable_pair(self, supplied_input):
        self.track('.github/workflows/caller.yml', """on: push
jobs:
  call:
    uses: ./.github/workflows/reusable.yaml
    with:
      INPUT_NAME: value
""".replace("INPUT_NAME", supplied_input))
        self.track('.github/workflows/reusable.yaml', """on:
  workflow_call:
    inputs:
      known:
        type: string
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: echo ok
""")
    def test_invalid_reusable_workflow_input_is_rejected(self):
        self.reusable_pair('missing_input')
        r = self.check()
        self.assertEqual(r.returncode, 1, r.stderr)
        self.assertIn('missing_input', r.stdout)

    def test_valid_reusable_workflow_call_passes(self):
        self.reusable_pair('known')
        r = self.check()
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertIn('2 tracked workflows; clean', r.stdout)

    def test_ambient_config_cannot_hide_an_invalid_expression(self):
        self.track('.github/workflows/bad.yml', GOOD.replace('echo ok', 'echo ${{ github.missing_field }}'))
        (self.root / '.github/actionlint.yml').write_text('paths:\n  "**":\n    ignore: [".*"]\n')
        r = self.check()
        self.assertEqual(r.returncode, 1, r.stderr)
        self.assertIn('missing_field', r.stdout)

    def test_inline_scripts_and_external_linters_are_never_executed(self):
        marker = self.root / 'EXECUTED'
        self.track('.github/workflows/safe.yml', GOOD.replace('echo ok', 'touch EXECUTED'))
        fake_bin = self.root / 'bin'
        fake_bin.mkdir()
        for name in ['shellcheck', 'pyflakes']:
            p = fake_bin / name
            p.write_text('#!/bin/sh\ntouch "' + str(marker) + '"\n')
            p.chmod(0o755)
        r = self.check(env=dict(os.environ, PATH=str(fake_bin) + ':' + os.environ['PATH']))
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertFalse(marker.exists())

    def test_substituted_tool_is_rejected_before_execution(self):
        self.track('.github/workflows/good.yml', GOOD)
        fake = self.root / 'fake'
        marker = self.root / 'EXECUTED'
        fake.write_text('#!/bin/sh\ntouch "' + str(marker) + '"\n')
        fake.chmod(0o755)
        r = self.check('--actionlint', str(fake))
        self.assertNotEqual(r.returncode, 0)
        self.assertIn('checksum differs', r.stderr)
        self.assertFalse(marker.exists())

    def test_missing_tool_is_not_a_clean_check(self):
        self.track('.github/workflows/good.yml', GOOD)
        r = self.check('--actionlint', str(self.root / 'absent'))
        self.assertNotEqual(r.returncode, 0)
        self.assertIn('absent', r.stderr)

    def test_empty_inventory_is_not_success(self):
        self.track('README', 'no workflows\n')
        r = self.check()
        self.assertNotEqual(r.returncode, 0)
        self.assertIn('no tracked workflows', r.stderr)

    def test_nested_workflow_is_rejected_instead_of_silently_skipped(self):
        self.track('.github/workflows/nested/job.yml', GOOD)
        r = self.check()
        self.assertNotEqual(r.returncode, 0)
        self.assertIn('nested workflow', r.stderr)

    def test_symlink_is_rejected(self):
        target = self.track('vendor/copied.yml', GOOD)
        path = self.root / '.github/workflows/linked.yml'
        path.parent.mkdir(parents=True)
        path.symlink_to(target)
        subprocess.run(['/usr/bin/git', '-C', str(self.root), 'add', '.github/workflows/linked.yml'], check=True)
        r = self.check()
        self.assertNotEqual(r.returncode, 0)
        self.assertIn('symlink', r.stderr)

    def test_invalid_later_workflow_cannot_be_omitted(self):
        self.track('.github/workflows/a.yml', GOOD)
        self.track('.github/workflows/z.yaml', GOOD.replace('echo ok', 'echo ${{ github.missing_field }}'))
        r = self.check()
        self.assertEqual(r.returncode, 1, r.stderr)
        self.assertIn('z.yaml', r.stdout)
        self.assertIn('missing_field', r.stdout)

    def test_odd_workflow_filename_is_checked(self):
        self.track('.github/workflows/odd name\nfile.yaml', GOOD.replace('echo ok', 'echo ${{ github.missing_field }}'))
        r = self.check()
        self.assertEqual(r.returncode, 1, r.stderr)
        self.assertIn('missing_field', r.stdout)

    def test_make_target_fails_on_seeded_expression_and_recovers(self):
        p = self.track('.github/workflows/good.yml', GOOD)
        for name in ['Makefile', 'mise.toml', 'mise.lock', 'tools/engineering/artifacts.json',
                     '.github/actionlint.yaml', 'scripts/engineering/provision.py',
                     'scripts/engineering/shellcheck.py', 'scripts/engineering/workflow_lint.py']:
            target = self.root / name
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(ROOT / name, target)
        env = dict(os.environ, PATH=str(BINARY.parent) + ':' + os.environ['PATH'])
        for text, expected in [(GOOD, 0), (GOOD.replace('echo ok', 'echo ${{ github.missing_field }}'), 2), (GOOD, 0)]:
            p.write_text(text)
            before = p.read_bytes(), (self.root / '.git/index').read_bytes()
            r = subprocess.run(['make', 'workflow-lint'], cwd=self.root, env=env, capture_output=True, text=True)
            self.assertEqual(r.returncode, expected, r.stdout + r.stderr)
            self.assertEqual(before, (p.read_bytes(), (self.root / '.git/index').read_bytes()))
            if expected:
                self.assertIn('missing_field', r.stdout)


if __name__ == '__main__':
    unittest.main()
