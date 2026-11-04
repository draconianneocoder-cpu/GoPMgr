<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-013 P1: auxiliary-tool provisioning

## Scope and setup

P1 provides opt-in provisioning on macOS arm64 and glibc Linux x64. It does not
install Go, Node, npm, Wails, Rust, OS packages or production dependencies.
Make remains canonical. No new lint gate or hosted CI job is enabled.
Python **3.11 or newer** is required for these targets (`tomllib` is stdlib).
The existing application build requirements are unchanged.

| Tool | Exact version | Built-in backend |
|---|---|---|
| mise | 2026.11.0 | Verified upstream archive bootstrap |
| ShellCheck | 0.11.0 | `github:koalaman/shellcheck` |
| shfmt | 3.14.1 | `github:mvdan/sh` |
| actionlint | 1.7.12 | `github:rhysd/actionlint` |
| zizmor | 1.32.0 | `github:zizmorcore/zizmor` |

From the repository root, explicitly opt in:

```sh
make tools-bootstrap  # downloads pinned mise; verifies before publication
make tools-install    # installs the reviewed lock; may access the network
PATH="$PWD/.tmp/engineering/bin:$PATH" make tools-check
```

No shell activation, global install, persistent PATH edit or update hook is
performed. Existing runtime commands and pins remain authoritative. Keep your
normal Go/Node/Wails setup on PATH. The auxiliary prefix applies only to the
command shown. `make verify`, `make check-release` and `make tools-check` do not
call these installers. `tools-check` does not invoke mise.

The supported provisioning boundary is `make tools-install`, not an ambient
`mise install`, `mise exec` or `mise run`. The wrapper validates and snapshots
only the reviewed configuration, uses private HOME/XDG/cache/data directories,
content-bound explicit trust, disables hooks/environment injection/skills sync,
and verifies actual discovered config paths before installing. The mise subprocess does not inherit caller tokens,
proxy settings, user configuration or shell hooks. This can
prevent provisioning behind a proxy; do not weaken the wrapper to work around
that failure without a reviewed acquisition-policy change.

The read-only check verifies policy/lock coherence, the selected auxiliary
executable's SHA-256 **before execution**, then its bounded version probe. It
reports shadowed PATH candidates without executing them. A different build of
the same version, including a package-manager rebuild, is deliberately rejected.
Host checks establish command presence; on Linux, the qualified system
`/usr/bin/pkg-config` checks GTK 4/WebKitGTK 6.0 development packages. System
executables and their OS installation are a separate trust boundary. This is
not native application execution, SDK provenance or release qualification.

## Integrity and version ownership

`mise.toml` declares exact auxiliary versions. `mise.lock` is genuine mise
2026.11.0 output for `macos-arm64,linux-x64`, with eight platform entries.
`tools/engineering/artifacts.json` records reviewed release URLs and exact API asset IDs, archive/member
identities and executable hashes; it pins mise outside mise's own configuration.
The checker rejects drift between declaration, lock and inventory, unknown tools,
extra executable configuration sections and acquisition-option changes. It
compares parsed policy, not whole-file hashes, so harmless comments need not be
repinned. Go/Wails, npm and isolated Rust version authorities are untouched.

At adoption on 2026-11-04:

- Official release APIs reported all five selected releases stable. The exact
  versions are frozen after review; there is no automatic release-age window,
  `latest` request or unattended update.
- GitHub reported the annotated mise tag object
  `b7aa353ba6f8a12f28f15429b70673544710dd80` verified, reason `valid`.
- GPG verified `SHASUMS256.asc` using the upstream documented fingerprint
  `24853EC9F655CE80B48E6C3A8B81C9D17413A06D`. The key came from upstream HTTPS;
  its web-of-trust status was undefined. Both downloaded mise archives matched
  the signed sums, and their selected executable hashes matched release metadata.
- Tool archive hashes were cross-checked against official release asset metadata.
  mise's lock generation recorded GitHub-attestation verification for shfmt,
  actionlint and zizmor. ShellCheck has checksum evidence without that recorded
  attestation. Executable hashes were calculated from verified artifact members.

Subsequent bootstrap runs enforce those committed archive and executable hashes;
they do not refetch a signing key or treat mutable release metadata as authority.
This proves correspondence to reviewed bytes, not absence of upstream defects,
reproducible builds, or the compiler provenance of every upstream binary.

For a future update, review stable release metadata and provenance first, then
regenerate the lock with the verified pinned mise's
`lock --platform macos-arm64,linux-x64` in an isolated, explicitly trusted staging
directory. Review every release/API URL and artifact hash; derive executable
hashes only from verified downloads. Commit declaration, generated lock and
artifact inventory together, and rerun both test targets on both qualified
hosts. Updating mise itself also requires fresh trust/default-behavior review.
Do not use an unattended upgrade command or relax locks to resolve drift.

### Safe-mode limitation

**Do not use `MISE_SAFE=1` as the artifact-URL security boundary.** A raw mise
2026.11.0 `install --locked` experiment in safe mode attempted a substituted
lock URL at a closed loopback endpoint instead of ignoring it. This contradicted
the expected release behavior. The repository wrapper rejects any such URL or
checksum substitution before invoking mise, including with `--safe`.
Both supported modes install the same approved binaries. Do not bypass policy
validation or relax checksum requirements to make an install succeed.

## Failure and recovery

A missing/truncated lock, missing checksum, checksum mismatch or source drift
fails. A failed bootstrap never replaces the previous binary. Installs have a
five-minute subprocess limit; network/provenance requests can still fail. An
empty offline cache fails; a lockfile does not make downloads or OS SDKs offline
or hermetic. Preserve the original failure diagnostic, then retry explicitly
when the dependency is available. Neither verification nor a developer shell
retries installation automatically.

Installed caches are not trusted merely because mise marks a version installed:
the wrapper rehashes all four binaries before exposing the tool links. If a
cached binary is altered, the wrapper fails without executing it. Stop using
that directory, inspect the cause, and remove the affected task-local version
before explicitly reinstalling. Do not edit the inventory to accept unknown bytes.

Make-only recovery does not require the mise executable:

1. Download the exact platform assets named in `tools/engineering/artifacts.json`
   from their recorded URLs, using a reviewed downloader.
2. Check each archive SHA-256. Extract only its named regular-file member, or use
   the raw asset when `member` is empty. Check the extracted executable SHA-256,
   then grant execute permission and place it in a separate directory.
3. Put that directory first on PATH for the Make invocation. Run `make tools-check`,
   then the existing build/verification targets. Keep the declaration, lock and
   inventory as the recovery policy record; `tools-check` deliberately needs them.

This procedure was exercised using direct archive extraction, not copies from
mise's installed tool directory. Removing the task-local mise executable leaves
these independent binaries usable. A complete rollback of P1 must also revert
its configuration-registry entries and opt-in targets as a cohesive change;
deleting only registry-required files is not a valid rollback. Existing Make
build/test/release recipes have not been replaced or weakened.

## Qualification and remaining gates

```sh
make tools-tests       # offline stdlib boundary suite
make tools-qualify     # explicit network tests; requires tools-bootstrap first
```

Qualification covers real trusted and safe-mode installs, configuration-source
exclusion (including a failing ancestor-isolation mutation), unchanged skills
files/directories/symlinks, no implicit Go install, poisoned installed cache,
missing lock/checksum, wrong checksum, untrusted configuration and empty offline
cache. Offline tests cover version drift, substituted artifact/API URLs, malformed
policy, wrong or fake PATH binaries, hash-before-execution, bootstrap publication
and failure preservation, dropped credentials and a shadowed `pkg-config`.

The offline suite has 26 tests; the opt-in network suite has three tests with
normal/safe installs and multiple negative cases. On macOS the independent
binary `tools-check` also passed under `sandbox-exec` with network access and
file writes denied. Native Make builds and the existing regression gate are
separate from these provisioning tests.

Development execution is qualified on macOS 27.2 arm64 and Ubuntu 26.04.1 x64.
Both hosts passed `tools-bootstrap`, `tools-install`, `tools-check`, `tools-tests`,
`tools-qualify`, `make verify` (including 742 frontend tests) and native `make build`.
Make-only recovery passed with independent binaries and the task-local mise
executable physically absent. macOS additionally passed `make lint-go` (zero
issues), `reuse lint` and whitespace checks.
The Linux export initially failed setup checks for missing tracked-file inventory
and missing `frontend/dist`; its disposable index and prepared frontend were
corrected before rerunning the gate. These were fixture setup failures, not
passing evidence or reasons to disable the checks. Existing macOS test-linker
minimum-version and Linux bundled-SQLite compiler warnings remain visible; this
work does not establish older-OS compatibility.

Raw logs and failed experimental hypotheses are retained privately under
`.tmp/adr-013-p1/`; they are not shipped artifacts. The source, tests and committed
artifact inventory reproduce the checks. No hosted CI pilot was created or run;
`tools-qualify` is the local pilot under the owner's local-CI constraint.
macOS Intel, Linux arm64/musl, Windows and unavailable minimum-version OS hosts
have not been qualified by P1. This does not change product support promises.

Maintainer acceptance of this tested configuration/version-control policy is
still required **before supplying mandatory lint tools**. P2–P9 remain separate.
ShellCheck/actionlint findings recorded in P0 are not suppressed by provisioning.
The next slice is P2's dialect-aware ShellCheck inventory and adjudication.
