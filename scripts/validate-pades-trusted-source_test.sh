#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT_DIR="$ROOT/.tmp/pmforge-pades-trusted-source"
REPORT="$OUT_DIR/trusted-source-validation-report.txt"
TRUSTED_LOCK="$ROOT/.tmp/pmforge-pades-trusted-source.lock"
FAKE_BIN="$ROOT/.tmp/pades-trusted-source-bin-test"
PDF_PATH="$FAKE_BIN/trusted-input.pdf"
PDF_BEFORE="$FAKE_BIN/trusted-input.before.pdf"
LOCK_OWNED="false"

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

cleanup() {
	rm -rf "$FAKE_BIN"
	if [ "$LOCK_OWNED" = "true" ]; then
		rm -rf "$TRUSTED_LOCK"
	fi
}
trap cleanup EXIT

# Hold the harness lock through every assertion so another validation cannot
# replace the fixed report path between a child exit and this test reading it.
mkdir -p "$ROOT/.tmp"
while ! mkdir "$TRUSTED_LOCK" 2>/dev/null; do
	sleep 0.1
done
LOCK_OWNED="true"
echo "$$" >"$TRUSTED_LOCK/pid"
export PMFORGE_PADES_TRUSTED_LOCK_HELD=1

rm -rf "$OUT_DIR" "$FAKE_BIN"
mkdir -p "$OUT_DIR" "$FAKE_BIN"
printf 'stale pdfsig output\n' >"$OUT_DIR/pdfsig-output.txt"
printf 'stale veraPDF output\n' >"$OUT_DIR/verapdf-signature-features.xml"

cat >"$FAKE_BIN/qpdf" <<'EOF'
#!/bin/bash
if [ "$1" != "--check" ] || [ ! -s "$2" ]; then
	exit 64
fi
if [ "${PMFORGE_FAKE_QPDF_MODE:-valid}" = "invalid" ]; then
	exit 1
fi
EOF
chmod +x "$FAKE_BIN/qpdf"

cat >"$FAKE_BIN/pdfsig" <<'EOF'
#!/bin/bash
case "${PMFORGE_FAKE_PDFSIG_MODE:-trusted}" in
	trusted)
		echo "Signature Validation: Signature is Valid."
		echo "Certificate Validation: Certificate is Trusted"
		;;
	indeterminate)
		echo "Signature Validation: Signature is Valid."
		echo "Certificate Validation: Certificate issuer is unknown."
		;;
	invalid)
		echo "Signature Validation: Signature is Invalid."
		echo "Certificate Validation: Certificate issuer is unknown."
		;;
esac
EOF
chmod +x "$FAKE_BIN/pdfsig"

cat >"$FAKE_BIN/verapdf" <<'EOF'
#!/bin/bash
if [ "${PMFORGE_FAKE_VERAPDF_MODE:-valid}" = "invalid" ]; then
	echo "<report><featureReports failedJobs=\"1\"/></report>"
	exit 0
fi
cat <<'XML'
<?xml version="1.0" encoding="utf-8"?>
<report>
  <jobs>
    <job>
      <featuresReport>
        <signatures>
          <signature>
            <filter>Adobe.PPKLite</filter>
            <subFilter>ETSI.CAdES.detached</subFilter>
          </signature>
        </signatures>
      </featuresReport>
    </job>
  </jobs>
  <batchSummary failedToParse="0" encrypted="0" outOfMemory="0" veraExceptions="0">
    <featureReports failedJobs="0">1</featureReports>
  </batchSummary>
</report>
XML
EOF
chmod +x "$FAKE_BIN/verapdf"

unset PMFORGE_TRUSTED_SIGNED_PDF PMFORGE_PADES_TRUSTED_REQUIRED
bash "$ROOT/scripts/validate-pades-trusted-source.sh" >"$FAKE_BIN/not-configured.out"

[ -s "$REPORT" ] || fail "trusted-source report was not written"
if ! grep -q "^status=NOT_CONFIGURED$" "$REPORT"; then
	cat "$REPORT" >&2
	fail "missing NOT_CONFIGURED status"
fi
if grep -Eq "^status=(PASS|TRUST_VERIFIED)$" "$REPORT"; then
	cat "$REPORT" >&2
	fail "unconfigured trusted-source validation claimed verified trust"
fi
if [ -e "$OUT_DIR/pdfsig-output.txt" ] || [ -e "$OUT_DIR/verapdf-signature-features.xml" ]; then
	fail "not-configured validation retained stale derived evidence"
fi

if PMFORGE_PADES_TRUSTED_REQUIRED=1 bash "$ROOT/scripts/validate-pades-trusted-source.sh" >"$FAKE_BIN/required-not-configured.out" 2>&1; then
	fail "required trusted-source validation passed without a configured PDF"
fi
if ! grep -q "^status=NOT_CONFIGURED$" "$REPORT"; then
	cat "$REPORT" >&2
	fail "required-mode report did not preserve NOT_CONFIGURED status"
fi

printf 'deterministic signed PDF fixture\n' >"$PDF_PATH"
cp "$PDF_PATH" "$PDF_BEFORE"
PATH="$FAKE_BIN:$PATH" PMFORGE_FAKE_PDFSIG_MODE=trusted \
	bash "$ROOT/scripts/validate-pades-trusted-source.sh" "$PDF_PATH" >"$FAKE_BIN/trusted.out"

if ! grep -q "^status=TRUST_VERIFIED$" "$REPORT"; then
	cat "$REPORT" >&2
	fail "trusted certificate output did not produce TRUST_VERIFIED"
fi
if ! grep -q "^pdfsig trust-chain validation=PASS$" "$REPORT"; then
	cat "$REPORT" >&2
	fail "trusted certificate output was not recorded"
fi
if ! grep -Eq "^pdf_sha256=[0-9a-f]{64}$" "$REPORT" ||
	! grep -Eq "^validator_script_sha256=[0-9a-f]{64}$" "$REPORT" ||
	! grep -Eq "^validator_revision=[0-9a-f]+$" "$REPORT"; then
	cat "$REPORT" >&2
	fail "trusted-source report is missing provenance hashes"
fi
if ! cmp -s "$PDF_BEFORE" "$PDF_PATH"; then
	fail "trusted-source validation modified the supplied PDF"
fi

PATH="$FAKE_BIN:$PATH" PMFORGE_FAKE_PDFSIG_MODE=indeterminate \
	bash "$ROOT/scripts/validate-pades-trusted-source.sh" "$PDF_PATH" >"$FAKE_BIN/indeterminate.out"
if ! grep -q "^status=STRUCTURE_VALID_TRUST_INDETERMINATE$" "$REPORT"; then
	cat "$REPORT" >&2
	fail "untrusted certificate output did not preserve indeterminate trust"
fi
if grep -q "^status=PASS$" "$REPORT"; then
	cat "$REPORT" >&2
	fail "indeterminate trust retained the ambiguous PASS status"
fi

if PATH="$FAKE_BIN:$PATH" PMFORGE_FAKE_PDFSIG_MODE=indeterminate PMFORGE_PADES_TRUSTED_REQUIRED=1 \
	bash "$ROOT/scripts/validate-pades-trusted-source.sh" "$PDF_PATH" >"$FAKE_BIN/required-indeterminate.out" 2>&1; then
	fail "required trusted-source validation passed with indeterminate trust"
fi
if ! grep -q "^status=STRUCTURE_VALID_TRUST_INDETERMINATE$" "$REPORT"; then
	cat "$REPORT" >&2
	fail "required indeterminate report lost its explicit outcome"
fi

if PATH="$FAKE_BIN:$PATH" PMFORGE_FAKE_PDFSIG_MODE=invalid \
	bash "$ROOT/scripts/validate-pades-trusted-source.sh" "$PDF_PATH" >"$FAKE_BIN/invalid.out" 2>&1; then
	fail "trusted-source validation passed an invalid signature"
fi
if ! grep -q "^status=VALIDATION_FAILED$" "$REPORT"; then
	cat "$REPORT" >&2
	fail "invalid signature did not produce VALIDATION_FAILED"
fi

missing_pdf="$FAKE_BIN/missing.pdf"
if PATH="$FAKE_BIN:$PATH" bash "$ROOT/scripts/validate-pades-trusted-source.sh" "$missing_pdf" >"$FAKE_BIN/missing.out" 2>&1; then
	fail "trusted-source validation passed a missing PDF"
fi
if ! grep -q "^status=INPUT_INVALID$" "$REPORT"; then
	cat "$REPORT" >&2
	fail "missing PDF did not produce INPUT_INVALID"
fi

echo "validate-pades-trusted-source tests passed."
