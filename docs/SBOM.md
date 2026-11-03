<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Software bill of materials (SBOM)

Every GoPMgr release package gets two SBOMs, written from one inventory:

- `<package>.cdx.json`: CycloneDX 1.7
- `<package>.spdx.json`: SPDX 3.0.1, JSON-LD serialization

For example, `GoPMgr-1.2.0-arm64.dmg` is published with
`GoPMgr-1.2.0-arm64.dmg.cdx.json` and `GoPMgr-1.2.0-arm64.dmg.spdx.json`. They
are attached to the GitHub release next to the package. Once a signing key is
published, each SBOM also has a detached signature, `<sbom>.sig`.

This page maps GoPMgr's SBOMs to CISA's *2026 Minimum Elements for a
Software Bill of Materials* (version 2.1, 29 July 2026). It says which elements are met, which go beyond
the minimum, and which are not met yet.

## Verify an SBOM

From a checkout of the release tag:

```sh
go run ./tools/sbom verify GoPMgr-1.2.0-arm64.dmg.cdx.json GoPMgr-1.2.0-arm64.dmg.spdx.json
```

`verify` checks each `.sig` file against the public key committed at
`tools/sbom/signing-key.pub`. It also checks that the SBOM names that key, by
SHA-256 of the raw key, as its signing key. To confirm that an SBOM describes
the package you have, compare the package's SHA-256 with the root component's
hash (`metadata.component.hashes` in CycloneDX, the root package's
`verifiedUsing` in SPDX).

## CISA data fields

"Unknown" means the SBOM states the field is unknown and why. Unknowns are
CycloneDX properties named `gopmgr:unknown:<field>` and SPDX `Annotation`
elements whose statement starts with `gopmgr:unknown:<field>:`.

| Element | CycloneDX 1.7 | SPDX 3.0.1 | Status |
|---|---|---|---|
| SBOM Author | `metadata.authors` | `CreationInfo.createdBy` (Organization) | Met: James L. Burns and The GoPMgr Contributors |
| SBOM Author Signature | detached `<sbom>.sig`, named by the `gopmgr:signature` property | detached `<sbom>.sig`, named by a `gopmgr:signature` annotation | **Not met yet.** The tooling is built and tested; the key does not exist yet (see below) |
| SBOM Data Format Name | `bomFormat` | JSON-LD `@context` (SPDX 3.0.1) | Met |
| SBOM Data Format Version | `specVersion` (1.7) | `CreationInfo.specVersion` (3.0.1) | Met; both are the current versions, neither deprecated |
| SBOM Generation Context | `metadata.lifecycles[].phase` = `build` | `software_Sbom.software_sbomType` = `build` | Met |
| SBOM Timestamp | `metadata.timestamp` | `CreationInfo.created` | Met; RFC 3339 UTC, which RFC 9557 extends |
| SBOM Tool Name | `metadata.tools.components[].name` | `Tool.name` | Met: GoPMgr SBOM generator (tools/sbom) |
| SBOM Tool Version | `metadata.tools.components[].version` | in `Tool.name` | Met: the source commit, or "unknown" |
| SBOM Version | `version`, with an RFC 9562 UUID `serialNumber` | `SpdxDocument.externalIdentifier` ("SBOM version", "SBOM serial number") | Met |
| Component Producer | `manufacturer` | `originatedBy` (Agent) | Met; unknown where the component names no producer |
| Component Dependency Relationship | `dependencies[].dependsOn` | `Relationship` `dependsOn`, `completeness: complete` | Met |
| Component Hash Value | `hashes[].content` | `verifiedUsing[].hashValue` | Met; see [hashes](#hashes) |
| Component Hash Algorithm | `hashes[].alg` | `verifiedUsing[].algorithm` | Met: SHA-256 and SHA-512 (IANA `sha-256`, `sha-512`) |
| Component Identifiers | `purl`, `cpe`, `gopmgr:git-commit`, `gopmgr:go-module-sum` | `software_packageUrl`, `externalIdentifier` | Met: every component has at least one |
| Component License | `licenses[].expression` | `hasDeclaredLicense` relationship to a license expression | Met; unknown only for platform components GoPMgr does not ship |
| Component Name | `name` | `name` | Met |
| Component Version | `version` | `software_packageVersion` | Met; unknown where the producer publishes none |

Each format maps CISA's hash names to its own spelling: CycloneDX `SHA-256`,
SPDX `sha256`.

## CISA practices and processes

| Practice | How GoPMgr meets it |
|---|---|
| Machine-Processable Data | Both widely used open formats, each checked against its official schema (below) |
| Coverage | Every Go module linked into the shipped executable, the Go standard library, native code inside those modules (including the WebView2 bootstrapper that the Windows executable and installer carry, and the 33 third-party libraries compiled into DuckDB's static libraries, each with a dependency from DuckDB), every npm package in the embedded web bundle, every embedded third-party file, and the platform components the app needs but does not ship |
| Frequency | The release workflow writes the SBOMs for every package of every release |
| Distribution and Delivery | Attached to the public GitHub release beside each package |
| Explicitly Identifying Unknown Information | Every missing field is marked unknown with a reason. The SBOM author withholds nothing, and each SBOM says so (`gopmgr:withheld`) |
| Accommodation of Updates to SBOM Data | A corrected SBOM is reissued for the same release with a higher SBOM version and a new serial number, and replaces the release asset. Report SBOM errors through the GitHub repository's issue tracker |
| SBOM Author Signature (validation) | Ed25519, detached, verified by `tools/sbom verify`; not met until the key exists |

## Beyond the minimum

- **Two formats from one inventory.** CycloneDX and SPDX cannot disagree.
- **Taken from the shipped executable.** The module list comes from the
  build information Go records in the executable. It is cross-checked with
  `go list` for the same build tags, and any difference stops the release.
  On macOS the executable is re-read from inside the `.dmg`. On Linux and
  Windows it is the build output, which the packaging copies unchanged; that
  copy is assumed, not checked.
- **Two executable-artifact hashes.** One for the installer package, one for
  the executable inside it.
- **Reviewed coverage, failing closed.** `make sbom-check`, part of
  `make verify`, fails when a linked module, an embedded file or a bundled npm
  package has no reviewed entry in `tools/sbom/components.json`, or when an
  entry no longer ships.
- **Every embedded file is accounted for.** A file GoPMgr's own packages
  embed must sit under a reviewed third-party folder or a declared
  first-party one (`frontend/dist`, `internal/templates`). The built
  frontend's copies of `frontend/public` fonts are reviewed under their
  source. Anything else stops the check.
- **Binaries inside dependencies.** A dependency that embeds an executable or
  library file (`.exe`, `.dll`, `.so`, `.dylib`, `.a`, `.lib`) with
  `//go:embed` stops the check until it has a reviewed entry. This found
  Microsoft's WebView2 bootstrapper, which Wails embeds on Windows.
- **Platform dependencies.** WebKit, WebKitGTK, GTK and WebView2 are listed
  as runtime dependencies and marked not shipped (`gopmgr:shipped=false`).
- **Archive hashes are labelled.** Each hash says what it was computed over.

## Hashes

| Hash kind | What it covers | CISA Component Hash? |
|---|---|---|
| `installer` | The release package (`.dmg`, `.deb`, `.rpm`, `.exe`) | Yes |
| `executable` | The GoPMgr executable inside the package | Yes |
| `go-module-archive` | The module `.zip` from the Go module cache, checked against the `h1:` sum recorded in the executable before it is used | No: a source archive |
| `npm-package-archive` | The npm package tarball, from `package-lock.json` integrity | No: a source archive |
| `embedded-file` | A font or colour-profile file embedded in the executable | Yes, for that file |
| `file-in-go-module` | A file another producer ships inside a Go module, such as the WebView2 bootstrapper | Yes, for that file |

A `go.sum` `h1:` value is not a hash in the IANA sense. It appears only as an
identifier (`gopmgr:go-module-sum`). Components compiled from source in the
repository, the Go standard library, and components GoPMgr does not ship have
no hash and say why.

## Current unknowns

In the macOS SBOM of 2026-11-02 the unknowns are:

- **Producer:** three Go modules whose license files name no copyright
  holder (`mscfb`, `msoleps`, `go-reflector`), SQLite and LibTomCrypt, and
  three npm packages that declare no author (chart.js, svelte, tailwindcss).
- **Version:** LibTomCrypt, the sRGB colour profile, the IBM Plex Sans and
  Mono web fonts, and WebKit.
- **Hash:** the Go standard library; the local SQLCipher module and the
  SQLite, SQLCipher and LibTomCrypt code compiled from it in the repository;
  DuckDB's native library, which its parent module's archive hash covers;
  and WebKit.
- **License:** WebKit, which GoPMgr does not ship.

The Windows SBOMs also list the Microsoft Edge WebView2 Runtime Evergreen
Bootstrapper. Its hash is known, but its version is not recorded. Microsoft's
terms for it have no SPDX identifier, so its license is unknown, with a
pointer to Microsoft's WebView2 page.

Producers are the copyright holders that each component's own license file
names, not organization names looked up elsewhere. License identifiers are
as declared by each component. They are checked for SPDX expression syntax,
not against the SPDX License List, and are not legal conclusions.

## Create the signing key (owner, once)

The SBOM key is separate from the update-manifest key and signs only SBOMs.

```sh
go run ./tools/sbom keygen -private-out sbom-signing.key > tools/sbom/signing-key.pub
```

1. Add the contents of `sbom-signing.key` as the GitHub Actions secret
   `GOPMGR_SBOM_PRIVATE_KEY`, then delete the local file. Never commit it.
2. Commit `tools/sbom/signing-key.pub`. Its REUSE annotation is already in
   `REUSE.toml`.

From then on, the release fails if the secret is missing or does not match
the committed public key. Until then, releases attach unsigned SBOMs, each
SBOM says it is unsigned, and the workflow prints a warning.

## How the release workflow builds them

1. After `wails build`, and before the macOS job clears its Go caches,
   `tools/sbom generate -inventory-out` records the inventory. It also
   records a fingerprint of the executable's build information.
2. After packaging, `tools/sbom render` writes both SBOMs for each package.
   It hashes the package and the executable as shipped, and refuses an
   executable whose build information differs from the fingerprint. Code
   signing changes an executable's bytes, not its build information, so this
   works once signing is enabled.
3. The publish job signs and verifies the SBOMs when the key exists, then
   attaches them to the draft release.

To make SBOMs for a local build:

```sh
go run ./tools/sbom generate -target darwin/arm64 \
  -binary build/bin/gopmgr.app/Contents/MacOS/gopmgr \
  -artifact build/packages/<package>.dmg -version <version> \
  -commit "$(git rev-parse HEAD)" -out-dir build/packages
```

## Maintenance

- Run `make sbom-check` after changing `go.mod`, the frontend dependencies or
  the embedded assets. Add or remove entries in `tools/sbom/components.json`
  to match.
- Native code is listed under the module that links it. When DuckDB is
  removed from `go.mod`, its native entries become stale and `sbom-check`
  says so; delete them then.

## Verification evidence (2026-11-02)

On a local macOS arm64 build at commit `3ac1f96`, with its `.dmg`:

- **Output:** both SBOMs listed 105 components.
- **Hashes:** the executable hash matched the executable inside the `.dmg`,
  and the root hash matched the `.dmg`.
- **CycloneDX:** passed the official CycloneDX 1.7 JSON schema (`ajv` 6.15).
  Four corrupted copies were rejected.
- **SPDX:** passed the official SPDX 3.0.1 JSON schema and SHACL model, with
  `jsonschema` 4.26.0, `pyshacl` 0.40.2 and `rdflib` 7.6.0. Five corrupted
  copies were rejected.
- **Signing:** with a throwaway key, signatures verified. A one-byte change,
  the wrong key, and a private key that does not match the published key were
  each refused.
- **Two-phase release path:** produced the same SBOM as generating in one
  step, and refused a different executable.
- **Tests:** `tools/sbom` tests check every CISA field in both formats.
  Fourteen seeded faults were each caught, two of them by `make sbom-check`
  against the real repository.

Not verified: Linux and Windows packages were not built here, and the
release workflow's SBOM steps run for the first time in hosted CI. For
Windows, the bootstrapper's hash lookup was checked against the module cache. Schema
validation is a manual step and not part of `make verify`, because it needs
the downloaded schemas.
