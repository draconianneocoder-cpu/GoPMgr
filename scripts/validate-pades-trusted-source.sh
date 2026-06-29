#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Manual trusted-source PAdES validation harness.
#
# The deterministic release gate uses a self-signed sample, so it can prove
# PMForge's PAdES structure but not release-certificate trust. This script
# records evidence for a separately supplied signed PDF created with a trusted
# certificate. When no trusted source is configured it writes an explicit
# NOT CONFIGURED report instead of implying validation passed.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

OUT_DIR="$ROOT/.tmp/pmforge-pades-trusted-source"
REPORT="$OUT_DIR/trusted-source-validation-report.txt"
PDF_PATH="${PMFORGE_TRUSTED_SIGNED_PDF:-${1:-}}"
REQUIRED="${PMFORGE_PADES_TRUSTED_REQUIRED:-0}"

mkdir -p "$OUT_DIR"

not_configured() {
	{
		echo "status=NOT_CONFIGURED"
		echo "reason=PMFORGE_TRUSTED_SIGNED_PDF is not set and no PDF path argument was supplied."
		echo "next_step=Export a PDF signed with a trusted certificate, then run:"
		echo "next_step_command=PMFORGE_TRUSTED_SIGNED_PDF=/path/to/trusted-signed.pdf make check-pades-trusted"
		echo "note=This is not a passing trust-chain validation result."
	} >"$REPORT"
	echo "Trusted-source PAdES validation not configured. Report: $REPORT"
	if [ "$REQUIRED" = "1" ]; then
		exit 1
	fi
	exit 0
}

if [ -z "$PDF_PATH" ]; then
	not_configured
fi

if [ ! -s "$PDF_PATH" ]; then
	{
		echo "status=FAIL"
		echo "reason=trusted signed PDF does not exist or is empty: $PDF_PATH"
	} >"$REPORT"
	cat "$REPORT" >&2
	exit 1
fi

status="PASS"
{
	echo "status=STARTED"
	echo "pdf=$PDF_PATH"
	echo

	if command -v qpdf >/dev/null 2>&1; then
		if qpdf --check "$PDF_PATH" >/dev/null 2>&1; then
			echo "qpdf syntax check=PASS"
		else
			echo "qpdf syntax check=FAIL"
			status="FAIL"
		fi
	else
		echo "qpdf syntax check=SKIP (qpdf not installed)"
	fi

	if command -v pdfsig >/dev/null 2>&1; then
		PDFSIG_OUTPUT="$OUT_DIR/pdfsig-output.txt"
		pdfsig "$PDF_PATH" >"$PDFSIG_OUTPUT" 2>&1 || true
		echo "pdfsig output=$PDFSIG_OUTPUT"
		if grep -q "Signature Validation: Signature is Valid" "$PDFSIG_OUTPUT"; then
			echo "pdfsig signature validation=PASS"
		else
			echo "pdfsig signature validation=FAIL"
			status="FAIL"
		fi
		if grep -Eq "Certificate is Trusted|Certificate Validation: Certificate is Trusted|Trusted: yes" "$PDFSIG_OUTPUT"; then
			echo "pdfsig trust-chain validation=PASS"
		else
			echo "pdfsig trust-chain validation=INDETERMINATE"
			echo "trust_note=CLI trust output did not prove a trusted certificate chain; capture Acrobat evidence separately."
		fi
	else
		echo "pdfsig signature validation=SKIP (pdfsig not installed)"
		echo "pdfsig trust-chain validation=SKIP (pdfsig not installed)"
	fi

	if command -v verapdf >/dev/null 2>&1; then
		VERAPDF_XML="$OUT_DIR/verapdf-signature-features.xml"
		if verapdf --off --extract signature --format xml "$PDF_PATH" >"$VERAPDF_XML" 2>"$OUT_DIR/verapdf.stderr"; then
			echo "veraPDF signature feature extraction=PASS"
			echo "veraPDF feature artifact=$VERAPDF_XML"
		else
			echo "veraPDF signature feature extraction=FAIL"
			status="FAIL"
		fi
	else
		echo "veraPDF signature feature extraction=SKIP (verapdf not installed)"
	fi

	echo
	echo "acrobat_evidence=REQUIRED_MANUAL_CAPTURE"
	echo "acrobat_next_step=Open the signed PDF in Acrobat, verify the signature panel shows a trusted chain, and archive a screenshot or PDF validation report with this file."
	echo "status=$status"
} >"$REPORT"

if grep -q "status=FAIL" "$REPORT"; then
	cat "$REPORT" >&2
	exit 1
fi

echo "Trusted-source PAdES validation report: $REPORT"
