#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SAMPLE_DIR="$ROOT/.tmp/pmforge-pades-test"
FAKE_BIN="$ROOT/.tmp/pades-external-bin-test"
FAKE_LOG="$FAKE_BIN/verapdf.args"
DSS_LOG="$FAKE_BIN/dss-validation-tool.args"

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

rm -rf "$FAKE_BIN"
mkdir -p "$FAKE_BIN"
cleanup() {
	rm -rf "$FAKE_BIN"
}
trap cleanup EXIT

cat > "$FAKE_BIN/verapdf" <<'EOF'
#!/bin/bash
printf '%s\n' "$*" >> "$PMFORGE_FAKE_VERAPDF_LOG"
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
printf '%s\n' "$*" >> "$PMFORGE_FAKE_DSS_LOG"
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
echo "signature.format=PAdES-BASELINE-B"
echo "signature.indication=INDETERMINATE"
echo "signature.sub_indication=NO_CERTIFICATE_CHAIN_FOUND"
EOF
chmod +x "$FAKE_BIN/dss-validation-tool"

mkdir -p "$SAMPLE_DIR"
PMFORGE_FAKE_VERAPDF_LOG="$FAKE_LOG" PMFORGE_FAKE_DSS_LOG="$DSS_LOG" PATH="$FAKE_BIN:$PATH" \
	bash "$ROOT/scripts/validate-pades-external.sh" >/tmp/pmforge-pades-external-test.out

report="$SAMPLE_DIR/external-validation-report.txt"
[ -s "$report" ] || fail "external validation report was not written"

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

if grep -q "DSS PAdES interoperability: TODO" "$report"; then
	cat "$report" >&2
	fail "DSS branch still reports a manual TODO"
fi

if ! grep -q "^validate .*/signed-sample.pdf$" "$DSS_LOG"; then
	cat "$DSS_LOG" >&2
	fail "DSS validation tool was not invoked against the signed sample"
fi

echo "validate-pades-external tests passed."
