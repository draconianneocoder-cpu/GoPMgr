#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Keep Cargo, compiler subprocesses and evidence probes on the same toolchain,
# including when Homebrew precedes rustup proxies on PATH. No automatic install.
set -euo pipefail
if [ "$#" -eq 0 ]; then
	echo "usage: with-rust-toolchain.sh COMMAND [ARG ...]" >&2
	exit 2
fi
toolchain=1.99.1
cargo="$(rustup which --toolchain "$toolchain" cargo)"
bin="$(dirname "$cargo")"
for tool in cargo rustc rustdoc; do
	if [ ! -x "$bin/$tool" ]; then
		echo "with-rust-toolchain: missing $tool in selected toolchain" >&2
		exit 1
	fi
done
export PATH="$bin:$PATH"
export RUSTUP_TOOLCHAIN="$toolchain"
export RUSTC="$bin/rustc"
export RUSTDOC="$bin/rustdoc"
if [ "$1" = cargo ]; then
	shift
	exec rustup run "$toolchain" "$cargo" "$@"
fi
exec rustup run "$toolchain" "$@"
