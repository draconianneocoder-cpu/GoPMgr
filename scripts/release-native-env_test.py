#!/usr/bin/env python3
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
"""Exercise standalone release/validator boundaries in disposable repositories."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]


class ReleaseNativeEnvironmentTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='release-native-env-')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        (self.root/'scripts').mkdir()
        for name in ['check-release.sh','native-go-env.py','validate-encrypted-db.sh',
                     'validate-pdfa.sh','validate-pdfa-lib.sh','validate-pades.sh',
                     'pades-lock.sh','pades-publish.sh']:
            shutil.copyfile(ROOT/'scripts'/name,self.root/'scripts'/name)
        self.bin = self.root/'bin'
        self.bin.mkdir()
        self.log = self.root/'calls.jsonl'
        self.env = {k:v for k,v in os.environ.items() if not k.startswith(('GOPMGR_','CGO_','GO','MAKE'))
                    and k not in ['MACOSX_DEPLOYMENT_TARGET','HOST_OS','TARGET_OS','CHILD_STATUS']}
        self.env.update(PATH=str(self.bin)+os.pathsep+os.environ['PATH'],
                        CALL_LOG=str(self.log), TARGET_OS='darwin', CHILD_STATUS='73',
                        GOPMGR_VERAPDF_FORCE_CLI='1',GOPMGR_ICC_PROFILE=str(self.root/'profile.icc'))
        (self.root/'profile.icc').write_text('fixture profile')
        self.stub('go', '''import json,os,sys
if sys.argv[1]=='env':
    with open(os.environ['CALL_LOG'],'a') as f:f.write('"go-env"\\n')
    print(json.dumps({'GOOS':os.environ['TARGET_OS'],'GOHOSTOS':os.environ['TARGET_OS']}))
else:
    with open(os.environ['CALL_LOG'],'a') as f:f.write(json.dumps({'args':sys.argv[1:],
        'floor':os.environ.get('MACOSX_DEPLOYMENT_TARGET'),
        'cflags':os.environ.get('CGO_CFLAGS')})+'\\n')
    if sys.argv[1]=='run' and os.environ['CHILD_STATUS']=='0':
        from pathlib import Path
        source=Path(sys.argv[2])
        names={'gen_schedule.go':['schedule.pdf'],
               'gen_documents.go':['document-charter.pdf','document-multilingual.pdf',
                   'document-japanese.pdf','document-korean.pdf','document-chinese.pdf','combined-report.pdf'],
               'gen_montecarlo.go':['monte-carlo-risk-report.pdf'],
               'gen_signed.go':['signed-document.pdf']}
        for name in names.get(source.name,[]):(source.parent/name).write_text('fixture PDF')
    sys.exit(int(os.environ['CHILD_STATUS']))
''')
        self.stub('verapdf', "import sys\nprint('veraPDF 1.30.2' if sys.argv[1]=='--version' else '<isCompliant>true</isCompliant>')\n")
        self.stub('reuse', '''import os,sys
with open(os.environ['CALL_LOG'],'a') as f:f.write('"reuse:'+sys.argv[1]+'"\\n')
sys.exit(int(os.environ.get('REUSE_EXIT','0')))
''')
        self.stub('make', '''import subprocess,sys
if sys.argv[1:]==['license-check']:
    sys.exit(subprocess.run(['reuse','lint']).returncode)
''')
        parser=self.root/'internal/cli/parser.go'
        parser.parent.mkdir(parents=True)
        parser.write_text('const Version = "1.2.3"\n')
        (self.root/'scripts/wails-build-config.py').write_text('print("1.2.3")\n')
        (self.root/'scripts/check-wails-version.sh').write_text('#!/bin/sh\nexit 0\n')
        self.sample = self.root/'.tmp/gopmgr-pades-test'
        self.sample.mkdir(parents=True)
        (self.sample/'sentinel').write_text('prior sample')
        self.pdfa = self.root/'.tmp/gopmgr-pdfa-test'
        self.pdfa.mkdir()
        (self.pdfa/'sentinel').write_text('prior PDF/A')

    def stub(self,name,body):
        p=self.bin/name
        p.write_text('#!'+sys.executable+'\n'+body)
        p.chmod(0o755)

    def run_script(self,name,extra=None):
        return subprocess.run(['bash','scripts/'+name],cwd=self.root,
                              env={**self.env,**(extra or {})},capture_output=True,text=True,timeout=25)

    def calls(self):
        return [json.loads(x) for x in self.log.read_text().splitlines()] if self.log.exists() else []

    def test_direct_go_execution_uses_floor_and_preserves_failure(self):
        expected = {'check-release.sh':'test','validate-encrypted-db.sh':'test',
                    'validate-pdfa.sh':'run','validate-pades.sh':'run'}
        for script,verb in expected.items():
            with self.subTest(script=script):
                self.log.unlink(missing_ok=True)
                result=self.run_script(script)
                self.assertNotEqual(result.returncode,0,result.stdout+result.stderr)
                go=[x for x in self.calls() if isinstance(x,dict)]
                self.assertEqual(len(go),1, self.calls())
                self.assertEqual(go[0]['args'][0],verb)
                self.assertEqual(go[0]['floor'],'26.0')
                self.assertEqual(go[0]['cflags'],'-O2 -g -mmacosx-version-min=26.0')
                if script=='check-release.sh':
                    self.assertEqual(go[0]['args'],['test','-race','-p=1','-count=1','.',
                                                   './internal/...','./tools/update-manifest'])
                    self.assertLess(self.calls().index('reuse:--version'),self.calls().index('go-env'))
                    self.assertLess(self.calls().index('reuse:lint'),self.calls().index('go-env'))

    def test_all_encrypted_database_commands_keep_the_guard(self):
        result=self.run_script('validate-encrypted-db.sh',{'CHILD_STATUS':'0'})
        self.assertEqual(result.returncode,0,result.stdout+result.stderr)
        go=[x for x in self.calls() if isinstance(x,dict)]
        self.assertEqual([x['args'][-1] for x in go],
                         ['./internal/sqlitedriver','./internal/db','./internal/desktop'])
        for call in go:
            self.assertEqual(call['floor'],'26.0')
            self.assertEqual(call['args'][:2],['test','-count=1'])

    def test_all_pdf_generators_keep_the_guard(self):
        result=self.run_script('validate-pdfa.sh',{'CHILD_STATUS':'0'})
        self.assertEqual(result.returncode,0,result.stdout+result.stderr)
        go=[x for x in self.calls() if isinstance(x,dict)]
        self.assertEqual([Path(x['args'][1]).name for x in go],
                         ['gen_schedule.go','gen_documents.go','gen_montecarlo.go','gen_signed.go'])
        self.assertTrue(all(x['floor']=='26.0' for x in go))
        self.assertIn('gate PASSED',result.stdout)
        self.assertEqual(len(list(self.pdfa.glob('*.pdf'))),9)

    def test_conflicts_do_not_compile_or_mutate_pdf_evidence(self):
        for script in ['check-release.sh','validate-encrypted-db.sh','validate-pdfa.sh','validate-pades.sh']:
            with self.subTest(script=script):
                self.log.unlink(missing_ok=True)
                self.pdfa.mkdir(exist_ok=True)
                (self.pdfa/'sentinel').write_text('prior PDF/A')
                result=self.run_script(script,{'MACOSX_DEPLOYMENT_TARGET':'13'})
                self.assertNotEqual(result.returncode,0,result.stdout+result.stderr)
                self.assertFalse(any(isinstance(x,dict) for x in self.calls()))
                self.assertIn('native-go-env:',result.stdout+result.stderr)
                self.assertEqual((self.pdfa/'sentinel').read_text(),'prior PDF/A')
                self.assertEqual((self.sample/'sentinel').read_text(),'prior sample')
                self.assertFalse((self.root/'.tmp/gopmgr-pades-test.lock').exists())

    def test_conflict_preserves_an_existing_pades_lock(self):
        lock=self.root/'.tmp/gopmgr-pades-test.lock'
        lock.mkdir()
        owner=lock/'owner'
        owner.write_text('another validation owns this lock')
        result=self.run_script('validate-pades.sh',{'MACOSX_DEPLOYMENT_TARGET':'13'})
        self.assertNotEqual(result.returncode,0,result.stdout+result.stderr)
        self.assertIn('native-go-env:',result.stderr)
        self.assertEqual(owner.read_text(),'another validation owns this lock')
        self.assertEqual((self.sample/'sentinel').read_text(),'prior sample')
        self.assertEqual(self.calls(),['go-env'])

    def test_pdfa_non_strict_conflict_is_explicit_skip_without_generation(self):
        result=self.run_script('validate-pdfa.sh',{'MACOSX_DEPLOYMENT_TARGET':'13','GOPMGR_PDFA_STRICT':'0'})
        self.assertEqual(result.returncode,0,result.stdout+result.stderr)
        self.assertIn('SKIP:',result.stdout)
        self.assertNotIn('gate PASSED',result.stdout)
        self.assertEqual(self.calls(),['go-env'])
        self.assertTrue((self.pdfa/'sentinel').exists())

    def test_release_failed_reuse_stops_before_any_go(self):
        result=self.run_script('check-release.sh',{'REUSE_EXIT':'1'})
        self.assertNotEqual(result.returncode,0)
        self.assertIn('reuse', (result.stdout+result.stderr).lower())
        self.assertFalse(any(isinstance(x,dict) for x in self.calls()))
        self.assertNotIn('go-env',self.calls())

    def test_non_darwin_commands_preserve_supplied_settings(self):
        for script in ['check-release.sh','validate-encrypted-db.sh','validate-pdfa.sh','validate-pades.sh']:
            with self.subTest(script=script):
                self.log.unlink(missing_ok=True)
                result=self.run_script(script,{'TARGET_OS':'linux','CGO_CFLAGS':'-DKEEP'})
                self.assertNotEqual(result.returncode,0)
                go=[x for x in self.calls() if isinstance(x,dict)]
                self.assertEqual(len(go),1)
                self.assertIsNone(go[0]['floor'])
                self.assertEqual(go[0]['cflags'],'-DKEEP')


if __name__=='__main__':
    unittest.main()
