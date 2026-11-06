#!/usr/bin/env python3
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
"""Exercise the scanner's execution contract in isolated source trees."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]

class ScannerTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='memory-scan-test-')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        (self.root/'scripts').mkdir()
        for name in ['memory-safety-scan.sh', 'native-go-env.py']:
            shutil.copyfile(ROOT/'scripts'/name, self.root/'scripts'/name)
        for name in ['internal', 'tools', 'bin']:
            (self.root/name).mkdir()
        (self.root/'main.go').write_text('package main\nfunc main() {}\n')
        self.bin = self.root/'bin'
        self.log = self.root/'calls.jsonl'
        self.env = {k:v for k,v in os.environ.items() if not k.startswith(('GO','CGO_'))
                    and k not in ['MACOSX_DEPLOYMENT_TARGET']}
        self.env.update(PATH=str(self.bin), CALL_LOG=str(self.log), TEST_OS='darwin')
        for name in ['bash','dirname','grep','mktemp','cat','rm','tail','head']:
            (self.bin/name).symlink_to(shutil.which(name))
        (self.bin/'python3').symlink_to(sys.executable)
        self.stub('go', '''import json,os,sys
args=sys.argv[1:]
if args[0]=='env':
 if os.environ.get('ENV_EXIT'):sys.exit(int(os.environ['ENV_EXIT']))
 print(json.dumps({'GOOS':os.environ['TEST_OS'],'GOHOSTOS':os.environ['TEST_OS']}))
 sys.exit(0)
with open(os.environ['CALL_LOG'],'a') as f:f.write(json.dumps({'tool':'go','args':args,
 'floor':os.environ.get('MACOSX_DEPLOYMENT_TARGET'),'flags':os.environ.get('CGO_CFLAGS')})+'\\n')
if args[0]=='vet':sys.exit(int(os.environ.get('VET_EXIT','0')))
if args[0]=='run':
 print(os.environ.get('HELPER_TEXT',''),end='')
 sys.exit(int(os.environ.get('HELPER_EXIT','0')))
sys.exit(99)
''')
        for name in ['staticcheck','gosec','govulncheck']:
            self.stub(name, '''import json,os,sys
with open(os.environ['CALL_LOG'],'a') as f:f.write(json.dumps({'tool':os.path.basename(sys.argv[0]),
 'args':sys.argv[1:],'floor':os.environ.get('MACOSX_DEPLOYMENT_TARGET')})+'\\n')
sys.exit(int(os.environ.get('OPTIONAL_EXIT','0')) if os.environ.get('OPTIONAL_FAIL_TOOL',os.path.basename(sys.argv[0]))==os.path.basename(sys.argv[0]) else 0)
''')

    def stub(self, name, body):
        p=self.bin/name
        p.write_text('#!'+sys.executable+'\n'+body)
        p.chmod(0o755)

    def run_scan(self, **extra):
        return subprocess.run([str(self.bin/'bash'),'scripts/memory-safety-scan.sh'],
                              cwd=self.root,env={**self.env,**extra},capture_output=True,text=True,timeout=20)

    def calls(self):
        return [json.loads(x) for x in self.log.read_text().splitlines()] if self.log.exists() else []

    def test_silent_helper_failure_cannot_pass(self):
        r=self.run_scan(HELPER_EXIT='73')
        self.assertNotEqual(r.returncode,0,r.stdout+r.stderr)
        self.assertNotIn('Memory-safety gate PASSED.',r.stdout)
        self.assertNotIn('OK: every os.Open',r.stdout)

    def test_helper_findings_and_execution_diagnostics_are_visible(self):
        for status,text in [('1','fixture.go:4: possible leak\n'),('2','fixture read failed\n')]:
            with self.subTest(status=status):
                r=self.run_scan(HELPER_EXIT=status,HELPER_TEXT=text)
                self.assertNotEqual(r.returncode,0,r.stdout+r.stderr)
                self.assertIn(text.strip(),r.stdout+r.stderr)
                self.assertNotIn('Memory-safety gate PASSED.',r.stdout)

    def test_vet_failure_and_missing_go_cannot_pass(self):
        r=self.run_scan(VET_EXIT='7')
        self.assertNotEqual(r.returncode,0)
        (self.bin/'go').unlink()
        r=self.run_scan()
        self.assertNotEqual(r.returncode,0,r.stdout+r.stderr)
        self.assertNotIn('Memory-safety gate PASSED.',r.stdout)

    def test_native_floor_reaches_all_commands_and_scope_is_explicit(self):
        r=self.run_scan()
        self.assertEqual(r.returncode,0,r.stdout+r.stderr)
        calls=self.calls()
        self.assertEqual([c['tool'] for c in calls],['go','go','staticcheck','gosec','govulncheck'])
        self.assertTrue(all(c['floor']=='26.0' for c in calls),calls)
        self.assertIn('-mmacosx-version-min=26.0',calls[0]['flags'])
        self.assertEqual(calls[1]['args'],['run','./tools/os-open-scan','./main.go','./internal','./tools'])
        self.assertEqual(calls[0]['args'],['vet','-tags','production','.','./internal/...','./tools/update-manifest','./tools/os-open-scan'])

    def test_conflicting_native_floor_runs_no_checks(self):
        r=self.run_scan(MACOSX_DEPLOYMENT_TARGET='15.0')
        self.assertNotEqual(r.returncode,0,r.stdout+r.stderr)
        self.assertEqual(self.calls(),[])
        self.assertNotIn('Memory-safety gate PASSED.',r.stdout)

    def test_go_identity_failure_runs_no_checks(self):
        r=self.run_scan(ENV_EXIT='11')
        self.assertNotEqual(r.returncode,0,r.stdout+r.stderr)
        self.assertEqual(self.calls(),[])
        self.assertNotIn('Memory-safety gate PASSED.',r.stdout)

    def test_successful_helper_diagnostics_are_not_findings(self):
        r=self.run_scan(HELPER_TEXT='fixture compiler diagnostic\n')
        self.assertEqual(r.returncode,0,r.stdout+r.stderr)
        self.assertIn('fixture compiler diagnostic',r.stdout)

    def test_linux_preserves_environment(self):
        r=self.run_scan(TEST_OS='linux',CGO_CFLAGS='-O1 -g',MACOSX_DEPLOYMENT_TARGET='fixture-preserved')
        self.assertEqual(r.returncode,0,r.stdout+r.stderr)
        self.assertTrue(all(c['floor']=='fixture-preserved' for c in self.calls()))
        self.assertEqual(self.calls()[0]['flags'],'-O1 -g')

    def test_optional_policy_is_unchanged(self):
        r=self.run_scan(OPTIONAL_EXIT='8')
        self.assertEqual(r.returncode,0,r.stdout+r.stderr)
        self.assertIn('warn:',r.stdout)
        r=self.run_scan(OPTIONAL_EXIT='8',GOPMGR_STRICT_OPTIONAL_SCANS='1')
        self.assertNotEqual(r.returncode,0,r.stdout+r.stderr)
        for name in ['staticcheck','gosec','govulncheck']:
            with self.subTest(analyzer=name):
                r=self.run_scan(OPTIONAL_EXIT='8',OPTIONAL_FAIL_TOOL=name)
                self.assertEqual(r.returncode,0,r.stdout+r.stderr)
                r=self.run_scan(OPTIONAL_EXIT='8',OPTIONAL_FAIL_TOOL=name,GOPMGR_STRICT_OPTIONAL_SCANS='1')
                self.assertNotEqual(r.returncode,0,r.stdout+r.stderr)
        for name in ['staticcheck','gosec','govulncheck']:(self.bin/name).unlink()
        r=self.run_scan(GOPMGR_STRICT_OPTIONAL_SCANS='1')
        self.assertEqual(r.returncode,0,r.stdout+r.stderr)
        self.assertIn('skip:',r.stdout)

if __name__=='__main__':
    unittest.main()
