<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-012: Rust 1.99.1 qualification

The maintainer directed active Rust tooling to use 1.99.1 on 2026-10-30.
This updates the isolated DF0 spike, not the production application. The
workspace and code generator retain MSRV 1.94 and their existing dependency,
feature and lockfile selections. No minimum-compiler test is implied.
This records the compiler slice before the later, separately approved
[Zstd remediation](adr-012-df0-zstd-assessment.md); current dependency results
are recorded there rather than substituted into these historical measurements.

## Selection contract

`rust/rust-toolchain.toml` pins 1.99.1. The reproduction and generation scripts
use `scripts/with-rust-toolchain.sh` to resolve that toolchain's Cargo through
rustup, prepend its binary directory, set `RUSTC`, `RUSTDOC` and
`RUSTUP_TOOLCHAIN`, and dispatch through `rustup run`. Cargo is passed by its
resolved absolute path. Rustup supplies its runtime library search paths and
may prepend its proxy directory or normalize the toolchain name with the host
triple. Missing tools or a failed lookup stop execution;
there is no automatic installation or fallback.

This is necessary when a separately packaged Cargo precedes rustup proxies on
PATH. On the validation host, Homebrew Cargo rejected `+toolchain` syntax, and
`rustup run 1.99.1 sh` still found Homebrew rustc through PATH. Direct rustup
selection reported LLVM 23.1.1; Homebrew reported LLVM 23.1.3. Matching the
release number alone did not establish identical compiler provenance.

The launcher changes its child environment only. It preserves arguments,
working-directory identity, target directory and exit status. Existing compiler
wrappers are not neutralized: qualification records their effective settings.
The Go evidence collectors run through the same launcher so their ambient
compiler probes agree, but those probes alone do not prove how a binary was
built. Retain build logs and source/lock/compiler/artifact hashes together.

## Behavioral verification

`go test -count=1 ./scripts -run 'TestRustToolchain|TestDF0Entrypoints'` passes
on macOS arm64. The fixtures read the real TOML pin, then execute isolated
stand-ins for rustup, Cargo, rustc and rustdoc. They verify:

- shadowed tools and ambient compiler overrides do not win;
- empty, quoted and space-bearing arguments retain their boundaries;
- working-directory identity, target directory and nonzero status survive;
- partial-output lookup failure, runtime refusal and missing selected tools
  cannot start Cargo;
- all three reproduction/generation entrypoints select the reviewed tools,
  including the Go collectors' compiler probes.

A passing control and 15 seeded faults in disposable copies demonstrate test
sensitivity, including bypassing either rustup dispatch, dropping compiler
selection, splitting arguments, hiding failures, changing directories and
restoring each old entrypoint. Faults failed the intended assertions, rather
than compilation. Complete Cargo sequences and invocation counts are checked.

These are executable routing and failure fixtures, not native Rust execution
or protobuf regeneration. Windows skips the POSIX fixture; native Windows
qualification remains outstanding.

## Fresh native qualification

Validation host: macOS arm64, using rustup rustc 1.99.1 commit
`58cf8f9c35b355199727627d07ae34a4e2846d67`, LLVM 23.1.1, and Cargo commit
`5f94df4789f005f9a352888e8355ffc645b7ed0e`. Ambient compiler overrides,
wrappers, Rust flags and `RUSTC_BOOTSTRAP` were unset. A fresh external
`CARGO_TARGET_DIR` and `CARGO_BUILD_JOBS=2` avoided earlier build artifacts.

The initial release test run passed but warned that `rust-objcopy` could not
load `libLLVM.dylib`. Direct compiler invocation bypassed rustup's library
setup ([upstream loader issue](https://github.com/rust-lang/rust/issues/163947)).
Dispatch through `rustup run` corrected this; after cleaning the three
workspace packages, recompilation and all nine release tests passed without
that warning. The pinned objcopy also executed successfully. This is why
routing fixtures alone did not establish native build readiness.

Both workspace and generator formatting and release Clippy checks pass with
warnings denied. Clippy found an unnecessary cloned generator include path;
[`std::slice::from_ref`](https://doc.rust-lang.org/std/slice/fn.from_ref.html)
preserves the value without copying and predates the declared MSRV. The
generator's release build passes, as does the locked standalone workspace
release build, with no warnings.

The native CLI run uses the unchanged seeded corpus (`df0-fixtures -check`:
94 cases). Both portfolio candidates match the legacy oracle in 24/24 cases.
All 69 non-performance tabular results and all four filesystem probes equal
the historical DF0-B records after removing only `wall_ns`; portfolio records
also match. The large performance case is excluded by the existing harness.
This preserves existing verdicts, including known losses, ambiguities and
valid-input rejections; it does not qualify import parity. The worker rejects
empty and oversized bootstraps with exit 2. The generator runs and rejects
missing arguments with exit 1.

To repeat the native build checks from `spikes/adr012-datafusion/rust`, set a
fresh external `CARGO_TARGET_DIR` and run each Cargo command through
`bash ../scripts/with-rust-toolchain.sh`:

```sh
bash ../scripts/with-rust-toolchain.sh cargo test --locked --release --workspace
bash ../scripts/with-rust-toolchain.sh cargo build --locked --release --workspace
bash ../scripts/with-rust-toolchain.sh cargo fmt --all -- --check
bash ../scripts/with-rust-toolchain.sh cargo clippy --locked --release --workspace --all-targets -- -D warnings
```

The standalone generator repeats the build, format and Clippy checks from
`rust/codegen` using `bash ../../scripts/with-rust-toolchain.sh` and its own
external target directory. For native candidate evidence, from the spike root,
set `corpus` and `out` to fresh directories outside tracked evidence and
`target` to the build's `CARGO_TARGET_DIR`, then run:

```sh
go run ./cmd/df0-fixtures -out "$corpus" -check
bash scripts/with-rust-toolchain.sh go run ./cmd/df0-candidates \
  -corpus "$corpus" -cli "$target/release/df0-cli" -out "$out"
```

Do not overwrite historical records when collecting compiler qualification.

Artifact SHA-256 from this macOS arm64 run:

| Artifact | SHA-256 |
|---|---|
| `df0-cli` | `f0775a6e0c6e19e88275b23caf605c162d8d0956a82ade1ce4ee7c789aef2c63` |
| `df0-worker` | `bcec27b15486ec2a5fe1d37350e958d1f79c53d4cf17f78ec3f828b4878e3583` |
| `df0-codegen` | `1b411740be06c759dc1a85430f5c1199373d93aee9de100ea4ff7092dfeb7945` |

The build source is base `9582f9f` plus this toolchain slice. SHA-256 over
sorted tracked paths relative to `rust/`, each followed by a NUL and its file
bytes (including manifests, locks, sources and toolchain pin), is
`35545c507fd98bdb25db96bd6bae3f6776ac60b340612fd41ae5d02ff01a3b4b`.
Workspace lock SHA-256:
`682d7c39f6ce319e3fb6aefafa95f965ae2716d15e59c2d4511ae032b9b3d717`.
Generator lock SHA-256:
`272835398c525d4ff3d0e4bde672efe65df44af1a82df947408c23b475d64222`.
Both locks are byte-identical to base. These are local artifact records, not
published releases or cross-platform reproducibility claims.

Repository checks also pass: `make verify`, `make coverage-ledger-drift`,
`make license-check`, `make release-scope`, `go test -race -count=1 ./scripts`,
`go vet ./scripts`, spike candidate/internal Go tests, shell syntax and diff
whitespace checks. Darwin `ipchost` has no test files; it is not an IPC test
pass. No production code or dependency selection changed.

Historical DF0-B/C JSON records remain unchanged; these compiler checks do
not rerun or supersede the Linux measurements.

The compiler review uses the official
[1.99.0 notes](https://blog.rust-lang.org/2026/10/01/Rust-1.99.0/) and
[1.99.1 corrections](https://blog.rust-lang.org/2026/10/15/Rust-1.99.1/).
The latter include x86_64 FFI boolean-return and macOS linker corrections;
macOS arm64 execution does not establish native x86_64 or Windows behavior.
No newly stabilized API or unsafe operation is introduced by this update.

## Dependency audit

At toolchain qualification,
`cargo-audit audit --file spikes/adr012-datafusion/rust/Cargo.lock --json`
**failed** with three vulnerabilities: RUSTSEC-2026-0391 in `zstd` 0.13.3 and
RUSTSEC-2026-0403 / RUSTSEC-2026-0398 in `zstd-safe` 7.3.0. It also reported ten
unsoundness warnings in that old wrapper. The standalone generator lockfile's
audit passed with no warnings. Neither lockfile was changed or filtered by
the compiler slice.

These findings predate this toolchain change. Locked metadata enables only
`std` in the old `zstd-safe` and shared `zstd-sys`, without `experimental`,
`seekable`, `zstdmt` or `legacy`. The inspected Parquet and Arrow IPC callers
use bulk contexts; DataFusion's synchronous compressed reader creates a fresh
read decoder. No borrowed-stream-context, dictionary-training or mixed
stream/one-shot call was found in the owned sources or these callers. This
bounded inspection does not establish exploitability or security clearance.

The findings were left open for a separate dependency disposition before
qualifying the reader for production or advancing its security acceptance.
Changing dependency baselines or disabling supported codecs required a
separate reviewed slice, rather than an undocumented compiler-update side effect.
The follow-up [Zstd assessment](adr-012-df0-zstd-assessment.md) records the
subsequently approved temporary spike-only overrides and active graph's audit.
The vulnerable baseline above remains historical evidence, not the current
dependency selection.

## Remaining gates

The separate Zstd remediation permits isolated DF0-C2 bounded host-mediated
read work to resume; that phase remains NOT-RUN. This toolchain update does
not qualify the replacement, change SQLCipher authority, remove DuckDB, or
satisfy native-platform, no-spill, packaging/licensing or production-cutover
gates. Native Linux/Windows execution, MSRV compilation and the complete
macOS IPC suite were not run for this slice. Darwin process inspection is
unimplemented. Protobuf regeneration was not run because its pinned protoc
and Go generators were unavailable; committed bindings are unchanged, and
compilation plus disposable routing fixtures do not replace regeneration.
Miri/sanitizers and production PDF, signing and installer qualification were
not run: this slice adds no unsafe operation or production artifact. See the
[DF0 evidence matrix](adr-012-df0-evidence.md).
