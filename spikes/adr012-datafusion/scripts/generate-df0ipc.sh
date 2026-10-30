#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Regenerate the DF0-C Go and Rust code from proto/df0ipc.proto. The output
# is committed, so builds and CI need no code generator. Tools are pinned,
# as for the ADR-010 G0 spike:
#   protoc 25.9 (Maven Central com.google.protobuf:protoc:3.25.9),
#   protoc-gen-go v1.36.12, protoc-gen-go-grpc v1.6.2 (go install),
#   tonic-prost-build 0.14.6 (rust/codegen).
#
# Usage: TOOLS=/path/with/bin scripts/generate-df0ipc.sh
set -euo pipefail
cd "$(dirname "$0")/.."
bin="${TOOLS:?set TOOLS to the directory holding bin/}/bin"

check() {
	local got
	got="$("$bin/$1" --version 2>&1)"
	if [ "$got" != "$2" ]; then
		echo "generate-df0ipc: need $1 '$2', found '$got'" >&2
		exit 1
	fi
}
check protoc "libprotoc 25.9"
check protoc-gen-go "protoc-gen-go v1.36.12"
check protoc-gen-go-grpc "protoc-gen-go-grpc 1.6.2"

rm -rf internal/df0pb rust/worker/src/pb
mkdir -p internal/df0pb rust/worker/src/pb
PATH="$bin:$PATH" "$bin/protoc" -I proto \
	--go_out=internal/df0pb --go_opt=paths=source_relative \
	--go-grpc_out=internal/df0pb --go-grpc_opt=paths=source_relative \
	proto/df0ipc.proto
(cd rust/codegen && PROTOC="$bin/protoc" CARGO_TARGET_DIR="${CARGO_TARGET_DIR:-${TMPDIR:-/tmp}/df0-codegen-target}" \
	cargo run --locked --release -q -- ../../proto ../worker/src/pb)
echo "generate-df0ipc: wrote internal/df0pb and rust/worker/src/pb"
