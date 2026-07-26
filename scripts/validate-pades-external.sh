#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# External PAdES validation harness.
#
# This script complements validate-pades.sh. With no argument it regenerates a
# fresh PMForge timestamped sample; with an explicit PDF it validates that file
# without modification. It extracts the CMS DER and signed ByteRange bytes,
# records provenance and hashes, verifies detached CMS with OpenSSL, and runs
# locally installed deterministic PDF/PAdES validators. Acrobat still requires
# a separate manual validation environment.

set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

SAMPLE_DIR="$ROOT/.tmp/pmforge-pades-test"
PADES_LOCK="$ROOT/.tmp/pmforge-pades-test.lock"
if [ "$#" -gt 1 ]; then
	echo "usage: $0 [signed-pdf]" >&2
	exit 64
fi
if [ "$#" -eq 0 ]; then
	PDF_PATH="$SAMPLE_DIR/signed-sample.pdf"
	EVIDENCE_SOURCE="generated_current_checkout"
	EVIDENCE_GENERATOR="scripts/validate-pades.sh"
else
	# Resolve explicit input once so every validator and provenance record refers
	# to the same artifact even when the caller used a relative path.
	PDF_PATH="$(python3 - "$1" <<'PY'
import sys
from pathlib import Path

print(Path(sys.argv[1]).expanduser().resolve())
PY
)"
	EVIDENCE_SOURCE="supplied_pdf"
	EVIDENCE_GENERATOR="not_applicable"
fi
CMS_DER="$SAMPLE_DIR/signed-sample.cms.der"
SIGNED_BYTES="$SAMPLE_DIR/signed-sample.byterange.bin"
EXTRACT_INFO="$SAMPLE_DIR/signed-sample.extract.txt"
VERAPDF_XML="$SAMPLE_DIR/verapdf-signature-features.xml"
VERAPDF_ERR="$SAMPLE_DIR/verapdf-signature-features.stderr"
DSS_OUTPUT="$SAMPLE_DIR/dss-validation-output.txt"
REPORT="$SAMPLE_DIR/external-validation-report.txt"

echo "=== PAdES External Validation Harness ==="

acquire_pades_lock() {
	if [ "${PMFORGE_PADES_LOCK_HELD:-0}" = "1" ]; then
		return
	fi
	mkdir -p "$ROOT/.tmp"
	while ! mkdir "$PADES_LOCK" 2>/dev/null; do
		sleep 0.1
	done
	echo "$$" > "$PADES_LOCK/pid"
	trap 'rm -rf "$PADES_LOCK"' EXIT INT TERM
	export PMFORGE_PADES_LOCK_HELD=1
}

acquire_pades_lock

# Default validation is build evidence, so it must never inherit a non-empty
# artifact from a previous checkout or run. The external harness already owns
# the shared lock; validate-pades.sh observes PMFORGE_PADES_LOCK_HELD and
# regenerates the sample without attempting to acquire the lock recursively.
if [ "$EVIDENCE_SOURCE" = "generated_current_checkout" ]; then
	echo "Generating fresh local PAdES sample..."
	bash "$ROOT/scripts/validate-pades.sh" >/dev/null
elif [ ! -s "$PDF_PATH" ]; then
	echo "supplied PAdES PDF is missing or empty: $PDF_PATH" >&2
	exit 66
fi

# Explicit-PDF mode preserves its input but must not inherit derived evidence
# from an earlier validation. Remove only the harness-owned files while the
# lock is held, guarding the unlikely case that a caller supplied one of those
# exact paths as input.
for artifact in "$CMS_DER" "$SIGNED_BYTES" "$EXTRACT_INFO" "$VERAPDF_XML" "$VERAPDF_ERR" "$DSS_OUTPUT" "$REPORT"; do
	if [ "$artifact" != "$PDF_PATH" ]; then
		rm -f "$artifact"
	fi
done

python3 - "$PDF_PATH" "$CMS_DER" "$SIGNED_BYTES" "$EXTRACT_INFO" <<'PY'
import binascii
import hashlib
import re
import sys
from pathlib import Path

pdf_path = Path(sys.argv[1])
cms_path = Path(sys.argv[2])
signed_path = Path(sys.argv[3])
info_path = Path(sys.argv[4])

pdf = pdf_path.read_bytes()
marker = b"/ByteRange ["
idx = pdf.rfind(marker)
if idx < 0:
    raise SystemExit("PDF missing /ByteRange")

start = idx + len(marker)
end = pdf.find(b"]", start)
if end < 0:
    raise SystemExit("PDF missing /ByteRange closing bracket")

fields = pdf[start:end].split()
if len(fields) != 4:
    raise SystemExit(f"ByteRange field count = {len(fields)}, want 4")

try:
    br = [int(field) for field in fields]
except ValueError as exc:
    raise SystemExit(f"invalid ByteRange integer: {exc}") from exc

if any(value < 0 for value in br):
    raise SystemExit(f"ByteRange contains negative values: {br}")
if br[0] + br[1] > len(pdf) or br[2] + br[3] > len(pdf):
    raise SystemExit(f"ByteRange {br} extends past {len(pdf)}-byte PDF")
if br[1] >= br[2] or pdf[br[1]:br[1] + 1] != b"<" or pdf[br[2] - 1:br[2]] != b">":
    raise SystemExit(f"ByteRange does not enclose a PDF hex /Contents string: {br}")

contents_hex = re.sub(rb"\s+", b"", pdf[br[1] + 1:br[2] - 1])
try:
    contents = binascii.unhexlify(contents_hex)
except binascii.Error as exc:
    raise SystemExit(f"decode /Contents hex: {exc}") from exc

if len(contents) < 2 or contents[0] != 0x30:
    raise SystemExit("embedded CMS does not start with a DER SEQUENCE")

length_byte = contents[1]
if length_byte & 0x80:
    length_octets = length_byte & 0x7F
    if length_octets == 0:
        raise SystemExit("embedded CMS uses indefinite-length BER, not DER")
    if len(contents) < 2 + length_octets:
        raise SystemExit("embedded CMS DER length is truncated")
    body_len = int.from_bytes(contents[2:2 + length_octets], "big")
    total_len = 2 + length_octets + body_len
else:
    total_len = 2 + length_byte

if total_len > len(contents):
    raise SystemExit("embedded CMS DER length extends beyond /Contents")

padding = contents[total_len:]
if any(padding):
    raise SystemExit("non-zero data after CMS DER in padded /Contents")

cms_der = contents[:total_len]
signed = pdf[br[0]:br[0] + br[1]] + pdf[br[2]:br[2] + br[3]]

cms_path.write_bytes(cms_der)
signed_path.write_bytes(signed)
info_path.write_text(
    "\n".join([
        f"pdf={pdf_path}",
        f"pdf_bytes={len(pdf)}",
        f"pdf_sha256={hashlib.sha256(pdf).hexdigest()}",
        f"byte_range={br}",
        f"cms_der_bytes={len(cms_der)}",
        f"cms_der_sha256={hashlib.sha256(cms_der).hexdigest()}",
        f"signed_bytes={len(signed)}",
        f"signed_bytes_sha256={hashlib.sha256(signed).hexdigest()}",
    ]) + "\n",
    encoding="utf-8",
)
PY

VALIDATED_AT_UTC="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
VALIDATOR_REVISION="$(git rev-parse HEAD 2>/dev/null || echo unknown)"
VALIDATOR_SCRIPT_SHA256="$(python3 - "$ROOT/scripts/validate-pades-external.sh" <<'PY'
import hashlib
import sys
from pathlib import Path

print(hashlib.sha256(Path(sys.argv[1]).read_bytes()).hexdigest())
PY
)"
GENERATOR_SCRIPT_SHA256="not_applicable"
if [ "$EVIDENCE_SOURCE" = "generated_current_checkout" ]; then
	GENERATOR_SCRIPT_SHA256="$(python3 - "$ROOT/scripts/validate-pades.sh" <<'PY'
import hashlib
import sys
from pathlib import Path

print(hashlib.sha256(Path(sys.argv[1]).read_bytes()).hexdigest())
PY
)"
fi
VALIDATOR_WORKTREE_DIRTY="false"
if ! git diff --quiet --ignore-submodules -- 2>/dev/null ||
	! git diff --cached --quiet --ignore-submodules -- 2>/dev/null ||
	[ -n "$(git ls-files --others --exclude-standard 2>/dev/null)" ]; then
	VALIDATOR_WORKTREE_DIRTY="true"
fi

{
	echo "PDF: $PDF_PATH"
	echo "evidence_source=$EVIDENCE_SOURCE"
	echo "evidence_generator=$EVIDENCE_GENERATOR"
	echo "validator_revision=$VALIDATOR_REVISION"
	echo "validator_script_sha256=$VALIDATOR_SCRIPT_SHA256"
	echo "generator_script_sha256=$GENERATOR_SCRIPT_SHA256"
	echo "validator_worktree_dirty=$VALIDATOR_WORKTREE_DIRTY"
	echo "validated_at_utc=$VALIDATED_AT_UTC"
	cat "$EXTRACT_INFO"
	echo

	if command -v openssl >/dev/null 2>&1; then
		echo "OpenSSL: $(openssl version)"
		if openssl asn1parse -inform DER -in "$CMS_DER" -noout >/dev/null 2>&1; then
			echo "OpenSSL ASN.1 parse: PASS"
		else
			echo "OpenSSL ASN.1 parse: FAIL"
			exit 1
		fi
		if openssl cms -verify -binary -inform DER -in "$CMS_DER" -content "$SIGNED_BYTES" -noverify -out /dev/null >/dev/null 2>&1; then
			echo "OpenSSL detached CMS verification: PASS"
		else
			echo "OpenSSL detached CMS verification: FAIL"
			exit 1
		fi
	else
		echo "OpenSSL detached CMS verification: SKIP (openssl not installed)"
	fi

	echo
	if command -v qpdf >/dev/null 2>&1; then
		if qpdf --check "$PDF_PATH" >/dev/null 2>&1; then
			echo "qpdf syntax check: PASS"
		else
			echo "qpdf syntax check: FAIL"
			exit 1
		fi
	else
		echo "qpdf syntax check: SKIP (qpdf not installed)"
	fi

	if command -v pdfsig >/dev/null 2>&1; then
		echo "pdfsig output:"
		PDFSIG_OUTPUT="$(mktemp "$SAMPLE_DIR/pdfsig-output.XXXXXX")"
		PDFSIG_NSS_DIR=""
		if command -v certutil >/dev/null 2>&1; then
			PDFSIG_NSS_DIR="$(mktemp -d "$SAMPLE_DIR/pdfsig-nss.XXXXXX")"
			if ! certutil -N -d "sql:$PDFSIG_NSS_DIR" --empty-password >/dev/null 2>&1; then
				rm -rf "$PDFSIG_NSS_DIR"
				PDFSIG_NSS_DIR=""
			fi
		fi
		if [ -n "$PDFSIG_NSS_DIR" ]; then
			pdfsig -nssdir "$PDFSIG_NSS_DIR" "$PDF_PATH" >"$PDFSIG_OUTPUT" 2>&1 || true
		else
			pdfsig "$PDF_PATH" >"$PDFSIG_OUTPUT" 2>&1 || true
		fi
		cat "$PDFSIG_OUTPUT"
		if grep -q "Signature Validation: Signature is Valid" "$PDFSIG_OUTPUT"; then
			echo "pdfsig signature validation: PASS"
		else
			echo "pdfsig signature validation: FAIL"
			rm -f "$PDFSIG_OUTPUT"
			if [ -n "$PDFSIG_NSS_DIR" ]; then
				rm -rf "$PDFSIG_NSS_DIR"
			fi
			exit 1
		fi
		rm -f "$PDFSIG_OUTPUT"
		if [ -n "$PDFSIG_NSS_DIR" ]; then
			rm -rf "$PDFSIG_NSS_DIR"
		fi
	else
		echo "pdfsig signature validation: SKIP (pdfsig not installed)"
	fi

	if command -v verapdf >/dev/null 2>&1; then
		echo "veraPDF CLI: available ($(verapdf --version 2>/dev/null | head -1 || true))"
		if verapdf --off --extract signature --format xml "$PDF_PATH" >"$VERAPDF_XML" 2>"$VERAPDF_ERR"; then
			if python3 - "$VERAPDF_XML" <<'PY'
import sys
import xml.etree.ElementTree as ET
from pathlib import Path

xml_path = Path(sys.argv[1])
root = ET.parse(xml_path).getroot()
summary = root.find(".//batchSummary")
if summary is not None:
    for attr in ("failedToParse", "encrypted", "outOfMemory", "veraExceptions"):
        if summary.attrib.get(attr, "0") != "0":
            raise SystemExit(f"veraPDF batch summary {attr}={summary.attrib.get(attr)}")
feature_reports = root.find(".//featureReports")
if feature_reports is not None and feature_reports.attrib.get("failedJobs", "0") != "0":
    raise SystemExit(f"veraPDF feature extraction failedJobs={feature_reports.attrib.get('failedJobs')}")
matches = []
for sig in root.findall(".//signature"):
    filter_text = (sig.findtext("filter") or "").strip()
    sub_filter = (sig.findtext("subFilter") or "").strip()
    if filter_text == "Adobe.PPKLite" and sub_filter == "ETSI.CAdES.detached":
        matches.append(sig)
if not matches:
    raise SystemExit("veraPDF did not extract the expected PAdES signature metadata")
PY
			then
				echo "veraPDF signature feature extraction: PASS"
				echo "veraPDF signature feature report: $VERAPDF_XML"
				if [ -s "$VERAPDF_ERR" ]; then
					echo "veraPDF stderr: $VERAPDF_ERR"
				fi
			else
				echo "veraPDF signature feature extraction: FAIL"
				exit 1
			fi
		else
			echo "veraPDF signature feature extraction: FAIL"
			if [ -s "$VERAPDF_ERR" ]; then
				cat "$VERAPDF_ERR"
			fi
			exit 1
		fi
	else
		echo "veraPDF CLI: SKIP (verapdf not installed)"
	fi

	if command -v dss-validation-tool >/dev/null 2>&1; then
		echo "DSS validation tool: available"
		if dss-validation-tool validate "$PDF_PATH" >"$DSS_OUTPUT" 2>&1; then
			echo "DSS validation: PASS"
			echo "DSS validation report: $DSS_OUTPUT"
			cat "$DSS_OUTPUT"
			if grep -q "PAdESBaselineRequirementsChecker" "$DSS_OUTPUT"; then
				echo "DSS PAdES baseline requirements: FAIL"
				exit 1
			fi
			if grep -q "^signature.format=" "$DSS_OUTPUT"; then
				if grep -q "^signature.format=PAdES-BASELINE-T$" "$DSS_OUTPUT"; then
					echo "DSS PAdES baseline format: PASS"
				else
					echo "DSS PAdES baseline format: FAIL"
					exit 1
				fi
			fi
		else
			echo "DSS validation: FAIL"
			if [ -s "$DSS_OUTPUT" ]; then
				cat "$DSS_OUTPUT"
			fi
			exit 1
		fi
	else
		echo "DSS validation tool: SKIP (dss-validation-tool not installed)"
	fi

	echo
	echo "External validation artifacts:"
	echo "  $PDF_PATH"
	echo "  $CMS_DER"
	echo "  $SIGNED_BYTES"
	if [ -s "$DSS_OUTPUT" ]; then
		echo "  $DSS_OUTPUT"
	fi
	echo "  $REPORT"
} | tee "$REPORT"

echo "PAdES external validation harness completed."
