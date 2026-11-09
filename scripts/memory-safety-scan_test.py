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
        for name in ['bash','dirname','grep','mktemp','cat','rm','sed']:
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

    # Grep-based stages 2, 4, 5 and 6.

    def write_source(self, rel, text):
        path=self.root/rel
        path.parent.mkdir(parents=True,exist_ok=True)
        path.write_text(text)
        return path

    def reset_sources(self):
        shutil.rmtree(self.root/'internal')
        (self.root/'internal').mkdir()

    def wrap_tool(self, name, fail_env):
        """Replace a PATH tool with a logging wrapper that exits 2 when
        fail_env is set and, for grep, its value occurs in the arguments."""
        real=shutil.which(name)
        (self.bin/name).unlink()
        self.stub(name, f"""import json,os,sys
args=sys.argv[1:]
with open(os.environ['CALL_LOG'],'a') as f:f.write(json.dumps({{'tool':{name!r},'args':args}})+'\\n')
want=os.environ.get({fail_env!r})
if want is not None and (want=='' or any(want in a for a in args)):
 sys.stderr.write({name!r}+': injected operational failure\\n');sys.exit(2)
os.execv({real!r},[{real!r}]+args)
""")

    def assert_stage_failed(self, r, message):
        out=r.stdout+r.stderr
        self.assertNotEqual(r.returncode,0,out)
        self.assertIn('FAIL: '+message,r.stdout,out)
        self.assertIn('Memory-safety gate FAILED.',r.stdout,out)
        self.assertNotIn('Memory-safety gate PASSED.',r.stdout,out)

    def test_grep_stage_heuristics_are_preserved(self):
        filler='\n'*28
        cases=[
            ('clean tree passes',{},0,['OK: no unsafe.Pointer usage','OK: every sql.Open','OK: no obvious mutex','OK: no explicit goroutines'],[]),
            ('unsafe.Pointer is a finding',{'internal/a.go':'package a\nvar p = unsafe.Pointer(nil)\n'},1,
             ['FAIL: unsafe.Pointer use is forbidden','./internal/a.go:2:'],[]),
            ('vendored unsafe.Pointer is ignored',{'internal/vendor/x/a.go':'package x\nvar p = unsafe.Pointer(nil)\n'},0,
             ['OK: no unsafe.Pointer usage'],['vendor/x/a.go']),
            ('Close on the 30th line passes',{'internal/db.go':'package db\ndb, _ := sql.Open("x", "y")\n'+filler+'db.Close()\n'},0,
             ['OK: every sql.Open'],[]),
            ('Close on the 31st line is a finding',{'internal/db.go':'package db\ndb, _ := sql.Open("x", "y")\n'+filler+'\ndb.Close()\n'},1,
             ['FAIL: sql.Open call sites missing Close()','./internal/db.go:2: sql.Open without nearby Close'],[]),
            ('an owning Store passes',{'internal/db.go':'package db\ndb, _ := sql.Open("x", "y")\nreturn &Store{db: db}\n'},0,
             ['OK: every sql.Open'],[]),
            ('mutex value copy is a finding',{'internal/m.go':'package m\nm :=\ts.mu\n'},1,
             ['FAIL: possible value-copy of a mutex','./internal/m.go:2:'],[]),
            ('mutex method use is not a copy',{'internal/m.go':'package m\nheld = s.mu.TryLock()\n'},0,
             ['OK: no obvious mutex value-copies'],[]),
            # Known blind spot kept from the original heuristic: a line that
            # also calls through a .mu. selector is excluded, copy or not.
            ('a line with a .mu. selector is excluded',{'internal/m.go':'package m\nm := s.mu; s.mu.Lock()\n'},0,
             ['OK: no obvious mutex value-copies'],[]),
            ('a /vendor/ text match outside vendor is still a finding',{'internal/m.go':'package m\nm := s.mu // see /vendor/ notes\n'},1,
             ['FAIL: possible value-copy of a mutex','./internal/m.go:2:'],[]),
            ('goroutines are listed without failing',{'internal/g.go':'package g\nfunc f() {\n\tgo func() {}()\n\tgo worker(1)\n}\n'},0,
             ['goroutines detected','./internal/g.go:3:','./internal/g.go:4:'],[]),
            ('commented goroutines are not listed',{'internal/g.go':'package g\n// go func() {}()\n'},0,
             ['OK: no explicit goroutines'],[]),
        ]
        for name,files,want_rc,present,absent in cases:
            with self.subTest(case=name):
                self.reset_sources()
                for rel,text in files.items():self.write_source(rel,text)
                r=self.run_scan()
                out=r.stdout+r.stderr
                self.assertEqual(r.returncode,want_rc,out)
                for text in present:self.assertIn(text,r.stdout,out)
                for text in absent:self.assertNotIn(text,r.stdout,out)

    def test_grep_operational_error_fails_each_stage(self):
        # Each pattern selects one grep call; the fixture gives every stage a
        # match so a failure cannot hide behind an empty result.
        self.write_source('internal/f.go','package f\nvar p = unsafe.Pointer(nil)\ndb, _ := sql.Open("x", "y")\ndb.Close()\nm := s.mu\nfunc g() {\n\tgo func() {}()\n}\n')
        self.wrap_tool('grep','GREP_FAIL')
        stages=[
            ('unsafe\\.Pointer','unsafe.Pointer scan could not complete'),
            ('sql\\.Open(','sql.Open scan could not complete'),
            ('[a-zA-Z_]+\\.mu([^.]|$)','mutex value-copy scan could not complete'),
            ('\\.mu\\.','mutex value-copy scan could not complete'),
            ('go (func|','goroutine inventory could not complete'),
            (':[[:space:]]*//','goroutine inventory could not complete'),
        ]
        for pattern,message in stages:
            with self.subTest(pattern=pattern):
                self.log.unlink(missing_ok=True)
                r=self.run_scan(GREP_FAIL=pattern)
                self.assertTrue(any(c['tool']=='grep' and any(pattern in a for a in c['args']) for c in self.calls()),self.calls())
                self.assert_stage_failed(r,message)
                self.assertIn('grep: injected operational failure',r.stderr)

    def test_unreadable_source_fails_every_grep_stage(self):
        if os.geteuid()==0:
            self.skipTest('root can read a mode-000 file')
        self.write_source('internal/a.go','package a\nfunc f() {\n\tgo func() {}()\n}\ndb, _ := sql.Open("x", "y")\ndb.Close()\n')
        locked=self.write_source('internal/locked.go','package a\n')
        locked.chmod(0)
        self.addCleanup(locked.chmod,0o644)
        r=self.run_scan()
        for message in ['unsafe.Pointer scan could not complete','sql.Open scan could not complete',
                        'mutex value-copy scan could not complete','goroutine inventory could not complete']:
            with self.subTest(stage=message):
                self.assert_stage_failed(r,message)
        self.assertIn('locked.go',r.stderr)

    def test_missing_source_root_fails(self):
        shutil.rmtree(self.root/'tools')
        r=self.run_scan()
        for message in ['unsafe.Pointer scan could not complete','sql.Open scan could not complete',
                        'mutex value-copy scan could not complete','goroutine inventory could not complete']:
            with self.subTest(stage=message):
                self.assert_stage_failed(r,message)
        self.assertIn('source root ./tools does not exist',r.stderr)

    def test_sql_window_read_failure_fails(self):
        self.write_source('internal/db.go','package db\ndb, _ := sql.Open("x", "y")\ndb.Close()\n')
        self.wrap_tool('sed','SED_FAIL')
        r=self.run_scan(SED_FAIL='')
        self.assertTrue(any(c['tool']=='sed' for c in self.calls()),self.calls())
        self.assert_stage_failed(r,'sql.Open scan could not complete')
        self.assertNotIn('sql.Open without nearby Close',r.stdout)

    def test_unparseable_sql_match_location_fails_closed(self):
        self.write_source('internal/a:b.go','package a\ndb, _ := sql.Open("x", "y")\ndb.Close()\n')
        r=self.run_scan()
        self.assert_stage_failed(r,'sql.Open scan could not complete')
        self.assertIn('cannot parse grep match location',r.stderr)

if __name__=='__main__':
    unittest.main()
