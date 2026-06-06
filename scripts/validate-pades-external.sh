#!/bin/bash
# SPDX-FileCopyrightText: 2026 The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# External PAdES validation harness.
#
# This script complements validate-pades.sh. It generates the PMForge signed
# sample, extracts the CMS DER and signed ByteRange bytes, verifies the detached
# CMS with OpenSSL, and records which higher-level PDF/PAdES validators are
# available locally. Acrobat/DSS/veraPDF interoperability still needs a machine
# with those validators installed.

set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

SAMPLE_DIR="$ROOT/.tmp/pmforge-pades-test"
PDF_PATH="${1:-$SAMPLE_DIR/signed-sample.pdf}"
CMS_DER="$SAMPLE_DIR/signed-sample.cms.der"
SIGNED_BYTES="$SAMPLE_DIR/signed-sample.byterange.bin"
EXTRACT_INFO="$SAMPLE_DIR/signed-sample.extract.txt"
REPORT="$SAMPLE_DIR/external-validation-report.txt"

echo "=== PAdES External Validation Harness ==="

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
		echo "veraPDF PAdES interoperability: TODO run manually with the signed sample and record results"
	else
		echo "veraPDF CLI: SKIP (verapdf not installed)"
	fi

	if command -v dss-validation-tool >/dev/null 2>&1; then
		echo "DSS validation tool: available"
		echo "DSS PAdES interoperability: TODO run manually with the signed sample and record results"
	else
		echo "DSS validation tool: SKIP (dss-validation-tool not installed)"
	fi

	echo
	echo "External validation artifacts:"
	echo "  $PDF_PATH"
	echo "  $CMS_DER"
	echo "  $SIGNED_BYTES"
	echo "  $REPORT"
} | tee "$REPORT"

echo "PAdES external validation harness completed."
