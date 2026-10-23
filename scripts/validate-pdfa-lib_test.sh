#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
. "$ROOT/scripts/validate-pdfa-lib.sh"

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

# Release validation must not drift with a mutable container tag, and the image
# repository must remain the official self-contained veraPDF CLI distribution.
if rg -q 'verapdf/[^[:space:]]*:latest' "$ROOT/scripts/validate-pdfa.sh"; then
	fail "validate-pdfa uses a mutable latest container tag"
fi
if ! grep -Fq 'VERAPDF_IMAGE="verapdf/cli:v${VERAPDF_VERSION}"' "$ROOT/scripts/validate-pdfa.sh"; then
	fail "validate-pdfa does not bind the official CLI image to VERAPDF_VERSION"
fi
# The installer fallback must be pinned by SHA-256, never run unverified.
if ! grep -Eq '^VERAPDF_INSTALLER_SHA256="[0-9a-f]{64}"$' "$ROOT/scripts/validate-pdfa.sh"; then
	fail "validate-pdfa does not pin the veraPDF installer by SHA-256"
fi
if ! grep -Fq 'file_has_sha256 "$tmp_zip" "$VERAPDF_INSTALLER_SHA256"' "$ROOT/scripts/validate-pdfa.sh"; then
	fail "validate-pdfa does not verify a downloaded installer before using it"
fi

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
assert_not_compliant 'The file is not compliant with PDF/A-2U.' "text false positive guard"

assert_eq "$(verapdf_installer_url 1.30.2)" "https://software.verapdf.org/rel/1.30/verapdf-greenfield-1.30.2-installer.zip" "versioned installer URL"
case "$(verapdf_auto_install_xml '/tmp/a&b<c>')" in
	*'<installpath>/tmp/a&amp;b&lt;c&gt;</installpath>'*) ;;
	*) fail "auto-install XML does not escape the install path" ;;
esac

sha_probe="$ROOT/.tmp/sha256-probe"
mkdir -p "$ROOT/.tmp"
printf 'abc' > "$sha_probe"
file_has_sha256 "$sha_probe" ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad || fail "file_has_sha256 rejected a matching file"
if file_has_sha256 "$sha_probe" 0000000000000000000000000000000000000000000000000000000000000000; then
	fail "file_has_sha256 accepted a mismatched file"
fi
if file_has_sha256 "$ROOT/.tmp/does-not-exist" ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad; then
	fail "file_has_sha256 accepted a missing file"
fi
rm -f "$sha_probe"

sample="$ROOT/.tmp/gopmgr-pdfa-test/schedule.pdf"
assert_eq "$(verapdf_sample_arg docker "$ROOT" "$sample")" "/work/.tmp/gopmgr-pdfa-test/schedule.pdf" "docker sample path"
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
fake_verapdf_log="$fake_verapdf_dir/verapdf.args"
cat > "$fake_verapdf" << EOF
#!/bin/bash
printf '%s\\n' "\$*" >> "$fake_verapdf_log"
printf '%s\\n' '<report><jobs><job><validationReport><isCompliant>true</isCompliant></validationReport></job></jobs></report>'
EOF
chmod +x "$fake_verapdf"

# Force the intended CLI branch. GitHub's Ubuntu runners provide Docker, which
# previously made this supposedly hermetic fixture pull and execute a real
# container instead of the fake veraPDF binary.
gate_output="$(
	GOPMGR_VERAPDF_FORCE_CLI=1 PATH="$fake_verapdf_dir:$PATH" \
		bash "$ROOT/scripts/validate-pdfa.sh" 2>&1
)"
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
	*"Checking monte-carlo-risk-report.pdf"* ) ;;
	*)
		printf '%s\n' "$gate_output" >&2
		fail "validate-pdfa gate did not generate and validate monte-carlo-risk-report.pdf"
		;;
esac
for sample in document-multilingual.pdf document-japanese.pdf document-korean.pdf document-chinese.pdf signed-document.pdf; do
	case "$gate_output" in
		*"Checking $sample"* ) ;;
		*)
			printf '%s\n' "$gate_output" >&2
			fail "validate-pdfa gate did not generate and validate $sample"
			;;
	esac
done
if [ "$(grep -c -- '-f 2u' "$fake_verapdf_log")" -ne 9 ]; then
	cat "$fake_verapdf_log" >&2
	fail "validate-pdfa gate did not validate all nine samples against the PDF/A-2u profile"
fi
case "$gate_output" in
	*"Gate passed (nothing to check)"* )
		printf '%s\n' "$gate_output" >&2
		fail "validate-pdfa gate silently passed without samples"
		;;
esac

# Exercise the Docker branch without contacting a registry. The fake records
# the exact invocation and returns compliant XML, proving the gate selects the
# pinned official image and translates repo-local samples to /work paths.
fake_docker="$fake_verapdf_dir/docker"
fake_docker_log="$fake_verapdf_dir/docker.args"
cat > "$fake_docker" <<'EOF'
#!/bin/bash
printf '%s\n' "$*" >> "$GOPMGR_FAKE_DOCKER_LOG"
printf '%s\n' '<report><jobs><job><validationReport><isCompliant>true</isCompliant></validationReport></job></jobs></report>'
EOF
chmod +x "$fake_docker"
(
	# Model a release using the CLI override; the Docker fixture must own its selection.
	export GOPMGR_VERAPDF_FORCE_CLI=1
	GOPMGR_VERAPDF_FORCE_CLI=0 GOPMGR_FAKE_DOCKER_LOG="$fake_docker_log" \
		PATH="$fake_verapdf_dir:$PATH" bash "$ROOT/scripts/validate-pdfa.sh" >/dev/null
)
if ! grep -Fq "verapdf/cli:v1.30.2" "$fake_docker_log"; then
	cat "$fake_docker_log" >&2
	fail "Docker validation did not invoke the pinned official veraPDF CLI image"
fi
if grep -q ":latest" "$fake_docker_log"; then
	cat "$fake_docker_log" >&2
	fail "Docker validation invoked a mutable latest tag"
fi
if ! grep -Fq "/work/.tmp/gopmgr-pdfa-test/schedule.pdf" "$fake_docker_log"; then
	cat "$fake_docker_log" >&2
	fail "Docker validation did not translate the schedule sample to its mounted path"
fi

echo "validate-pdfa-lib tests passed."
