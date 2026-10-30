<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Contract and recovery reference — provider-neutral framework v2.0

**Status:** Design reference. Examples and layouts below are *not migration-ready SQL or generated protobuf*. The supplemental ADR is the normative candidate; ADR-010 and ADR-011 remain separately gated.

## A. Authority, stores and flows

| Object | Storage owner | Version/fence | Allowed inputs | Recovery rule |
|---|---|---|---|---|
| Connector artifact + operation policy | Verified signed application artifact | version, digest, protocol range | signed release approval only | reject substituted/downgraded policy; preserve cached snapshots |
| Connection credential | Encrypted account vault | account incarnation, grant version, refresh serialization | host authentication/OAuth broker only | ambiguous refresh → reauthorize |
| External account workspace | SQLCipher encrypted *account* artifact | workspace schema writer gate + active snapshot revision | broker-validated provider observations | last committed revision survives failed sync |
| Workspace staged sync attempt | Same encrypted account artifact | job ID + binding scope revision + fence generation | bounded typed batch, independently verified page digest | resume from last committed checkpoint or restart bounded scope; never activate partial |
| Project source evidence | Encrypted project-owned evidence + `.gopmgr` | project copy, import receipt, source digest | explicitly reviewed immutable workspace snapshot | sealed import remains self-contained, independently backupable |
| Local canonical task/plan | Per-project SQLCipher `.gopmgr` | approved normalized generation + project revision | host command with mapping/fidelity/approval | all-or-nothing project transaction/shadow generation |
| Future outbound intent | Project transaction/audit | intent ID, user/tenant, expected revision, dispatch receipt | user reviewed approved fields only | outcome uncertain means reconcile; do not blindly resend |

**Invariants:** no shared transaction spans account workspace, project file, vault or remote Jira. A project does not require the workspace/vault to be present to open and edit offline. An account workspace cannot inherit a project's data encryption by simply pointing at its path; use reviewed account-wrapped key semantics. A deleted provider connection cannot silently erase locally authored work.

### B. Typed architecture vocabulary

```
PluginFamily: FILE_INTERCHANGE | EXTERNAL_WORK | ANALYTICS | DOCUMENT_EXPORT | (future AI_MCP | UI_DECLARATIVE)
PluginCapability: {family, operation_id, direction, schema_major/minor, risk_class, consent, policy_digest}
WorkerSession: {verified_artifact_digest, launch_nonce, protocol_range, account_incarnation, job, epoch, resource_policy}
ProviderConnection: {provider_id, tenant_id, external_principal_id, grant_id, account_incarnation}
RemoteScope: {tenant, immutable_collection_ids, permitted-field-set, scope_version, consent/retention}
ProviderObservation: {entity_type, native_id_namespace, native_id, native_revision?, fields[], source_time?, observed_time}
FieldObservation: {field_id, type, presence, fidelity, representation, value_or_encrypted_evidence_ref, privacy_class}
SnapshotReceipt: {scope_hash, attempt, fence, root_digest, counts, coverage, consistency_class, schema/policy versions}
MaterializationReceipt: {source_snapshot_digest, project_copy_id, operation_id, fidelity_ack, project_audit_receipt}
```

**Do not** implement the above as a generic `map[string]any`, protobuf `Any`, JSON mutable schedule payload, or a table of untyped status strings. `FieldObservation` has a discriminated, versioned value type; extensions remain tagged non-authoritative source evidence when no canonical mapping exists. Wire negative/unknown enum values fail closed. Business capability protocols differ by family while session/transport management is shared.

### C. Runtime authority and broker permit

Effective network permission is `signed provider policy ∩ host supported operation ∩ user grant ∩ binding/field scope ∩ remote OAuth scope ∩ job lease ∩ runtime platform policy`. A signed manifest declares requested privileges but does not issue them. The worker cannot select a new hostname, path template, HTTP verb, credential or arbitrary JQL; broker policy uses signed fixed routes and a bounded typed query/filter AST. Each request gets a job-scoped correlation ID, bounded payload, TLS verification, redirect prohibition, response schema validation, deadline and cost allocation. Tokens live in the host/vault only.

**Trusted-worker limitation:** unless real OS confinement is implemented, same-user signed bundled workers retain ambient OS permissions even though GoPMgr withholds host capabilities. Broker design is the authorized application channel, *not* proof the process cannot connect elsewhere. Community executable plugins must remain unavailable until independently reviewed OS isolation and distribution controls exist.

### D. Initial sync recovery example

```
UNBOUND
  → authorized BINDING/GRANT [account encrypted]
  → ATTEMPT_CREATED(binding_rev, fencing_generation)
  → PAGES_STAGED(page_digest, last_token, cumulative_digest) [not active]
  → COVERAGE_VERIFIED(permissions, counts, fields, uncertainty)
  → REVISION_SEALED(root_digest)
  → ACTIVE_POINTER_ATOMIC_COMMIT + workspace audit receipt
  → worker/lease cleanup
```

Failure before activation leaves the previous active revision unchanged. A delayed stage writer with a stale fencing generation must never commit. A duplicate page/attempt receipt is rejected or merged idempotently under the same source scope, stable object identity and typed revision. Page tokens expire; recovery may restart an overlapping scan with dedup. Source `updated` time and cached count are not snapshot isolation; the UI must show coverage/consistency qualification. Do not infer remote deletion from missing JQL results.

### E. New project materialization (cross-store saga)

```
1. Authenticated user selects immutable workspace snapshot revision, source scope, mapping rules.
2. Host verifies revision/digest, field fidelity, authorization and project import permissions.
3. Host creates operation_id and writes durable, project-scoped import journal.
4. Host builds project-owned encrypted candidate with required independent source evidence.
5. Host validates foreign keys, audit chain, exact counts, digests, SQLite/SQLCipher integrity.
6. Host seals and publishes complete project folder via ADR-010 approved no-replace path.
7. Host acknowledges external workspace association in account store (best effort).
8. On crash after step 6: locate published operation_id, reconcile link; NEVER publish twice.
```

No network request or credential is required to finish/recover a fully sealed local project publication; however, ADR-010 still requires fresh local session approval to publish a SEALED-but-unpublished artifact. Import source retains the snapshot revision and per-field provenance. Cross-store link acknowledgment never determines whether a project is valid.

### F. Remote outbox (future, distinct approval)

```
APPROVED_INTENT (atomic project transaction, audit)
  → fresh session + tenant + field + role + local revision verification
  → DISPATCH_ATTEMPT_PERSISTED
  → HTTPS request via signed endpoint policy
  → provider correlation evidence (if available) / direct object reread
  → CONFIRMED_CORRELATED | POSTCONDITION_OBSERVED | OUTCOME_UNKNOWN | REJECTED
```

Never claim exactly-once remote effects. An identical remote field value does not establish who wrote it. Jira may not provide a strong compare-and-swap for a given field; if it does not, unattended bidirectional updating of that field is blocked. No network inside SQLCipher writer transactions. Never automatically retry unknown non-idempotent effects. Unknown outcomes remain inspectable and user-resolvable.

### G. Capacity and compatibility checklist

- First pass: immutable IDs, changed markers, narrow single-valued fields. Second pass: hydrate expensive multi-value, ADF, changelog/worklog metadata based on request, mapping and completeness policy.
- Jira bulk-fetch up to 1,000 is **conditional** on endpoint rules and field selection; use fallback ≤100 for ineligible shapes. Verify the current contract before coding; small HTTP request count alone is not performance proof.
- Bound total in-flight bytes across gRPC/HTTP/SQL/Wails, field-rich payload expansion, CPU, process count, disk, file descriptors, retries, quota points, and time. Use fair tenant/binding scheduling and cancelable workers.
- Account-workspace schema and per-project `.gopmgr` schema have *separate* min-reader/writer compatibility, encrypted backups, repair/restore acceptance. Signed plugin protocol/manifest and provider API policies evolve independently.
- A second **real different work provider** must pass field-identity, pagination, security, quota and recovery tests before the product claims a generally proven provider-neutral work framework. A different-family fake worker demonstrates shared runtime isolation only, not source compatibility.

### H. Approval interfaces

Unresolved decisions require dated owner dispositions with evidence: OAuth topology and service cost; account store/key/recovery; signed broker policy and real worker sandbox posture; exact record/value schemas; indexed SQLCipher migration; per-platform transport; privacy/retention; release performance budgets; Jira field-level concurrency; second-provider selection. These are not implementation defaults. If evidence contradicts an accepted ADR, submit an ACR rather than silently editing the gate.
