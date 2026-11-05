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


class ReleaseLicenseTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(prefix='release license ')
        self.addCleanup(self.directory.cleanup)
        self.base = Path(self.directory.name)
        self.root = self.base / 'repository'
        self.bin = self.base / 'bin'
        self.root.mkdir()
        self.bin.mkdir()
        for name in ('Makefile', 'scripts/check-release.sh',
                     'scripts/check-release-tag.sh', 'scripts/check-release-tag_test.sh'):
            path = self.root / name
            path.parent.mkdir(exist_ok=True)
            shutil.copyfile(ROOT / name, path)
        for command in ('bash', 'env', 'dirname', 'grep', 'head', 'sed', 'find'):
            (self.bin / command).symlink_to(shutil.which(command))
        (self.bin / 'python3').symlink_to(sys.executable)
        path = self.root / 'internal/cli/parser.go'
        path.parent.mkdir(parents=True)
        path.write_text('const Version = "1.2.3"\n')
        (self.root / 'scripts/wails-build-config.py').write_text('print("1.2.3")\n')
        (self.root / 'scripts/check-wails-version.sh').write_text('#!/bin/bash\nexit 0\n')
        for name in ('.DS_Store', 'nested/.DS_Store', '.git/index'):
            path = self.root / name
            path.parent.mkdir(exist_ok=True)
            path.write_bytes(b'preserve release sentinel')
        self.trace = self.base / 'reuse.jsonl'
        self.make_trace = self.base / 'make.jsonl'
        self.env = {'PATH': str(self.bin), 'TRACE': str(self.trace),
                    'MAKE_TRACE': str(self.make_trace), 'REAL_MAKE': MAKE,
                    'PROBE_EXIT': '0', 'LINT_EXIT': '0', 'GOPMGR_RELEASE_TAG': 'v1.2.3'}
        self.tool('make', r"""import json, os, sys
with open(os.environ['MAKE_TRACE'], 'a') as output:
    output.write(json.dumps(sys.argv[1:]) + '\n')
if sys.argv[1:] in [[name] for name in ('config-check', 'installer-tool-pins',
                                      'required-font-assets', 'windows-installer-scaffold')]:
    sys.exit(0)
if sys.argv[1:] == ['license-check']:
    os.execv(os.environ['REAL_MAKE'], [os.environ['REAL_MAKE'], *sys.argv[1:]])
sys.exit('unexpected Make boundary: ' + repr(sys.argv[1:]))
""")
        self.tool('go', "import sys\nprint('DOWNSTREAM-GO-MARKER')\nsys.exit(77)\n")
        self.install_reuse()

    def tool(self, name, body):
        path = self.bin / name
        path.write_text('#!' + sys.executable + '\n' + body)
        path.chmod(0o755)

    def install_reuse(self):
        self.tool('reuse', r"""import json, os, sys
with open(os.environ['TRACE'], 'a') as output:
    output.write(json.dumps(sys.argv[1:]) + '\n')
if sys.argv[1:] == ['--version']:
    sys.exit(int(os.environ['PROBE_EXIT']))
if sys.argv[1:] == ['lint']:
    print('LINT-DIAGNOSTIC', file=sys.stderr)
    sys.exit(int(os.environ['LINT_EXIT']))
sys.exit(95)
""")

    def snapshot(self):
        return {str(p.relative_to(self.root)): p.read_bytes()
                for p in self.root.rglob('*') if p.is_file()}

    def invoke(self, entry):
        command = ['/bin/bash', 'scripts/check-release.sh'] if entry == 'script' else [MAKE, entry]
        return subprocess.run(command, cwd=self.root, env=self.env,
                              capture_output=True, text=True, timeout=30)

    def calls(self, path):
        return [json.loads(line) for line in path.read_text().splitlines()] if path.exists() else []

    def test_every_entry_rejects_unavailable_or_failed_licensing(self):
        for entry in ('script', 'check-release', 'tag-preflight'):
            for mode in ('missing', 'broken', 'probe-failure', 'lint-failure'):
                with self.subTest(entry=entry, mode=mode):
                    self.install_reuse()
                    self.trace.unlink(missing_ok=True)
                    self.make_trace.unlink(missing_ok=True)
                    self.env.update(PROBE_EXIT='0', LINT_EXIT='0')
                    if mode == 'missing':
                        (self.bin / 'reuse').unlink()
                    elif mode == 'broken':
                        (self.bin / 'reuse').write_text('#!/nonexistent/reuse-interpreter\n')
                    elif mode == 'probe-failure':
                        self.env['PROBE_EXIT'] = '91'
                    else:
                        self.env['LINT_EXIT'] = '92'
                    before = self.snapshot()
                    result = self.invoke(entry)
                    output = result.stdout + result.stderr
                    self.assertNotEqual(result.returncode, 0, output)
                    self.assertNotIn('DOWNSTREAM-GO-MARKER', output)
                    self.assertNotIn('Licensing compliant.', output)
                    self.assertNotIn('local release gates passed', output)
                    self.assertNotIn('skipping license check', output)
                    self.assertIn('reuse', output.lower())
                    if mode == 'lint-failure':
                        self.assertIn(['lint'], self.calls(self.trace))
                        self.assertIn(['license-check'], self.calls(self.make_trace))
                        self.assertIn('LINT-DIAGNOSTIC', output)
                    else:
                        self.assertNotIn(['lint'], self.calls(self.trace))
                    self.assertEqual(self.snapshot(), before)

    def test_healthy_licensing_delegates_once_and_allows_downstream_gate(self):
        for entry in ('script', 'check-release', 'tag-preflight'):
            with self.subTest(entry=entry):
                self.trace.unlink(missing_ok=True)
                self.make_trace.unlink(missing_ok=True)
                before = self.snapshot()
                result = self.invoke(entry)
                output = result.stdout + result.stderr
                # The deliberate race-gate failure prevents actual build/packaging.
                self.assertNotEqual(result.returncode, 0)
                self.assertIn('DOWNSTREAM-GO-MARKER', output)
                self.assertIn('Race detector flagged tests', output)
                self.assertIn('Licensing compliant.', output)
                self.assertEqual(self.calls(self.trace), [['--version'], ['lint']])
                self.assertEqual(self.calls(self.make_trace).count(['license-check']), 1)
                self.assertEqual(self.snapshot(), before)

    def test_invalid_tag_stops_before_licensing_execution(self):
        self.env['GOPMGR_RELEASE_TAG'] = 'v9.9.9'
        before = self.snapshot()
        result = self.invoke('tag-preflight')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('does not match product version', result.stdout + result.stderr)
        self.assertEqual(self.calls(self.trace), [])
        self.assertEqual(self.snapshot(), before)

    def test_failed_licensing_can_recover_without_changing_sources(self):
        before = self.snapshot()
        self.env['LINT_EXIT'] = '92'
        result = self.invoke('check-release')
        self.assertNotIn('DOWNSTREAM-GO-MARKER', result.stdout + result.stderr)
        self.assertIn(['lint'], self.calls(self.trace))
        self.trace.unlink()
        self.env['LINT_EXIT'] = '0'
        result = self.invoke('check-release')
        self.assertIn('DOWNSTREAM-GO-MARKER', result.stdout + result.stderr)
        self.assertEqual(self.calls(self.trace), [['--version'], ['lint']])
        self.assertEqual(self.snapshot(), before)


if __name__ == '__main__':
    unittest.main()
