<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Dependencies

GoPMgr is a CGO-enabled Go and Svelte desktop application. Dependency
changes affect build reproducibility, release packaging, security
posture, and validator coverage, so keep them intentional and verified.

## Toolchain

- Go: 1.27.3 minimum from `go.mod`; CI and release workflows select that version.
- Wails: v2.17.0.
- Node frontend: Node.js 26 in CI/release workflows, with Vite 8, Svelte 5,
  TypeScript 6, and npm scripts in `frontend/package.json`. Browser storage
  code uses `window.localStorage`, and Vitest workers disable Node's
  process-wide Web Storage so jsdom remains the authoritative per-test browser
  environment.
- CGO: required for the SQLite/SQLCipher driver path and the shipped DuckDB
  analytics build.

### Go security baseline, 2026-10-22

Go 1.27.3 includes the `crypto/x509` fix for
[GO-2026-6809](https://pkg.go.dev/vuln/GO-2026-6809). The previous Go 1.27.2
CI scan reported a call path from RFC 3161 token validation through PKCS#7
to `x509.Certificate.Verify`. Configured TSA trust chains containing
denylisted keys can now be rejected even when validating a token at its
historical generation time. This is an intentional trust-policy correction,
not a reason to bypass verification or restore the vulnerable toolchain.

Rebuild executables with Go 1.27.3 or later and replace running instances
to receive the fix; changing `go.mod` or updating the machine's Go
installation does not patch previously built binaries. No database or
wire-format migration is required. Keep
historical test reports at their measured versions. Local validation and its
platform/publication limits are recorded in
[the upgrade evidence](testing/go-1.27.3-security.md).

## Core Go Dependencies

- `github.com/wailsapp/wails/v2`: Desktop app runtime and Go/JS bridge.
- `github.com/mutecomm/go-sqlcipher/v4`: SQLCipher-capable SQLite driver
  registered through `internal/sqlitedriver`. The v4.4.2 module is replaced by
  the runtime-only copy in `third_party/go-sqlcipher` to carry SQLite's
  WAL-reset generation-check correction and failed driver-transaction cleanup. The
  replacement retains SQLite 3.33.0, SQLCipher 4.4.2, the LibTomCrypt provider, and the existing encrypted
  file format; provenance, hashes, and the removal procedure are recorded in
  `third_party/go-sqlcipher/GOPMGR.md`.
- `golang.org/x/crypto`: Argon2id and related cryptographic support.
- `golang.org/x/sys`: Windows `MoveFileEx` with replace-existing and
  write-through flags for atomic canonical-path publication after SQLCipher
  migration.
- `github.com/digitorus/pkcs7`: CMS/PKCS#7 parsing and OID support for
  PAdES-related code.
- `github.com/digitorus/timestamp`: BSD-2-Clause RFC 3161 request, token, and
  test-fixture primitives. `internal/rfc3161` adds GoPMgr's HTTPS, response
  binding, TSA certificate, size, redirect, and optional trust-root policy.
- `github.com/go-pdf/fpdf`: PDF generation (community-maintained continuation
  of the archived jung-kurt/gofpdf; see ADR-003).
- `github.com/gomutex/godocx`: DOCX generation.
- `github.com/xuri/excelize/v2`: XLSX generation.
- `github.com/rickar/cal/v2`: Country holiday calendars.
- `github.com/gorules/zen-go`: JDM launchpad template-seeding rules.
- `gonum.org/v1/gonum`: Numerical/statistical support.
- `gopkg.in/yaml.v3` and `github.com/pelletier/go-toml/v2`: build-time
  configuration parsing for `make config-check`. The isolated DF0 Cargo and
  Rust toolchain files are included in that syntax inventory; Cargo retains
  responsibility for their build semantics. These parser packages are imported
  only by the `scripts` command and are not linked into the GoPMgr desktop binary.
- `github.com/duckdb/duckdb-go/v2`: in-memory DuckDB analytics engine
  (ADR-002 decision), compiled under the `duckdb` build tag
  (`internal/analytics`). Production/package builds enable that tag so
  installers include the analytics engine; untagged developer builds link the
  stub. See `docs/design/duckdb-analytics-engine.md`.
- `google.golang.org/grpc`, `google.golang.org/protobuf`, and
  `github.com/Microsoft/go-winio`: approved ADR-010 G1 dependency pins in
  `internal/interop/deps_pin.go`, behind the unused `interop_deps_pin` tag.
  Production G1 contract generation and transport implementation are still
  pending. The approved transport requires native Unix sockets or Windows
  named pipes, with no TCP fallback. The separate ADR-010 and ADR-012 spikes
  are not production G1 implementations.

Check `go.mod` for the authoritative version list.

### ADR-010 worker interop dependencies (G1-1, 2026-10-26)

Approved by the maintainer under OWN-11, with conditions
([P04-D2](review/ADR-010-G1-P04-authorization-decision.md)).
The full review is in
[adr-010-g1-dependency-review.md](interoperability/adr-010-g1-dependency-review.md).

**Not yet compiled into GoPMgr.** Until G1-2 imports them, the modules
are pinned by `internal/interop/deps_pin.go`. Its build tag
(`interop_deps_pin`) is never set. GoPMgr's compiled package set is
identical with and without this change, test builds included: Linux,
macOS and Windows, untagged and with `duckdb`, `webkit2_41`,
`desktop,production` and their combinations.

**Modules a G1 build will compile** (the same on Linux, macOS and Windows):

| Module | Version | License | Status |
|---|---|---|---|
| `google.golang.org/grpc` | v1.84.0 | Apache-2.0, with `NOTICE.txt` | New direct |
| `google.golang.org/protobuf` | v1.36.12 | BSD-3-Clause | New direct |
| `github.com/Microsoft/go-winio` | v0.6.4 | MIT | New direct |
| `google.golang.org/genproto/googleapis/rpc` | v0.0.0-20260706201446-f0a921348800 | Apache-2.0 | New indirect (`google.rpc.Status`) |
| `golang.org/x/net` | v0.60.0 | BSD-3-Clause | Existing. **Keep at v0.60.0 or later** (GO-2026-6603, -6610, -6611, -6612, -6617) |
| `golang.org/x/text` | v0.43.0 | BSD-3-Clause | Existing. **Keep at v0.41.0 or later** (GO-2026-6629) |
| `golang.org/x/sys` | v0.49.0 | BSD-3-Clause | Existing |

**Versions raised in the module graph.** These modules were already in
GoPMgr's graph, declared by `arrow-go` (through DuckDB) and by Wails, but
no GoPMgr package compiled them:

- go-winio: v0.6.2 → v0.6.4;
- grpc: v1.78.0 → v1.84.0;
- protobuf: v1.36.11 → v1.36.12;
- genproto/googleapis/rpc: 2025-10-29 → 2026-07-06;
- go-cmp: v0.6.0 → v0.7.0, needed only by grpc's own tests. GoPMgr's code
  still uses no go-cmp.

**Graph-only additions.** gRPC's `go.mod` declares 32 more modules that
GoPMgr never compiles, for xDS, OpenTelemetry, OAuth2 and Google Cloud.
They are in the module graph only, recorded in `go.sum` where Go needs
their `go.mod` hashes:

- `cel.dev/expr`;
- `cloud.google.com/go/auth`, `cloud.google.com/go/compute/metadata`;
- `github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp`;
- `github.com/cncf/xds/go`;
- `github.com/envoyproxy/go-control-plane` (with `/envoy` and
  `/ratelimit`), `github.com/envoyproxy/protoc-gen-validate`;
- `github.com/felixge/httpsnoop`, `github.com/go-jose/go-jose/v4`;
- `github.com/go-logr/logr`, `github.com/go-logr/stdr`,
  `github.com/golang/glog`;
- `github.com/golang/protobuf` (grpc's tests only);
- `github.com/google/s2a-go`;
- `github.com/googleapis/enterprise-certificate-proxy`,
  `github.com/googleapis/gax-go/v2`;
- `github.com/planetscale/vtprotobuf`, `github.com/sirupsen/logrus`,
  `github.com/spiffe/go-spiffe/v2`;
- `go.opentelemetry.io/auto/sdk`, `go.opentelemetry.io/contrib/...` (two
  modules), `go.opentelemetry.io/otel` (with `/metric`, `/sdk`,
  `/sdk/metric` and `/trace`);
- `golang.org/x/oauth2`;
- `google.golang.org/api`, `google.golang.org/genproto/googleapis/api`.

A G1 package that would start compiling any of these needs a new review.

**Verification at G1-1:**

- `go mod verify` passes.
- All 12 new `go.sum` lines match `sum.golang.org`.
- govulncheck v1.9.0 (symbol scan):
  - the pinned set, for linux, darwin and windows: none;
  - the default build (`-tags webkit2_41`): none in GoPMgr's code;
  - `-tags duckdb,webkit2_41`: none.
- govulncheck module scan: one advisory, GO-2026-5932 (`x/crypto/openpgp`
  is unmaintained). It predates this change, and GoPMgr does not call it.
- A program that links grpc, protobuf and go-winio builds bit-for-bit
  identically from two cold caches with `-trimpath`. Checked for
  linux/amd64, linux/arm64, darwin/arm64, windows/amd64 and windows/arm64.
- `reuse lint`: compliant.

**Release notices (current packaging).**

- gRPC and genproto are Apache-2.0, and gRPC ships a `NOTICE.txt`.
- Package scripts now copy SQLCipher, font, Go-module, DuckDB-native and
  frontend notices. `scripts/verify-module-notices.sh` checks the binary's
  linked modules before packaging; the native notices have an additional
  reviewed-input check. See [LICENSES.md](../LICENSES.md).
- Before G1 ships, regenerate the Go-module notices for the resulting linked
  package set and qualify its native packages. Notices do not themselves
  authorize redistribution or satisfy the independent LC-GATE.
- Production G1 worker release integration is still pending.

**Production Java worker dependencies approved for G1-8 (not yet integrated):**

- grpc-java 1.85.1;
- Netty, the latest 4.2.x patch when G1-8 starts. NIO only, with no
  epoll, kqueue or unix-common native libraries;
- protobuf-java 3.25.x;
- Temurin 21 LTS for jlink, and Maven 3.9.x, both pinned and
  checksum-verified as in the G0 toolchain script.

The dependency review §3 lists the artifacts.
`spikes/adr010-g0-transport/worker/pom.xml` belongs to the isolated transport
proof, not a shipped worker.

### ADR-012 isolated Rust dependency security gate

The DF0 workspace is outside the application build. Its four temporary source
overrides select `zstd` 0.14.3 and `zstd-safe` 8.2.0 without retaining the old
vulnerable pair. The upstream edits are five dependency constraints and marked
modification notices; Rust code and features are preserved. The [maintenance record](../spikes/adr012-datafusion/third_party/GOPMGR.md)
records origin checksums, licensing, source guards, ownership and removal.
The [Zstd assessment](testing/adr-012-df0-zstd-assessment.md) distinguishes the
earlier vulnerable baseline from adoption evidence. This is spike-only
authorization, not a dependency waiver or production replacement approval.

## Frontend Dependencies

Runtime:

- `chart.js`: Chart rendering in the frontend.
- `read-excel-file`: `.xlsx` parsing for the Sigma data import. Replaced
  the npm `xlsx` (SheetJS) package (commit `8d58828`), which was frozen
  at 0.18.5 with unpatched prototype-pollution and ReDoS CVEs — a
  permanent Dependabot dead-end since SheetJS ships fixes only via their
  own CDN, not npm. No ADR exists for this swap; legacy binary `.xls` is
  no longer supported as a result (`.xlsx`-only).

Development:

- Svelte 5, Vite 8, TypeScript 6, svelte-check, ESLint 10,
  eslint-plugin-svelte 3, Tailwind CSS 4, PostCSS, and
  `@tailwindcss/postcss`.

Check `frontend/package.json` for the authoritative version list.

### Advisory review, 2026-10-11

The frontend lockfile resolves `source-map-js` 1.2.2 and
`postcss-selector-parser` 7.1.6, removing the resolved package versions
listed by GHSA-68fv-2mgg-jv7q and GHSA-rj75-hqrm-r3gf without changing
manifest ranges. These are transitive
build/test/lint dependencies, not evidence of a desktop request-path exploit.
`npm --prefix frontend audit` checks the resolved dependency graph; it does
not inventory code embedded in published bundles. Magicast 0.5.4, used by
Vitest coverage tooling, still embeds source-map-js 1.2.1. The published
Magicast 0.5.5 bundle also contains that copy. Its attacker-controlled
indexed-map reachability has not been established; do not claim this copy
was fixed by refreshing the separately resolved package.

Vite 8.3.2 also embeds postcss-selector-parser 7.1.1 in its CSS Modules
pipeline. The published Vite 8.3.3 and 8.3.4 bundles retain the quadratic
index searches. Attacker-controlled CSS Modules therefore retain the
reported CPU-exhaustion path; a resolved-package refresh or override does
not replace that embedded code. No desktop request-path exploit was
established. Check the published bundle and exercise Vite's `.module.css`
pipeline before claiming an eventual fix.

The former Excelize v2.11.0 pin was affected by 15 package-level advisories
in the 2026-10-11 GitHub snapshot. The stable release list still ends at
v2.11.0. To include those upstream fixes, GoPMgr deliberately pins
`v2.11.1-0.20261003002531-6258dcebc4e2`, the immutable
[upstream revision](https://github.com/qax-os/excelize/commit/6258dcebc4e2a2aed985c38a08098dfd908521d1)
identified by the newest of those advisories. This is a pre-stable
pseudo-version, not a released v2.11.1. Its Go 1.26 minimum is satisfied by
the then-current Go 1.27.2 pin. It also raises `mscfb` to 1.0.8, `efp` to
0.0.2, `x/image` to 0.44.0, and `x/net` to 0.59.0.

This revision changes automatic float writing to 15 significant digits.
The schedule exporter therefore writes Go's shortest round-trip decimal
representation through `SetCellDefault` for all six finite numeric columns,
retaining numeric cells without the automatic truncation. Only formatted
floats reach this API, never user text. NaN and infinity retain text cells;
IDs/titles remain strings and Critical remains boolean. Raw worksheet XML
tests detect truncation independently of Excelize's reader, alongside
sheet/order/metadata/formula checks. A separate
regression verifies overflowing column rejection with valid boundary controls;
it does not independently exercise all 15 advisory fixes. GitHub alert closure
requires a dependency refresh after publication and is separate evidence.

GoPMgr's production Excelize caller constructs new XLSX exports with internal
sheet and cell coordinates; it does not open supplied workbooks or evaluate
formulas. Sigma XLSX import instead uses `read-excel-file/browser`. The prior
reachability review did not demonstrate a production route to the reported
malformed-workbook or uncontrolled-coordinate sinks. This is not a claim
that Excelize is universally safe. Reassess before adding an Excelize import
path, and replace the pseudo-version only through compatibility verification.

## External Tools

Some gates use optional or required tools outside Go/npm:

- `reuse`: REUSE/SPDX license checks.
- `veraPDF`, Docker with the veraPDF image, or Java to run the SHA-256-pinned
  veraPDF installer the gate downloads: strict PDF/A-2u validation.
- `qpdf`: PDF syntax validation in external PAdES checks.
- `pdfsig`: Poppler signature validation in external PAdES checks.
- `openssl`: CMS ASN.1 and detached signature verification.
- `dss-validation-tool`: DSS PAdES baseline classification when
  installed.
- `wails`: development server and desktop packaging workflow; also builds
  the Windows NSIS installer (`wails build -nsis`).
- `nfpm` v2.47.0: builds the Linux `.deb` and `.rpm` packages from
  `build/linux/nfpm.yaml`.
- `hdiutil`: the built-in macOS release path for the `.dmg`. `create-dmg`
  remains an optional local-only presentation path when
  `GOPMGR_FANCY_DMG=1`; the tag workflow does not install or enable it.
- NSIS (`makensis`) 3.12.0: the Chocolatey-pinned toolchain behind
  `wails build -nsis` on Windows.
- `ripgrep` (`rg`): required by release-scope and packaging source contracts;
  the tag preflight installs and discovers it explicitly instead of depending
  on the current hosted-runner image.

`scripts/release-tool-versions.env` is the version record for installer tools
that are fetched by the tag workflow. `make installer-tool-pins` verifies the
workflow, the record, and local Linux packaging guidance as one contract.
GoPMgr tracks `build/windows/installer/project.nsi`, `info.json`, and the
application manifest. The pinned Wails CLI regenerates `wails_tools.nsh`, the
WebView2 bootstrapper, and platform icon during native builds; those derived
files remain ignored.

`make check-release` is strict where release correctness requires proof.
If a required validator is missing, install the tool rather than
weakening the release claim.

## Dependency Change Rules

1. Read the current code path before adding a dependency.
2. Prefer standard library or existing dependencies when they are
   adequate.
3. Avoid dependencies that duplicate existing project abstractions.
4. For security-sensitive dependencies, inspect maintenance status,
   licenses, native build requirements, and release packaging impact.
5. Run `go mod tidy` or npm install only when dependency metadata should
   actually change.
6. Verify with focused tests plus the relevant release gates.
7. After any `go.mod` change, including Dependabot updates, run
   `make module-notices` and commit the regenerated
   `third_party/GO-MODULE-NOTICES.txt`. Its diff shows every license text
   that changes. `make verify` fails until the file is current; see
   [LICENSES.md](../LICENSES.md#go-module-notices-in-release-packages).
8. A duckdb-go-bindings update also changes the prebuilt DuckDB libraries
   and the third-party code inside them. `make verify` fails until the
   DuckDB native notices are reviewed for the new release; see
   [LICENSES.md](../LICENSES.md#duckdb-native-library-notices). It also
   fails if reviewed native inputs change or the TPC exclusion audit cannot
   complete; never remove an entry from `third_party/EXCLUDED-NATIVE-CODE.txt` to get past
   that (see
   [LICENSES.md](../LICENSES.md#excluded-the-tpc-h-and-tpc-ds-generators)).
9. After a frontend dependency change, or a new import of an npm package,
   run `make frontend-notices` and commit the regenerated
   `third_party/FRONTEND-NOTICES.txt`; `make verify` fails until it is
   current. See [LICENSES.md](../LICENSES.md#frontend-package-notices).

The notices generator reads Go's LICENSE and PATENTS from `GOROOT`. For a
Homebrew-style `libexec` GOROOT, a missing file may be read from that same
installation's parent directory. Existing root files take precedence;
unreadable, empty or invalid text remains an error rather than triggering
fallback. License selection does not change the linked-module inventory.

For SQLCipher specifically, remember that the selected driver owns the
SQLite implementation in the binary. Driver changes need encryption
tests, migration tests, build checks, packaging review, and a causal WAL-reset
regression when the embedded SQLite core changes. Do not remove the local
replacement until an upstream driver release containing both corrections passes
the COMMIT/ROLLBACK regressions and the same encrypted-file interchange checks.
