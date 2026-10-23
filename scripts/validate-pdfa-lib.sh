#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later

# Shared helpers for scripts/validate-pdfa.sh. Keep this file free of
# side effects so validate-pdfa-lib_test.sh can source it directly.

verapdf_output_is_compliant() {
	local output compact
	output="$(cat)"
	compact="$(printf '%s' "$output" | tr -d '[:space:]')"

	case "$compact" in
		*'<isCompliant>true</isCompliant>'* | *'isCompliant="true"'* | *"isCompliant='true'"* | *'"isCompliant":true'*)
			return 0
			;;
	esac

	return 1
}

verapdf_sample_arg() {
	local mode root sample rel
	mode="$1"
	root="${2%/}"
	sample="$3"

	case "$mode" in
		docker)
			case "$sample" in
				"$root"/*)
					rel="${sample#"$root"/}"
					printf '/work/%s\n' "$rel"
					;;
				*)
					printf 'validate-pdfa: sample is outside repo root: %s\n' "$sample" >&2
					return 1
					;;
			esac
			;;
		cli)
			printf '%s\n' "$sample"
			;;
		*)
			printf 'validate-pdfa: unknown veraPDF mode: %s\n' "$mode" >&2
			return 1
			;;
	esac
}

find_verapdf_executable() {
	local dir candidate
	dir="$1"

	while IFS= read -r candidate; do
		if [ -x "$candidate" ]; then
			printf '%s\n' "$candidate"
			return 0
		fi
	done < <(find "$dir" -type f -name verapdf -print 2>/dev/null)

	return 1
}

zip_archive_is_valid() {
	local path
	path="$1"

	if [ ! -s "$path" ]; then
		return 1
	fi

	if command -v unzip >/dev/null 2>&1; then
		unzip -tq "$path" >/dev/null 2>&1
		return $?
	fi

	return 0
}

verapdf_cli_needs_refresh() {
	local cli jar
	cli="$1"
	jar="$2"

	if [ ! -x "$cli" ]; then
		return 0
	fi

	if grep -F "$jar" "$cli" >/dev/null 2>&1 && ! zip_archive_is_valid "$jar"; then
		return 0
	fi

	return 1
}

# verapdf_installer_url <version>
# Prints the URL of veraPDF's versioned installer, for example
# https://software.verapdf.org/rel/1.30/verapdf-greenfield-1.30.2-installer.zip.
verapdf_installer_url() {
	local version
	version="$1"
	printf 'https://software.verapdf.org/rel/%s/verapdf-greenfield-%s-installer.zip\n' "${version%.*}" "$version"
}

# file_has_sha256 <path> <expected-hex>
# Succeeds only when the file's SHA-256 matches. Fails closed when no
# SHA-256 tool is available.
file_has_sha256() {
	local path expected actual
	path="$1"
	expected="$2"
	if [ ! -f "$path" ]; then
		return 1
	fi
	if command -v sha256sum >/dev/null 2>&1; then
		actual="$(sha256sum "$path" | awk '{print $1}')"
	elif command -v shasum >/dev/null 2>&1; then
		actual="$(shasum -a 256 "$path" | awk '{print $1}')"
	else
		return 1
	fi
	[ "$actual" = "$expected" ]
}

# verapdf_auto_install_xml <install-dir>
# Prints the IzPack answers that install veraPDF without prompts: the core
# application and its command-line scripts, without the validation model,
# documentation, or sample plugins.
verapdf_auto_install_xml() {
	local dir
	dir="$(printf '%s' "$1" | sed -e 's/&/\&amp;/g' -e 's/</\&lt;/g' -e 's/>/\&gt;/g')"
	cat << XML
<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<AutomatedInstallation langpack="eng">
  <com.izforge.izpack.panels.htmlhello.HTMLHelloPanel id="welcome"/>
  <com.izforge.izpack.panels.target.TargetPanel id="install_dir">
    <installpath>${dir}</installpath>
  </com.izforge.izpack.panels.target.TargetPanel>
  <com.izforge.izpack.panels.packs.PacksPanel id="sdk_pack_select">
    <pack index="0" name="veraPDF GUI" selected="true"/>
    <pack index="1" name="veraPDF Mac and *nix Scripts" selected="true"/>
    <pack index="2" name="veraPDF Validation model" selected="false"/>
    <pack index="3" name="veraPDF Documentation" selected="false"/>
    <pack index="4" name="veraPDF Sample Plugins" selected="false"/>
  </com.izforge.izpack.panels.packs.PacksPanel>
  <com.izforge.izpack.panels.install.InstallPanel id="install"/>
  <com.izforge.izpack.panels.finish.FinishPanel id="finish"/>
</AutomatedInstallation>
XML
}
