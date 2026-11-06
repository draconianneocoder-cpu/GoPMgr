# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
"""Argument, failure and display contracts affected by P2 shell repairs."""
import json
import os
import pathlib
import pty
import errno
import select
import time
import shutil
import subprocess
import tempfile
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[2]
BINARY = ROOT / '.tmp/engineering/bin/shellcheck'


class ShellContracts(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(prefix='p2 shell contracts ')
        self.addCleanup(self.directory.cleanup)
        self.root = pathlib.Path(self.directory.name)
        (self.root / 'scripts').mkdir()
        (self.root / 'bin').mkdir()
        self.env = dict(os.environ, PATH=str(self.root / 'bin') + ':' + os.environ['PATH'],
                        ARG_LOG=str(self.root / 'arguments.json'), VERSION='1.2.3')

    def copy(self, name):
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(ROOT / name, path)
        return path

    def stub(self, name, body):
        path = self.root / 'bin' / name
        path.write_text('#!/usr/bin/env python3\n' + body)
        path.chmod(0o755)

    def test_repaired_command_scripts_are_shellcheck_clean(self):
        paths = ['scripts/memory-safety-scan.sh', 'scripts/verify-module-notices.sh',
                 'scripts/check-release.sh', 'scripts/package-linux.sh']
        result = subprocess.run([str(BINARY), '--norc', '--source-path=SCRIPTDIR',
                                 *paths, 'scripts/package-version-lib.sh'], cwd=ROOT,
                                capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_release_race_preserves_arguments_and_failure(self):
        self.copy('scripts/check-release.sh')
        self.copy('scripts/native-go-env.py')
        parser = self.root / 'internal/cli/parser.go'
        parser.parent.mkdir(parents=True)
        parser.write_text('const Version = "1.2.3"\n')
        (self.root / 'scripts/wails-build-config.py').write_text('print("1.2.3")\n')
        (self.root / 'scripts/check-wails-version.sh').write_text('#!/bin/bash\nexit 0\n')
        self.stub('make', 'pass\n')
        self.stub('reuse', 'pass\n')
        self.stub('go', 'import json,os,sys\n'
                  'if sys.argv[1] == "env": print(json.dumps({"GOOS":"linux","GOHOSTOS":"linux"})); sys.exit(0)\n'
                  'open(os.environ["ARG_LOG"],"w").write(json.dumps(sys.argv[1:]))\n'
                  'sys.exit(77)\n')
        result = subprocess.run(['/bin/bash', 'scripts/check-release.sh'], cwd=self.root,
                                env=self.env, capture_output=True, text=True)
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertIn('Race detector flagged tests', result.stdout)
        self.assertEqual(json.loads((self.root / 'arguments.json').read_text()),
                         ['test', '-race', '-p=1', '-count=1', '.', './internal/...', './tools/update-manifest'])

    def test_memory_scanner_preserves_package_arguments_and_strict_failures(self):
        self.copy('scripts/memory-safety-scan.sh')
        (self.root / 'main.go').write_text('package main\n')
        (self.root / 'internal').mkdir()
        (self.root / 'tools').mkdir()
        logger = ('import json,os,sys\n'
                  'with open(os.environ["ARG_LOG"],"a") as output: output.write(json.dumps(sys.argv[1:])+"\\n")\n')
        self.stub('go', logger)
        for name in ['staticcheck', 'gosec', 'govulncheck']:
            self.stub(name, logger + 'sys.exit(1)\n')
        for strict, expected in [('0', 0), ('1', 1)]:
            self.env['GOPMGR_STRICT_OPTIONAL_SCANS'] = strict
            (self.root / 'arguments.json').unlink(missing_ok=True)
            result = subprocess.run(['/bin/bash', 'scripts/memory-safety-scan.sh'], cwd=self.root,
                                    env=self.env, capture_output=True, text=True)
            self.assertEqual(result.returncode, expected, result.stdout + result.stderr)
            calls = [json.loads(line) for line in (self.root / 'arguments.json').read_text().splitlines()]
            self.assertEqual(calls[0], ['vet', '-tags', 'production', '.', './internal/...', './tools/update-manifest'])
            self.assertEqual(calls[-3:], [
                ['.', './internal/...', './tools/update-manifest'],
                ['-quiet', '.', './internal/...', './tools/update-manifest'],
                ['.', './internal/...', './tools/update-manifest'],
            ])
            self.assertNotIn('\\033', result.stdout)
            self.assertIn('Memory-safety gate', result.stdout)

    def test_memory_scanner_terminal_colors_are_preserved(self):
        self.copy('scripts/memory-safety-scan.sh')
        (self.root / 'main.go').write_text('package main\n')
        (self.root / 'internal').mkdir()
        (self.root / 'tools').mkdir()
        for name in ['go', 'staticcheck', 'gosec', 'govulncheck']:
            self.stub(name, 'pass\n')
        master, slave = pty.openpty()
        process = subprocess.Popen(['/bin/bash', 'scripts/memory-safety-scan.sh'], cwd=self.root,
                                   env=self.env, stdout=slave, stderr=slave)
        os.close(slave)
        output = b''
        deadline = time.monotonic() + 15
        try:
            while time.monotonic() < deadline:
                ready, _, _ = select.select([master], [], [], 0.1)
                if ready:
                    try:
                        chunk = os.read(master, 4096)
                    except OSError as error:
                        if error.errno != errno.EIO:
                            raise
                        break
                    if not chunk:
                        break
                    output += chunk
                elif process.poll() is not None:
                    break
            self.assertEqual(process.wait(timeout=max(0.1, deadline - time.monotonic())), 0)
        finally:
            if process.poll() is None:
                process.kill()
                process.wait()
            os.close(master)
        self.assertIn(b'\x1b[1;33m', output)
        self.assertIn(b'\x1b[0;32m', output)
        self.assertNotIn(b'\\033', output)

    def test_linux_artifact_display_escapes_newlines_and_preserves_names(self):
        for name in ['scripts/package-linux.sh', 'scripts/package-version-lib.sh',
                     'scripts/verify-module-notices.sh', 'scripts/release-tool-versions.env']:
            self.copy(name)
        binary = self.root / 'build/bin/gopmgr'
        binary.parent.mkdir(parents=True)
        header = bytearray(20)
        header[:6] = b'\x7fELF\x02\x01'
        header[18] = 0x3e
        binary.write_bytes(header)
        binary.chmod(0o755)
        notices = self.root / 'third_party/GO-MODULE-NOTICES.txt'
        notices.parent.mkdir()
        notices.write_text('Module: std\n')
        version = next(line.split('=', 1)[1] for line in
                       (ROOT / 'scripts/release-tool-versions.env').read_text().splitlines()
                       if line.startswith('NFPM_VERSION='))
        self.stub('go', 'import sys\n'
                  f'print("\\tmod\\tgithub.com/goreleaser/nfpm/v2\\t{version}" if sys.argv[-1].endswith("nfpm") else "fixture: go1.27.3")\n')
        self.stub('readelf', 'print("(NEEDED) [libgtk-4.so.1]\\n(NEEDED) [libwebkitgtk-6.0.so.4]")\n')
        self.stub('nfpm', 'import pathlib,sys\npathlib.Path(sys.argv[sys.argv.index("--target")+1]).touch()\n')
        packages = self.root / 'build/packages'
        packages.mkdir()
        (packages / 'odd\nname.deb').touch()
        (packages / 'space name.rpm').touch()
        result = subprocess.run(['/bin/bash', 'scripts/package-linux.sh'], cwd=self.root,
                                env=self.env, capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertNotIn('odd\nname.deb', result.stdout)
        self.assertIn('odd\\nname.deb', result.stdout)
        self.assertIn('space\\ name.rpm', result.stdout)
        self.assertIn('  gopmgr-1.2.3-amd64.deb', result.stdout)


if __name__ == '__main__':
    unittest.main()
