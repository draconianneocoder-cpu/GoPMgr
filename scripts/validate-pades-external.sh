#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# External PAdES validation harness.
#
# This script complements validate-pades.sh. It generates the PMForge signed
# sample, extracts the CMS DER and signed ByteRange bytes, verifies the detached
# CMS with OpenSSL, and runs locally installed PDF/PAdES validators where their
# command-line checks are deterministic. Acrobat and DSS validation still need a
# machine with those validators installed.

set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

SAMPLE_DIR="$ROOT/.tmp/pmforge-pades-test"
PADES_LOCK="$ROOT/.tmp/pmforge-pades-test.lock"
PDF_PATH="${1:-$SAMPLE_DIR/signed-sample.pdf}"
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

if [ ! -s "$PDF_PATH" ]; then
	echo "Generating local PAdES sample first..."
	bash "$ROOT/scripts/validate-pades.sh" >/dev/null
fi

python3 - "$PDF_PATH" "$CMS_DER" "$SIGNED_BYTES" "$EXTRACT_INFO" <<'PY'
import binascii
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
        f"byte_range={br}",
        f"cms_der_bytes={len(cms_der)}",
        f"signed_bytes={len(signed)}",
    ]) + "\n",
    encoding="utf-8",
)
PY

{
	echo "PDF: $PDF_PATH"
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
				if grep -q "^signature.format=PAdES-BASELINE-B$" "$DSS_OUTPUT"; then
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
	echo "  $DSS_OUTPUT"
	echo "  $REPORT"
} | tee "$REPORT"

echo "PAdES external validation harness completed."
