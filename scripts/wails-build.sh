#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Wrapper around `wails build` that owns macOS code-signing.
#
# Why: when the project lives in an iCloud-synced location (e.g. anything
# under ~/Documents with "Desktop & Documents" iCloud sync on), freshly
# written build artifacts acquire extended attributes (com.apple.FinderInfo
# and iCloud file-provider metadata). Apple's codesign refuses to sign any
# file carrying such attributes:
#
#   "resource fork, Finder information, or similar detritus not allowed"
#
# Wails' built-in ad-hoc self-sign step therefore fails. Since Wails signs
# in-process (no hook to strip attributes first), we let Wails compile,
# package, and embed the app, then strip the extended attributes and ad-hoc
# sign the bundle ourselves - the sequence Apple documents in QA1940.
#
# This wrapper is a no-op beyond a redundant re-sign on machines where Wails'
# own sign already succeeds (it uses codesign --force).
#
# Usage: scripts/wails-build.sh [extra wails build flags]

set -uo pipefail
cd "$(dirname "$0")/.."

GOOS="$(go env GOOS)"

# Start from a clean bin dir so a stale bundle can't mask a failed compile,
# and so no previously-attributed binary lingers.
rm -rf build/bin

# Run the Wails build. On macOS its internal self-sign may fail on the xattr
# detritus described above; we re-sign below, so do not abort on that here.
wails build "$@"
wails_status=$?

if [ "$GOOS" != "darwin" ]; then
	# Non-macOS: there is no bundle/self-sign, so surface Wails' real status.
	exit "$wails_status"
fi

app="$(find build/bin -maxdepth 1 -name '*.app' -type d 2>/dev/null | head -1)"
if [ -z "$app" ] || [ ! -d "$app" ]; then
	echo "wails-build: no .app produced under build/bin (wails exit ${wails_status})." >&2
	exit "${wails_status:-1}"
fi

# Strip the extended-attribute detritus, then ad-hoc sign. Done here (not
# inside Wails) so the strip happens immediately before the sign.
if command -v xattr >/dev/null 2>&1; then
	xattr -cr "$app"
fi
if command -v codesign >/dev/null 2>&1; then
	if ! codesign --force --deep --sign - "$app"; then
		echo "wails-build: ad-hoc codesign failed for $app." >&2
		echo "  Inspect leftover attributes with: xattr -lr \"$app\"" >&2
		echo "  If they are re-added by iCloud, build from a non-synced path (e.g. ~/Developer)." >&2
		exit 1
	fi
	codesign --verify --deep --strict "$app" || true
fi

echo "wails-build: built and ad-hoc signed $app"
exit 0
