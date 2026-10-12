# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

"""Exercise real harness lifecycle code with isolated, non-cryptographic fixtures."""

import os
from pathlib import Path
import select
import shutil
import signal
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parent.parent
DEADLINE = 15


class HarnessLifecycle(unittest.TestCase):
    def setUp(self):
        self.fixture = tempfile.TemporaryDirectory(prefix="gopmgr-pades-lifecycle-")
        self.addCleanup(self.fixture.cleanup)
        self.root = Path(self.fixture.name).resolve()
        scripts = self.root / "scripts"
        scripts.mkdir()
        for name in ("validate-pades-external.sh", "validate-pades-trusted-source.sh", "pades-lock.sh"):
            shutil.copyfile(ROOT / "scripts" / name, scripts / name)
        self.sample = self.root / ".tmp/gopmgr-pades-test"
        self.lock = self.root / ".tmp/gopmgr-pades-test.lock"
        self.trusted = self.root / ".tmp/gopmgr-pades-trusted-source"
        self.trusted_lock = self.root / ".tmp/gopmgr-pades-trusted-source.lock"
        self.pdf = self.root / "supplied.pdf"
        pdf = b"%PDF-1.7\n/ByteRange [0 0000000000 0000000000 0000000000] /Contents <3000>\n%%EOF\n"
        start, end = pdf.index(b"<"), pdf.index(b">") + 1
        self.pdf.write_bytes(pdf.replace(b"0000000000 0000000000 0000000000",
                                        f"{start:010d} {end:010d} {len(pdf)-end:010d}".encode()))
        self.env = {key: value for key, value in os.environ.items() if not key.startswith("GOPMGR_")}
        self.env["GOPMGR_TEST_PDF"] = str(self.pdf)
        self.env["GOPMGR_TEST_ROOT"] = str(self.root)
        self.bin = self.root / "bin"
        self.bin.mkdir()
        # Control every optional validator. These stubs test artifact ownership
        # and process flow, not signatures, trust, or PDF conformance.
        stub = f"#!{sys.executable}\n" + '''
import os, pathlib, subprocess, sys
name = pathlib.Path(sys.argv[0]).name
if name == "openssl":
    print("OpenSSL fixture")
elif name == "pdfsig":
    print("Signature Validation: Signature is Valid.")
    print("Certificate Validation: Certificate is Trusted")
elif name == "verapdf":
    print('<report><signature><filter>Adobe.PPKLite</filter><subFilter>ETSI.CAdES.detached</subFilter></signature></report>')
elif name == "certutil":
    sys.exit(1)
elif name == "rm":
    subprocess.run(["/bin/rm", *sys.argv[1:]], check=True)
    marker = pathlib.Path(os.environ["GOPMGR_TEST_ROOT"]) / "cleanup-seen"
    if any(arg.endswith("gopmgr-pades-test.lock") for arg in sys.argv[1:]) and "GOPMGR_TEST_READY_FD" in os.environ and not marker.exists():
        marker.touch()
        os.write(int(os.environ["GOPMGR_TEST_READY_FD"]), b"cleanup\\n")
        sys.stdin.readline()
'''
        for name in ("openssl", "qpdf", "pdfsig", "verapdf", "dss-validation-tool", "certutil", "rm"):
            path = self.bin / name
            path.write_text(stub)
            path.chmod(0o700)
        self.env["PATH"] = str(self.bin) + os.pathsep + self.env["PATH"]
        generator = scripts / "validate-pades.sh"
        generator.write_text(f'''#!/bin/bash
set -eu
mkdir -p "$GOPMGR_TEST_ROOT/.tmp/gopmgr-pades-test"
cp "$GOPMGR_TEST_PDF" "$GOPMGR_TEST_ROOT/.tmp/gopmgr-pades-test/signed-sample.pdf"
exec "{sys.executable}" -c 'import os,sys; os.write(int(os.environ["GOPMGR_TEST_READY_FD"]), b"generator\\n"); sys.stdin.readline()'
''')

    def run_harness(self, name, *args, **overrides):
        return subprocess.run(["bash", str(self.root / "scripts" / name), *map(str, args)],
                              env=self.env | overrides, capture_output=True, timeout=DEADLINE)

    def assert_success(self, result):
        self.assertEqual(result.returncode, 0, (result.stdout + result.stderr).decode())

    def test_explicit_pdf_without_prior_generation(self):
        self.assertFalse(self.sample.exists())
        before = self.pdf.read_bytes()
        result = self.run_harness("validate-pades-external.sh", self.pdf)
        self.assert_success(result)
        self.assertEqual(self.pdf.read_bytes(), before)
        self.assertTrue((self.sample / "signed-sample.cms.der").is_file())
        report = (self.sample / "external-validation-report.txt").read_text()
        self.assertIn("evidence_source=supplied_pdf", report)
        self.assertIn("evidence_generator=not_applicable", report)
        self.assertFalse((self.sample / "signed-sample.pdf").exists())

    def test_trusted_timeout_preserves_owner_evidence(self):
        self.trusted.mkdir(parents=True)
        self.trusted_lock.write_bytes(b"owner-lock\n")
        temporary = self.trusted / "trusted-source-validation-report.tmp"
        final = self.trusted / "trusted-source-validation-report.txt"
        temporary.write_bytes(b"owner-in-progress\n")
        final.write_bytes(b"previous-report\n")
        result = self.run_harness("validate-pades-trusted-source.sh",
                                  GOPMGR_PADES_TRUSTED_LOCK_TIMEOUT_SECONDS="0")
        self.assertEqual(result.returncode, 75, result.stderr.decode())
        self.assertEqual(self.trusted_lock.read_bytes(), b"owner-lock\n")
        self.assertTrue(temporary.exists(), "contender removed owner's temporary report")
        self.assertEqual(temporary.read_bytes(), b"owner-in-progress\n")
        self.assertEqual(final.read_bytes(), b"previous-report\n")

    def test_trusted_prelock_rejection_preserves_owner_report(self):
        self.trusted.mkdir(parents=True)
        temporary = self.trusted / "trusted-source-validation-report.tmp"
        temporary.write_bytes(b"owner-in-progress\n")
        result = self.run_harness("validate-pades-trusted-source.sh", temporary)
        self.assertEqual(result.returncode, 64)
        self.assertTrue(temporary.exists(), "pre-lock rejection removed owner's report")
        self.assertEqual(temporary.read_bytes(), b"owner-in-progress\n")

    def test_inherited_locks_survive_success_and_failure(self):
        self.sample.mkdir(parents=True)
        self.lock.write_bytes(b"caller-lock\n")
        self.trusted_lock.write_bytes(b"trusted-caller\n")
        self.assert_success(self.run_harness("validate-pades-external.sh", self.pdf,
                                            GOPMGR_PADES_LOCK_HELD="1"))
        failed = self.run_harness("validate-pades-external.sh", self.root / "missing.pdf",
                                  GOPMGR_PADES_LOCK_HELD="1")
        self.assertEqual(failed.returncode, 66)
        self.assertEqual(self.lock.read_bytes(), b"caller-lock\n")
        self.assert_success(self.run_harness("validate-pades-trusted-source.sh",
                                            GOPMGR_PADES_TRUSTED_LOCK_HELD="1"))
        failed = self.run_harness("validate-pades-trusted-source.sh", self.root / "missing.pdf",
                                  GOPMGR_PADES_TRUSTED_LOCK_HELD="1")
        self.assertEqual(failed.returncode, 1)
        self.assertEqual(self.trusted_lock.read_bytes(), b"trusted-caller\n")
        self.assertFalse((self.trusted / "trusted-source-validation-report.tmp").exists())

    def check_signal(self, requested, expected, inherited=False):
        self.sample.mkdir(parents=True)
        if inherited:
            self.lock.write_bytes(b"caller-lock\n")
        ready, writer = os.pipe()
        self.addCleanup(os.close, ready)
        env = self.env | {"GOPMGR_TEST_READY_FD": str(writer)}
        if inherited:
            env["GOPMGR_PADES_LOCK_HELD"] = "1"
        proc = subprocess.Popen(["bash", str(self.root / "scripts/validate-pades-external.sh")],
                                env=env, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                stderr=subprocess.PIPE, pass_fds=(writer,), start_new_session=True)
        os.close(writer)

        def reap():
            # Reap descendants even if a signalled harness already exited.
            try:
                os.killpg(proc.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            proc.communicate(timeout=DEADLINE)
        self.addCleanup(reap)

        def wait_ready(message):
            self.assertTrue(select.select([ready], [], [], DEADLINE)[0], "readiness deadline expired")
            self.assertEqual(os.read(ready, 64), message)

        wait_ready(b"generator\n")
        self.assertTrue(self.lock.exists())
        os.kill(proc.pid, requested)  # Only the harness; foreground child remains held.
        proc.stdin.write(b"release generator\n")
        proc.stdin.flush()
        successor = None
        if not inherited:
            wait_ready(b"cleanup\n")
            # The old signal trap removed the lock and then ran EXIT cleanup
            # again. Acquire a real successor while the first rm is held.
            result = subprocess.run(["bash", "-c", 'source "$1"; pades_acquire_lock "$2" 0',
                                     "fixture", str(self.root / "scripts/pades-lock.sh"), str(self.lock)],
                                    env=self.env, capture_output=True, timeout=DEADLINE)
            self.assert_success(result)
            successor = self.lock.read_bytes()
            proc.stdin.write(b"release cleanup\n")
            proc.stdin.flush()
        stdout, stderr = proc.communicate(timeout=DEADLINE)
        self.assertTrue(self.lock.exists(), "harness removed a caller/successor lock")
        self.assertEqual(self.lock.read_bytes(), b"caller-lock\n" if inherited else successor)
        self.assertEqual(proc.returncode, expected, (stdout + stderr).decode())
        self.assertFalse((self.sample / "signed-sample.cms.der").exists())
        self.assertFalse((self.sample / "signed-sample.extract.txt").exists())
        self.assertNotIn(b"harness completed", stdout)

    def test_term_preserves_successor(self):
        self.check_signal(signal.SIGTERM, 143)

    def test_int_preserves_successor(self):
        self.check_signal(signal.SIGINT, 130)

    def test_term_preserves_inherited_lock(self):
        self.check_signal(signal.SIGTERM, 143, inherited=True)

    def test_int_preserves_inherited_lock(self):
        self.check_signal(signal.SIGINT, 130, inherited=True)


if __name__ == "__main__":
    unittest.main()
