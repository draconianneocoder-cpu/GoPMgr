#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Regenerate the spike's Go and Java protobuf code from proto/g0spike.proto.
# The generated files are committed so CI on macOS and Windows needs no
# code generator. Tools are pinned; this script refuses other versions.
#
# Usage: TOOLS=/path/with/bin scripts/generate.sh
#   $TOOLS/bin must hold: protoc 3.25.9 (Maven Central com.google.protobuf:
#   protoc:3.25.9, SHA-1 367ec95cd2a07a8039e92bf714645291d0b817ca for
#   linux-x86_64), protoc-gen-grpc-java 1.85.1 (io.grpc:protoc-gen-grpc-java,
#   SHA-1 19389e4a786d57d4f73ba8af5887ca406aff3c3e for linux-x86_64),
#   protoc-gen-go v1.36.12 and protoc-gen-go-grpc v1.6.2 (go install).
set -euo pipefail

here="$(cd "$(dirname "$0")/.." && pwd)"
bin="${TOOLS:?set TOOLS to the directory holding bin/}/bin"

want_protoc="libprotoc 25.9"
got_protoc="$("$bin/protoc" --version)"
if [ "$got_protoc" != "$want_protoc" ]; then
	echo "generate: need $want_protoc, found $got_protoc" >&2
	exit 1
fi

export PATH="$bin:$PATH"
rm -rf "$here/host/g0pb" "$here/worker/src/main/java/org/gopmgr/g0spike/pb"
mkdir -p "$here/host/g0pb" "$here/worker/src/main/java"

"$bin/protoc" -I "$here/proto" \
	--go_out="$here/host/g0pb" --go_opt=paths=source_relative \
	--go-grpc_out="$here/host/g0pb" --go-grpc_opt=paths=source_relative \
	--java_out="$here/worker/src/main/java" \
	--plugin=protoc-gen-grpc-java="$bin/protoc-gen-grpc-java" \
	--grpc-java_out="$here/worker/src/main/java" \
	"$here/proto/g0spike.proto"

echo "generate: wrote host/g0pb and worker/src/main/java/org/gopmgr/g0spike/pb"
