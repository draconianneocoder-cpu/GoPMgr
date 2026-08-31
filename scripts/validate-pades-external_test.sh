#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SAMPLE_DIR="$ROOT/.tmp/gopmgr-pades-test"
PADES_LOCK="$ROOT/.tmp/gopmgr-pades-test.lock"
FAKE_BIN=""
FAKE_LOG="$FAKE_BIN/verapdf.args"
DSS_LOG="$FAKE_BIN/dss-validation-tool.args"
LOCK_OWNED="false"
source "$ROOT/scripts/pades-lock.sh"

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

cleanup() {
	[ -z "$FAKE_BIN" ] || rm -rf "$FAKE_BIN"
	if [ "$LOCK_OWNED" = "true" ]; then
		rm -rf "$PADES_LOCK"
	fi
}
trap cleanup EXIT

# Keep setup, both child validations, and report assertions in one critical
# section. Otherwise another test can replace the fake tools, or a waiting
# generator can remove SAMPLE_DIR before this test reads its evidence report.
pades_acquire_directory_lock "$PADES_LOCK" "${GOPMGR_PADES_LOCK_TIMEOUT_SECONDS:-30}"
LOCK_OWNED="true"
export GOPMGR_PADES_LOCK_HELD=1

FAKE_BIN="$(mktemp -d "$ROOT/.tmp/pades-external-bin-test.XXXXXX")"
FAKE_LOG="$FAKE_BIN/verapdf.args"
DSS_LOG="$FAKE_BIN/dss-validation-tool.args"
mkdir -p "$FAKE_BIN"
cat > "$FAKE_BIN/verapdf" <<'EOF'
#!/bin/bash
printf '%s\n' "$*" >> "$GOPMGR_FAKE_VERAPDF_LOG"
case "$1" in
	--version)
		echo "veraPDF fake 1.0.0"
		exit 0
		;;
esac
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

cat > "$FAKE_BIN/dss-validation-tool" <<'EOF'
#!/bin/bash
printf '%s\n' "$*" >> "$GOPMGR_FAKE_DSS_LOG"
if [ "$1" != "validate" ]; then
	echo "unexpected dss command: $*" >&2
	exit 64
fi
if [ ! -s "$2" ]; then
	echo "missing signed sample: $2" >&2
	exit 66
fi
echo "DSS 6.4 local validation wrapper"
echo "signatures=1"
echo "signature.format=PAdES-BASELINE-T"
echo "signature.indication=INDETERMINATE"
echo "signature.sub_indication=NO_CERTIFICATE_CHAIN_FOUND"
EOF
chmod +x "$FAKE_BIN/dss-validation-tool"

mkdir -p "$SAMPLE_DIR"
printf 'stale default artifact\n' >"$SAMPLE_DIR/signed-sample.pdf"
GOPMGR_FAKE_VERAPDF_LOG="$FAKE_LOG" GOPMGR_FAKE_DSS_LOG="$DSS_LOG" PATH="$FAKE_BIN:$PATH" \
	bash "$ROOT/scripts/validate-pades-external.sh" >"$FAKE_BIN/default.out"

report="$SAMPLE_DIR/external-validation-report.txt"
[ -s "$report" ] || fail "external validation report was not written"

if grep -q "stale default artifact" "$SAMPLE_DIR/signed-sample.pdf"; then
	fail "default external validation reused a stale signed sample"
fi
if ! grep -q "^evidence_source=generated_current_checkout$" "$report"; then
	cat "$report" >&2
	fail "default external validation did not identify generated evidence"
fi
if ! grep -Eq "^pdf_sha256=[0-9a-f]{64}$" "$report"; then
	cat "$report" >&2
	fail "external validation report did not bind the generated PDF hash"
fi
if ! grep -Eq "^validator_revision=[0-9a-f]+$" "$report"; then
	cat "$report" >&2
	fail "external validation report did not identify the validator revision"
fi
if ! grep -Eq "^validator_script_sha256=[0-9a-f]{64}$" "$report"; then
	cat "$report" >&2
	fail "external validation report did not bind the validator script hash"
fi
if ! grep -Eq "^generator_script_sha256=[0-9a-f]{64}$" "$report"; then
	cat "$report" >&2
	fail "external validation report did not bind the generator script hash"
fi

if ! grep -q "veraPDF signature feature extraction: PASS" "$report"; then
	cat "$report" >&2
	fail "veraPDF extraction was not recorded as a pass"
fi

if grep -q "veraPDF PAdES interoperability: TODO" "$report"; then
	cat "$report" >&2
	fail "veraPDF branch still reports a manual TODO"
fi

if ! grep -q -- "--off --extract signature --format xml" "$FAKE_LOG"; then
	cat "$FAKE_LOG" >&2
	fail "veraPDF was not invoked with signature feature extraction"
fi

if ! grep -q "DSS validation: PASS" "$report"; then
	cat "$report" >&2
	fail "DSS validation was not recorded as a pass"
fi

if ! grep -q "DSS PAdES baseline format: PASS" "$report"; then
	cat "$report" >&2
	fail "DSS baseline format was not enforced"
fi

if ! grep -q "signature.format=PAdES-BASELINE-T" "$report"; then
	cat "$report" >&2
	fail "DSS did not classify the timestamped fixture as PAdES Baseline T"
fi

if grep -q "DSS PAdES interoperability: TODO" "$report"; then
	cat "$report" >&2
	fail "DSS branch still reports a manual TODO"
fi

if ! grep -Fqx "validate $SAMPLE_DIR/signed-sample.pdf" "$DSS_LOG"; then
	cat "$DSS_LOG" >&2
	fail "DSS validation tool was not invoked against the signed sample"
fi

explicit_pdf="$FAKE_BIN/explicit-sample.pdf"
explicit_before="$FAKE_BIN/explicit-sample.before.pdf"
cp "$SAMPLE_DIR/signed-sample.pdf" "$explicit_pdf"
cp "$explicit_pdf" "$explicit_before"
GOPMGR_FAKE_VERAPDF_LOG="$FAKE_LOG" GOPMGR_FAKE_DSS_LOG="$DSS_LOG" PATH="$FAKE_BIN:$PATH" \
	bash "$ROOT/scripts/validate-pades-external.sh" "$explicit_pdf" >"$FAKE_BIN/explicit.out"

if ! cmp -s "$explicit_before" "$explicit_pdf"; then
	fail "external validation modified the explicitly supplied PDF"
fi
if ! grep -q "^evidence_source=supplied_pdf$" "$report"; then
	cat "$report" >&2
	fail "explicit external validation did not identify supplied evidence"
fi
if ! grep -q "^generator_script_sha256=not_applicable$" "$report"; then
	cat "$report" >&2
	fail "explicit external validation incorrectly attributed a local generator"
fi
if ! grep -Fqx "pdf=$explicit_pdf" "$report"; then
	cat "$report" >&2
	fail "explicit external validation did not record the supplied PDF path"
fi
if ! grep -Fqx "validate $explicit_pdf" "$DSS_LOG"; then
	cat "$DSS_LOG" >&2
	fail "DSS validation tool was not invoked against the supplied PDF"
fi

echo "validate-pades-external tests passed."
