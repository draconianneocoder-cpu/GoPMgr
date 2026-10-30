#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Fetches the pinned JDK and Maven for the G0 spike, verifies their
# checksums, unpacks them and prints the JDK home. Used by the G0 workflow
# so every runner, including the Arm ones, uses the same toolchain.
#
# Usage: toolchain.sh <platform> <dest-dir>
#   platform: linux-x64 | linux-aarch64 | mac-x64 | mac-aarch64 |
#             windows-x64 | windows-aarch64
#
# Pins (checksums from the publishers, 2026-10-25):
#   Eclipse Temurin jdk-21.0.12.1+1: SHA-256 per package from
#     https://api.adoptium.net/v3/assets/release_name/eclipse/jdk-21.0.12.1+1
#   Apache Maven 3.9.11: SHA-512 from
#     https://archive.apache.org/dist/maven/maven-3/3.9.11/binaries/apache-maven-3.9.11-bin.tar.gz.sha512
set -euo pipefail

platform="${1:?platform}"
dest="${2:?dest-dir}"

jdk_base="https://github.com/adoptium/temurin21-binaries/releases/download/jdk-21.0.12.1%2B1"
case "$platform" in
  linux-x64)
    jdk_file=OpenJDK21U-jdk_x64_linux_hotspot_21.0.12.1_1.tar.gz
    jdk_sha256=ce79869e1307ed8ee1e2baa86a412b1eb5b75d10a01006d788a6f968bcfaee94 ;;
  linux-aarch64)
    jdk_file=OpenJDK21U-jdk_aarch64_linux_hotspot_21.0.12.1_1.tar.gz
    jdk_sha256=23e37e026f12f3e706f18938ff611db3032d075b09d0879a25d06718c773e223 ;;
  mac-x64)
    jdk_file=OpenJDK21U-jdk_x64_mac_hotspot_21.0.12.1_1.tar.gz
    jdk_sha256=44db0f08196daf19a47f90d13388b0c943b67663cb537f998fe29e836fa842ce ;;
  mac-aarch64)
    jdk_file=OpenJDK21U-jdk_aarch64_mac_hotspot_21.0.12.1_1.tar.gz
    jdk_sha256=3623232f33a9c3baadf304480b2535f9a3cba8a58d42ecbb438ba267315d9998 ;;
  windows-x64)
    jdk_file=OpenJDK21U-jdk_x64_windows_hotspot_21.0.12.1_1.zip
    jdk_sha256=f9d6e191ab098c0d416e7d588a24420a8621cd2f4720dab2459b8b7b2d2d8b4e ;;
  windows-aarch64)
    jdk_file=OpenJDK21U-jdk_aarch64_windows_hotspot_21.0.12.1_1.zip
    jdk_sha256=ccf2e51f527d542a70ba5794a600d3aac04b4e967950e227834c7566cb1bec7b ;;
  *)
    echo "toolchain: unknown platform $platform" >&2
    exit 2 ;;
esac
mvn_file=apache-maven-3.9.11-bin.tar.gz
mvn_url="https://archive.apache.org/dist/maven/maven-3/3.9.11/binaries/$mvn_file"
mvn_sha512=bcfe4fe305c962ace56ac7b5fc7a08b87d5abd8b7e89027ab251069faebee516b0ded8961445d6d91ec1985dfe30f8153268843c89aa392733d1a3ec956c9978

fetch() {
  local url="$1" out="$2" i
  for i in 1 2 3 4; do
    if curl -fsSL -o "$out" "$url"; then
      return 0
    fi
    sleep $((i * 5))
  done
  echo "toolchain: could not download $url" >&2
  return 1
}

# verify <bits> <want> <file>: fails unless the digest matches.
verify() {
  local bits="$1" want="$2" file="$3" got
  if command -v "sha${bits}sum" >/dev/null 2>&1; then
    got="$("sha${bits}sum" "$file" | cut -d' ' -f1)"
  else
    got="$(shasum -a "$bits" "$file" | cut -d' ' -f1)"
  fi
  if [ "$got" != "$want" ]; then
    echo "toolchain: checksum mismatch for $file: got $got, want $want" >&2
    return 1
  fi
}

mkdir -p "$dest"
cd "$dest"
fetch "$jdk_base/$jdk_file" "$jdk_file"
verify 256 "$jdk_sha256" "$jdk_file"
fetch "$mvn_url" "$mvn_file"
verify 512 "$mvn_sha512" "$mvn_file"

case "$jdk_file" in
  *.zip)
    # Windows: unpack with the OS's own PowerShell, present on every edition.
    powershell.exe -NoProfile -NonInteractive -Command \
      "Expand-Archive -LiteralPath '$(cygpath -w "$PWD/$jdk_file")' -DestinationPath '$(cygpath -w "$PWD")'" ;;
  *)
    tar -xzf "$jdk_file" ;;
esac
tar -xzf "$mvn_file"

jdk_home="$PWD/jdk-21.0.12.1+1"
if [ -d "$jdk_home/Contents/Home" ]; then
  jdk_home="$jdk_home/Contents/Home"
fi
if [ ! -x "$jdk_home/bin/java" ] && [ ! -x "$jdk_home/bin/java.exe" ]; then
  echo "toolchain: no java under $jdk_home" >&2
  exit 1
fi
echo "JDK_HOME=$jdk_home"
echo "MAVEN_BIN=$PWD/apache-maven-3.9.11/bin"
