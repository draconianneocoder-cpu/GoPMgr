<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-TBD — Provider-neutral external work-management integration framework

**Status:** APPROVED IN PRINCIPLE by maintainer on 2026-10-28; **normative text remains a proposed supplemental ADR** pending explicit final acceptance and dependency reconciliation. No implementation, benchmark, migration, or external operation is thereby authorized.

**Draft date:** 2026-10-28.

**Decision authority:** GoPMgr maintainer.

**Repository examined:** `draconianneocoder-cpu/GoPMgr`, `main` observed at `ec7b73ab7d913a2398d333f6dad9ea5734ee8f93`; revalidate against HEAD before PRs.

**Relationship:** A supplemental integration ADR to proposed ADR-010 v2.2 and reserved ADR-011 normalized persistence—not a replacement, retroactive acceptance, or permission to implement either. The current repository `docs/VISION.md`, `docs/ROADMAP.md`, `docs/ARCHITECTURE.md`, beta backlog, and applicable accepted ADRs take precedence where relevant.

**Companions:** `ROADMAP-provider-integrations.md`, `TEST-provider-integrations.md`, `AGENT-architecture-preservation.md` (that file was kept out of the public tree), `CRITICAL-REVIEW-and-DECISIONS.md`, `CONTRACT-AND-RECOVERY-REFERENCE.md`. **Revision:** v2.0 critical-review strengthened; the source v1 is preserved in the prior ZIP.

> **Normative words:** MUST/MUST NOT become hard gates only after formal ADR acceptance; SHOULD is recommended unless evidence and maintainer approval justify deviation. Proposal text never constitutes proof of implementation or a passed test. This proposal does not authorize edits to source, approved ADRs, credentials, releases, or remote Jira data.

## 1. Decision and intended outcomes

Create a **provider-neutral, opt-in external work-management connection layer**, sharing ADR-010's internal signed capability registry, supervisor, lifecycle, typed protobuf/gRPC-over-local-IPC, provenance, host-side staging, and extensibility controls. Build **Jira Cloud as the first candidate cloud adapter**, not as the definition of the interchange data model. Future adapters could include GitHub Issues/Projects, Azure DevOps, Linear, other scheduling systems, and internal enterprise work APIs after separate provider-specific security/fidelity acceptance.

**Architectural rule:** provider APIs never become core GoPMgr schema, scheduling, audit, or identity authority. GoPMgr's Go host is the sole authority for local project mutations, key management, SQLCipher, canonical normalization, schedule computations, source mapping decisions, revisions, and audit. A worker may propose facts and desired remote operations; it never authorizes, schedules, or commits canonical local state.

This is **not** a generic SaaS platform, collaborative realtime project database, public plugin marketplace, or promise of lossless Jira–Microsoft Project equivalence. Local work, project opening, scheduling, reporting, and export MUST remain functional when no connector or network exists.

### 1.1 Non-negotiable continuity from the existing design

- ADR-010 v2.2 remains **proposed**. Its D-01…D-19 and QI-01…QI-15 must be reconciled and accepted before the corresponding implementation; this ADR does not weaken them.
- Bundled MPXJ is the **first installable ADR-010 worker**; Jira is a subsequent optional candidate. No arbitrary downloaded binaries or public SDK in G1–G4.
- Typed versioned protobuf over actual gRPC/HTTP2 using Unix domain sockets on supported Linux/macOS targets; on Windows, a **tested** native named-pipe transport (or validated Windows UDS alternative). No silent loopback TCP, raw stdio data transport, or compatibility downgrade. Windows Go↔Java feasibility for MPXJ is a separate G0 hard gate.
- Existing SQLCipher per-project databases and audit hashes must not be rewritten. The Go kernel remains authoritative; Arrow/DataFusion and optional Iceberg are analytics/projection decisions in separate ADRs, **never transactional project truth**. DuckDB removal is a separate approved migration with parity tests. The controlling gate is the 2026-10-27 architecture directive: DuckDB stays until Slice J, and Slice J waits on DF0.
- ADR-011 owns normalized schedule schema, file-version gates, encrypted migration, revision checks, and shadow-generation strategy. WBS v2's distinct 10,000-node/8 MiB boundary is not bypassed by external hierarchy imports.
- Import as a new project precedes approved comparison/selective merge. The 50,000-task acceptance goal and 100,000+ stress goal remain program targets; integration must not displace their reliability work.
- User grants are short-lived and rechecked at commit/publish; after recovery **fresh authorization** is required for any pending publication or write. AI/MCP and third-party executable distribution remain separate ADR subjects.

## 2. Separate three concepts that can otherwise be conflated

1. **Interchange worker**: parses or emits a user-selected static file (e.g. MPP through MPXJ). It has a bounded selected-byte stream and a one-shot inspection/import/export lifecycle.
2. **Provider connector**: communicates with an external multi-user API and observes changing external records (e.g. Jira). It needs a connection identity, user consent, scopes, sync leases, network budgets, incremental reconciliation, permission handling, and eventually a guarded remote outbox. A connector is an adapter, never a new project database or a privileged tool host.
3. **Canonical project model**: typed project and schedule state owned by GoPMgr. It stores imported/approved facts without adopting provider-specific workflow rules. A remote work-item snapshot is **not** automatically a scheduled activity.

The worker supervisor, registry, lifecycle/event envelope, evidence conventions, and job APIs can be shared. **Business record contracts and authorization cannot be homogenized.** Use separately versioned *domain protocols* for `file-interchange`, `external-work`, `analytics`, and, only if independently approved, `AI/MCP` and `UI extensions`. Each must pass a distinct security review. **Do not force an API provider into the static-file publication state machine**. Use distinct orchestrators: `file-import` and `provider-sync`, both calling host-owned validation, mapping, approval, audit, and persistence services. Native Go plugins, arbitrary UI injection, direct DB plugin APIs, and an unrestricted generic HTTP proxy remain prohibited.

## 3. Logical component and trust architecture

```text
                 Wails / Svelte (local authorized session)
   Connection Center | Snapshot Browser | Mapping/Preview | Job Center
                                    |
                                    v
+---------------------------- AUTHORITATIVE Go host ----------------------------+
| Identity/session epochs | permission and consent | connection policy          |
| Versioned capability registry | verified worker supervisor | local IPC        |
| Account-scoped encrypted credential vault | TOKEN/NETWORK BROKER            |
| Provider sync coordinator | durable checkpoints | leases/fencing            |
| Typed canonical mapper/fidelity ledger | validation | diff/approval         |
| Atomic host persistence / project-owned journal / existing audit chain       |
| Go scheduling, calendars, EVM, money, resources, project revision authority   |
+------------+----------------------------------------+------------------------+
             | authenticated gRPC/HTTP2 local IPC      | authorized SQLCipher
             v                                         v
+-----------------------------+             +-------------------------------+
| Signed bundled connector    |             | Account integration workspace |
| worker (Go Jira candidate)  |             | encrypted snapshots / mapping |
| API-specific transforms     |             | checkpoints, receipts / audit |
| NO DB/DEK/token/filesystem  |             | projects: separately encrypted |
| authority or hosted socket  |             | canonical data and evidence |
+-------------+---------------+             +-------------------------------+
              | typed request/response via broker
              v
       Host Network Broker ---- TLS ----> Jira Cloud REST / Agile APIs
          (host owns credentials, allowlist, quotas, security policy)

      Other providers plug into the worker boundary; core never imports
      `jira.*` types, routes, fields, OAuth details, or workflow state.
```

### 3.1 Recommended package boundaries (names provisional, verify repo conventions)

- `internal/interop/{contract,registry,supervisor,ipc,job,fidelity,mapping,evidence}`: **share ADR-010**, do not fork them for cloud.
- `internal/integrations/{connection,consent,broker,sync,checkpoint,projection,conflict,outbox,privacy}`: host-owned cloud coordination, typed policies, encrypted **account-scoped workspace** state; project publication remains a separate ADR-010/011 domain command.
- `internal/integrations/providers` should contain **only canonical capability vocabulary** and test fakes, no `jira` import.
- `workers/jira-cloud/`: optional signed Go executable implementing Jira-specific conversion and pagination strategy. Do not bundle unless a release explicitly opts in.
- `proto/gopmgr/integration/v1/`: *external-work domain* records and typed broker operations, referencing a shared versioned ADR-010 session/transport envelope; future analytics, document, AI and UI extension services get their own domain contracts, not new ad hoc worker transports.
- `frontend/src/...`: Connection Center, sync history, mapping inspector, privacy controls, and review UI through thin, revision-bound Wails APIs. The frontend is never the security authority.
- Repository ADR-010, ADR-011, release and testing documents maintain the source of truth. No copied competing contracts.

### 3.2 Runtime and security classes

1. **Built-in signed modules** only for initial Jira releases. Verify exact binary hash, signature/release provenance and manifest before every launch. No mutable self-trusting plugin directory.
2. Third-party executables remain **disabled** until a separate approved ADR demonstrates practical OS containment, revocation, update/signing and per-capability permission enforcement. A distinct process prevents crash propagation, **not** same-user filesystem/network access. Avoid unverified claims of sandboxing.
3. Scheduler, SQLCipher, project keys, signing and audit live in the Go host, never in workers. No host SQL broker. An OS process boundary **alone does not prevent token, filesystem or network access**; signing proves release provenance, not containment. Because a same-user bundled worker could bypass a host-only network broker, the reviewed distribution MUST either enforce egress/filesystem confinement with tested OS controls, or explicitly classify it as a **trusted bundled adapter with residual ambient OS authority** and prohibit claims of enforced egress isolation. No untrusted/third-party adapters under the latter classification. No raw arbitrary file paths, DB handles, key material, or shell evaluation.
4. The host owns the private UDS/named-pipe endpoint and authenticates every worker launch with a single-use operation-bound credential and worker digest; bind session incarnation, account, project-copy, job, scope, protocol version, TTL, and nonce. Restrictive IPC ACLs; authenticated per-method authorization; no silent transport fallback.
5. Process limits: max parallel workers, CPU/RSS, deadline, direct/child processes, stdout/stderr quota, request/response sizes, HTTP response sizes, decrypted scratch, disk quota, network concurrency. OS enforcement must be shown independently of advisory limits. Cancellation kills/reaps the process tree and does not expose partial project mutations. A shared fair scheduler prevents one connector or tenant from monopolizing CPU, memory, network, quota, or DB writers.

### 3.3 Plugin families and extension-scope invariant

A **plugin** is a signed/authorized unit of capability, **not** a promise that every type of extension shares one business API. The shared host controls registration, provenance, IPC, lifecycle, resource accounting, capability grants and revocation. Provider adapters are one category; file import/export, analytical functions, report generators and prospective AI/MCP modules are separate categories, each with purpose-limited domain services. Existing GoPMgr deterministic scheduling and encrypted persistence remain host-only even when future modules can request computations. Generic UI contributions are **declarative, signed, and host-rendered only after a separate UI permission/compatibility ADR**; no arbitrary JavaScript, DOM hooks or dynamic native module loading.

| Extension family | Initial authority | Distinct contract | Release posture |
|---|---|---|---|
| Static schedule file interchange (MPXJ) | selected source bytes; inspect/import proposal | ADR-010 file records | first bundled worker; G0–G4 gated |
| Remote work provider (Jira first) | scoped read operations via host broker | external observations, connection and sync | later optional, read-only first |
| Analytical computation | explicitly bounded immutable Arrow-compatible slices/results | typed analytical job / non-authoritative projections | independent ADR; no scheduling authority |
| Report/document exporter | explicitly authorized read-only project projection | typed export job; host-owned publishing | separate export safety and licensing gate |
| AI/MCP | proposal-only or read-only resources | separately permissioned tool/resource protocol | separate ADR; never generic plugin privileges |
| Community executable/UI | none by default | separately admitted capability-specific contracts | disabled until sandbox, signing, UX, revocation and governance proof |

**Extension evolution rule:** Adding a second provider MUST NOT change canonical table definitions, source identity semantics, host trust boundary, or scheduling APIs. It MAY introduce new **reviewed, signed operation-policy descriptors** and a provider-specific adapter; this is deliberate trust-boundary maintenance, not a failure of neutrality. A second *distinct family* validates whether registry/supervisor reuse is genuine, without requiring a universal business `Any` contract.

## 4. Manifest, capabilities, and contract evolution

The **internal** registry must be provider-neutral from its first accepted release. Its persisted identity keys are `(plugin_family, plugin_id, plugin_release_digest, protocol_major)`; provider connections are distinct `(local_user_incarnation, provider_id, tenant_id, external_principal_id, grant_id)` records. A registry manifest is **declarative metadata**, never a grant of authority. The host separately signs/approves an effective operation-policy digest and verifies that it is within the installed app release policy before enabling any capability. A manifest identifies `family_id`, `provider_id` (if applicable), `worker_id`, vendor/build provenance, version/digest, protocol min/max major+minor, compatible host/schema revisions, OS/transport matrix, required domains, data classes, claimed permissions, resource budgets, and *operation-specific* capabilities. Capability examples: `work.inspect`, `work.snapshot.read`, `work.delta.read`, `work.identity.resolve`, `work.field-metadata.read`, `work.issue-links.read`, `work.sprint.read`, `work.outbound.preview`, and conditional `work.outbound.apply`. The registry does not infer capabilities from a single `sync=true` flag.

**Provisional manifest, NOT an accepted configuration format:**

```yaml
api_version: gopmgr.integration/v1
provider_id: com.atlassian.jira.cloud
worker_id: org.gopmgr.jira-cloud
vendor: GoPMgr
version: 0.1.0
protocol: {major: 1, min_minor: 0, max_minor: 1}
runtime: {kind: signed_subprocess, transport: native_local_grpc}
remote:
  origins: [https://api.atlassian.com, https://auth.atlassian.com]
  authorization: oauth2_3lo_host_brokered # candidate, NOT verified
  default_schedule: manual
capabilities:
  - {id: work.snapshot.read, direction: inbound, maturity: experimental}
  - {id: work.field-metadata.read, direction: inbound, maturity: experimental}
  - {id: work.outbound.apply, direction: outbound, maturity: disabled}
privileges:
  project_db: false
  token_material: false
  host_files: false
  raw_network_broker: false # policy claim; not OS-enforced sandbox
```

The broker does not allow arbitrary HTTPS requests merely because an origin is listed; it enforces **typed endpoint policies** and per-operation permission. Provider policy packs MUST be distributed and verified as first-party signed artifacts or compiled audited descriptors, never mutable connector-supplied network policies. Each pack constrains method, scheme/host/path, tenant binding, request/response schema and size, query/body template, sensitivity, retries, credential scopes, and protocol version; **worker-supplied route templates and headers are data, not authority**. The example does not assert that OS-level egress denial is already enforced for a bundled process.

### 4.1 RPC design principles

- Reuse ADR-010 `WorkerSession.Connect` (single authenticated bidirectional session) or a versioned compatible service extension; no second incompatible bootstrap.
- Keep **source observations** separate from **canonical commands**: worker emits `SourceRecord` (provider-native identity, field state and provenance) and metadata; host derives proposed canonical changes. Do not accept `UpdateProject` from a worker.
- Typed protobuf `oneof` records for work items, users (privacy-limited), links, iterations, field definitions, field observations, attachments metadata, revision markers and capability errors. `Any`/untyped JSON cannot carry authoritative canonical mutations. Opaque raw source evidence may be retained **only** as tagged, encrypted, bounded, non-authoritative evidence with content type and fidelity classification.
- Explicit distinction between `absent`, `null`, `unreadable`, `unauthorized`, `unsupported`, `empty`, `zero`, `approximate`, and `not_requested`. Preserve native units, timezone, precision and source identifier namespaces.
- Versioned descriptor hash, canonical serialization for receipts, reserved protobuf tag numbers, golden compatibility fixtures across supported versions, known-error taxonomy and fail-closed unknown-major behavior. Unknown enum values and optional capability requests are explicitly rejected unless negotiated; never treat unknown as write-enabled or silently coerce to an old meaning. Host and worker maintain a pinned, signed support matrix with a defined deprecation window and upgrade/rollback policy.
- Bounded server/stream credit, chunk checksums, canonical record count/digest, independent host validation, ordered page acknowledgments and job cancellation. Never buffer all 50,000 records in a single Wails or gRPC message.
- Worker can request `RemoteRead` / later `RemoteWriteIntent` with an **opaque broker handle**; host supplies transport and authenticates remotely. Broker callback lives on the authenticated job stream and cannot reach unrelated accounts, connections, jobs or origins.

## 5. Auth, secrets and outbound network isolation — gating design

### 5.1 Host-owned broker (required)

The worker is **not given** API tokens, OAuth client secrets, refresh tokens, credentials, DEKs, direct project paths or SQL handles. Rather, it submits typed, policy-checked remote-operation requests to the host network broker. The host attaches credentials, performs HTTPS with verified certificate/name, enforces exactly permitted API routes/methods/body/response limits, masks headers, observes rate-limit signals, and returns filtered or bounded response data. No arbitrary URL fetch, redirect to unknown host, proxy override, cookie/header injection, localhost/cloud-metadata SSRF, or unlimited dynamic JQL pass-through.

**Broker contract:** `(connection_lease, provider_operation_id, typed parameters, host-issued request_id)` → `(sanitized status, retry policy, bounded response chunks, content digest)` with policy mapping `provider_operation_id` to one reviewed route + verbs + allowed query/body fields. Restrict token **use** to a specific connection and approved site `cloudId`; user permissions and granted scopes must be checked independently. Atlassian 3LO grants can authorize multiple sites for one account, so a bearer token alone is **not** an adequate single-tenant isolation boundary. Host records only redacted transport metadata in the diagnostic/audit channel.

A bundled Go worker may still possess ambient same-user OS permissions; until a real OS confinement mechanism exists, do not claim the worker literally cannot contact the network. Production code must avoid direct network use, and release tests must show the broker is the **only supported** data path; arbitrary third-party workers remain untrusted and out of scope.

### 5.1a Transport policy and broker bypass

A connection-scoped broker lease names the exact `(account_incarnation, project_copy_or_workspace, connection, tenant, provider_operation, scope, worker_digest, job, epoch, expiry)` and is single-purpose. The host must validate after parsing and immediately before the outbound request; redirect following is **disabled** by default, DNS responses and proxies cannot redirect requests to private/link-local/metadata ranges or a different tenant, and TLS hostname/certificate checks remain enforced. Query languages (including JQL) are constructed from approved scope selectors and an allowlisted typed filter AST, never unconstrained connector-supplied strings. Bulk lists have explicit cardinality, result-field and expansion caps. Every response is treated as untrusted input even after TLS. Broker and worker retain no raw token/log payloads. A credentialed browser/Forge sidecar, if ever approved, is a different trust model, not a silent fallback.

### 5.2 Credential vault and connection identity

- Account-scoped, encrypted **connection vault** separate from each `.gopmgr`, wrapping secrets through reviewed user-account key derivation/recovery semantics. Its key rotation, user password reset, admin/escrow access, lost device, profile restore and recovery-code workflows MUST be exercised against existing ADR-001/004/005 invariants; the vault must not invent a parallel weak key hierarchy. No token in `system.db` plaintext, project backups, source evidence, log files, CLI args, environment variables, or manifests.
- Binding identity: `local_account_incarnation + provider_id + external_tenant_immutable_id (Jira cloudId) + external_user_account_id + auth_grant_id`. A connection can be linked to multiple local projects through individually authorized project bindings. Never use hostnames or display names as immutable keys.
- Local project state holds an opaque `connection_id` and tenant/work-scope identity; backup/restore to another machine **does not transfer credentials**. Reconnect intentionally and verify tenant/user/permissions before resuming. Prevent restored forked projects from sharing a writable sync identity accidentally.
- Every local session switch, password reset, rollback/restore, account deletion, project-copy event and connector disable must invalidate or revalidate broker leases and sync workers. Project copy identity is not original project identity.
- OAuth access and refresh tokens are encrypted at rest and minimized in memory; refresh token rotation uses a serialized refresh operation, durable replace, and lost-response/reuse recovery without indiscriminate retries. On ambiguous recovery, require renewed authorization rather than guessing a working refresh token.
- Provide account-level connection revoke, site-binding disconnect, per-project unlink, stop schedules, and local source-data retention/purge controls; make their effects explicit.

### 5.3 Jira OAuth feasibility must be proven before shipping

Atlassian's documented external 3LO token exchange requires a **client secret**; a publicly distributed desktop executable cannot safely contain a shared client secret. Atlassian also says marketplace apps must not collect customer API tokens or tell customers to register separate 3LO apps. **Do not claim desktop-only public OAuth is solved.** G-C-AUTH must select and test an officially supported deployment/auth topology before public Jira release.

**Preferred candidate:** narrow optional confidential OAuth/token-exchange broker with protected secret, authenticated local handoff, least-privilege processing, no storage of project schedules, documented privacy policy and data-flow review. Crucially it serves **only optional cloud connector authentication**; core GoPMgr, imported project viewing, and scheduling require no broker. A desktop connector should not silently proxy all Jira records through this service; host-device direct API calls where supported are preferred. Examine operational burden, breach response, trust, uptime, costs, hosted-state minimization, callback protections, and GDPR/enterprise policy; this is **not yet approved**. If no compliant acceptable auth topology exists, Jira Cloud connector remains BLOCKED for general distribution, not downgraded to embedded secret or token collection.

### 5.4 Hosted auth decision alternatives and user privacy

The feasibility review MUST compare at least: (A) optional confidential credential-exchange service using verified OAuth `state`, callback/CSRF defenses, **PKCE only if Atlassian documents and supports it in the selected flow**, protected secrets, bounded callback lifecycle and explicit minimal metadata retention; (B) any **actually documented and supported** public/native client flow, if available at implementation time; and (C) no broadly distributed Jira connector until an acceptable flow exists. Do not assume (B) exists. A hosted callback/exchange service increases attack surface, availability and ongoing maintenance costs; its outage must affect only connect/refresh, not existing local project use. Document whether the service sees access/refresh tokens, how it avoids retaining them, how grants are revoked, and user-visible trust boundaries. No hosted service may silently become a project-data relay.

## 6. Connection and data model

### 6.1 Separate external observational truth from local editable truth

- `ExternalConnection`: opaque account-owned auth reference; tenant/user/scope/supported capability state. No credential is attached to a portable project file. A project may contain a *non-secret* revoked/inert source reference for provenance. Secrets not in project.
- `ProviderBinding`: links an encrypted **account-level integration workspace**, and optionally a project copy, to one provider tenant and selected remote scope; includes owner, privacy/retention policy, permitted fields and scope revision. A project-less linked workspace is a valid first-class object.
- `SourceSnapshot` / `Observation`: the remote object/field as observed (with provenance, source version/time, type, visibility state, and read completeness). This is immutable evidence, **not a canonical GoPMgr activity**. Remote raw payloads are privacy-bounded source evidence; indexed typed fields are a separately rebuildable projection.
- `EntityLinkEvent`: append-only asserted/rejected link from `(provider, tenant, object_type, immutable external object ID, source lineage)` to a stable local host entity ID, with mapping schema version, confidence and author/approval. Renaming `ABC-123` to `XYZ-123` must not produce a new identity; ID reuse and clone forks need review.
- `CanonicalProjection`: versioned, explicitly configured and validated transformation from observations into eligible work items/activities. Every projected value records transformation, presence, ownership, source and update policy.
- `SyncCheckpoint`: connection/binding/epoch, job attempt, remote page marker or incremental fence, overlap policy, last known coverage, batch digests, next state. A cursor is **not** proof of a remote snapshot transaction.
- `RemoteOutboxIntent`: only after later approval; a human-reviewed field-level change with expected local revision and independently verified remote preconditions. No worker writes or silently retries it.

**No one `sync_status` or `issue_id` string is sufficient.** Provider IDs, site IDs, collection/project IDs, entity IDs, types and revisions are independently typed and versioned. External work-item status, sprint, story points, assignees and links remain source fields until a mapping explicitly confers schedule meaning.

### 6.1a Storage placement, isolation and ownership — resolves v1 ambiguity

The previous proposal described *project-owned* snapshots while promising **linked workspaces with no local project**. That is inconsistent. Adopt two explicitly different encryption and lifecycle domains:

1. **Per-account encrypted integration workspace store** (proposed new SQLCipher-backed account artifact, subject to ADR-011/versioned schema approval): authoritative only for *external observations*, scope metadata, candidate/active read-only snapshot pointers, staging pages, lease fences, import/export evidence receipts, sync checkpoints and local diagnostics. Its decryption key follows reviewed account DEK/recovery semantics. No tokens are stored here. An account can browse cached linked workspaces without creating a GoPMgr project; user switch revokes all in-memory references. Versioning and encrypted backups apply independently of project files.
2. **Per-project `.gopmgr`**: authoritative for GoPMgr schedule/project state and its own local revision/audit. An approved import materializes a bounded, selected, self-contained **project-owned source evidence copy** from an immutable workspace snapshot using ADR-010 staging/seal/publication; external-to-local entity links are recorded with project-copy identity. This may not depend on the continued existence of the account workspace for the project's offline opening, editing or backup/restore.
3. **Per-account encrypted credential vault**: independent from both above, holds OAuth secrets and grants. Vault lifecycle and storage must be reconciled with the local account key model and backup policy.

No transaction spans these stores. Materialization is a **saga with a durable, idempotent import operation ID**: pin immutable workspace revision and content digest; stage a self-contained encrypted project; verify counts/fidelity and hashes; seal and publish through the existing atomic project-folder boundary; only then record a best-effort *account workspace link acknowledgment*. If acknowledgment is lost, replay inspects the published project receipt by operation ID and never duplicates the project. If the source workspace is deleted after successful publication, the project remains usable and honestly reports lost live-sync eligibility until re-linked. Conversely, disconnecting a provider never deletes user-owned schedule work.

Do not persist private remote data in `system.db` or global unencrypted caches. No cross-account content-addressed deduplication. In-account snapshot reuse across bindings is an **optional future** optimization requiring identical authorized scope/visibility, matching consent and robust revocation proofs. Account-level caches MUST have quota/retention and purge controls, with access revocation states represented separately from explicit user-owned project materialization.

### 6.2 Three user modes, never silently conflated

1. **Linked read-only external workspace:** browse/filter/search *account-scoped encrypted* cached observations and provider metadata without converting them to schedule activities or requiring a project to exist. Earliest recommended Jira production mode.
2. **Create new GoPMgr project from external scope:** use ADR-010 G3 host-controlled inspect → map → preview → stage → seal → publish (project-owned source evidence), only if fidelity and normalized storage gates pass. Reconciliation after publication acts on the link/snapshot, not as an implicit destructive re-import.
3. **Compare and selectively merge into existing schedule:** requires ADR-010 S6 and ADR-011 approved shadow-generation/same-transaction atomic pointer switch; explicit field ownership/conflicts, dry-run diff and renewed authorization. Never make synced records canonical just because they share a name or Jira key.

If a local project contains editable GoPMgr fields plus externally linked observations, each field declares ownership (`remote`, `local`, `derived`, `manual_shared`), allowed direction, and conflict behavior. Ownership is **not** an implicit override.

### 6.3 Fidelity and semantics

Use ADR-010 D-09's **multidimensional** ledger for source presence, transformation exactness, representation, editability, exportability, validation and retention. Track per-field Jira custom-field ID + schema and options; allow schema drift. Preserve Atlassian Document Format (ADF) or an explicit non-rendered opaque representation where policy permits, never silently flatten rich text. Never conflate story points with effort-hours, sprint close with a deadline, status with percent-complete, `blocks` links with CPM finish-to-start, or Jira `due date` with a hard date constraint. Calendars, timezones, currency and availability require separate user-reviewed policies. Attachments need explicit consent, quotas, malware/content scanning strategy, metadata-first imports, and safe rendering; exclude binary download in initial scope.

## 7. Durable synchronization protocol

### 7.1 Core state machine

`DISABLED → AUTH_REQUIRED → CONNECTED_IDLE → DISCOVERING → READING → STAGED → VALIDATING → RECONCILING → SEALED → COMMITTING → COMPLETED` **for a single attempt**; the currently active snapshot pointer remains independent. There is never a state in which a partially observed candidate is silently promoted.

Exceptional durable states: `PAUSED_OFFLINE`, `PAUSED_RATE_LIMIT`, `PAUSED_PERMISSION`, `NEEDS_MAPPING_REVIEW`, `CONFLICT`, `NEEDS_REAUTH`, `FAILED_RETRYABLE`, `FAILED_PERMANENT`, `QUARANTINED`. The **last verified active snapshot remains intact** until a new local revision pointer commits. Worker cancellation/death cannot advance it. Crash recovery resumes only from independently validated durable checkpoints; replay is idempotent by `(binding, attempt, external key, revision/version, field observation hash)`.

### 7.2 Initial read

1. User explicitly enables connection, selects Atlassian tenant/site and Jira work scope, sees scopes/data retention and authorizes read-only permissions. Binding is created with authenticated local account/project copy and expected project revision.
2. Host obtains a fresh, tightly scoped sync lease, launches the signed worker through ADR-010 native gRPC, and exposes **only** reviewed broker read operations.
3. Worker enumerates remote metadata, issue fields and selected work objects with bounded pagination; host revalidates IDs, permissions, data size/type and page checksums; each page is transactionally committed as an **invisible staging attempt**. Separate page acknowledgments avoid the fatal failure mode of advancing cursors before persistence.
4. Host computes a consistency/coverage report: pages, selected scope and scope hash, permissions, record counts, dedup count, unsupported types, remote change window, metadata versions, source-query class, missing/unknown count and truncation. Since Jira APIs are eventually consistent, **never claim an instantaneously consistent remote snapshot**. Use source object IDs and server-observed versions for dedup; a timestamp-only watermark is insufficient when multiple items share a timestamp or visibility shifts. Use an inclusive `(updated_at, stable_provider_id)` boundary where the provider supports it, otherwise a conservatively overlapping window with periodic full inventory. Use overlapping windows and a second reconciliation sweep where needed.
5. Host validates mapping and required field coverage; creates a candidate snapshot revision; retains last known-good revision while new one is incomplete. User sees incomplete/error truth, not success.
6. In one authorized **account integration workspace SQLCipher** transaction, commit snapshot revision pointer, host-owned typed receipt, mapping observations and a compatible audit/provenance record. Any later project import writes its **own** project audit record in its own separate transaction; do not assert a nonexistent atomic cross-database audit. External source evidence and any necessary encrypted sidecars are sealed and verified first. All still-subordinate to project ownership and ADR-011 migration gates.
7. Release worker, broker lease and scratch resources. Reconcile an expired/crashed `RUNNING` lease using a monotonic **fencing generation**, file/process ownership lock and durable single-writer election; lease timeout alone must never authorize concurrent writers or split-brain promotion. Local monotonic timers may govern in-process deadlines, while persisted wall-clock timestamps are only hints subject to clock skew. Account/session change invalidates leases immediately; no publication after recovery without fresh authority.

### 7.3 Incremental read and reconciliation

- Incremental polling is **user-enabled**, off by default, governed by per-connection rate limits and device battery/network policy. Manual `Sync now` must always be available. No cloud service is needed to read previously committed snapshots offline.
- Jira `/rest/api/3/search/jql` uses `nextPageToken` for pagination and has eventual-consistency behavior. Persist page tokens **only after** the staged page and digest are durable. Tokens may expire or results may move while paging; on invalid token, restart the bounded time window and deduplicate.
- Prefer documented `updated >= lower_watermark` queries with overlap, deterministic local dedup and re-read of uncertain records. Track an **observed** remote update high watermark, never advance it based on an unsuccessful or partial scan. A full/periodic scope inventory is necessary to detect deletions, reparenting, and permission changes missed by deltas. Full scan success proves only **coverage of the requested observable scope under the observed permissions**, not a globally consistent Jira point-in-time snapshot. The active revision MUST carry that distinction. Never treat missing from a partial/permission-restricted query as deletion.
- Local snapshot revision commits are atomic even though the remote scan is not an atomic point-in-time Jira read. Persist source-observed timestamps, coverage windows and uncertainty flags. Users must be able to identify stale or incomplete fields.
- Webhooks are optional hints only: verify authenticity and replay/reordering, store minimal event references, deduplicate and enqueue normal reconciliation. **No embedded inbound server or externally reachable callback required for local GoPMgr.** Jira dynamic webhook registrations expire; without a user-approved hosted ingress, use polling exclusively.
- Broker enforces Jira quotas, `429`/`Retry-After`, adaptive budget and jittered backoff, and singleflight requests per binding. Fair scheduling is global across active bindings and scoped to per-app, tenant, endpoint and per-record budgets; distinct endpoints may have separate point-cost and burst rules. Keep service-aware quotas learned from actual headers and policies, not hardcoded promises. A rate-limit event never corrupts or discards a committed snapshot. Do not run scheduled background polling on every app start without user consent.

### 7.3a Efficient 50k+ processing without semantic shortcuts

Use a two-tier ingestion strategy: **sparse inventory** of stable IDs, revision markers and selected single-value fields; then **selective hydration** of ADF, multi-value custom fields, worklogs, changelogs, sprint/link details or attachments metadata only for requested views, changes, and audited mappings. Metadata/schema is cached by tenant and scope revision with explicit invalidation. Page/chunk streaming writes into typed indexed tables and bounded append-only staging, not unbounded JSON aggregation, Arrow as transactional state, or Wails megablobs. Worker/host have explicit application-level flow-control credits. Use keyset paging from indexed local snapshot projections for the UI and narrow SQL queries; calculation and rendering use cancellable revision-bound jobs.

**Jira-specific performance opportunity (not generic protocol):** As of the 2026-08-28 Atlassian changelog, `/rest/api/3/issue/bulkfetch` accepts up to 1,000 issues when explicit ≤100 eligible single-valued fields and permitted expands are used; otherwise the normal limit is 100. Changelogs have a separate bulk endpoint. The connector can negotiate and instrument these operations; it MUST fall back to supported smaller batches, and MUST NOT silently omit multi-valued data to claim full fidelity. Measure API **point cost, burst budget, per-issue write limit** and total wall-clock time rather than optimizing request count alone. A server-imposed rate limit may make 50k sync slow without violating local performance obligations; expose the verified constraint and quality of cached data.

### 7.4 Deletes, privacy, permissions and uncertain absence

Classify source states separately: `ACTIVE`, `DELETED_CONFIRMED`, `ARCHIVED`, `MOVED`, `NOT_VISIBLE`, `AUTH_EXPIRED`, `SCOPE_REMOVED`, `PROVIDER_UNAVAILABLE`, `UNKNOWN`. `404`, `403`, vanished JQL membership and unsupported filters must not be treated interchangeably. If a confirmed delete has associated local edits, retain local user data under the approved retention policy while retiring the external link with an audited tombstone. Changes in visibility trigger quarantine/redaction policy and compliance review, not blind preservation of inaccessible external sensitive data.

Minimize stored personal data; support explicit disconnect, revocation, source purge and policy-bound retention. Never place personal payloads or credentials directly in immutable audit events (use hashes/reason codes/receipt IDs instead). Deletion/erasure must preserve audit-chain integrity via redaction/tombstone receipts and encrypted payload removal, not audit rehashing. Privacy policy may require erasing external data even if the local user would prefer to keep it; document this before enabling enterprise connections.

## 8. Outbound writes (NOT initial delivery)

The default provider contract is **read-only**. Remote writes are a distinct capability requiring separate scope, grant, policy, test evidence, human review, and release flag. Because a cloud API operation can commit remotely while the client loses its response, distributed **exactly-once effects cannot be promised**.

### 8.1 Two independently durable boundaries

**Local transaction:** host atomically validates local session, source ownership, project revision, chosen fields, remote target, and approval; writes immutable `RemoteOutboxIntent` and audit event to the project DB. No network call inside the transaction. The user can cancel unsubmitted intents, not already sent writes.

**Remote delivery:** a bounded sender obtains a fresh user-authorized lease, checks scopes/tenant/identity/remote current state, marks `DISPATCHING` durably (with attempt receipt), and asks the broker to execute only a typed, allowlisted API operation. After HTTP success, it re-reads the exact target, verifies semantic postconditions and records the remote version/evidence. On timeout or lost response, mark `OUTCOME_UNKNOWN`, **do not blindly retry non-idempotent writes**, and reconcile by reading remote state; if ambiguous, stop for manual resolution. An outbox row becomes `CONFIRMED` only on adequate remote evidence. **A matching remote value alone does not prove this request caused it** (another actor may have made the same change); receipt wording must distinguish `POSTCONDITION_OBSERVED` from `PROVIDER_CORRELATED_COMMIT` where a provider supplies transaction/correlation evidence. No speculative local rollback of a known or unknown remote effect.

### 8.2 Concurrency and collision controls

- A fresh read before write is **not a guaranteed compare-and-swap**. If Jira lacks a strong provider-supported per-field conditional update, arbitrary racing edits can still be overwritten. Automatic bidirectional updates to those fields MUST be disabled; individual user-reviewed writes may be offered only with an explicit remaining race warning and verified policies.
- Status changes use provider-supported workflow **transitions**, never a generic direct status field overwrite. Do not perform mass create/delete/move, attachment upload or bulk transition in the first writable release.
- Revoke/cancel at every phase that has no irreversible remote effect. Revalidate user, session epoch, tenant, permission, remote target, local project generation and field scope immediately before dispatch. After crash, require **fresh authentication and fresh dispatch authorization**; persisted approvals alone never cause new writes.
- A write received by Jira is not necessarily instantly searchable; use direct item read and the documented `reconcileIssues` consistency aid where relevant, with bounded retries and explicit uncertainty.
- Reverse conflicts are never resolved by implicit timestamp-last-writer-wins. Present three-way source/local/base diff, dependency implications, users and approval choices. Log provenance without sensitive raw content.

## 9. Transactional durability, backups and restores

- The **account-scoped encrypted integration workspace** is authoritative for connection binding, read-only snapshots, checkpoints and sync receipts; **project SQLCipher** is authoritative for project mapping/materialization, local scheduling state, approved local merges and any future project-scoped remote outbox. The two MUST NOT be represented as one atomic database. The credential vault is separately account-owned, encrypted and access controlled; **there is no cross-database atomic transaction**. Recovery must tolerate a project referencing a missing/revoked connection by requiring reconnection rather than attempting a cross-DB rollback.
- Every persisted **committed** state transition requires a durable idempotency key and atomic transaction in its own store; transient probe/progress counters may be lossy and MUST NOT be misrepresented as authoritative state; preserve old audit bytes. New audit events use exact string encodings for currency/revisions and minimal privacy-safe payloads. Use verified schema constraints and `foreign_key_check`/cipher-integrity tests under the actual bundled SQLCipher version.
- Project-owned encrypted source evidence (or in-DB encrypted evidence) must travel with project backups, project copies and restores; workspace snapshots have a separate account-level encrypted backup/recovery contract, with secrets explicitly excluded; any separate vault token never does. A restored project is **read-only relative to cloud synchronization until rebound**, because restoration can fork old identity and otherwise duplicate outbound writes.
- Crash consistency is at the local transaction or atomic shadow-generation activation boundary, not across an HTTP request. Anti-corruption: no worker or connector can bypass the project's canonical command path, min-writer-version gate, or revision fencing.
- No normal cleanup may delete staging for which ownership or completion is uncertain; quarantine with size limits, release evidence, repair tooling and user-visible recovery. Remount/reopen tests must verify receipts, snapshots, mapping and journal state, not only byte-level DB opening.
- A reliable disconnect does not necessarily delete user-owned locally edited data. The UI must distinguish `stop network`, `unlink external identity`, `purge cached provider data`, `revoke remote grant` and `delete local project`. Disclose that remote revocation may require an Atlassian account action.

## 10. Security and abuse cases

| Threat | Mandatory control |
|---|---|
| Malicious/compromised worker or supply chain | signed digest-pinned binaries, launcher validation, bounded execution, no keys, no DB access; third parties blocked until sandbox ADR |
| Confused-deputy broker / SSRF | typed operation allowlist, exact approved Atlassian API host and cloudId, TLS verification, no arbitrary URLs/redirects/headers |
| Credential exfiltration | host-only encrypted vault, scrubbed diagnostics, secret not in worker/RPC/persistent project, broker per-job token use |
| Cross-account/project read | binding includes authenticated principal incarnation, project-copy, immutable tenant, session epoch; recheck at use/commit |
| Replay or outdated worker | one-time bootstrap, nonce/job binding, signed artifact proof, revocation, expected revision and typed capability checks |
| Partial query/data loss | staged attempts, dedup, page-ack after commit, completeness report, never activate partial as complete |
| Misinterpreted absent record | permission-vs-deletion state taxonomy, recurring full reconciliation, no destructive inference |
| Remote overwrite race | read-only default; preflight re-read; no autonomous write without proven provider CAS semantics |
| Unknown remote result | outbox `OUTCOME_UNKNOWN`, verify by read, halt non-idempotent retry |
| Metadata/PII leak | encrypted cached snapshots, consent, retention/purge, no sensitive audit/log payload, safe rich-text handling |
| Poisoned/oversized rich text | content/expansion quotas, safe ADF parser, URL and attachment allowlists, strict UI escaping |
| Retried OAuth refresh and revoked grants | serialized rotation + atomic replace, reauth on ambiguity, per-account revoke handling |
| Abandoned job/crash/power loss | durable per-page/checkpoint receipts, audit verification, bounded recovery, no stale authorization |
| OS network/file privileges | explicitly distinguish host policy from OS-enforced sandbox; no public binary extensions before real confinement |

### 10.1 Local-only observability and supportability

Record versioned, stable reason codes and correlation IDs across host, worker, broker and account/project receipts; partition runtime metrics by binding and provider endpoint without recording bearer tokens, JQL content, PII or project descriptions. Support a user-controlled, time-bounded **redacted diagnostic export** and on-device introspection of recent sync attempts. No external telemetry, automatic crash reporting or hidden network pings. Require deterministic support bundle schemas, retention and log rotation; avoid audit-chain bloat from repeated progress events. Add administrative actions to disable connector, repair an abandoned staging attempt and verify integrity without requiring re-authentication to merely view old results.

### 10.2 Compatibility, upgrade and deprecation

Define compatibility as a **triple**: `(host/project storage version, shared worker-session protocol, domain-specific capability schema)`, alongside plugin manifest/policy digest and provider API/version. Upgrades run preflight, encrypted backup and tested migration of workspaces and project links separately. Disallow a new worker from silently opening or rewriting older persisted data; the worker never opens storage at all. Project file format gates cover every writer including repair/CLI/restore; account workspace format gates cover every workspace writer. Older hosts MUST fail read-only or offer supported migration when they encounter unknown newer schema; no blanket rollback promise for pre-gate builds. Retirement keeps project snapshots accessible offline, disables outbound activity, and gives migration/export/purge choices; vendor API deprecation should not corrupt local state. Pin protocol descriptors and golden backward/forward compatibility tests, support window and staged feature kill switches.

## 11. Hard non-goals, deferred decisions and rollback

- No mandatory hosted GoPMgr sync, continuous collaboration, single cloud truth for `.gopmgr`, or direct Jira-powered scheduling kernel.
- No Jira fields in canonical DB tables or hard-coded Jira dependencies in common packages; provider-specific extras live as namespaced, typed versioned observations.
- No unreviewed DB format upgrade, SQLCipher key change, audit canonicalization rewrite, or automatic chart-to-schedule migration.
- No initial cross-provider write orchestration, cross-tenant identity auto-matching, multi-account autonomous dispatch, binary attachment replication, public SDK, marketplace or third-party executable installation.
- No provider write when offline or application closed unless a separately approved user-controlled background process and persisted authorization model exists. This proposal assumes **app-open/manual sync first**.
- **Rollback strategy:** provider feature disabled without changing project editability; keep existing committed snapshot and audit; stop broker/network and worker; invalidate lease and outbox dispatch; inspect/repair staged attempts; project file version gates govern app downgrade; do not roll back a remotely committed edit by local DB rollback.

## 12. Explicit unresolved decisions (BLOCK until evidence and maintainer acceptance)

1. **G-C-AUTH**: Atlassian-compliant confidential OAuth deployment and secure user handoff, secret custody, operational service/security/compliance cost and acceptable no-host fallback (if supported).
2. **G-C-IPC**: real packaged Windows named-pipe or approved Windows UDS gRPC for Go cloud worker; do not infer Java G0 from Go-only tests or vice versa.
3. **G-C-DB**: ADR-011 exact **account workspace + project** storage, separate migration/format guards and audit receipts, shadow pointer atomicity, encrypted evidence, private credential-vault schema and independent backup/restore recovery. Validate import-saga interrupted cross-store acknowledgment and same-user multi-process fencing.
4. **G-C-PRIVACY**: Atlassian data, user identity, attachments, consent, erasure, enterprise data-access policy and local retention; adjudicate conflicting privacy and audit requirements.
5. **G-C-SYNC**: Jira tenant/site API eventual-consistency coverage, full reconciliation, rate-limit characteristics and actual 50k-object performance; do not invent provider atomicity.
6. **G-C-WRITE**: per-field Jira conditional-write/transition capability and concurrency guarantees. Without real provider CAS, no unattended bidirectional write capability.
7. **G-C-EXT**: extension-family boundaries, signed immutable provider endpoint policy, protocol backward compatibility, deterministic fake-provider/second-real-provider exercise, worker egress/OS sandbox claims; reproducible signing/SBOM/licensing and accessibility.
8. Compatibility with current `docs/VISION.md` focus on small teams and roadmap P0 release backlog; expansion to 50k and integration must be explicitly approved as a product-direction refinement, not silently treated as already implemented.

## 13. Primary sources and repository anchors (recheck at implementation)

- GoPMgr architecture: https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/ARCHITECTURE.md
- GoPMgr roadmap: https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/ROADMAP.md
- GoPMgr vision: https://github.com/draconianneocoder-cpu/GoPMgr/blob/main/docs/VISION.md
- Jira REST v3 overview: https://developer.atlassian.com/cloud/jira/platform/rest/v3/intro
- Jira enhanced JQL search: https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-search/
- Jira search and reconcile (only specified issues; not globally consistent): https://developer.atlassian.com/cloud/jira/platform/search-and-reconcile/
- Jira 2026-08-28 bulk-fetch scaling change: https://developer.atlassian.com/cloud/jira/platform/changelog/
- Jira bulk fetch issue/changelog contract: https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issues/
- Jira 3LO OAuth: https://developer.atlassian.com/cloud/jira/platform/oauth-2-3lo-apps/
- Jira rate limits: https://developer.atlassian.com/cloud/jira/platform/rate-limiting/
- Jira webhooks: https://developer.atlassian.com/cloud/jira/platform/webhooks/
- Jira ADF: https://developer.atlassian.com/cloud/jira/platform/apis/document/structure/
- Atlassian Cloud app security: https://developer.atlassian.com/platform/marketplace/security-requirements/
- Atlassian Forge future: https://developer.atlassian.com/cloud/jira/platform/forge/

**Acceptance:** approval in principle is recorded, but maintainer must explicitly accept normative ADR text and independently approve scoped implementation work packages. Blocking gates are resolved by verified evidence and tracked owner decisions (ACRs only for actual architectural changes), not by lowering requirements. Identified open questions remain open until then. The first Jira release is **read-only and opt-in**; stronger capabilities advance only after their own proof and approval.
