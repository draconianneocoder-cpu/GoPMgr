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
# shellcheck disable=SC2016 # Match literal source text; expansion would change the guard.
if ! grep -Fq 'VERAPDF_IMAGE="verapdf/cli:v${VERAPDF_VERSION}"' "$ROOT/scripts/validate-pdfa.sh"; then
	fail "validate-pdfa does not bind the official CLI image to VERAPDF_VERSION"
fi
# The installer fallback must be pinned by SHA-256, never run unverified.
if ! grep -Eq '^VERAPDF_INSTALLER_SHA256="[0-9a-f]{64}"$' "$ROOT/scripts/validate-pdfa.sh"; then
	fail "validate-pdfa does not pin the veraPDF installer by SHA-256"
fi
# shellcheck disable=SC2016 # Match literal source text; expansion would change the guard.
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
printf 'abc' >"$sha_probe"
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
cat >"$probe_dir/nested/verapdf" <<'EOF'
#!/bin/bash
exit 0
EOF
chmod +x "$probe_dir/nested/verapdf"
assert_eq "$(find_verapdf_executable "$probe_dir")" "$probe_dir/nested/verapdf" "portable veraPDF executable lookup"
rm -rf "$probe_dir"

# A selected validator with the wrong identity must not reach generation or
# modify samples, even if it would return compliant XML for every document.
identity_root="$(mktemp -d "$ROOT/.tmp/verapdf-identity.XXXXXX")"
cleanup_identity() {
	rm -r "$identity_root"
}
trap cleanup_identity EXIT
mkdir -p "$identity_root/scripts" "$identity_root/bin" "$identity_root/.tmp/gopmgr-pdfa-test"
cp "$ROOT/scripts/validate-pdfa.sh" "$ROOT/scripts/validate-pdfa-lib.sh" "$identity_root/scripts/"
printf 'profile' >"$identity_root/profile.icc"
printf 'preserve sample' >"$identity_root/.tmp/gopmgr-pdfa-test/sentinel"
cat >"$identity_root/bin/verapdf" <<'EOF'
#!/bin/bash
if [ "$1" = '--version' ]; then
	printf '%s\n' "$FAKE_IDENTITY"
	exit "${FAKE_PROBE_STATUS:-0}"
fi
printf 'validation reached' >"$IDENTITY_ROOT/validated"
printf '<isCompliant>true</isCompliant>\n'
EOF
cat >"$identity_root/bin/go" <<'EOF'
#!/bin/bash
printf 'generation reached' >"$IDENTITY_ROOT/generated"
exit 1
EOF
printf '# legacy jar: %s\n' "$identity_root/.tmp/verapdf/verapdf-app.jar" >>"$identity_root/bin/verapdf"
chmod +x "$identity_root/bin/"*
external_hash="$(shasum -a 256 "$identity_root/bin/verapdf")"
external_mode="$(python3 -c 'import os, sys; print(os.stat(sys.argv[1]).st_mode)' "$identity_root/bin/verapdf")"
for mode in cli docker; do
	if [ "$mode" = docker ]; then
		cat >"$identity_root/bin/docker" <<'EOF'
#!/bin/bash
# The gate must probe the same container command it uses for validation.
while [ "$#" -gt 0 ] && [ "$1" != 'verapdf/cli:v1.30.2' ]; do shift; done
[ "$#" -gt 0 ] || exit 2
shift
exec "$IDENTITY_ROOT/bin/verapdf" "$@"
EOF
		chmod +x "$identity_root/bin/docker"
	fi
	for identity in 'veraPDF 1.30.0' 'veraPDF 1.30.20' 'not a version' $'veraPDF 1.30.2\nveraPDF 1.30.0' $'veraPDF 1.30.2\nveraPDF 1.30.2'; do
		for strict in 1 0; do
			status=0
			output="$(IDENTITY_ROOT="$identity_root" FAKE_IDENTITY="$identity" GOPMGR_ICC_PROFILE="$identity_root/profile.icc" GOPMGR_PDFA_STRICT="$strict" GOPMGR_VERAPDF_FORCE_CLI="$([ "$mode" = cli ] && echo 1 || echo 0)" PATH="$identity_root/bin:$PATH" bash "$identity_root/scripts/validate-pdfa.sh" 2>&1)" || status=$?
			if [ "$strict" = 1 ]; then
				[ "$status" -ne 0 ] || fail "$mode accepted mismatched identity"
			else
				[ "$status" -eq 0 ] || fail "$mode non-strict identity did not skip"
				case "$output" in *'SKIP:'*) ;; *) fail "missing explicit skip" ;; esac
			fi
			case "$output" in *'expected veraPDF 1.30.2'*) ;; *) fail "$mode did not reject identity before generation: $output" ;; esac
			[ ! -e "$identity_root/generated" ] || fail "identity failure generated PDFs"
			[ ! -e "$identity_root/validated" ] || fail "identity failure validated PDFs"
			assert_eq "$(cat "$identity_root/.tmp/gopmgr-pdfa-test/sentinel")" 'preserve sample' 'identity failure preserves samples'
		done
	done
	for strict in 1 0; do
		status=0
		output="$(IDENTITY_ROOT="$identity_root" FAKE_IDENTITY='veraPDF 1.30.2' FAKE_PROBE_STATUS=1 GOPMGR_ICC_PROFILE="$identity_root/profile.icc" GOPMGR_PDFA_STRICT="$strict" GOPMGR_VERAPDF_FORCE_CLI="$([ "$mode" = cli ] && echo 1 || echo 0)" PATH="$identity_root/bin:$PATH" bash "$identity_root/scripts/validate-pdfa.sh" 2>&1)" || status=$?
		if [ "$strict" = 1 ]; then
			[ "$status" -ne 0 ] || fail "failed version probe accepted"
		else
			[ "$status" -eq 0 ] || fail "failed non-strict probe did not skip"
			case "$output" in *'SKIP:'*) ;; *) fail 'failed probe skip not explicit' ;; esac
		fi
		case "$output" in *'expected veraPDF 1.30.2'*) ;; *) fail "failed probe not diagnosed" ;; esac
		[ ! -e "$identity_root/generated" ] || fail "failed probe generated PDFs"
	done
done
assert_eq "$(shasum -a 256 "$identity_root/bin/verapdf")" "$external_hash" 'external executable bytes'
assert_eq "$(python3 -c 'import os, sys; print(os.stat(sys.argv[1]).st_mode)' "$identity_root/bin/verapdf")" "$external_mode" 'external executable permissions'
# Exercise installer failures without downloads or a global Java installation.
# These boundary fakes model only the pinned archive and installer side effects.
rm "$identity_root/bin/verapdf" "$identity_root/bin/docker"
for tool in dirname mkdir awk tr cat mktemp rm mv chmod; do
	ln -s "$(command -v "$tool")" "$identity_root/bin/$tool"
done
cat >"$identity_root/bin/sha256sum" <<'EOF'
#!/bin/bash
printf '6cc6341cb1af644044054b81f00a6590a7918abb18f762243de115258bcad838  %s\n' "$1"
EOF
cat >"$identity_root/bin/unzip" <<'EOF'
#!/bin/bash
mkdir -p "$4/verapdf-greenfield-1.30.2"
printf 'fixture installer' >"$4/verapdf-greenfield-1.30.2/verapdf-izpack-installer-1.30.2.jar"
EOF
cat >"$identity_root/bin/java" <<'EOF'
#!/bin/bash
install_dir="${2%/installer/*}/verapdf"
mkdir -p "$install_dir"
cat >"$install_dir/verapdf" <<'CLI'
#!/bin/bash
printf 'veraPDF %s\n' "${FAKE_CANDIDATE_VERSION:-1.30.0}"
CLI
chmod +x "$install_dir/verapdf"
EOF
chmod +x "$identity_root/bin/sha256sum" "$identity_root/bin/unzip" "$identity_root/bin/java"
mkdir -p "$identity_root/.tmp/verapdf"
printf 'fixture archive' >"$identity_root/.tmp/verapdf/verapdf-greenfield-1.30.2-installer.zip"
status=0
output="$(GOPMGR_ICC_PROFILE="$identity_root/profile.icc" GOPMGR_PDFA_STRICT=1 GOPMGR_VERAPDF_FORCE_CLI=1 PATH="$identity_root/bin" /bin/bash "$identity_root/scripts/validate-pdfa.sh" 2>&1)" || status=$?
[ "$status" -ne 0 ] || fail 'wrong installed identity accepted'
[ ! -e "$identity_root/.tmp/verapdf/verapdf-1.30.2" ] || fail 'wrong installed identity promoted to managed cache'
mkdir -p "$identity_root/.tmp/verapdf/verapdf-1.30.2"
printf 'preserve cache' >"$identity_root/.tmp/verapdf/verapdf-1.30.2/sentinel"
status=0
output="$(GOPMGR_ICC_PROFILE="$identity_root/profile.icc" GOPMGR_PDFA_STRICT=1 GOPMGR_VERAPDF_FORCE_CLI=1 PATH="$identity_root/bin" /bin/bash "$identity_root/scripts/validate-pdfa.sh" 2>&1)" || status=$?
[ "$status" -ne 0 ] || fail 'unusable existing cache accepted'
assert_eq "$(cat "$identity_root/.tmp/verapdf/verapdf-1.30.2/sentinel")" 'preserve cache' 'failed installation preserves existing cache'
# A failed promotion must not turn into a successful install, and an existing
# symlink must never be followed or replaced by automatic cache repair.
rm "$identity_root/.tmp/verapdf/verapdf-1.30.2/sentinel"
rmdir "$identity_root/.tmp/verapdf/verapdf-1.30.2"
rm "$identity_root/bin/mv"
cat >"$identity_root/bin/mv" <<'EOF'
#!/bin/bash
exit 1
EOF
chmod +x "$identity_root/bin/mv"
status=0
output="$(FAKE_CANDIDATE_VERSION=1.30.2 GOPMGR_ICC_PROFILE="$identity_root/profile.icc" GOPMGR_PDFA_STRICT=1 GOPMGR_VERAPDF_FORCE_CLI=1 PATH="$identity_root/bin" /bin/bash "$identity_root/scripts/validate-pdfa.sh" 2>&1)" || status=$?
[ "$status" -ne 0 ] || fail 'promotion failure accepted'
case "$output" in *'Could not install the pinned veraPDF CLI'*) ;; *) fail 'promotion failure not propagated' ;; esac
[ ! -e "$identity_root/.tmp/verapdf/verapdf-1.30.2" ] || fail 'failed promotion left managed installation'
ln -s "$identity_root/.tmp/gopmgr-pdfa-test" "$identity_root/.tmp/verapdf/verapdf-1.30.2"
status=0
output="$(GOPMGR_ICC_PROFILE="$identity_root/profile.icc" GOPMGR_PDFA_STRICT=1 GOPMGR_VERAPDF_FORCE_CLI=1 PATH="$identity_root/bin" /bin/bash "$identity_root/scripts/validate-pdfa.sh" 2>&1)" || status=$?
[ "$status" -ne 0 ] || fail 'unusable cache symlink accepted'
[ -L "$identity_root/.tmp/verapdf/verapdf-1.30.2" ] || fail 'cache symlink replaced'
assert_eq "$(cat "$identity_root/.tmp/gopmgr-pdfa-test/sentinel")" 'preserve sample' 'cache symlink target preserved'
cleanup_identity
trap - EXIT

fake_verapdf_dir="$ROOT/.tmp/verapdf-bin-test"
fake_verapdf="$fake_verapdf_dir/verapdf"
rm -rf "$fake_verapdf_dir"
mkdir -p "$fake_verapdf_dir"
cleanup_fake_verapdf() {
	rm -rf "$fake_verapdf_dir"
}
trap cleanup_fake_verapdf EXIT
fake_verapdf_log="$fake_verapdf_dir/verapdf.args"
cat >"$fake_verapdf" <<EOF
#!/bin/bash
if [ "\$1" = '--version' ]; then
	printf 'WARNING: fixture Java warning\\n' >&2
	printf 'veraPDF 1.30.2\\n'
	exit 0
fi
printf '%s\\n' "\$*" >> "$fake_verapdf_log"
printf '%s\\n' '<report><jobs><job><validationReport><isCompliant>true</isCompliant></validationReport></job></jobs></report>'
exit "\${FAKE_VALIDATION_STATUS:-0}"
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
*"Checking schedule.pdf"*) ;;
*)
	printf '%s\n' "$gate_output" >&2
	fail "validate-pdfa gate did not generate and validate schedule.pdf"
	;;
esac
case "$gate_output" in
*"Checking document-charter.pdf"*) ;;
*)
	printf '%s\n' "$gate_output" >&2
	fail "validate-pdfa gate did not generate and validate document-charter.pdf"
	;;
esac
case "$gate_output" in
*"Checking combined-report.pdf"*) ;;
*)
	printf '%s\n' "$gate_output" >&2
	fail "validate-pdfa gate did not generate and validate combined-report.pdf"
	;;
esac
case "$gate_output" in
*"Checking monte-carlo-risk-report.pdf"*) ;;
*)
	printf '%s\n' "$gate_output" >&2
	fail "validate-pdfa gate did not generate and validate monte-carlo-risk-report.pdf"
	;;
esac
for sample in document-multilingual.pdf document-japanese.pdf document-korean.pdf document-chinese.pdf signed-document.pdf; do
	case "$gate_output" in
	*"Checking $sample"*) ;;
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
*"Gate passed (nothing to check)"*)
	printf '%s\n' "$gate_output" >&2
	fail "validate-pdfa gate silently passed without samples"
	;;
esac

# A successful identity probe must not mask a later validator execution error,
# even if the failed invocation prints a positive compliance fragment.
status=0
output="$(FAKE_VALIDATION_STATUS=1 GOPMGR_VERAPDF_FORCE_CLI=1 PATH="$fake_verapdf_dir:$PATH" bash "$ROOT/scripts/validate-pdfa.sh" 2>&1)" || status=$?
[ "$status" -ne 0 ] || fail 'validator execution failure accepted as conformance success'
case "$output" in *'PDF/A validation gate FAILED.'*) ;; *) fail 'validation execution failure was not diagnosed' ;; esac

# Exercise the Docker branch without contacting a registry. The fake records
# the exact invocation and returns compliant XML, proving the gate selects the
# pinned official image and translates repo-local samples to /work paths.
fake_docker="$fake_verapdf_dir/docker"
fake_docker_log="$fake_verapdf_dir/docker.args"
cat >"$fake_docker" <<'EOF'
#!/bin/bash
printf '%s\n' "$*" >> "$GOPMGR_FAKE_DOCKER_LOG"
case "$*" in
*' --version') printf 'veraPDF 1.30.2\n'; exit 0 ;;
esac
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
