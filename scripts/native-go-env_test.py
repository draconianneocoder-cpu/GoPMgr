#!/usr/bin/env python3
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
"""Run Make's real test recipes against controlled command boundaries."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]


class NativeMakeEnvironmentTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(prefix="gopmgr-native-env-")
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        (self.root / "scripts").mkdir()
        shutil.copy2(ROOT / "Makefile", self.root / "Makefile")
        launcher = ROOT / "scripts/native-go-env.py"
        if launcher.exists():
            shutil.copy2(launcher, self.root / "scripts/native-go-env.py")
        self.bin = self.root / "bin"
        self.bin.mkdir()
        self.go = self.bin / "go"
        self.go.write_text('''#!/usr/bin/env python3
import json, os, sys
from pathlib import Path
if sys.argv[1] == 'env':
    Path(os.environ['QUERY_RECEIPT']).touch()
    if os.environ.get('ENV_FAILURE'): sys.exit(37)
    if os.environ.get('ENV_JSON'):
        print(os.environ['ENV_JSON']); sys.exit(0)
    print(json.dumps({'GOOS':os.environ.get('TARGET_OS','darwin'),
                      'GOHOSTOS':os.environ.get('HOST_OS','darwin')}))
else:
    Path(os.environ['RECEIPT']).write_text(json.dumps({'args':sys.argv[1:], 'executor':os.environ.get('EXECUTOR_TAG'),
        'env':{k:os.environ.get(k) for k in ['CC','MACOSX_DEPLOYMENT_TARGET',
        'CGO_CFLAGS','CGO_CXXFLAGS','CGO_LDFLAGS','CGO_CPPFLAGS']}}))
    sys.exit(int(os.environ.get('CHILD_STATUS','0')))
''')
        self.go.chmod(0o755)
        self.env = dict(os.environ)
        for key in list(self.env):
            if key.startswith(('CGO_', 'GO', 'MAKE')) or key in ('CC', 'MACOSX_DEPLOYMENT_TARGET'):
                self.env.pop(key, None)
        self.env.update(PATH=str(self.bin) + os.pathsep + os.environ['PATH'],
                        RECEIPT=str(self.root / 'receipt.json'), QUERY_RECEIPT=str(self.root/'query'))
        for name in ['check-coverage-ledger-drift.sh', 'coverage-ratchet.sh']:
            (self.root / 'scripts' / name).write_text('#!/bin/sh\nexec go test -cover "$@"\n')
        (self.root / 'scripts/check-coverage-ledger-drift_test.sh').write_text('#!/bin/sh\nexit 0\n')

    def make(self, target='test', extra=(), env=None):
        return subprocess.run(['make', '-s', target, *extra], cwd=self.root,
                              env={**self.env, **(env or {})}, capture_output=True, text=True)

    def receipt(self):
        return json.loads((self.root / 'receipt.json').read_text())

    def test_native_make_compilation_uses_26_with_go_defaults(self):
        for target in ['test', 'race', 'coverage-ledger-drift', 'coverage-ratchet', 'coverage-ratchet-update']:
            with self.subTest(target=target):
                result = self.make(target)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                env = self.receipt()['env']
                self.assertEqual(env['MACOSX_DEPLOYMENT_TARGET'], '26.0')
                for name in ['CGO_CFLAGS', 'CGO_CXXFLAGS', 'CGO_LDFLAGS']:
                    self.assertEqual(env[name], '-O2 -g -mmacosx-version-min=26.0')

    def test_compiler_precedence_and_race_arguments_are_preserved(self):
        result = self.make('race', env={'CC':'environment-cc'})
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.receipt()['env']['CC'], 'gcc')
        self.assertEqual(self.receipt()['args'][:5], ['test','-race','-p=1','-count=1','-tags'])
        result = self.make(extra=['CC=explicit-cc', 'GO='+str(self.go)])
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.receipt()['env']['CC'], 'explicit-cc')

    def test_coverage_probes_path_go_even_when_make_go_is_overridden(self):
        other = self.bin / 'other-go'
        other.write_text(self.go.read_text().replace("os.environ.get('TARGET_OS','darwin')", "'darwin'").replace("os.environ.get('HOST_OS','darwin')", "'darwin'"))
        other.chmod(0o755)
        result = self.make('coverage-ratchet', extra=['GO='+str(other)],
                           env={'HOST_OS':'linux', 'TARGET_OS':'linux'})
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertIsNone(self.receipt()['env']['MACOSX_DEPLOYMENT_TARGET'])
        result = self.make(extra=['GO='+str(other)],env={'HOST_OS':'linux','TARGET_OS':'linux'})
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual(self.receipt()['env']['MACOSX_DEPLOYMENT_TARGET'],'26.0')

    def test_make_go_command_prefix_is_used_for_probe_and_compilation(self):
        result = self.make(extra=['GO=env EXECUTOR_TAG=selected '+str(self.go)])
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual(self.receipt()['executor'],'selected')
        self.assertEqual(self.receipt()['env']['MACOSX_DEPLOYMENT_TARGET'],'26.0')

    def test_custom_flags_keep_quoting_and_empty_is_not_unset(self):
        flags = '-I"/path with spaces" -DVALUE=1'
        result = self.make(env={'CGO_CFLAGS':flags, 'CGO_CXXFLAGS':'', 'CGO_CPPFLAGS':'-DCPP=1'})
        self.assertEqual(result.returncode, 0, result.stderr)
        env = self.receipt()['env']
        self.assertEqual(env['CGO_CFLAGS'], flags+' -mmacosx-version-min=26.0')
        self.assertEqual(env['CGO_CXXFLAGS'], '-mmacosx-version-min=26.0')
        self.assertEqual(env['CGO_CPPFLAGS'], '-DCPP=1')

    def test_conflicts_stop_before_compilation_from_env_or_make(self):
        cases = [('MACOSX_DEPLOYMENT_TARGET','13.0'), ('MACOSX_DEPLOYMENT_TARGET','27'),
                 ('CGO_CFLAGS','-mmacosx-version-min=13'),
                 ('CGO_CXXFLAGS','-mmacos-version-min=27'),
                 ('CGO_LDFLAGS','-Wl,-platform_version,macos,13.0,27.2'),
                 ('CGO_LDFLAGS','-Wl,@target-flags.rsp'),
                 ('CGO_CPPFLAGS','-mtargetos=macos13'), ('CGO_CFLAGS','@flags')]
        for key, value in cases:
            for command_line in [False, True]:
                with self.subTest(key=key, value=value, command_line=command_line):
                    receipt = self.root/'receipt.json'
                    receipt.unlink(missing_ok=True)
                    result = self.make(extra=[key+'='+value] if command_line else [],
                                       env={} if command_line else {key:value})
                    self.assertNotEqual(result.returncode, 0, result.stdout)
                    self.assertIn('native-go-env:', result.stderr)
                    self.assertFalse(receipt.exists())

    def test_non_native_targets_preserve_all_input_settings(self):
        supplied = {'MACOSX_DEPLOYMENT_TARGET':'13', 'CGO_CFLAGS':'-DKEEP', 'CGO_CPPFLAGS':'@untouched'}
        for host, target in [('linux','linux'), ('windows','windows'), ('darwin','linux'), ('linux','darwin')]:
            result = self.make(env={**supplied, 'HOST_OS':host, 'TARGET_OS':target})
            self.assertEqual(result.returncode, 0, result.stderr)
            for key, value in supplied.items(): self.assertEqual(self.receipt()['env'][key],value)

    def test_equivalent_floor_does_not_grow_in_nested_calls(self):
        flags = '-O3 -mmacosx-version-min=26.0.0'
        result = self.make(env={'MACOSX_DEPLOYMENT_TARGET':'26', 'CGO_CFLAGS':flags})
        self.assertEqual(result.returncode, 0, result.stderr)
        first = self.receipt()['env']
        self.assertEqual(first['CGO_CFLAGS'], flags)
        result = self.make(env={k:v for k,v in first.items() if v is not None})
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.receipt()['env'],first)

    def test_failed_target_lookup_does_not_launch_child(self):
        result = self.make(env={'ENV_FAILURE':'1'})
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('native-go-env:',result.stderr)
        self.assertFalse((self.root/'receipt.json').exists())

    def test_invalid_target_metadata_stops_before_child(self):
        for output in ['not-json', '{}', '{"GOOS":"", "GOHOSTOS":"darwin"}', '[]']:
            with self.subTest(output=output):
                result = self.make(env={'ENV_JSON':output})
                self.assertNotEqual(result.returncode,0)
                self.assertIn('native-go-env:',result.stderr)
                self.assertFalse((self.root/'receipt.json').exists())

    def test_environment_probe_timeout_is_reported_without_child(self):
        import contextlib
        import importlib.util
        import io
        from unittest import mock
        spec = importlib.util.spec_from_file_location('native_env', ROOT/'scripts/native-go-env.py')
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        with mock.patch.object(module.subprocess, 'run', side_effect=subprocess.TimeoutExpired('go env',30)), \
                mock.patch.object(module.os, 'execvpe') as child, contextlib.redirect_stderr(io.StringIO()) as errors:
            self.assertEqual(module.main([str(self.go),'--',str(self.go),'test']),1)
            child.assert_not_called()
            self.assertIn('native-go-env:',errors.getvalue())

    def test_child_failure_propagates_and_help_never_queries_go(self):
        result = self.make(env={'CHILD_STATUS':'42'})
        self.assertNotEqual(result.returncode,0)
        self.assertTrue((self.root/'receipt.json').exists())
        (self.root/'query').unlink()
        result = self.make('help', extra=['GO=/not-installed/go'])
        self.assertEqual(result.returncode,0,result.stderr)
        self.assertIn('Show this help',result.stdout)
        self.assertFalse((self.root/'query').exists())


if __name__ == '__main__':
    unittest.main()
