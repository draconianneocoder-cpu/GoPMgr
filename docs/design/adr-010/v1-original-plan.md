<!--
SPDX-FileCopyrightText: 2026 GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-010: Secure, Local-First Plugin Architecture for GoPMgr

**Status:** PROPOSED — design for maintainer review; **not implemented or accepted**  
**Decision date:** 2026-10-23  
**Target repository:** [`draconianneocoder-cpu/GoPMgr`](https://github.com/draconianneocoder-cpu/GoPMgr)  
**Recommended location:** `docs/design/ADR-010-plugin-architecture.md`  
**Design reference:** inspected `main` at `a38b06542365cc74ec656839188ac63e6c2e8da8` (2026-10-23); recheck HEAD before implementation  
**Decision owner:** GoPMgr maintainer  
**First reference plugin:** MPXJ schedule interoperability (bundled, Java, local only)  
**Future consumers:** optional AI model providers, MCP gateway, reporting/analytics adapters, other file-format integrations  
**Supersedes:** no prior ADR automatically; any direct conflict must be identified and decided explicitly  
**Review scope:** architecture, trust boundaries, contracts, implementation sequencing, acceptance tests, release packaging, licensing, rollback, and agent execution instructions

> **Approval boundary.** This document is a proposed architectural decision and an implementation plan, not evidence that any code exists, that any test gate has passed, or that the maintainer has accepted a dependency. An agent must begin by validating current repository state and recording any differences from this reference. It must not overwrite current work, silently alter existing ADRs, change unrelated database decisions, or claim acceptance on the maintainer's behalf.

---

## Document navigation

| Decision and constraints | Implementation and assurance |
|---|---|
| [0. Executive decision](#0-executive-decision-and-implementation-order) | [11. Scale and performance](#11-data-scale-performance-and-capacity-design) |
| [1. Evidence baseline](#1-evidence-baseline-verified-repository-facts-vs-proposals) | [12. Delivery phases G0–G7](#12-phased-delivery-plan-work-breakdown-and-hard-gates) |
| [2. Scope and success criteria](#2-scope-success-criteria-and-exclusions) | [13. Pull-request sequencing](#13-dependency-graph-and-recommended-pull-request-slices) |
| [3. Alternatives](#3-alternatives-and-decision-rationale) | [14. Verification and testing](#14-verification-and-test-assurance-plan) |
| [4. Architecture/ownership](#4-reference-architecture-and-component-ownership) | [15. Release and rollback](#15-release-migration-and-rollback-plan) |
| [5. Bundles and trust](#5-plugin-package-format-discovery-and-trust) | [16. Risk register](#16-risk-register-and-mitigations) |
| [6. RPC contracts](#6-local-ipc-service-definitions-and-protocol-guarantees) | [17. Maintainer decisions](#17-review-checkpoints-and-maintainer-decisions) |
| [7. Security and authorization](#7-security-identity-confidentiality-and-authorization) | [18. Coding-agent instructions](#18-coding-agent-execution-instructions-copyable-mission-brief) |
| [8. Data fidelity and mapping](#8-high-fidelity-project-interchange-model) | [19. Adversarial review](#19-architecture-quality-checklist-and-adversarial-self-review) |
| [9. MPXJ specification](#9-detailed-mpxj-plugin-specification) | [20. References](#20-reference-material-and-source-of-truth-priority) |
| [10. AI and MCP](#10-future-ai-integrations-and-mcp-extension-boundary) | |

---

## 0. Executive decision and implementation order

**Decision:** Introduce a **language-neutral, out-of-process, capability-scoped plugin boundary**, owned by a Go plugin supervisor. The core application remains Go/Wails/Svelte and remains authoritative for project identity, encryption, transactions, scheduling, EVM, WBS, audit, and durable storage. Plugins supply specialized computations and file transformations. Plugins **never receive** SQLCipher keys, database handles, administrator escrow material, or unrestricted application APIs.

**Protocol decision:** Define a versioned Protocol Buffers API with local gRPC streaming and narrow service-specific contracts. **Evaluate** `hashicorp/go-plugin` as the supervisor implementation in a mandatory cross-language spike; adopt it only if a Java MPXJ worker passes the actual handshake/lifecycle/security/packaging gates. The fallback is a small Go-owned subprocess supervisor exposing **the same protobuf services**, not a change to the application contract. Do not conflate selection of gRPC with irreversible selection of a particular process manager. No network-based or public remote plugin service is authorized by this ADR.

**First delivery:** A **bundled, release-trusted, offline MPXJ plugin**, initially read-only, with accurate import coverage, preview, diagnostics, and transactional staged commit. Existing MSPDI import/export must continue to work without the plugin. The plugin is an optional augmentation, not a new dependency of core scheduling or project opening.

**Later deliveries:** Format export, AI-provider adapters, an opt-in read-only MCP server/gateway, proposal-only AI edits, and finally user-approved write transactions. General third-party/community plugins and plugin auto-updates **are not authorized** until separate signing, OS sandbox, provenance, and release gates pass.

**Non-negotiable constraints:**

1. No loss or silent reinterpretation of pre-existing project, WBS, calendar, baseline, earned-value, currency, or audit data.
2. No plugin has direct SQL/database access, DEK access, administrator access, or unrestricted project write access.
3. A failed, cancelled, or stale-session import cannot partially commit to the destination project.
4. A user can open, edit, schedule, export existing supported formats, and recover encrypted projects with **all plugins disabled**.
5. A plugin crash, Java runtime failure, or incompatible plugin cannot crash or corrupt the main application.
6. No network connection, cloud account, telemetry, AI model, or Java installation is mandatory for core GoPMgr operation.
7. Source-file ingestion is bounded and auditable; unknown fields are preserved when possible or identified explicitly. “Full fidelity” is a **measured claim**, never inferred merely because MPXJ parsed a file.
8. The plugin boundary does not decide the separate DuckDB replacement, DataFusion/Iceberg/Arrow strategy, SQLCipher engine, or internal Go scheduling-kernel migration.

### Decision record

| ID | Topic | Decision / required gate |
|---|---|---|
| D-01 | Extension style | Out-of-process subprocess; reject Go `plugin` shared objects/JNI in host |
| D-02 | Transport contract | Protobuf + streaming local gRPC; no public listener |
| D-03 | Supervisor library | Time-box `hashicorp/go-plugin` proof; fallback custom Go launcher preserving ABI |
| D-04 | Trust model | Only release-signed bundled MPXJ allowed initially; untrusted third-party disabled |
| D-05 | Core ownership | Project/session/DEK/SQLCipher/audit/schedule kernel never owned by plugin |
| D-06 | API safety | Explicit capabilities + authorization on every operation; deny by default |
| D-07 | Import | Read → normalize → validate → preview → stage → revalidate → atomic commit |
| D-08 | Export | Snapshot of authorized data → convert → validate → atomic destination publish |
| D-09 | Provenance | Preserve source bytes/hash, source IDs, mapping version, warnings, field accounting |
| D-10 | Scheduling | Imported scheduled values preserved; recomputation only via explicit user operation |
| D-11 | AI integration | MCP is an AI adapter, **not** the general plugin transport |
| D-12 | AI write safety | Propose-only default; preview + explicit user approval + optimistic revision check |
| D-13 | Networking | None needed for MPXJ; remote AI connections must be separately authorized |
| D-14 | Java | Bundled private compatible runtime for packaged MPXJ module if selected; verified size/licenses |
| D-15 | Availability | Plugin missing/disabled means graceful capability downgrade, no core-app failure |
| D-16 | Upgrade | Host/plugin protocol compatibility and package signatures checked before execution |
| D-17 | Auditing | Security-sensitive changes logged in host-owned tamper-evident audit chain |
| D-18 | Community plugins | Separate future ADR + real OS confinement/permission enforcement required |
| D-19 | WBS | Existing unified-WBS model and project revision/receipt invariants take precedence |
| D-20 | Build hygiene | Pinned dependencies, reproducible artifacts, SBOM/notices, macOS/Windows/Linux gates |

---

## 1. Evidence baseline: verified repository facts vs proposals

### 1.1 Verified in GoPMgr (inspection reference above)

- Application: Go 1.27.3 (`go.mod`), Wails v2.16.0, Svelte 5/TypeScript; `main.go` is the Wails entry point and exposes `App` methods to the frontend.
- Local-first, offline-capable project controls; the kernel provides CPM, dependency/constraint handling, baselines, EVM, resource calculations and Monte Carlo risk simulations (`docs/ARCHITECTURE.md`, `internal/kernel`).
- Per-user SQLCipher-capable `.gopmgr` project databases, user-specific DEKs, `system.db`, authentication/session boundaries and project audits (`ADR-001`, `ADR-004`, `ADR-005`, `internal/db`, `internal/users`).
- Session epochs are advanced by sign-in/sign-out and protect against publishing stale operations (`main.go`, `session_state.go`).
- Native `.mpp`, legacy `.mpx`, and `.pod` imports currently return actionable errors in `import_formats_test.go`; MSPDI input currently has an application entry point and mapping-receipt test in `mspdi_import_test.go`.
- Existing MSPDI path in the reviewed tests materializes a CPM chart, **not proven comprehensive preservation of all source project entities**. New code must not mistake chart fidelity for full project fidelity.
- The repository already has `internal/export/mspdi.go`, `internal/export/mspdi_import.go`, and an internal scheduling kernel; do not replace those without measured justification and regression evidence.
- `internal/analytics` uses DuckDB conditionally behind a build tag in the inspected design. Removal/replacement is separate work.
- `docs/design/wbs-unified-model-contract.md` governs WBS identity, linked records, revisions, and staged production activation. A plugin must adapt to the authoritative WBS boundary, never create a competing WBS truth.
- `docs/VISION.md` explicitly prioritizes kernel correctness, local-first operation, security, open formats, and stability. `docs/ROADMAP.md` defers community extensions until a narrow permissioned boundary proves safe.
- `docs/design/` already contains ADR-001 through ADR-009. ADR-010 is available **at the inspected commit**, not guaranteed at future HEAD.
- Existing project test/release commands and REUSE/SPDX conventions are described in `AGENTS.md`, `docs/TESTING.md`, `docs/DEPENDENCIES.md`, and `docs/release-preflight.md`.

### 1.2 Verified in external projects/documentation

- MPXJ is a Java library with broad import/export capabilities; it can **read** proprietary MPP but **cannot write native MPP**. Prefer MSPDI for compatible Microsoft Project interchange when output is required. See [MPXJ home](https://www.mpxj.org/), [FAQ](https://www.mpxj.org/faq/), and [source](https://github.com/joniles/mpxj).
- At planning time, MPXJ release **v16.10.0** was published on 2026-10-05. Re-pin after a compatibility/license review; never use an unbounded `latest` artifact in production.
- MPXJ is distributed under GNU LGPL; inspect exact release license, third-party notices and transitive runtime licenses before packaging.
- HashiCorp `go-plugin` supports separate-process gRPC plugins implemented in other languages, but non-Go workers must implement its handshake/control protocol. See [non-Go guide](https://github.com/hashicorp/go-plugin/blob/main/docs/guide-plugin-write-non-go.md) and [internals](https://github.com/hashicorp/go-plugin/blob/main/docs/internals.md).
- Go's standard `plugin` shared-object mechanism lacks Windows support and imposes fragile toolchain/build coupling. See [Go package documentation](https://pkg.go.dev/plugin).
- The official [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk) is suitable for an **optional** AI interoperability gateway; MCP does not replace the core plugin lifecycle and format-import contracts.

### 1.3 Not yet established (must not be represented as tested)

- Actual security/isolation behavior of Java plugin on each supported operating system.
- Go-to-Java `go-plugin` interoperability with packaged Java launcher and startup paths.
- A complete, accurate MPXJ-to-GoPMgr field mapping, including calendars, resources, assignments, 11 baselines, timephased values, custom/enterprise fields, source schedules and multi-project files.
- Peak memory, latency, CPU, error rates or package size on representative customer data.
- Presence of public safe-to-redistribute sample MPP/XER files sufficient for compliance testing.
- Legal approval for the exact MPXJ distribution arrangement and required third-party source/notices.
- Existing WBS production activation state on future branches or whether a revised application aggregate is necessary.
- Available OS confinement that can enforce no-network/no-filesystem policies against *untrusted* third-party code on all platforms.

**Rule:** The implementation agent must validate each unknown through source inspection, experimental evidence or an explicit owner decision; never silently turn assumptions into facts.

---

## 2. Scope, success criteria, and exclusions

### 2.1 In scope for this ADR

- A host-owned plugin registry, verified manifest loader, capability system, lifecycle supervisor, typed transport, per-operation permissions, health and cancellation.
- A bundled Java MPXJ module implementing `inspect`, `import`, `validate`, and later `export` capabilities.
- A host-owned normalized schedule-import contract and a fidelity/reporting model without bypassing current GoPMgr data ownership.
- Staging/preview, transaction-safe import, source provenance, rollback, logs and test fixtures.
- A narrow Svelte UI for displaying available bundled plugins and permissions/status; no public plugin store.
- Reserved extensibility points for AI model providers and MCP interfaces, without shipping autonomous project mutation.

### 2.2 Explicitly out of scope until separate approval

- A plugin marketplace, user-installed arbitrary executables, unrestricted dynamic code-loading, auto-updated plugin executables.
- Automatic uploads of project data to remote AI providers or cloud accounts.
- Native `.mpp` writing (not supported by MPXJ), claims of byte-for-byte roundtripping across proprietary formats, or unsourced ‘100% compatibility’ claims.
- Replacement of GoPMgr's SQLCipher persistence, DuckDB analytics engine, WBS v2 rollout, CPM kernel, PDF/PAdES subsystem, update system, or login/escrow mechanism.
- New third-party API access to live DEKs, encrypted DB files, direct SQL, unrestricted host paths or admin escrow.
- Agent-driven automatic commits, irreversible writes, or permission grants inferred from model output.
- Network-facing plugin servers or multi-user remote plugin execution.

### 2.3 Definition of done (program level)

Release is not complete until **all applicable** phases' entry/exit gates pass on **all advertised OS/architecture targets**, existing release gates are green, the plugin can be completely disabled without loss of core functionality, and no data-loss/security regression remains unexplained. All unsupported fields, output limitations, fixtures, benchmark baselines, and skipped checks must be documented explicitly.

---

## 3. Alternatives and decision rationale

| Architecture | Strengths | Drawbacks | Outcome |
|---|---|---|---|
| In-process Go `plugin` (`.so`) | Fast direct calls | Windows unsupported; unload/crash/security/toolchain coupling | **Reject** |
| JNI/JVM embedded in GoPMgr | Direct Java API | CGO/JNI risk; difficult lifecycle, crash and packaging isolation | **Reject** |
| Reimplement MPXJ in Go/Rust | One-language or native-only distribution | Years of format parity work and maintenance | **Reject as immediate solution** |
| Call MPXJ CLI once per import | Minimal prototype complexity | Startup cost, opaque JSON fidelity, poor cancellation/progress | **Use as reference spike only** |
| Private separate process + JSON stdio | Simple portable bootstrap, inspectable | Custom framing/versioning/backpressure; larger conversion cost | **Viable fallback/prototype** |
| **Private subprocess + gRPC/protobuf** | Typed, streaming, language-neutral, explicit service versions | More build plumbing and protocol discipline | **Choose** |
| `hashicorp/go-plugin` supervisor + gRPC | Existing handshake/lifecycle/runtime helpers | Java handshake complexity and third-party dependency | **Conditional implementation choice (Gate G0)** |
| Custom Go subprocess supervisor + same gRPC | Total control over launch/OS integration | More own lifecycle code to maintain | **Fallback if G0 fails** |
| MCP as *all* plugin infrastructure | Familiar to AI agents | Does not define application-specific transaction/fidelity semantics | **Reject; MCP only as AI adapter** |

**Why not reuse the application's database as the plugin API?** It would export privileged schema, bypass audits and revisions, couple extensions to migrations, and erode encryption and per-user isolation. **Why not embed all plugin functionality in-process?** A single malformed project input or Java crash would increase host availability risk. **Why a shared transport but distinct services?** An importer is not an AI model, and both require different semantics, permissions and error guarantees.

---

## 4. Reference architecture and component ownership

```text
+-------------------------------------------------------------+
| GoPMgr Desktop: Svelte 5 UI / Wails v2 / Go App              |
|   Existing session/auth + project open-lifetime handle       |
|   Plugins UI, import preview, explicit approvals             |
+----------------------------+--------------------------------+
                             |
+----------------------------v--------------------------------+
| HOST-OWNED PLUGIN CONTROL PLANE (Go)                          |
| manifest/registry  signature checks  compatibility           |
| process supervisor  capability broker  job queue             |
| deadlines/cancel    resource budgets    audit events         |
+----------------------------+--------------------------------+
                             | local versioned gRPC only
           +-----------------+------------------+
           |                                    |
+----------v----------------+       +-----------v-------------+
| MPXJ worker (Java)        |       | Future workers           |
| supported format readers  |       | Go/Rust AI/analysis etc. |
| normalized record stream  |       | separate contracts       |
| no host DB/DEK handle      |       | default DENY             |
+----------+-----------------+       +-------------------------+
           |
   selected input bytes and
   validated output stream
           |
+----------v--------------------------------------------------+
| HOST-OWNED DATA PLANE (Go): ImportAdapter/ProjectServices     |
| normalization, field accounting, ID remapping, staging       |
| source snapshot, validation, scheduling comparison            |
| permission/revision/session recheck, atomic DB commit        |
+----------------------------+--------------------------------+
                             |
+----------------------------v--------------------------------+
| Existing per-user project databases, WBS, kernel, audit       |
| SQLCipher/DEK/admin escrow remain wholly inside GoPMgr        |
+-------------------------------------------------------------+

AI (later): AI Workspace -> Go AI orchestrator -> opt-in MCP gateway
             -> same host-owned permissioned read/proposal APIs
             -> user approval -> existing transactional write service.
```

### 4.1 Component separation

| Proposed module | Owns | Must never own |
|---|---|---|
| `internal/plugin/registry` | Verified metadata, enablement, ABI/capability records | Project keys; untrusted code execution from discovery alone |
| `internal/plugin/supervisor` | Process launch, handshake, health, deadlines, teardown | Project SQL, permissions override |
| `internal/plugin/protocol` | `.proto` definitions, generated bindings, compatibility fixtures | Application-wide DB model |
| `internal/plugin/authz` | Capability grants and host authorization mapping | Authentication source of truth |
| `internal/plugin/jobs` | Import/export job states, progress/cancel, bounded streaming | Direct SQL writes |
| `internal/interoperability/model` | Neutral transfer objects, typed units and field accounting | Persistent duplicate WBS/schedule authority |
| `internal/interoperability/importer` | Staged mapping and changeset validation | Bypassing session/epoch/revision checks |
| `internal/interoperability/exporter` | Snapshot generation, output validation/publish | Plugin-controlled filesystem destination |
| `internal/plugin/fixtures` (tests) | Golden compatibility fixtures, fake workers | Sensitive customer files in Git |
| `plugins/mpxj-bridge/` (new Java component) | MPXJ adapter and serialization | Database access, user secrets or application business rules |
| `internal/ai` (deferred) | Model routing, MCP tools and proposal approval contracts | Direct scheduling truth or unaudited mutations |

Directory names above are **design targets**, not claims that these packages exist. Inspect existing namespaces before adding them. Prefer domain-oriented packages and minimal public API surface; avoid a generic reflection-based plug-everything framework.

### 4.2 Boundary rules

- UI calls **Go host methods**, never plugin RPCs directly. Wails-generated frontend bindings remain synchronized with TypeScript declarations.
- A plugin is a **worker**, not an account principal: every operation carries a *host-issued, bound job context*; a plugin cannot claim a new user/project/session.
- Core host verifies project identity and current authenticated principal on **every data access and before every commit**; a job ID is not proof of authorization.
- No recursive plugin calls or arbitrary callbacks into `App`; workers return data/results to a **fixed** host-owned service interface.
- Data contracts contain explicit schema/version/unit semantics and source IDs. Do not pass raw Go `db.*` structs as plugin API payloads; never turn database layouts into a public ABI.
- Existing `internal/kernel`, `internal/wbs`, `internal/money` and `internal/db` remain authoritative for normalized application invariants. The import layer must expose mismatches instead of coercing them away.
- Plugin-specific errors cannot mutate project state. Any output untrusted by source provenance must be revalidated by the host.

### 4.3 Bundled versus community extensions

**Tier 0:** Internal Go feature packs compiled into the app; not separate executable plugins.  
**Tier 1:** Signed, bundled first-party adapters (MPXJ initial scope); can run in a private process under documented OS restrictions; source-specific import permissions.  
**Tier 2:** User-installed third-party plugins; **disabled by policy** until OS confinement, publisher trust, permission UI, review and incident-revocation are separately accepted.  
**Tier 3:** External AI/MCP services; require explicit local/remote distinction and per-server authorization; not enabled automatically when Tier 1 is installed.

**Critical security qualification:** Process separation and a host permission broker do **not** stop arbitrary code running as the same OS user from independently reading that user's files or making network connections. Host permissions constrain the **API**, not necessarily the operating system. Until confinement is measured per OS, label Tier 1 as *trusted bundled code*, not sandboxed untrusted code. Do not ship Tier 2 merely because API denies database access.

---
## 5. Plugin package format, discovery and trust

### 5.1 Suggested bundle layout

```text
<GoPMgr app bundle>/plugins/
  org.gopmgr.mpxj/
    plugin.yaml                 # declarative metadata; no executable directives
    manifest.sha256             # content integrity list, covered by package signature
    licenses/                   # MPXJ + all required third-party notices/licenses
    sbom.spdx.json              # generated component inventory
    darwin-arm64/
      mpxj-bridge[launcher]     # fixed platform launcher, or configured runtime path
      runtime/                  # optional private, trimmed, compatible Java runtime
      lib/                      # pinned JARs, checksummed
    darwin-amd64/
    windows-amd64/
    linux-amd64/
```

Platform targets are illustrative: decide actual supported CPU architectures from existing GoPMgr CI/release configuration. Do not advertise an artifact that is not built and independently tested. The launcher should be a controlled executable or script interpreted by a known bundled runtime, *not* an arbitrary shell string evaluated through `sh -c` or `cmd /c`.

**Discovery:**

1. Search *only* immutable/bundled application plugin directories in initial release; no scanning `PATH`, cwd, Downloads, remote URLs, or user-writable generic directories.
2. Canonicalize paths, reject symlinks and path traversal at the host boundary, and reject wrong architecture/OS, unexpected owners or world-writable executable bundles where policy requires.
3. Verify bundle files against publisher-controlled, signed release metadata and SHA-256 manifest. SHA-256 alone does **not** authenticate a manifest if the attacker can replace both file and hash.
4. Parse manifest with duplicate-field rejection and tight size/depth constraints. Unknown security-critical attributes fail closed.
5. Compare host/API/manifest protocol versions. Disabled or incompatible plugins appear as unavailable, with diagnostics, and are **never launched**.
6. Independently recheck executable and manifest integrity immediately before launch; eliminate or minimize validation-to-execution race. Prefer immutable packaged paths.
7. No execution merely to discover metadata. Startup does not execute disabled plugins.

### 5.2 Proposed declarative manifest (illustrative, not yet an implemented schema)

```yaml
schema_version: 1
id: org.gopmgr.mpxj
name: MPXJ Schedule Interoperability
publisher: GoPMgr maintainers
plugin_version: 1.0.0
upstream:
  name: MPXJ
  version: 16.10.0
protocol:
  major: 1
  min_minor: 0
  max_minor: 0
host_compatibility:
  minimum_version: "<set from tested host release>"
runtime:
  kind: child_process
  transport: local_grpc
  platform_artifacts: manifest.sha256
capabilities:
  - schedule.inspect.v1
  - schedule.import.v1
  - schedule.validate.v1
# Add schedule.export.v1 only after verified end-to-end export tests.
permissions_requested:
  - selected_source_read
  - selected_destination_write_via_host
resource_policy:
  network_required: false
  maximum_parallel_jobs: 1
license_inventory: licenses/
sbom: sbom.spdx.json
```

**Manifest trust model:** `permissions_requested` advertises needs; it grants nothing. The runtime enforces the active host policy, confirmed current-user permissions, and any OS confinement available. Runtime claims are capabilities to validate, not assurances that untrusted code is secure.

**No automatic directory lookup by ID**: identity is bound to signed artifact digest, publisher, installed slot and protocol version. An ID collision or downgraded version must fail or trigger an explicit trusted migration review.

### 5.3 Compatibility policy

- Protocol: use `major.minor`; incompatible major → reject; minor features may be negotiated **only when advertised and understood**.
- Plugin manifest schema has its own version, separate from wire protocol and application data schema.
- Extension capability identifiers are namespaced and versioned (`schedule.import.v1`), never inferred from filename extensions alone.
- GoPMgr project schema revisions **must not** track plugin versions. Migration belongs to GoPMgr storage layer.
- Backward compatibility: additive protobuf fields are normally allowed; changing meanings, enum numbers, field cardinality, date units or currency representations requires a new capability/version.
- Do not reuse protobuf field numbers; reserve removed fields and enum IDs. The host must tolerate properly unknown additive fields only when safe; do not silently drop unknown *business data* on export.
- Matrix-test host at N with approved plugin at N, N−1, and intentionally incompatible N+1. Rejection must be clean and actionable.

### 5.4 Installation and update policy

- The bundled plugin ships with, and is verified by, the ordinary signed GoPMgr installer/app bundle. Host/plugin pairing is pinned in a release lock manifest.
- Independent plugin updates, remote downloads, enabling arbitrary external plugin locations, and runtime auto-repairing of missing JARs are **not part of v1**.
- Do not add MPXJ to the main Go binary as a Go dependency. Keep the Java JARs and optional JRE isolated from the Wails executable.
- Document the exact Java distribution/version, CVE monitoring responsibility, OS signing/notarization behavior and reproducible dependency acquisition.
- Installing the host must not automatically grant plugin access to user projects. File import is always user-initiated for v1.

---

## 6. Local IPC, service definitions and protocol guarantees

### 6.1 Transport requirements

**Normative requirements:**

- Host launches a **private child process** for a selected, trusted plugin. Child never binds a public TCP interface.
- Use a local Unix-domain socket on macOS/Linux where actually supported by the chosen Java/gRPC stack; on Windows use a verified local mechanism, or an authenticated `127.0.0.1` ephemeral endpoint. Do **not** assume Unix sockets or named pipes work uniformly across Java/Windows.
- If loopback TCP is used: bind loopback only, random ephemeral port, authenticated connection (e.g., per-launch randomly generated high-entropy bearer challenge supplied over a protected launch channel and verified by an interceptor), verify target process/endpoint where possible, reject unauthenticated calls. For sensitive environments, evaluate mTLS/OS socket access separately. **A random port alone is not authentication.**
- A protocol handshake must bind the child to the expected bundle digest, launch job, protocol version and host-launched channel. Detect unexpected child exit or output; never trust child-reported path or identity without host verification.
- No plaintext project dump to command-line arguments, environment variables or diagnostic logs. Do not put credentials into CLI flags; limit inherited environment, working directory, file descriptors and OS process privileges.
- Apply bounded gRPC messages and bounded in-flight streams; proposed starting values: ≤4 MiB per message, ≤1 MiB source/data chunk, 2–8 chunks in flight. These are **provisional**, performance-tested configuration values, not arbitrary fixed ceilings on total project size.
- Stream file bytes/records with backpressure; huge arrays of tasks or timephased points must not be sent as one `Project` protobuf message.
- Every call has a context deadline and explicit cancellation; the host kills and reaps nonresponsive children after a documented bounded grace period.
- Worker `stdout` is reserved for the selected handshake protocol; use bounded, sanitized stderr/log channels with redaction. Do not let stack traces print file contents or DEKs.
- The child must be unable to call a general host-service endpoint. Every allowed host callback (if any) must be specified, capability-bound and separately authenticated. **Prefer no callbacks for v1.**

### 6.2 Proposed API surface (conceptual `.proto`, not drop-in compiled code)

```proto
syntax = "proto3";
package gopmgr.plugin.v1;
option go_package = "gopmgr/internal/plugin/protocol/gen/pluginv1";

service PluginControl {
  rpc Hello(HelloRequest) returns (HelloResponse);
  rpc GetCapabilities(CapabilityRequest) returns (CapabilityResponse);
  rpc Health(HealthRequest) returns (HealthResponse);
  rpc Stop(StopRequest) returns (StopResponse);
}

service ScheduleInterop {
  rpc Inspect(stream SourceFrame) returns (InspectResult);
  rpc Import(stream ImportInput) returns (stream ImportEvent);
  rpc Export(stream ExportInput) returns (stream ExportEvent);
}

message CallContext {
  string operation_id = 1;             // Host-generated opaque ID
  string correlation_id = 2;           // For sanitized diagnostics
  uint32 schema_major = 3;
  uint32 schema_minor = 4;
  string capability = 5;               // Must be registry-approved
}

message SourceFrame {
  oneof payload {
    SourceBegin begin = 1;             // Metadata, format hint, declared bytes
    SourceChunk chunk = 2;             // Bounded byte segment
    SourceEnd end = 3;                 // Authoritative digest and counts
  }
}
message SourceBegin {
  CallContext context = 1;
  string source_basename = 2;          // Display-safe only, not a path grant
  string format_hint = 3;
  uint64 expected_size = 4;
}
message SourceChunk { uint64 sequence = 1; bytes data = 2; }
message SourceEnd { uint64 total_chunks = 1; bytes sha256 = 2; }

// Define ImportInput as {begin | chunk | end}, and ImportEvent as
// {project_header | typed_record | diagnostic | progress | completion}.
// ExportInput carries a validated host snapshot; ExportEvent carries
// output chunks, diagnostics and final digest. Define these messages
// fully during Phase 1, with canonical golden encoding fixtures.

message HelloRequest  { uint32 protocol_major = 1; uint32 protocol_minor = 2; }
message HelloResponse { uint32 protocol_major = 1; uint32 protocol_minor = 2;
                        string plugin_id = 3; string plugin_version = 4; }
message CapabilityRequest {}
message CapabilityResponse { repeated string capabilities = 1; }
message HealthRequest {}
message HealthResponse { string status = 1; }
message StopRequest {}
message StopResponse {}
message InspectResult { string format = 1; uint64 tasks = 2;
                        uint64 resources = 3; uint64 assignments = 4; }
message ImportInput { SourceFrame source = 1; }
message ImportEvent { bytes structured_payload = 1; } // PLACEHOLDER; replace
message ExportInput { bytes structured_payload = 1; } // with typed oneofs
message ExportEvent { bytes payload = 1; }            // before coding
```

**Mandatory design follow-up:** The `structured_payload` placeholder above is *not acceptable in the merged production protocol*. During Phase 1, implement **typed** `ProjectHeader`, `Task`, `Relation`, `Resource`, `Assignment`, `Calendar`, `Baseline`, `TimephasedSlice`, `CustomField`, `UnknownField`, `Diagnostic`, `Completion` messages and explicit `oneof` envelopes. No arbitrary `bytes` or `Any` bypass for authoritative data. Use file-spooling or record-streaming only after quotas and encryption implications are documented.

**Why supply a conceptual partial schema?** To establish the method boundaries and framing invariants without pretending unreviewed proto definitions are implementation-ready. The coding agent must produce and review the final schema and golden tests in G1 before implementing MPXJ mapping.

### 6.3 Exact call-state discipline

```text
DISCOVERED -> VERIFIED -> DISABLED (default for nonbundled)
                       -> AVAILABLE -> STARTING -> NEGOTIATING -> READY
                                                    |             |
                         VERSION/SECURITY ERROR <--+             v
                                                               BUSY
                                                                |
                                             READY <------------+----> STOPPING
                                                                |          |
                                                         UNHEALTHY/FAILED --> STOPPED
```

- Registry states and job states are **separate**. A running import must not make the manifest appear as an untrusted state.
- Handshake failure, invalid capabilities, duplicate IDs, signature mismatch, unsupported ABI, failed heartbeat or subprocess crash → `FAILED` with structured error and user-readable reason; host remains usable.
- Idempotent stop: a second stop or cleanup after crash is safe.
- A plugin can be disabled between jobs; a running job is either explicitly cancelled or allowed to complete under its already authorized scope, as determined by an approved lifecycle policy. **On sign-out, user switch, project switch or permission revocation, cancel immediately and prohibit commit.**
- Set `max_concurrent_jobs` per plugin; the first MPXJ release may serialize heavy Java parsing. No unbounded goroutines, queues, JVMs or simultaneous file descriptors.
- Track child PID/process group, startup time, last heartbeat, memory limit and exit code where supported. Reap descendants; never leave an orphan JVM after host exit in normal shutdown.

### 6.4 Error taxonomy and semantics

Typed errors should include stable code, human-safe message, operation ID, retryability, stage and optional field path; raw stack traces remain local developer diagnostics and are scrubbed.

| Error code | Meaning | Host action |
|---|---|---|
| `PLUGIN_NOT_INSTALLED` | Optional bundled worker absent | Show fallback, no project error |
| `PLUGIN_SIGNATURE_INVALID` | Artifact trust failure | Refuse to run; security diagnostic |
| `PLUGIN_INCOMPATIBLE` | Protocol/capability mismatch | Disable worker with actionable notice |
| `PLUGIN_START_FAILED` | JVM/process launch failure | Retry boundedly only if safe |
| `PLUGIN_UNHEALTHY` | Hung/crashed child | Cancel job, cleanup; no commit |
| `INPUT_UNSUPPORTED` | Source type not supported | Report actual supported formats |
| `INPUT_INVALID` | Malformed or corrupt source | Structured parse errors; preserve original |
| `INPUT_PROTECTED` | Password-protected or inaccessible file | No secret logging; explicit workflow |
| `RESOURCE_LIMIT` | File/decompression/memory/time quota | Fail safely; offer scoped tuning |
| `DATA_UNMAPPED` | Source fields exceed modeled contract | Report precisely; block destructive export |
| `STALE_SESSION` | User/epoch/project handle invalidated | Discard staged changes; no commit |
| `REVISION_CONFLICT` | Project changed after preview | Re-preview against new revision |
| `USER_CANCELLED` | Explicit cancel | Cleanup, no audit success claim |
| `EXPORT_UNSUPPORTED` | Requested destination unsupported | Do not invent native `.mpp` writer |
| `INTERNAL` | Unknown failure | Sanitized error ID; inspect local log |

Never swallow, reclassify or automatically retry a parse error as successful incomplete import. If a fallback parser is available, expose that as a **separate user-visible operation** with its own provenance and limits.

### 6.5 Streaming semantics

- `sequence` values are monotonic; duplicate/out-of-order chunk, count/digest mismatch, unexpected EOF or completion before all records is an error.
- Final `Completion` declares counts **per record type**, total bytes, checksum of canonical logical representation, warnings and source digest. The host independently validates all counts/references before commit.
- Enforce total source-byte size, decompressed expansion, record count, nesting, string lengths, calendar occurrences and timephased points; all bounds are profiled, documented and versioned. A bounded per-message stream without a bounded total job is not safe.
- Apply backpressure from DB/staging writer to parser; the plugin must not accumulate the entire dataset waiting for an infinitely slow host.
- Import record ordering cannot imply relational consistency. Host resolves identity references after receiving all required records and rejects dangling, duplicate or cross-project references unless the source format expressly represents unresolved external links.
- Host maintains a unique operation ID and a monotonically increasing stream generation. An old worker response is never accepted into a later project session or operation.

---

## 7. Security, identity, confidentiality and authorization

### 7.1 Trust boundaries and threat actors

Assume an attacker can supply a malicious `.mpp`/`.xer`/XML file; a local user can attempt to alter a plugin bundle; a plugin can be buggy or compromised; AI-generated tool arguments and imported document text may be malicious; the host may be cancelled mid-operation. Assume a local attacker running as the same OS user may share many OS filesystem rights absent sandboxing. SQLCipher at rest does not isolate project plaintext from code running within an already authenticated user session.

**Threat to mitigate:** corrupt input parser crashes/DoS, zip/XML expansion, path traversal, temp-file leakage, file replacement/symlinks, malicious output, unbounded logging, deserialization bugs, plugin spoofing, version downgrade, session switch during import, unauthorized cross-project reads, duplicate writes, compromised AI model/tool prompt injection, plugin-induced network exfiltration, partial commits and incorrect monetary/calendar interpretation.

### 7.2 Permission policy

| Capability | Default | Actual enforcing authority |
|---|---|---|
| Plugin discovery | Bundled signed only | Registry + release signature |
| User-selected file ingestion | User initiates per operation | Host file picker + controlled byte stream |
| Read project metadata | Deny until scoped job grant | Go authenticated project service |
| Read entire project | Deny initially | Host permission broker + per-project handle |
| Modify project | Deny for MPXJ import worker; host applies reviewed changes | Go authenticated transaction service |
| Read another account | Deny; no plugin escrow path | Existing account/admin authorization |
| Read DEK/credentials/database handle | Never | No wire interface; no key serialization |
| Write output file | Host-only publish | Go destination selector and atomic writer |
| Network | Not needed MPXJ; best-effort restriction for trusted tier | Enforced OS sandbox if verified; otherwise explicitly NOT guaranteed |
| AI server/model access | Disabled initially; explicit opt-in | Separate MCP/provider broker |
| AI mutation | Proposal-only; approval required | Authenticated host kernel/DB transaction |
| Install executable plugin | Not exposed in v1 | Signed installer/release pipeline |

**Do not pass `username`, roles, project path, DEK, account ID or session epoch as trusted assertions from the child.** Session and revision are host-side guards, not facts the child can establish. Do not persist capabilities as a plugin-editable database row without integrity controls.

### 7.3 Operation-scoped authorization

Each host-side job context binds:

- authenticated principal (including account incarnation), current session epoch, signed-in user, user-selected project copy identity and open-project handle;
- exact operation, approved plugin ID+digest, input file handle/digest, approved target project and scope;
- capability set, permissions, resource limits, creation time/deadline, expected project revision and cancellation source;
- deterministic correlation and audit IDs, user-approved import/export options, user consent to remote AI use if ever applicable.

**Before any host-owned write:** acquire appropriate session/project locks in established order; re-read authoritative session and principal; verify epoch, open handle, copy identity, authorization, source/target revision, permission and staging hash. Reject and dispose staging when any no longer match. Never treat a long-running job's initial authorization check as enough at commit time.

### 7.4 Encryption and temporary files

- Do not reveal SQLCipher key bytes or encrypted project paths to a plugin. Avoid passing project plaintext through filesystem temp files; stream from host memory or a clearly scoped, host-controlled interface.
- When staging many records, prefer in-memory bounded staging or host-owned **SQLCipher-encrypted** temporary staging under the current user's private data root. A new staging DB schema requires migration/cleanup review; using an unencrypted SQLite staging database is forbidden.
- MPXJ's Java format reader may need a seekable file. If the selected source was streamed, a private worker scratch copy may be necessary. Restrict it to the **source file**, make permissions private, never reuse a caller-controlled filename, track cleanup on success/failure/kill, and explicitly disclose that SSD secure deletion is not guaranteed.
- For sensitive sources, allow declining disk spooling if it cannot be contained. Do not falsely claim that `os.Remove` cryptographically erases plaintext. On macOS/Linux/Windows, validate private file semantics and directory permissions separately.
- Export uses a host-owned temporary sibling file in the approved directory with exclusive creation. Validate output, `fsync`/close where supported, then atomic rename without replacing an existing destination unless a separate explicit overwrite flow is approved. Defend against symlink/TOCTOU and destination changes. Respect GoPMgr's existing export-path safety conventions.
- Audit/log files exclude raw project contents, credentials, full notes and source paths by default; paths may be kept in a locally restricted debug record only when policy allows.

### 7.5 OS isolation and community-plugin release gate

| OS | Candidate mechanisms to evaluate | Non-assumption |
|---|---|---|
| macOS | Hardened runtime, code signing/notarization, sandbox entitlements/child restrictions | Do not assume App Sandbox inheritance alone enforces the exact requested restrictions |
| Windows | Job Objects for lifecycle/resources; AppContainer or equivalent brokered isolation | Job Objects alone do not prevent arbitrary filesystem/network access |
| Linux | Namespaces/seccomp/Landlock or an audited sandbox launcher | Namespaces/seccomp availability and distro/packaging behavior vary |

**Tier 2 gate:** demonstrate on each supported OS that a deliberately malicious test plugin cannot read other project files, retrieve keys, reach the network when denied, write outside its scratch area, or escape via symlink/path traversal. Until this is demonstrated, installation of arbitrary third-party executables must remain disabled. If OS-level confinement is unavailable on any advertised target, Tier 2 support must either omit that platform or be explicitly blocked there. Code signing is provenance verification, not sandboxing.

### 7.6 AI-specific misuse and prompt-injection rules

- Imported project task names, notes, document text and custom fields are **untrusted content**, not instructions. Never promote them into system/developer policy for an agent.
- Model outputs (including proposed edits) must be parsed as typed untrusted data, validated using existing kernel, money and scheduling constraints, and cannot grant permissions or execute arbitrary shell/SQL.
- A model can propose a `ChangeSet`, but **cannot** authorize it. Required preview includes before/after values, effect on schedule, baselines, resources, costs, provenance, responsible user and rollback path.
- No AI write may bypass existing session-epoch, project revision/receipt, audit or transaction machinery. No approvals by omission, prompt trick, background timer, or tool result text.
- User consent to a remote provider must specify project, fields, provider endpoint and transmission scope. Default local-only; no hidden fallback to cloud on local-model failure.
- Treat `MCP` tool results as untrusted; protect against confused-deputy attacks by keeping server/tool identity and operation authorization distinct from natural-language assertions.

---
## 8. High-fidelity project interchange model

### 8.1 Two models, not two sources of truth

**External interchange snapshot:** A versioned, immutable, format-neutral record stream built specifically to accurately represent the imported external project and its provenance. It may contain fields that GoPMgr does not yet support. It is *not* a live project DB.

**GoPMgr domain model:** The existing authoritative project, WBS, schedule, resource, cost-control and audit stores. Mapping is performed by a **host** adapter, using the approved schema and ownership rules. An imported field is never automatically assigned the same meaning as a similarly named GoPMgr field without verified unit/semantic correspondence.

**Crucial large-project limit:** `docs/design/wbs-unified-model-contract.md` currently specifies a proposed `wbs_v2` aggregate limited to **8 MiB**, **10,000 nodes**, **64 levels** and **50,000 associations**. A 100,000-task schedule **cannot** be silently stuffed into that aggregate. The implementation must design and approve an appropriate scale-capable **schedule/entity persistence strategy**, with benchmarked queries and transactional ID mapping, **before** claiming large-project full import. If this requires schema/migration decisions, create a separate ADR and migrate safely. The interoperability plugin itself should not assume a storage implementation.

### 8.2 Required entity categories

| Category | Required representation | Loss/fidelity hazards |
|---|---|---|
| Project metadata | Source app/format/version, project UID, title, GUID, time zone and defaults | Missing project UID, derived fields, template/subproject identity |
| Task | Source UID, ID/position, name, outline/WBS, status, hierarchy, flags, duration/work, dates and modes | ID vs UID, summary rolls, null dates, manual scheduling |
| Relationships | Predecessor/successor UIDs, FS/SS/FF/SF, lag sign+unit, external links | Lag units, cross-project references, unsupported relationship semantics |
| Calendars | Base calendar, weekly hours, exceptions, inheritance, work weeks, resource/task calendars | Time zones, DST, exceptions recurrence, inheritance |
| Resources | UID, resource type, allocation, rates, cost/use, cost rate tables, availability, calendar | Resource pool references, earned value, monetary scaling |
| Assignments | Task/resource IDs, units, work, actual/remaining work, start/finish, contour, delay, cost | Timephased contour, unassigned work, overtime |
| Baselines | Baseline 0 plus numbered 1–10 if source supports; by task/resource/assignment | Baseline absent vs actual zero, baseline names and original dates |
| Financials | Fixed/variable cost, actual/remaining/earned, currency and source precision | Signed overflow, integer cents vs source fractional currency |
| Timephased | Plan/actual/remaining work and costs, dates, interval/contour and source period | Expansion costs, timezone discontinuities, derived vs stored totals |
| Custom/enterprise fields | Scope, field ID/type, alias, lookup table, formula metadata, value | Losing lookup hierarchy/formula or silently flattening enterprise definition |
| Notes/attachments | Text/RTF state, external attachments references | RTF formatting, hyperlinked data, unsafe attachment paths |
| Source-specific data | Explicit `UnknownField`/extension bag with context and type | Undocumented bytes, writer inability to restore unknown fields |
| Scheduling outputs | Imported early/late dates, slack, critical, constraints; provenance | Import-derived vs recomputed dates, Primavera vs Microsoft semantics |

No record count suffices as a fidelity metric. Require field-level comparators and semantic comparisons with MPXJ outputs and representative Microsoft Project/Primavera application results where licensing/tools permit.

### 8.3 Identity rules

- Scope identities by `(source_digest, source_project_key, entity_kind, source_unique_id, source_scope)`; do not assume task row ID, WBS display code or name is globally unique.
- Use host-generated application IDs for new records. Persist reversible `source_uid -> target_id` mapping in an immutable import receipt with source identity and mapping-version information.
- On reimport, show explicit choices: **new project**, **replace a specifically selected imported snapshot**, or **reconcile into current project** (latter deferred). Never choose identity collision merging by matching names.
- Preserve current WBS chart/node IDs where existing approved boundaries demand them. Use explicit remap/receipts, and keep unresolved deleted-source links as prescribed by `wbs-unified-model-contract.md`.
- Foreign keys/references unresolved because the source contains external projects must be represented as **external unresolved references**, not invented internal task IDs.
- Dates/times: preserve raw source local representation, time zone/offset and interpreted UTC instant where available; use explicit DST ambiguity rules. Do not replace source date-only semantics with midnight UTC by default.
- Durations/work: exact units, source working-calendar settings and conversion provenance. Store integers/fixed rational or decimal strings rather than arbitrary binary floats for precision-sensitive values.
- Money: preserve source decimal precision, ISO currency and scale as **typed source values**. Only convert at the host's existing `internal/money` boundary after checked rounding/range policy. GoPMgr's current stored monetary semantics and JPY reporting behavior (ADR-009) must not be changed by an import plugin.
- Null/unset is different from numerical zero/false and different from an unsupported field. The wire model must distinguish `unset`, `present`, `unsupported`, `derived`, and `approximated` where relevant.

### 8.4 Provenance and loss ledger

Every import emits a durable `ImportReceipt` (host-owned, sanitized when exported) containing at least:

```json
{
  "receipt_schema": 1,
  "job_id": "host-generated-id",
  "plugin": {"id": "org.gopmgr.mpxj", "version": "pinned", "digest": "sha256:..."},
  "source": {"format": "MPP", "sha256": "...", "byte_length": 0},
  "target": {"project_copy_id": "host-generated", "revision": "decimal-string"},
  "mapping_schema": "schedule-transfer-v1",
  "counts": {"source_tasks": 0, "mapped_tasks": 0, "source_assignments": 0},
  "loss_summary": {"preserved": 0, "mapped": 0, "transformed": 0,
                   "approximated": 0, "unsupported": 0, "dropped": 0},
  "warnings": [],
  "source_retained": true,
  "audit_reference": "host-generated-id"
}
```

Values and strings above are illustrative schema examples, **not actual receipts**. A `0` count here means placeholder, not evidence of supported entity coverage. The production schema must include field paths, source entity scope, reason code, severity and intended re-export handling for each loss/approximation class. Explicitly disallow silent `dropped` values in any mode marketed as high fidelity.

**Preservation policy:** Retain original source bytes and normalized snapshot as **separate** artifacts where required, preferably within authorized encrypted project storage or an approved user-private encrypted evidence store. A hash and path reference alone do not guarantee that the source remains accessible. A byte-for-byte source copy is not proof that changes can be losslessly written back to its proprietary format.

### 8.5 Scheduling interpretation and comparison

- MPXJ reader extracts source schedule metadata; **GoPMgr kernel remains the calculation authority** when user explicitly invokes GoPMgr scheduling.
- Import stores original **source-computed schedule** and relevant constraints/baselines without involuntary recalculation.
- Provide an optional **compare-only** operation with both outputs: source dates/float/critical flags versus GoPMgr CPM dates/float/critical flags, with tolerances and reasons (calendar exceptions, lags, resource constraints, scheduling mode, time zones).
- Flag unsupported scheduling modes and differences; never silently force-fit imported Primavera behavior into Microsoft Project semantics.
- No plugin may directly rewrite baselines, recalculate EVM or change work packages while merely inspecting/importing.

### 8.6 Mapping specification deliverable

Before the first production import, create `docs/interoperability/mpxj-field-mapping.md` containing **one row per supported mapping**, with columns:

`source format | MPXJ accessor/field | source type/units | transfer message | GoPMgr destination | conversion rule | null rule | precision | round-trip claim | test fixture | known limitations`

The agent must generate this from audited source/API definitions and representative fixtures, not guess field names. For unsupported source attributes, record **why**: absent from MPXJ, available but not modeled, available but no compatible GoPMgr destination, or writer not able to reconstruct.

---

## 9. Detailed MPXJ plugin specification

### 9.1 Feature surface and truthful advertised capabilities

| Feature | v1 intent | Requires evidence |
|---|---|---|
| `.mpp` reader | Yes (documented MPXJ support) | Modern and selected legacy real-world files; error handling |
| `.mpt` reader | Candidate | Verify exact MPXJ template support for pinned release |
| `.xml` MSPDI read | Yes | Compare existing built-in GoPMgr MSPDI path |
| Primavera P6 `.xer` read | Yes | P6-specific calendar/resource/baseline fixtures |
| Primavera `.pmxml` read | Yes | XML content and external references |
| MPX/MPD/other import formats | Capability-discovered, phased | Do not list until verified in bridge/tests |
| MSPDI export | Later | Open/reparse results, field and timephased reporting |
| P6 XER/PMXML export | Later | Correct P6 semantics, round-trip validation |
| Native `.mpp` export | **No** | MPXJ does not supply this feature |
| MPXJ CPM schedule computation | Optional future compare engine | Compare with GoPMgr kernel; do not substitute automatically |
| Remote P6/Project Server connections | **Not v1** | Separate network/credential/authorization ADR |

Do not use file extension alone for detection. Combine filename hint with signature/parse checks; an incorrect extension must not be enough to force an unsafe code path. For XML, use secure parser configuration (no external entity resolution or uncontrolled external retrieval). Bound archives/decompression if underlying MPXJ readers support container formats.

### 9.2 Read-only import workflow — authoritative sequence

1. User explicitly selects import operation, plugin and source through GoPMgr UI. Host requires an authenticated session, current project/new-project target and valid session epoch.
2. Host validates the candidate file with safe open flags, regular-file policy, selected location, quotas, content sniffing and source SHA-256; avoid replacing the opened handle with a pathname later (TOCTOU defense).
3. Host authorizes a **job**, not the plugin globally. Bind source digest, selected target/copy/handle, session epoch, plugin digest, import options and expected revision.
4. Registry verifies bundle and launches worker on first use. Negotiate protocol, capability and actual supported format. If missing or incompatible, preserve existing MSPDI path as fallback.
5. Host streams opened-file bytes in bounded chunks; worker parses into `ProjectHeader + Record + Diagnostic` events. If MPXJ requires random access, worker stages **only source bytes** in its private scratch with enforced cleanup and size limit.
6. Host builds immutable normalized snapshot, collects full field ledger, validates quotas, typed units, references, dates and source/entity IDs; no project DB mutation yet.
7. Host compares snapshot to destination schema; classifies all unsupported or approximate mappings, checks WBS and project-scale limits. **If full mapping does not fit current model, fail safely or offer a clearly labeled partial import; never silently truncate tasks to fit.**
8. Host prepares proposed changes in bounded memory or a documented encrypted staging store. Produce summary with tasks, hierarchy, calendars, resources, assignments, baselines, custom-field counts and fidelity warnings.
9. User reviews and explicitly chooses to create a new project/import or approves a specific supported merge mode. **Initial v1 should prefer import-as-new-project** to limit destructive merge ambiguity.
10. Immediately before commit, recheck authenticated principal, epoch, project copy/open lifetime, expected revision, permissions, staging checksum, free-space/size constraints and approval scope.
11. In an **existing approved DB transaction API**, commit all approved schedule/source mapping changes and audit receipt atomically. Do not create a second transaction framework in the plugin layer. If the current API cannot provide this, introduce an explicit core-service change with its own tests/ADR before activation.
12. Verify committed counts/references, present success only after durable commit. Cancelled, failed or conflicted operations leave original project unchanged. Clear scratch/staging and worker resources on all exits.
13. Retain precise source/provenance artifacts according to user privacy policy. Never claim completion from the worker's `Completion` event alone.

**Partial-import policy:** A user-visible partial result may be supported later, but it must include a list of excluded entity classes/fields and opt-in, and must not be labeled “complete MS Project integration.” For source baseline/resource accounting, imported records must remain internally consistent even when certain optional classes are excluded.

### 9.3 Import job state machine

```text
CREATED -> AUTHORIZED -> SOURCE_VERIFIED -> PLUGIN_READY
          -> PARSING -> NORMALIZING -> VALIDATING -> STAGED
          -> PREVIEWED -> USER_APPROVED -> PRECOMMIT_RECHECK
          -> COMMITTING -> COMMITTED

Any precommit state --(cancel/timeout/crash/invalid/revoked/stale)-->
   ABORTING -> CLEANUP -> ABORTED

PRECOMMIT_RECHECK --(stale revision)-->
   CONFLICT -> NEW PREVIEW REQUIRED

COMMITTING --(DB failure)-->
   TRANSACTION_ROLLBACK -> FAILED (must prove original integrity)
```

- Only Go host can move job from `STAGED` to `USER_APPROVED`, and only after a real user action. The worker cannot provide approval.
- A job cannot be committed twice. Use host idempotency receipt and existing revision semantics; duplicate requests return the prior committed result or explicit conflict without replaying changes.
- Avoid stale session concurrency deadlocks: follow current `App`/`internal/db` lock ordering and use cancellable contexts. Never hold an application-wide mutex while waiting on a long Java import or UI approval.
- On crash between database commit and UI notification, durable receipt confirms success on recovery; do not blindly retry a possibly committed operation.

### 9.4 Export workflow (deferred until import proven)

1. User picks target interchange format from plugin's **verified writer** capabilities (MPP absent).
2. Host obtains a consistent authorized project snapshot and source-provenance ledger, recording revision.
3. Validate destination representability **before** worker writes; report loss risks (field downgrade, custom field loss, timephased aggregation, project-specific semantics).
4. Worker emits bounded, digest-verified bytes; host checks syntax/schema where possible, optionally reimports through MPXJ and independently verifies semantic round-trip metrics.
5. Host atomically publishes to explicitly selected location. On error, preserve prior destination, clean scratch and report exactly what failed.
6. Export does **not** mutate project schedule/baselines or an existing project source file. Retain export receipt and version/format mapping evidence.

### 9.5 MPXJ versioning and licensing

- Pin the **exact** upstream MPXJ artifact, checksums, complete Maven dependency tree, Java runtime version and bridge commit/tag in a lock/notice file; never track `latest`.
- Verify minimum compatible Java version from **that** MPXJ release and test the private runtime on real OS installers.
- Include MPXJ's LGPL text, appropriate copyright and modification notices; make precise corresponding-source/replacement obligations and third-party licenses available after **legal/compliance review**. Java-side LGPL replacement/relinking rights must be evaluated, especially if creating a bundled private runtime/application distribution; do not assume separate-process invocation waives all obligations.
- Bridge source itself should have an explicit license and SPDX records consistent with repository policy. Keep dependency notices, SBOM and reproducible build logs.
- Run dependency and CVE scans, JAR artifact provenance validation, license checks and code signing/notarization per OS.
- Provide an upgrade/removal path that preserves prior source/provenance receipts and avoids changing existing imported values.

---

## 10. Future AI integrations and MCP extension boundary

### 10.1 Modular services, not an AI super-plugin

The plugin infrastructure may later host different typed capabilities:

| Service family | Interface | Examples | Mutation authority |
|---|---|---|---|
| `schedule.*` | Import/export/inspect/validate | MPXJ, other native readers | Host transaction only |
| `analytics.*` | Query/forecast/report on authorized snapshot | Monte Carlo visualization adapter, specialized engine | Host controls outputs; no DB keys |
| `ai.model.*` | Model discovery, completion, embeddings, token budgets | Local llama.cpp/Ollama or opt-in cloud | No direct project writes |
| `ai.tools.*` | MCP tool/resource adapter | Read schedule, report risk, propose changes | Host authorization |
| `document.*` | Bounded extract/render | Third-party file conversion | Host-owned published output |

Keep these as separately versioned capability services. A plugin can implement multiple approved services, but this does not imply permission to read or alter unrelated project classes.

### 10.2 MCP gateway design (future phase; not part of v1 MPXJ release)

- Run an **opt-in** local MCP server controlled by GoPMgr; use the official Go MCP SDK subject to version/security review. Initially use local stdio or another approved local transport, not an unauthenticated LAN listener.
- Expose a small, read-only resource/tool subset first: `get_project_metadata`, `list_tasks`, `get_dependencies`, `get_baseline_summary`, `analyze_critical_path` (host kernel) and `get_import_diagnostics`.
- Each tool delegates to the **same host application services** as Wails. Never let MCP server directly open `.gopmgr`, access session keys or bypass admin reason capture.
- Maintain explicit connection identity and authorization; a session-bound read grant must expire on sign-out/user switch/project close. Test project selection changes while a tool is in flight.
- Add model-provider routing only after read-only tool access is demonstrated correct. Local models are the default; online providers and data transmission are opt-in per endpoint and project scope.
- Audit locally which tool was called and what category of data it accessed, without logging full project content by default.
- Backpressure and pagination are required for 10k+ task tools; do not dump an entire project into an LLM prompt/context window.
- All AI source notes and outputs are untrusted text. Adopt explicit prompt-injection and output-schema tests.

### 10.3 Proposal-only change workflow (later, separate approval)

```text
LLM suggestion (untrusted)
 -> typed ChangeSet validation (host)
 -> source citations + diff + confidence/limitations
 -> deterministic kernel impact preview
 -> user explicitly approves exact digest
 -> recheck session epoch/project handle/copy/revision and grant
 -> host-owned transaction and audit receipt
 -> verify committed outcome; report errors precisely
```

An AI agent **never receives `commit_schedule` with implicit approval**. For high-impact updates (baseline reset, resource/cost changes, deleting WBS nodes), consider separate per-operation confirmation and role policy. Model output cannot change authorization scope. Changes to current security/audit/escrow code require independent security review.

---

## 11. Data scale, performance and capacity design

### 11.1 Workload tiers

| Tier | Sample footprint | Required use |
|---|---|---|
| S | 100–500 tasks, ordinary resources/calendars | Fast smoke, backwards compatibility |
| M | 10,000 tasks plus realistic assignments/dependencies | First large-file acceptance |
| L | 50,000 tasks, 150k+ assignments/dependencies | High-scale design validation |
| XL | 100,000 tasks with extensive timephased records | Stress/long-term capacity target; **not a v1 guarantee** |

Test **entity cardinality**, not just `tasks`. Include 11 baselines, dense resources, calendar exceptions, multiple time zones, a high attachment count, custom fields and irregular timephased records. An all-simple-task fixture is insufficient evidence.

### 11.2 Performance principles

- Retain input and output as streaming chunks; avoid repeated full-file copies, massive JSON strings or quadratic reference resolution.
- Use maps/indices keyed by scoped source unique ID for relationship resolution. For cycle detection/topological tasks, prefer linear-time graph algorithms; do not recompute all predecessor walks for each record.
- Timephased values should be processed as bounded series; explicitly cap points and avoid expanding them to per-minute rows without need.
- Permit cancellation checkpoints during parse, normalization and staging. Do not hold GoPMgr UI thread or Wails locks for long work.
- Bound JVM heap separately from Go host memory; measure peak combined RSS, garbage collector pressure, CPU, disk scratch and process startup.
- Host DB writer must batch where supported while preserving one authenticated transaction and audit consistency. Batch writes are *not* permission to commit a partial import.
- WBS 8 MiB/10k invariant and current CPM chart serialization limits must be verified before permitting high-tier imports. A normalized schedule table and index migration may be needed; do not quietly raise limits in unrelated code.

### 11.3 Benchmarks and release claims

Record stable test hardware, OS, Go/JDK/MPXJ/plugin versions, fixture checksums, cold/warm state, input sizes and end-to-end timings. Compare:

- Existing native MSPDI import versus plugin MSPDI import for identical fixture.
- MPXJ standalone CLI versus MPXJ plugin (cold and persistent-worker warm).
- Parsing, normalized serialization, host validation, staging, database commit, view load and export separately.
- Combined peak host + Java memory, maximum scratch bytes, tasks/sec, assignments/sec and timephased points/sec.
- Crash/restart, cancellation latency, sign-out latency, and performance with 2–4 concurrent independent users/jobs (bounded per-plugin pool).

**Provisional quality target:** Support tier M without UI freeze, memory exhaustion, or field truncation on explicitly documented minimum supported hardware. Define numeric time/RSS gates from the G0 baseline and publicly documented target hardware before announcing a performance SLA. Tests must enforce the chosen thresholds and detect regressions; do not claim 50k/100k production capability based only on parser completion or one synthetic run.

---
## 12. Phased delivery plan: work breakdown and hard gates

**Execution discipline:** Implement only one narrowly scoped, reviewable vertical slice at a time. A passed phase establishes permission to *propose* the next phase, not authority to weaken existing invariants. No “implementation complete” claim is valid without attached reproducible evidence. If implementation exposes a contradiction with accepted ADRs, stop and draft the conflicting decision; do not hide a schema/security policy change within an importer commit.

### G0 — Baseline, dependency audit and feasibility spikes (no production behavior change)

**Goal:** Prove that the proposed Java bridge, chosen supervisor, IPC, license packaging and current GoPMgr architecture can coexist.

**Work packages:**

- G0.1: Confirm `git status --short`, branch, HEAD, Go/Node/JDK tooling, current release targets, existing import and WBS code; record a full architecture/dependency delta versus section 1. Read `AGENTS.md`, ADR-001/004/005/009, `docs/ARCHITECTURE.md`, `docs/DEPENDENCIES.md`, `docs/TESTING.md`, `docs/release-preflight.md`, `docs/design/wbs-unified-model-contract.md`, `session_state.go`, `main.go`, `app_projects.go`, and `internal/export/mspdi_import.go` **selectively**.
- G0.2: Verify no ADR-010 collision at current HEAD; save a copy of this document under `docs/design/` only after maintainer approval. Add a separate `docs/plans/plugin-implementation-plan.md` only if separating operational detail from the ADR improves maintainability; do not introduce duplicate/conflicting normative sections.
- G0.3: Read MPXJ's pinned release notes/license/dependency list, exact supported reader/writer features and Java runtime requirements; create a dependency license/SBOM draft.
- G0.4: Build **throwaway** Go ↔ Java `go-plugin` gRPC handshake experiment with a typed `Hello` RPC on each advertised OS. Confirm process shutdown, cancellation, stream transfer, plugin startup, code signing, packaging, Windows IPC path, macOS notarization prerequisites and license distribution options.
- G0.5: Build **fallback** minimal custom Go child supervisor with the *same* `Hello` RPC if G0.4 fails; do not maintain two production supervisors. Record measurable comparison: complexity, code volume, dependencies, cross-platform reliability, attack surface and launch latency.
- G0.6: Obtain or create a licensed/redacted MPXJ source corpus: minimal MSPDI, real MPP14, legacy if available, P6 XER and PMXML. Keep private/proprietary fixtures out of Git unless redistribution is clearly allowed. Record fixture hashes and provenance in an access-controlled test manifest.
- G0.7: Measure baseline GoPMgr import, large-project/wire/storage constraints and existing `make verify` / `make check-release` status; baseline failures must be preserved and distinguished from changes.

**Artifacts:** `docs/research/plugin-feasibility.md`, an initial threat model, license inventory, fixture inventory, baseline results, short decision memo selecting **one** supervisor. Experimental code either removed or clearly segregated and not activated in product.

**Exit gate G0:** Actual Go↔Java IPC round-trip works on required targets; plugin can be launched/reaped offline; handshake and health failure tested; packaging/license review has no unresolved blocker; correct target storage scale plan identified. **No compatibility experiment → no framework commitment.** If `hashicorp/go-plugin` fails, record evidence and select custom supervisor with identical protobuf API. Do not quietly switch to an insecure localhost server.

### G1 — Contract-first plugin framework (no MPXJ project writes)

**Goal:** Establish a small, stable, testable Go host extension boundary.

**Work packages:**

- G1.1: Write final `.proto` files and generated Go+Java code with pinned compiler and codegen versions, `buf` or equivalent generation rules and a golden protocol test.
- G1.2: Implement immutable signed-bundle discovery for bundled development/release plugin slots, manifest strict parsing, compatibility/capability negotiation, trust/publisher checks, and disabled-plugin behavior.
- G1.3: Implement process supervisor state machine, bounded queues, single-flight startup, cancellation, kill/reap, crash recovery, context deadlines, heartbeat, no unbounded retries, narrow logs.
- G1.4: Implement capability broker and a **fake test worker**, with no live project DB reads. Denied capabilities fail before child call. Define structured error codes, telemetry-free local status and sanitized logs.
- G1.5: Wire a host-facing `ListPlugins`, `GetPluginStatus`, `EnableBundledPlugin`, `DisableBundledPlugin` API with clear Wails/TypeScript types (implement enable semantics only after authorization policy is approved).
- G1.6: Add property/fuzz tests for manifest/path parsing, protocol decoding, unauthorized operations, malformed chunks, digest mismatches, stale handshakes and child process cleanup.

**Artifacts:** `internal/plugin/` packages, `proto/` contract, deterministic codegen tooling, fake worker, versioning documentation, targeted tests. Avoid building a general plugin marketplace/UI at this stage.

**Exit gate G1:** Zero plugin-enabled regressions to existing app; Linux/macOS/Windows launcher smoke (or precisely narrowed platform support), no unwanted network listener, no child process/FD leaks, and protocol compatibility tests green. No real customer data exposed to fake worker.

### G2 — MPXJ bridge and read-only project inspection

**Goal:** Use the actual Java library to read supported files without mutating GoPMgr.

**Work packages:**

- G2.1: Implement Java bridge as a **minimal adapter** around pinned MPXJ reader APIs. Keep MPXJ custom object graph inside Java and stream typed neutral records out.
- G2.2: Implement source-byte streaming, private scratch lifecycle where needed, secure parser options, quota controls, digest checks and cancellation propagation.
- G2.3: Add format/capability detection, header inspection, record counts, preview of WBS/tasks/dependencies and structured warnings; compare MPXJ direct output to bridge output.
- G2.4: Test real modern MPP and P6 formats, malformed/oversized/corrupted/protected files; return useful limitations without crashing host.
- G2.5: Include complete build/package license notices, private Java runtime/launcher verification and OS signing tests. Keep plugin fully removable/disabled.

**Artifacts:** `plugins/mpxj-bridge/`, cross-language fixture tests, inspection UI/API and precise advertised format list.

**Exit gate G2:** No DB modifications in inspection path; independently matched record counts/checksums for the checked fixtures; byte/record/CPU/time limits enforceable; no stack traces or full project files in logs; Java component fully isolated from core startup. A fake import-success flag cannot substitute for persisted correctness.

### G3 — Typed mapping, encrypted staging and transactional **new-project** import

**Goal:** End-to-end high-fidelity import into a newly created GoPMgr project, without silent data loss.

**Entry prerequisite:** Review `internal/db` persistence and WBS/schedule schema at **current** HEAD. If existing chart/DB layout cannot represent the imported entities at target cardinalities, prepare and approve a **separate storage/migration ADR** before coding schema changes. Do not bypass the 8 MiB/10k WBS boundary.

**Work packages:**

- G3.1: Implement full typed transfer records for calendars, tasks, dependencies, resources, assignments, baselines, timephased and custom fields, with explicit null/source-unit handling.
- G3.2: Create MPXJ field-mapping matrix, identity mapper, unknown-field representation, normalized validator and import receipt/loss ledger.
- G3.3: Build host-only staging store (memory/approved encrypted staging); implement import-as-new-project transaction and safe atomic project-file publication. New project creation must not leave an apparently usable partial `.gopmgr` after failure.
- G3.4: Preserve original source snapshot and source digest under approved retention policy; copy/backup/failure semantics and privacy must be documented.
- G3.5: Show preview and user confirmation, deterministic IDs/receipt, “no silent field dropping” behavior, and exact commit/error outcomes.
- G3.6: Recheck epoch/handle/principal/copy identity and revision after parsing and *immediately before* commit; test session sign-out, project switching, duplicate jobs, app crash/recovery, source replacement and overlapping users.
- G3.7: Run end-to-end import comparisons with original MPXJ readings and readback from encrypted GoPMgr DB. Do not claim success from only successful chart creation.

**Artifacts:** audited mapping document, model specification, staged import service, user preview UI, import receipts, end-to-end fixture suite and migration/recovery evidence (if schema was changed).

**Exit gate G3:** All supported fixture data classes either correctly mapped or explicitly warned/blocked; imports are atomic and idempotent; no stale-session commit; original project remains unchanged on every injected failure; pre-existing SQLCipher/recovery/admin/WBS tests pass. **If source coverage is incomplete, release labels must state the exact limitation.**

### G4 — Export, usability, realistic scale and packaged releases

**Goal:** Make MPXJ useful for trustworthy daily interoperability.

**Work packages:**

- G4.1: Add MSPDI export (and P6 XER/PMXML writers only when validated), with format-specific limitations, export receipt and destination safety.
- G4.2: Validate output with MPXJ reimport and official target applications where possible; compare all modeled fields and scheduling semantics. Explicitly refuse native MPP write requests.
- G4.3: Build a minimal Svelte plugin-management panel: installed/version/status, supported formats, toggles, permissions, why unavailable, last local error; no arbitrary package upload.
- G4.4: Benchmark S/M/L/XL workloads; optimize identified bottlenecks. Validate peak combined host/JVM memory, network off, GUI responsiveness and encrypted staging capacity.
- G4.5: Verify platform installers, private JVM inclusion, macOS signing/notarization, Windows signing, Linux package layout, offline install/upgrade/uninstall, plugin-disabled core behavior, SBOM and notices.
- G4.6: Update public documentation accurately; ensure actual supported extension lists are derived from verified worker capabilities and are not overinclusive.

**Exit gate G4:** Existing `make check-release` passes on supported targets (and new plugin-specific gates); output can be opened by relevant tools for validation fixtures; import/export results have integrity/loss reports; packaged offline app works without system Java; no core startup dependence on plugin. Any missing proprietary validation sample must appear as an explicit coverage limitation.

### G5 — Opt-in, read-only MCP + local model provider integration

**Goal:** Open a controlled AI *read* and analysis surface without permitting model mutation.

**Work packages:**

- G5.1: Introduce AI provider capability interface with local model adapter first; no mandatory model install, no default remote fallback.
- G5.2: Add optional MCP gateway using current official Go SDK, with authenticated/session-scoped host service adapters. Start with a small paginated read-only tool catalog.
- G5.3: Validate prompt injection, malicious field content, scope leakage, tool argument tampering, expired sessions, project switching and resource limits.
- G5.4: Add plain-language UI and privacy disclosures describing which project data may reach a model/server and which permissions it receives.

**Exit gate G5:** Read-only guarantee independently tested; no tool can modify DB or send project content off-machine without explicit user consent; disabled AI has zero impact on offline core.

### G6 — AI change proposals and approved writes (separate design approval)

**Goal:** Support reviewable AI-suggested project changes through existing GoPMgr transaction authority.

**Required prior approval:** Separate short ADR/update explicitly accepting each write-capability class, the authorization UX and rollback model. No silent extension of G5 tools to writes.

**Work packages:** Typed proposal schema, deterministic diff/impact preview, one-time user approval binding exact digest, revision/epoch checks, denial tests, audit receipt, no partial write and user-visible rollback capability where domain semantics permit.

**Exit gate G6:** Simulated hostile model prompts and malicious MCP servers cannot approve their own proposals; rejected/stale proposals cause no mutation; accepted changes are validated by the existing schedule/finance/WBS services and correctly audited.

### Community plugin release (G7) — **not included in this approval**

A new ADR and demonstrated per-OS confinement, publisher trust/revocation, user permissions, upgrade/rollback and independent security review are required **before** arbitrary third-party executable plugins are permitted. The Tier 1 MPXJ plugin must not be used as purported proof that Tier 2 is safe.

---

## 13. Dependency graph and recommended pull-request slices

```text
G0 (facts/benchmark/license + Java handshake + supervisor decision)
  |
  v
G1.A contracts/versioned protobuf/goldens
  -> G1.B trust registry/manifest
  -> G1.C supervisor/process lifecycle
  -> G1.D host authz + mock worker tests
  |
  v
G2.A MPXJ Java bridge inspect-only
  -> G2.B source-byte stream/safe scratch
  -> G2.C fixture comparison + packaging
  |
  v
G3.A neutral typed transfer and field ledger
  -> G3.B host model/storage ADR if needed
  -> G3.C encrypted staging + identity mapping
  -> G3.D authenticated atomic commit + audit receipt
  -> G3.E preview/UI + failure injection
  |
  v
G4.A writer adapters -> G4.B validation + scaling -> G4.C release gates
  |
  +---> G5 read-only MCP + local AI -> G6 separately approved AI writes
  +---> G7 separately approved third-party/community plugins
```

**Suggested PR boundaries:**

| PR | Change scope | Minimum proof | Rollback strategy |
|---|---|---|---|
| P0 | ADR and feasibility evidence only | Documentation/decision review, no behavior change | Revert docs |
| P1 | Protocol, generated clients, mock worker | Cross-language tests + strict version fixtures | Feature remains disabled |
| P2 | Host registry, signature verification, supervisor | Crash/deny/cancel tests, no DB access | Disable registry, preserve core |
| P3 | MPXJ read-only inspection | Fixture diff, malformed-file and startup tests | Disable bundle |
| P4 | Transfer model + validation + loss ledger | Field matrix & fixtures, no commit route | Disable import |
| P5 | Encrypted staging + new-project commit | Rollback/fault tests and encrypted readback | Feature flag off; migration rollback plan |
| P6 | Import preview/status UI | Wails/frontend smoke, a11y check | Hide UI feature flag |
| P7 | Export supported interchange formats | Round-trip and application reopen | Disable specific writer |
| P8 | Release hardening and capacity | Cross-platform + quotas + notices | Restore previous signed release |
| P9 | Optional read-only MCP/AI | Prompt injection + auth/session tests | Disable AI service |
| P10 | Proposed AI writes | Dedicated approved ADR/gates | Withdraw write capability |

Do **not** combine P2–P5 or P5–P8 into a single unreviewable broad commit. Every PR must document dependencies introduced, relevant accepted ADRs, files changed, migrations, security impact and tests actually executed.

---

## 14. Verification and test assurance plan

### 14.1 Test layers

| Layer | Minimum tests | Key failure proof |
|---|---|---|
| Unit | Manifest parser, version compare, authz, field values/units, ID remapping | Unknown/duplicate manifests rejected; overflow prevented |
| Property/fuzz | Manifest, chunk framing, transfer validators, binary/XML malicious inputs | No panic, unbounded allocation, out-of-range read or parser hang |
| Contract/golden | Go↔Java protobuf, capability negotiation, error codes, schema compatibility | Stable semantics across versions/languages |
| Supervisor | Startup, failed handshake, crash, timeout, reaping, cancellation, duplicate launch | No orphan children, no core crash, no stuck locks |
| Integration | Real MPXJ parsing to typed records and staged encrypted readback | Values/IDs/relationships match source within disclosed limits |
| Permission | Cross-user, cross-project, privilege escalation, lost login, revoked grant | Every forbidden operation denied before data release/commit |
| Fault-injection | JVM kill, corrupt stream, disk full, interrupted commit, source rename, partial output | No success claim; durable state consistent; no partial publish |
| Compatibility | Host/plugin N/N−1/N+1, old project files, plugin-disabled path | No silent downgrade or inability to open project |
| Packaging | Offline macOS/Windows/Linux launch, runtime integrity, signing/notices | No missing JRE, binary permissions or unsigned worker bypass |
| Performance | 500/10k/50k/100k realistic entities, cold/warm, UI and RSS | Meets measured gates; no unbounded materialization |
| AI (later) | Model prompt injection, malicious tool reply, unauthorized remote endpoint | No unapproved write or data egress |

### 14.2 High-priority acceptance scenarios (each needs a reproducible test ID)

| ID | Scenario | Expected result |
|---|---|---|
| PLG-001 | All plugins disabled | Core GoPMgr create/open/save/export/schedule works unchanged |
| PLG-002 | Tamper manifest/binary after install | Worker not launched, error classified as trust violation |
| PLG-003 | Unknown protocol major / wrong plugin ID | Negotiation rejected; host remains functional |
| PLG-004 | Corrupt input, expansion bomb, malformed chunks | Bounded, clean error; no crash or project mutation |
| PLG-005 | Java process killed mid-import | Job fails, subprocess reaped, no DB or export partial |
| PLG-006 | Cancel while worker parsing | Deterministic cancel and scratch cleanup; no commit |
| PLG-007 | Sign out/change account after preview | `STALE_SESSION`; no publish under old identity |
| PLG-008 | Switch project after preview | Destination handle invalid; no cross-project mutation |
| PLG-009 | Source file replaced/modified during ingest | Opened handle/digest guard rejects mismatch; no TOCTOU import |
| PLG-010 | Duplicate import command/replayed receipt | No duplicate task insertion, no duplicate irreversible work |
| PLG-011 | Import file with 11 baselines + custom fields + timephased data | Exact source accounting; unsupported differences shown |
| PLG-012 | Negative cost, unusual currencies, legacy JPY fraction | Source precision preserved; host checked conversion, no silent scale change |
| PLG-013 | P6 calendar inheritance and holiday exceptions | Accurate compare or explicit unsupported warning |
| PLG-014 | Unresolved external predecessor | Link remains unresolved with provenance; no invented task |
| PLG-015 | Import exceeding WBS 8 MiB or 10k nodes | Safe refusal or supported separate schedule storage; never truncation |
| PLG-016 | Disk full inside new-project import | No published incomplete `.gopmgr`; original state intact |
| PLG-017 | Export validation fails / destination exists | No destructive overwrite; original destination intact |
| PLG-018 | Request `.mpp` export | `EXPORT_UNSUPPORTED`, truthful user guidance |
| PLG-019 | Malicious worker tries unauthorized DB/API access | Broker denies; OS containment tested separately before Tier 2 |
| PLG-020 | Plugin writes sensitive notes to stderr | Log scrubbing, size limits and test reveal no raw notes |
| PLG-021 | Two projects/accounts, overlapping requests | No authorization crossover or wrong project IDs |
| PLG-022 | MCP tool access after sign-out (G5) | Immediate denial, no cached project leak |
| PLG-023 | LLM tries to approve its own edit (G6) | No mutation without user-signed scope/digest approval |
| PLG-024 | Upgrade from prior release with MPXJ absent | Existing projects and import/export paths still work |
| PLG-025 | Plugin package removed while app running | Controlled failure, no unsafe fallback execution |
| PLG-026 | Crash after commit but before UI receipt | Recovery finds durable committed receipt, no replay writes |
| PLG-027 | Plugin returns success before final frame/digest | Host rejects incomplete operation |
| PLG-028 | Untrusted source names try path traversal/formula injection | Sanitized; no arbitrary path or unsafe CSV formula |
| PLG-029 | Reimport same source into existing WBS (future) | Explicit mapping/remap/revision policy, not name-based merge |
| PLG-030 | Untrusted community plugin (G7 only) accesses adjacent files/network | Proven denial on each supported OS or Tier 2 blocked |

These are **required new test goals**, not claims that the test files currently exist.

### 14.3 Existing regression gates to preserve

Commands observed in the repository's `AGENTS.md`/README (run in the actual environment; use documented prerequisites):

```bash
git status --short
git diff --check && git diff --cached --check
go test . ./internal/...
go test -race . ./internal/...
npm --prefix frontend run check
npm --prefix frontend run build
make frontend-smoke
make license-check
make check-release
```

For the final release, use the repository's **actual current** `make verify`, platform CI, signing and `make check-release` contracts, not a copied list as a substitute. Check whether `make license-check` skips without `reuse` and explicitly record that coverage. Add new commands such as `make test-plugins`, `make test-mpxj-contract`, `make check-plugin-packaging` **only if implemented and documented**, with clear offline fixture behavior and explicit CI skip reasons for proprietary fixtures.

### 14.4 Evidence format required for every gate

Use a compact machine-checkable table or JSON with:

`gate_id, source_commit, fixture_hash, host_OS_arch, Go/JDK/MPXJ/plugin versions, command, exit_code, timestamp, test_count, skipped_count, artifact_hashes, observed_data_fidelity, exceptions, reviewer, decision`.

Do not mark a gate PASS if its crucial integration fixture was skipped or if the test only exercised a mock when real MPXJ behavior was required. Separate *unit PASS*, *integration NOT RUN*, *real-world parity UNVERIFIED*, and *package signing NOT TESTED*.

### 14.5 Publication claim discipline

Permitted wording after appropriately scoped validation: “Imports Microsoft Project MPP schedules using bundled MPXJ; preserve/report specific supported entities and limitations.”

Not permitted without much stronger evidence: “100% MS Project parity”, “full lossless MPP round-trip”, “write MPP natively”, “safe arbitrary third-party plugins”, “100,000-task production capacity” or “AI changes are always correct.”

---

## 15. Release, migration and rollback plan

### 15.1 Feature flags and compatibility

- Introduce a single host-owned release feature switch for plugin supervisor activation and a separate flag for MPXJ **import commit** activation. Inspection may be independently enabled for staged testing.
- Missing/incompatible plugin must not affect existing native MSPDI functionality, encrypted project opening, login/recovery, kernel, report generation or Wails startup.
- Plugin enablement metadata should live in an appropriate host-owned store with authenticated account/session semantics; **never in executable manifest content controlled by plugin**.
- Changes to `.gopmgr` project schemas require an independent migration and pre-existing file interoperability decision, with recovery tests against real old versions. Do not modify project schema simply to remember installed plugins.
- Keep new plugin-generated receipts versioned; old GoPMgr builds may be unable to understand new imported records. The compatibility/migration ADR must establish read-only fallback and backup behavior rather than silently rewriting old files.

### 15.2 Deployment sequence

1. Merge reviewed docs/contract/fake worker with plugin feature **off**.
2. Ship inspection-only bundled plugin to a constrained test channel, run real fixtures and platform installer smoke tests.
3. Activate new-project import for consenting testers only after G3 (separate feature flag).
4. Observe local opt-in diagnostics/bug reports, no unsolicited telemetry; resolve all unaccounted field-loss cases.
5. Activate verified interchange export formats one at a time and update supported-formats claims.
6. Add read-only MCP/AI only as an independent opt-in release with its own threat-model approval.

### 15.3 Rollback criteria and response

**Immediate disable/rollback triggers:** signature bypass, cross-user/cross-project leak, DEK exposure, unsafe native filesystem access, partial project corruption, stale-session commit, unmanaged Java process leakage, an unbounded malicious-file parser or loss of existing core flows.

**Actions:** disable plugin capability centrally via host feature flag; stop or cancel worker jobs; preserve error/provenance evidence without project content; restore previous signed build; keep imported projects readable under the compatible storage schema or follow the documented backup/read-only migration policy; verify prior project file integrity and audit chain before reopening write mode.

**Database caveat:** Rolling back a binary does not automatically reverse a schema migration. Reverting to an old executable against a new `.gopmgr` schema may be unsafe; the separate migration ADR must specify upgrade detection, backward-read rules, backup restoration and roll-forward choices *before* shipping schema changes.

---

## 16. Risk register and mitigations

| ID | Risk | Severity | Preventive measure | Evidence / release response |
|---|---|---|---|---|
| R-01 | Trusting plugin process == sandbox | Critical | Restrict Tier 1, explicit OS confinement gate for Tier 2 | Malicious test worker per OS; block Tier 2 if unsupported |
| R-02 | Stale session writes after long parse | Critical | Host epoch, principal/copy/revision check before commit | PLG-007/008/021; rollback |
| R-03 | Full import claim based on task/chart only | Critical | Typed model, loss ledger, field matrix | PLG-011 and real source compare; restrict claim |
| R-04 | WBS 10k/8 MiB exceeded | Critical | Separate large-scale schedule persistence ADR | PLG-015; no silent limit raise |
| R-05 | Currency/date semantic corruption | Critical | Typed units, checked money conversion, source preserve | PLG-012/013; block on mismatch |
| R-06 | MPXJ dependency/license noncompliance | High | Version pin, compliance review, SBOM/notices | Legal and release gate; do not bundle until passed |
| R-07 | Java process or decoder exhaustion | High | Quotas, streaming, cancellation, process management | PLG-004/005/006; fail safely |
| R-08 | Insecure loopback gRPC endpoint | High | Local transport, per-launch authenticated handshake | Spoofing/red-team test; reject insecure transport |
| R-09 | Plaintext project leakage via temp/log | High | Encrypted host staging, restricted source scratch, log scrub | PLG-020; block release on leak |
| R-10 | Reimport duplicates or cross-links | High | Scoped UID mapping, idempotency receipts, revision | PLG-010/014/029 |
| R-11 | Incomplete package/offline Java failure | High | Private runtime, all-OS installer smoke | PLG-024 and packaged offline test |
| R-12 | Wails UI lockup under large import | High | Worker/background jobs, no global lock while waiting | M/L benchmarks and frontend smoke |
| R-13 | Plugin supply-chain downgrade | High | Signed lock/digest, immutable bundle, no runtime auto-update | PLG-002/003/025 |
| R-14 | AI prompt injection / model data leakage | Critical (when G5/G6) | Trusted service boundaries; approval; network opt-in | PLG-022/023; feature disabled if not proven |
| R-15 | DB/analytics migration coupled to plugin | High | Separate ADR, independent release gates | Inspect diff and rollback coverage |
| R-16 | Cross-platform signing/packaging drift | High | Platform matrices and strict notices | All shipped artifacts reverified |
| R-17 | Field support diverges with MPXJ upgrade | High | Pinned artifacts, golden fixtures, loss ledger | Full regression before version bump |
| R-18 | Synthetic tests overstate fidelity | High | Real, licensed/redacted fixture corpus | Explicit fixture coverage matrix and limitations |
| R-19 | Different scheduling engines rewrite dates | High | Preserve source, compare-only mode; kernel authority | Schedule diff fixtures |
| R-20 | Incorrect future AI scope creep | High | Separate G5/G6/G7 decisions, deny-by-default | Review feature flag and API permissions |

---

## 17. Review checkpoints and maintainer decisions

The agent should maintain an explicit decision log, not use assumptions when implementation choices affect security, packaging or canonical project data.

| Question | Preferred decision | Status |
|---|---|---|
| Accept this ADR and numbered location? | Maintainer review then mark Accepted | **Open** |
| Which supervisor works for real Go↔Java? | Go-plugin if G0 passes, custom same protocol otherwise | **G0 pending** |
| How will Java be bundled? | Pinned private runtime + JARs, measured signed packages | **G0 pending** |
| Which OS/CPU artifacts ship? | Exactly existing supported tested targets | **Verify current CI** |
| How is a complete imported schedule stored? | Existing model if sufficient; separate schema ADR otherwise | **Critical gate** |
| What file formats can users import/export? | Only capability-verified readers/writers | **Fixture gate** |
| How to preserve full original files and receipts? | Approved private/encrypted retention contract | **G3 decision** |
| Which feature ships first? | Read-only MPXJ inspection, then new-project import | **Proposed** |
| Should users install community plugins now? | No | **Explicitly rejected for v1** |
| Should MCP be part of MPXJ plugin? | No; separate optional AI gateway | **Proposed** |
| Can AI models directly edit projects? | No; proposed changes only pending separate approval | **Rejected by default** |
| When may 100k-task performance be claimed? | After storage model/benchmark tests on published hardware | **Unverified** |

### Explicit owner approval checklist

- [ ] Accept the architectural boundaries and core ownership rules (D-01 through D-20).
- [ ] Approve G0 time-boxed feasibility/license/supervisor/fixture work.
- [ ] Confirm Tier 1 bundled-only policy and Tier 2 exclusion.
- [ ] Confirm initial user workflow: inspect and import **as new project**; no implicit merge.
- [ ] Approve separate large-project storage ADR if needed, before schema changes.
- [ ] Require a distinct decision before activating AI model providers, MCP read access or any AI write capability.
- [ ] Require release/packaging/legal review and full acceptance evidence before public format-support claims.

---
## 18. Coding-agent execution instructions (copyable mission brief)

> **Role:** Principal Go systems engineer + Java interoperability engineer + threat-model reviewer + release assurance lead. Deliver reviewable, evidence-backed changes to the actual GoPMgr repository. Treat this ADR as **proposed**, distinguish verified repository facts from design suggestions, and honor later accepted ADRs and user decisions. Avoid broad redesigns.

### 18.1 Startup procedure — mandatory

1. Locate the **actual** GoPMgr working tree and run `pwd`, `git status --short`, `git branch --show-current`, `git log -1 --oneline`. Identify any unstaged/untracked user/agent work and protect it. Do **not** `git reset --hard`, blindly stash, delete, broadly stage or overwrite unrelated work.
2. Read `AGENTS.md`, relevant `.agent_memory/` and local session notes **if present**, and the **current** project architecture/security/WBS/import/release documentation. Use targeted searches and small excerpts for oversized files.
3. Compare current HEAD to this ADR's referenced repository state. Identify accepted newer decisions, active branches, modified WBS or encryption behavior, and ADR filename collisions. Produce a concise discrepancy report before writing files.
4. Identify the *single next smallest priority* (initially G0). Write a dependency-aware mini-plan with exact files to inspect/change, test evidence and rollback. Do not start G1–G7 work because it is described here if G0 or acceptance is not complete.
5. For external dependencies, verify current official docs/releases/actual API surfaces, licenses, supported OS/architecture, pinned versions and supply-chain provenance. Never fabricate imports, versions, package names, flags or APIs.
6. Before any database schema, encryption, account, WBS or release-signing change, identify the relevant accepted ADR and provide a conflict/migration/security review. If the required owner decision is missing, implement only non-destructive preparatory work.

### 18.2 Work cycle for each reviewable slice

```text
Inspect current code and relevant tests
  -> state exact requirements/invariants & negative cases
  -> design minimal change and reversible deployment
  -> adversarially critique security, fidelity and failure paths
  -> write tests that prove new behavior (including failure)
  -> implement focused code without unrelated refactors
  -> run formatting + targeted tests + race/fuzz/contract tests as relevant
  -> run broader repository and release checks when scope warrants
  -> compare against pre-change baselines and source fixtures
  -> inspect diff; check license/SPDX and document evidence
  -> perform independent/adversarial review of edge cases
  -> repair uncovered problems; rerun affected gates
  -> stage intentionally by file/hunk, commit only if authorized
  -> record limitations, lessons learned, next smallest priority
```

### 18.3 Required agent deliverables per work package

1. **Plan:** work package ID, input commit, purpose, acceptance criteria, threat assumptions, affected paths, dependency review, estimated blast radius and rollback.
2. **Source change:** small focused diff implementing only approved slice; maintain GoPMgr code conventions, Wails/type bindings and existing layering.
3. **Tests:** named positive/negative tests, fixture hashes and test-output evidence; never rely on green tests that skip key cases.
4. **Documentation:** update ADR status only on explicit acceptance, mapping/security/dependency docs, codegen instructions, user-facing limitations, release notes as applicable.
5. **Verification table:** commands run, PASS/FAIL/NOT RUN/SKIPPED, skipped reason, environment, reproducible commands and artifact paths.
6. **Adversarial critique:** explicitly evaluate data loss, privilege escalation, source tampering, process crash, lifecycle leaks, concurrency, cross-platform behavior, performance and licensability. Fix findings or leave them as release blockers.
7. **Change review:** `git diff --check`, staged diff inspection, dependency and SPDX scan, documentation divergence and recovery tests.
8. **Handoff:** exact completion state, current blockers, unresolved maintainer decisions, next work unit, lessons learned. Do not claim “complete” for untested features.

### 18.4 Forbidden shortcuts

- No direct SQLCipher/database writes or key access from MPXJ/AI plugins.
- No `os/exec` invocation through a shell command assembled from manifest fields.
- No “permission enforced” claim based only on a manifest value or API-level deny policy when OS access is unrestricted.
- No indefinite subprocess, silent network listener, unbounded JVM pool, or unauthenticated loopback RPC.
- No export-to-MPP promise, no source-to-target field guessing, no silent precision conversion, and no automatic schedule recomputation during import.
- No UI success toast when only parsing/staging completed but the transaction did not commit.
- No bypass of existing project copy/open-handle, principal, epoch, revision or audit controls.
- No auto-approval or autonomous write execution by any AI model/agent.
- No jump straight to a plugin marketplace or remote execution framework.
- No unreviewed migration of encryption, DuckDB/analytics, WBS or project schema in the same change.
- No false PASS on proprietary fixtures absent from CI; no pretending source-derived examples prove actual MS Project/P6 compatibility.
- No destructive cleanup of unrelated files, automatic stage/commit of unreviewed work, or version bump unsupported by release gates.

### 18.5 Suggested initial agent response (G0 only)

The agent's **first substantive output** should be:

- Verified current repository HEAD and worktree status (without exposing private data).
- Confirmed ADR filename slot and any current accepted conflicts.
- Exact GoPMgr import/DB/WBS/session paths affected; evidence-based limits.
- Go-plugin Java handshake feasibility and fallback plan, including platform scope.
- MPXJ version, Java/JAR dependency and license audit checklist.
- Real/synthetic fixture access strategy and baseline benchmarks.
- A small G0 task list ordered by risk and dependency, with stop conditions.
- Clear list of owner approvals required before code or schema activation.

**Then implement G0 only** if approval exists, verify it thoroughly, and recommend the next phase. No giant one-shot refactor.

---

## 19. Architecture quality checklist and adversarial self-review

Before accepting this ADR or any implementation, reviewers should explicitly answer the following. Any **No**, **Unknown**, or **Not tested** in a critical row blocks relevant activation, not necessarily all preparatory work.

| Check | Standard | Required evidence |
|---|---|---|
| Core preserved | All existing login/project/kernel/MSPDI flows work with plugins disabled | Existing regression suite + offline smoke |
| Host authority | Worker cannot directly open application DB/DEK or commit outside host | API/code review + negative tests |
| Trust provenance | Only expected signed MPXJ artifact is launched | Signature/digest tamper tests |
| Network confinement honesty | Trusted bundle restrictions accurately stated; Tier 2 blocked | OS tests or explicit restriction disclosure |
| Session lifetime | Sign-out/project switch invalidates in-flight result | Fault-injection stale epoch tests |
| Transaction correctness | All approved records and audit commit atomically or none commit | DB fault injection, recovery |
| Import identity | Source UID and target ID mapping collision-safe, remapping explicit | Synthetic duplicate/external-ID fixtures |
| Semantic fidelity | Calendars, relationships, money, baselines and timephased fields validated | Real fixture field matrix |
| Scale bounded | 10k+ workload no unbounded memory/disk/UI lock | Measured benchmark + caps |
| WBS consistency | No 8 MiB/10k node bypass or second WBS authority | WBS contract review + PLG-015 |
| Export truth | Native MPP write never advertised; output limitations visible | Capability tests, docs and UI |
| Cross-platform | Every advertised installer includes runnable verified worker | Signed offline package smoke tests |
| Licensing | LGPL and all third-party runtime terms complied with | Legal/REUSE/SBOM audit |
| AI isolation | MCP and models separately opt-in; read-only before writes | Tool auth tests; no network by default |
| Third-party isolation | Tier 2 not enabled without OS confinement | Security review + malicious-plugin tests |
| Rollback | Signed previous release restores safe behavior; schema handling explicit | Rehearsed restore/upgrade test |
| Documentation | Claims match tested implementation, dates and fixtures | Release-claim check and reviewer sign-off |

### Key challenges this design intentionally does **not** hide

1. **Java IPC complexity:** cross-platform gRPC handshakes are feasible but not proven in this repository; G0 decides the supervisor. The stable ABI is more important than a favorite plugin framework.
2. **No strong same-user isolation by default:** trusted separate processes are not automatically safe against malicious filesystem/network access. Third-party plugins must wait for sandbox evidence.
3. **Import fidelity is a data-model problem:** MPXJ can parse more than the current GoPMgr chart-import path can necessarily preserve. A field ledger and perhaps independent persistence migration are required.
4. **Existing WBS cap is meaningful:** scaling a parser does not confer capacity on the database/UI/authoritative WBS.
5. **Offline packaging has real costs:** a Java runtime, JARs, code signing and SBOM can significantly enlarge and complicate releases. Measure rather than hand-wave.
6. **The source format may not be writable:** preserved source bytes are valuable for audit/reimport, not proof of lossless changed-file export.
7. **AI needs separate governance:** the MCP standard does not provide GoPMgr-specific revisions, audited project transactions or per-account authorization automatically.
8. **Future branch drift:** current ADR-010 numbering and code paths are based on the inspected reference HEAD; an agent must revalidate before taking action.

---

## 20. Reference material and source-of-truth priority

**GoPMgr primary sources** (prefer current checked-out commit over this document):

- [GoPMgr repository](https://github.com/draconianneocoder-cpu/GoPMgr)
- [`AGENTS.md`](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/AGENTS.md)
- [`docs/ARCHITECTURE.md`](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/ARCHITECTURE.md)
- [`docs/VISION.md`](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/VISION.md)
- [`docs/ROADMAP.md`](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/ROADMAP.md)
- [`docs/DEPENDENCIES.md`](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/DEPENDENCIES.md)
- [`docs/design/ADR-001-database-encryption-at-rest.md`](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/design/ADR-001-database-encryption-at-rest.md)
- [`docs/design/ADR-004-administrator-access-to-user-data.md`](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/design/ADR-004-administrator-access-to-user-data.md)
- [`docs/design/ADR-005-super-administrator-and-succession.md`](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/design/ADR-005-super-administrator-and-succession.md)
- [`docs/design/ADR-009-yen-reporting-currency.md`](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/design/ADR-009-yen-reporting-currency.md)
- [`docs/design/wbs-unified-model-contract.md`](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/design/wbs-unified-model-contract.md)
- [`mspdi_import_test.go`](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/mspdi_import_test.go)
- [`import_formats_test.go`](https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/import_formats_test.go)

**External technical sources** (verify latest versions before coding):

- [MPXJ documentation](https://www.mpxj.org/), [MPXJ FAQ](https://www.mpxj.org/faq/), [MPXJ GitHub](https://github.com/joniles/mpxj), [MPXJ releases](https://github.com/joniles/mpxj/releases)
- [HashiCorp go-plugin](https://github.com/hashicorp/go-plugin), [go-plugin non-Go guide](https://github.com/hashicorp/go-plugin/blob/main/docs/guide-plugin-write-non-go.md), [go-plugin handshake internals](https://github.com/hashicorp/go-plugin/blob/main/docs/internals.md)
- [Go native plugin limitations](https://pkg.go.dev/plugin)
- [gRPC](https://grpc.io/docs/), [Protocol Buffers best practices](https://protobuf.dev/programming-guides/proto3/)
- [Official MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk) and [MCP specifications](https://modelcontextprotocol.io/specification)
- [GNU LGPL version 2.1](https://www.gnu.org/licenses/old-licenses/lgpl-2.1.html) (confirm exact MPXJ release choice and obligations with counsel)

**Conflict resolution order:**

1. Explicit maintainer decisions and the checked-out repository's currently accepted ADRs and secure invariants.
2. Verified source code, fixtures and executed test evidence at current HEAD.
3. Documented release/security/legal requirements and upstream verified API/format behavior.
4. This proposed ADR's approved normative requirements.
5. Illustrative examples, tentative directory names, provisional limits and future opportunities.

**Final recommendation:** Approve the plugin boundary and G0 feasibility investigation before authorizing production code. Build the first feature as a signed, bundled, **read-only MPXJ inspector**; only enable full project import after the neutral transfer model, WBS/storage capacity, staging, atomic commit, provenance and failure-injection gates are independently satisfied. Treat read-only AI/MCP, AI writes and community executables as later, separately governed capabilities.

---

**Document revision:** Draft v1.0 — 2026-10-23  
**Evidence status:** Repository/documentation inspection performed; no GoPMgr code was modified, no project tests were executed, and no plugin binaries or installers were created in preparing this document.  
**Maintainer acceptance:** Pending.
