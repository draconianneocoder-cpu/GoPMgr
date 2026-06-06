#!/bin/bash
# SPDX-FileCopyrightText: 2026 The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
. "$ROOT/scripts/validate-pdfa-lib.sh"

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

assert_compliant() {
	if ! printf '%s\n' "$1" | verapdf_output_is_compliant; then
		fail "expected veraPDF output to be compliant: $2"
	fi
}

assert_not_compliant() {
	if printf '%s\n' "$1" | verapdf_output_is_compliant; then
		fail "expected veraPDF output to be non-compliant: $2"
	fi
}

assert_eq() {
	if [ "$1" != "$2" ]; then
		fail "$3: expected '$2', got '$1'"
	fi
}

assert_compliant '<report><jobs><job><validationReport><isCompliant>true</isCompliant></validationReport></job></jobs></report>' "xml true"
assert_compliant '<report><jobs><job><validationReport isCompliant="true"></validationReport></job></jobs></report>' "xml attribute true"
assert_not_compliant '<report><jobs><job><validationReport><isCompliant>false</isCompliant></validationReport></job></jobs></report>' "xml false"
assert_not_compliant '<report><jobs><job><validationReport isCompliant="false"></validationReport></job></jobs></report>' "xml attribute false"
assert_not_compliant 'The file is not compliant with PDF/A-3B.' "text false positive guard"

sample="$ROOT/.tmp/pmforge-pdfa-test/schedule.pdf"
assert_eq "$(verapdf_sample_arg docker "$ROOT" "$sample")" "/work/.tmp/pmforge-pdfa-test/schedule.pdf" "docker sample path"
assert_eq "$(verapdf_sample_arg cli "$ROOT" "$sample")" "$sample" "cli sample path"

probe_dir="$ROOT/.tmp/verapdf-find-test"
rm -rf "$probe_dir"
mkdir -p "$probe_dir/nested"
cat > "$probe_dir/nested/verapdf" << 'EOF'
#!/bin/bash
exit 0
EOF
chmod +x "$probe_dir/nested/verapdf"
assert_eq "$(find_verapdf_executable "$probe_dir")" "$probe_dir/nested/verapdf" "portable veraPDF executable lookup"
rm -rf "$probe_dir"

bad_archive="$ROOT/.tmp/not-a-valid-jar.zip"
mkdir -p "$ROOT/.tmp"
printf '%s\n' 'not a zip archive' > "$bad_archive"
command -v zip_archive_is_valid >/dev/null 2>&1 || fail "zip_archive_is_valid helper is missing"
if zip_archive_is_valid "$bad_archive"; then
	fail "invalid veraPDF archive was accepted"
fi
rm -f "$bad_archive"

bad_jar="$ROOT/.tmp/bad-verapdf.jar"
bad_wrapper="$ROOT/.tmp/bad-verapdf-wrapper"
printf '%s\n' 'not a jar' > "$bad_jar"
cat > "$bad_wrapper" << EOF
#!/bin/bash
java -jar "$bad_jar" "\$@"
EOF
chmod +x "$bad_wrapper"
command -v verapdf_cli_needs_refresh >/dev/null 2>&1 || fail "verapdf_cli_needs_refresh helper is missing"
if ! verapdf_cli_needs_refresh "$bad_wrapper" "$bad_jar"; then
	fail "invalid veraPDF jar wrapper was not marked for refresh"
fi
rm -f "$bad_jar" "$bad_wrapper"

fake_verapdf_dir="$ROOT/.tmp/verapdf-bin-test"
fake_verapdf="$fake_verapdf_dir/verapdf"
rm -rf "$fake_verapdf_dir"
mkdir -p "$fake_verapdf_dir"
cleanup_fake_verapdf() {
	rm -rf "$fake_verapdf_dir"
}
trap cleanup_fake_verapdf EXIT
cat > "$fake_verapdf" << 'EOF'
#!/bin/bash
printf '%s\n' '<report><jobs><job><validationReport><isCompliant>true</isCompliant></validationReport></job></jobs></report>'
EOF
chmod +x "$fake_verapdf"

gate_output="$(PATH="$fake_verapdf_dir:$PATH" bash "$ROOT/scripts/validate-pdfa.sh" 2>&1)"
case "$gate_output" in
	*"Checking schedule.pdf"* ) ;;
	*)
		printf '%s\n' "$gate_output" >&2
		fail "validate-pdfa gate did not generate and validate schedule.pdf"
		;;
esac
case "$gate_output" in
	*"Checking document-charter.pdf"* ) ;;
	*)
		printf '%s\n' "$gate_output" >&2
		fail "validate-pdfa gate did not generate and validate document-charter.pdf"
		;;
esac
case "$gate_output" in
	*"Checking combined-report.pdf"* ) ;;
	*)
		printf '%s\n' "$gate_output" >&2
		fail "validate-pdfa gate did not generate and validate combined-report.pdf"
		;;
esac
case "$gate_output" in
	*"Gate passed (nothing to check)"* )
		printf '%s\n' "$gate_output" >&2
		fail "validate-pdfa gate silently passed without samples"
		;;
esac

echo "validate-pdfa-lib tests passed."
