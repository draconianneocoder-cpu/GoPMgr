#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Trusted-source PAdES validation harness.
#
# The deterministic release gate uses a self-signed sample, so it can prove
# GoPMgr's PAdES structure but not release-certificate trust. This script
# records evidence for a separately supplied signed PDF created with a trusted
# certificate. When no trusted source is configured it writes an explicit
# NOT_CONFIGURED report instead of implying validation passed. Acrobat evidence
# remains a separate manual release artifact because CLI trust and Acrobat trust
# can use different certificate stores and validation policies.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

OUT_DIR="$ROOT/.tmp/gopmgr-pades-trusted-source"
REPORT="$OUT_DIR/trusted-source-validation-report.txt"
REPORT_TMP="$OUT_DIR/trusted-source-validation-report.tmp"
PDFSIG_OUTPUT="$OUT_DIR/pdfsig-output.txt"
VERAPDF_XML="$OUT_DIR/verapdf-signature-features.xml"
VERAPDF_ERR="$OUT_DIR/verapdf.stderr"
TRUSTED_LOCK="$ROOT/.tmp/gopmgr-pades-trusted-source.lock"
PDF_PATH="${GOPMGR_TRUSTED_SIGNED_PDF:-${1:-}}"
REQUIRED="${GOPMGR_PADES_TRUSTED_REQUIRED:-0}"
LOCK_OWNED=false
source "$ROOT/scripts/pades-lock.sh"

if [ "$#" -gt 1 ]; then
	echo "Usage: $0 [trusted-signed.pdf]" >&2
	exit 64
fi

case "$REQUIRED" in
0 | 1) ;;
*)
	echo "GOPMGR_PADES_TRUSTED_REQUIRED must be 0 or 1." >&2
	exit 64
	;;
esac

cleanup() {
	rm -f "$REPORT_TMP"
	if [ "$LOCK_OWNED" = true ]; then
		rm -rf "$TRUSTED_LOCK"
	fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

acquire_trusted_lock() {
	# The test harness deliberately holds this lock across multiple child
	# invocations so every assertion observes one uninterrupted evidence set.
	if [ "${GOPMGR_PADES_TRUSTED_LOCK_HELD:-0}" = "1" ]; then
		return
	fi

	pades_acquire_directory_lock "$TRUSTED_LOCK" "${GOPMGR_PADES_TRUSTED_LOCK_TIMEOUT_SECONDS:-30}"
	LOCK_OWNED=true
	export GOPMGR_PADES_TRUSTED_LOCK_HELD=1
}

file_sha256() {
	python3 - "$1" <<'PY'
import hashlib
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
digest = hashlib.sha256()
with path.open("rb") as source:
    for chunk in iter(lambda: source.read(1024 * 1024), b""):
        digest.update(chunk)
print(digest.hexdigest())
PY
}

normalize_path() {
	python3 - "$1" <<'PY'
import pathlib
import sys

print(pathlib.Path(sys.argv[1]).expanduser().resolve())
PY
}

validator_worktree_dirty() {
	if ! git diff --quiet --ignore-submodules -- ||
		! git diff --cached --quiet --ignore-submodules -- ||
		[ -n "$(git ls-files --others --exclude-standard)" ]; then
		echo "true"
	else
		echo "false"
	fi
}

write_validator_provenance() {
	echo "validated_at_utc=$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
	echo "validator_revision=$(git rev-parse HEAD 2>/dev/null || echo unknown)"
	echo "validator_script_sha256=$(file_sha256 "$ROOT/scripts/validate-pades-trusted-source.sh")"
	echo "validator_worktree_dirty=$(validator_worktree_dirty)"
}

publish_report() {
	mv "$REPORT_TMP" "$REPORT"
}

if [ -n "$PDF_PATH" ]; then
	PDF_PATH="$(normalize_path "$PDF_PATH")"
	for artifact in "$REPORT" "$REPORT_TMP" "$PDFSIG_OUTPUT" "$VERAPDF_XML" "$VERAPDF_ERR"; do
		if [ "$PDF_PATH" = "$artifact" ]; then
			echo "Trusted signed PDF must not be a harness-owned evidence path: $PDF_PATH" >&2
			exit 64
		fi
	done
fi

acquire_trusted_lock
mkdir -p "$OUT_DIR"

# Derived files must describe only the current invocation. Removing this known
# set under the lock prevents stale output from being mistaken for fresh trust
# evidence when a source is absent or an optional validator is unavailable.
for artifact in "$REPORT" "$REPORT_TMP" "$PDFSIG_OUTPUT" "$VERAPDF_XML" "$VERAPDF_ERR"; do
	if [ -z "$PDF_PATH" ] || [ "$artifact" != "$PDF_PATH" ]; then
		rm -f "$artifact"
	fi
done

not_configured() {
	{
		echo "status=NOT_CONFIGURED"
		echo "required_trust=$REQUIRED"
		write_validator_provenance
		echo "reason=GOPMGR_TRUSTED_SIGNED_PDF is not set and no PDF path argument was supplied."
		echo "next_step=Export a PDF signed with a trusted certificate, then run:"
		echo "next_step_command=GOPMGR_TRUSTED_SIGNED_PDF=/path/to/trusted-signed.pdf make check-pades-trusted"
		echo "note=This is not a passing trust-chain validation result."
	} >"$REPORT_TMP"
	publish_report
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
		echo "status=INPUT_INVALID"
		echo "required_trust=$REQUIRED"
		write_validator_provenance
		echo "reason=trusted signed PDF does not exist or is empty: $PDF_PATH"
	} >"$REPORT_TMP"
	publish_report
	cat "$REPORT" >&2
	exit 1
fi

PDF_SHA256="$(file_sha256 "$PDF_PATH")"
validation_failed=0
validation_incomplete=0
trust_verified=0
status="VALIDATION_FAILED"

{
	echo "pdf=$PDF_PATH"
	echo "pdf_sha256=$PDF_SHA256"
	echo "required_trust=$REQUIRED"
	echo "trust_scope=local_cli_certificate_store"
	write_validator_provenance
	echo

	if command -v qpdf >/dev/null 2>&1; then
		if qpdf --check "$PDF_PATH" >/dev/null 2>&1; then
			echo "qpdf syntax check=PASS"
		else
			echo "qpdf syntax check=FAIL"
			validation_failed=1
		fi
	else
		echo "qpdf syntax check=SKIP (qpdf not installed)"
		validation_incomplete=1
	fi

	if command -v pdfsig >/dev/null 2>&1; then
		pdfsig "$PDF_PATH" >"$PDFSIG_OUTPUT" 2>&1 || true
		echo "pdfsig output=$PDFSIG_OUTPUT"
		if grep -q "Signature Validation: Signature is Valid" "$PDFSIG_OUTPUT"; then
			echo "pdfsig signature validation=PASS"
		else
			echo "pdfsig signature validation=FAIL"
			validation_failed=1
		fi
		if grep -Eq "Certificate is Trusted|Certificate Validation: Certificate is Trusted|Trusted: yes" "$PDFSIG_OUTPUT"; then
			echo "pdfsig trust-chain validation=PASS"
			trust_verified=1
		else
			echo "pdfsig trust-chain validation=INDETERMINATE"
			echo "trust_note=CLI trust output did not prove a trusted certificate chain; capture Acrobat evidence separately."
		fi
	else
		echo "pdfsig signature validation=SKIP (pdfsig not installed)"
		echo "pdfsig trust-chain validation=SKIP (pdfsig not installed)"
		validation_incomplete=1
	fi

	if command -v verapdf >/dev/null 2>&1; then
		if verapdf --off --extract signature --format xml "$PDF_PATH" >"$VERAPDF_XML" 2>"$OUT_DIR/verapdf.stderr"; then
			if python3 - "$VERAPDF_XML" <<'PY'
import sys
import xml.etree.ElementTree as ET

root = ET.parse(sys.argv[1]).getroot()
batch = root.find(".//{*}batchSummary")
if batch is None:
    raise SystemExit("veraPDF XML is missing batchSummary")
for attribute in ("failedToParse", "encrypted", "outOfMemory", "veraExceptions"):
    if int(batch.attrib.get(attribute, "0")) != 0:
        raise SystemExit(f"veraPDF batchSummary reports {attribute}")
features = root.find(".//{*}featureReports")
if features is None or int(features.attrib.get("failedJobs", "0")) != 0:
    raise SystemExit("veraPDF signature feature extraction reports failed jobs")
matches = []
for signature in root.findall(".//{*}signature"):
    filter_text = (signature.findtext("{*}filter") or "").strip()
    sub_filter = (signature.findtext("{*}subFilter") or "").strip()
    if filter_text == "Adobe.PPKLite" and sub_filter == "ETSI.CAdES.detached":
        matches.append(signature)
if not matches:
    raise SystemExit("expected PAdES signature filter/subfilter was not extracted")
PY
			then
				echo "veraPDF signature feature extraction=PASS"
				echo "veraPDF feature artifact=$VERAPDF_XML"
			else
				echo "veraPDF signature feature extraction=FAIL"
				validation_failed=1
			fi
		else
			echo "veraPDF signature feature extraction=FAIL"
			validation_failed=1
		fi
	else
		echo "veraPDF signature feature extraction=SKIP (verapdf not installed)"
	fi

	echo
	echo "acrobat_evidence=REQUIRED_MANUAL_CAPTURE"
	echo "acrobat_next_step=Open the signed PDF in Acrobat, verify the signature panel shows a trusted chain, and archive a screenshot or PDF validation report with this file."

	if [ "$validation_failed" -ne 0 ]; then
		status="VALIDATION_FAILED"
	elif [ "$validation_incomplete" -ne 0 ]; then
		status="VALIDATION_INCOMPLETE"
	elif [ "$trust_verified" -eq 1 ]; then
		status="TRUST_VERIFIED"
	else
		status="STRUCTURE_VALID_TRUST_INDETERMINATE"
	fi
	echo "status=$status"
} >"$REPORT_TMP"
publish_report

case "$status" in
TRUST_VERIFIED)
	echo "Trusted-source PAdES trust verified with the local CLI certificate store. Report: $REPORT"
	;;
STRUCTURE_VALID_TRUST_INDETERMINATE)
	echo "Trusted-source PAdES structure is valid, but CLI trust is indeterminate. Report: $REPORT"
	if [ "$REQUIRED" = "1" ]; then
		exit 1
	fi
	;;
*)
	cat "$REPORT" >&2
	exit 1
	;;
esac
