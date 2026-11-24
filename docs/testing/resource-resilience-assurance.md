<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Resource resilience acceptance contract

**Status:** Test specifications, 2026-11-14. Every case below is **NOT RUN**.
The [development contract](../design/resource-resilience.md) defines MEM-01–12,
MR-0–5, authority, owners and per-slice completion obligations. Existing input-limit
unit tests are useful but do not qualify this application-wide contract.

| Test | Requirements | Observable oracle and prohibited effects |
| --- | --- | --- |
| MEM-T01 Effective capacity | MEM-01 | Vary physical RAM, lower container/job ceiling, unavailable telemetry and external pressure. Two app instances cannot both reserve the same assumed capacity under the selected policy. Observe named effective ceiling and headroom; no unknown-to-unlimited conversion or unapproved hardware floor. |
| MEM-T02 Whole envelope | MEM-02 | Include Go heap, webview, native SQLCipher/cache, workers/descendants, transport, staging and overlapping serialization/output copies. Compare estimated and measured peaks with the approved profile/error margin; exceed a component allowance without bypassing the global check. No claim that a heap cap or sampled RSS alone enforces the process tree. |
| MEM-T03 Admission race and early expansion | MEM-03 | Last fitting request reserves; simultaneous next requests wait before large read/decode/allocation. Seed estimate overflow, omitted decode/host copy and reservation race; intended assertions reject each defect. Oversized compressed/JSON/RPC data stops incrementally before expansion exceeds its allowance. No partial resource hold-and-wait deadlock. |
| MEM-T04 Queue and impossible work | MEM-04 | Fill descriptor count/bytes; exercise fairness, age, expiry, cancellation and producer backpressure. Capacity release starts an eligible waiting job. A never-fitting request is visibly blocked rather than retried forever; preserve the user's intent within the stated acceptance contract. No full dataset/transaction retained, dropped accepted job or silent smaller calculation. |
| MEM-T05 Pressure and critical work | MEM-05 | Reduce available memory after admission, vary estimate error and oscillate pressure. Stop new heavy work without restart thrash. While heavy queues are full, perform edit/save/cancel/quit and recover using the approved latency/storage profile. Exhaust checkpoint capacity: no false save acknowledgement, prior durable state intact, pending edits explained and unsafe work blocked. |
| MEM-T06 Worker failure containment | MEM-06 | In a bounded disposable environment, exceed a real worker's limit and exercise native allocations and descendants. Host/control plane remains alive and usable, project data/audit unchanged, keys and DB paths undisclosed. Test actual installed enforcement separately on each supported OS/profile; child exit alone does not prove containment. |
| MEM-T07 Cancellation and cleanup | MEM-07 | Cancel before start and during load, compute, result transfer and shutdown. Inject failed termination/reap and delayed resource reclamation; reservations remain unhealthy until ownership and cleanup are verified. No leaked process/handle, prematurely returned capacity, long transaction or abandoned sensitive staging. |
| MEM-T08 Replay and commit ambiguity | MEM-08 | Kill a worker before result, after result and around host commit/receipt boundaries. Only classified replay-safe attempts restart under bounded policy and feasible capacity; repeated identical OOM trips the circuit breaker. A committed result is reconciled rather than duplicated, and notification failure never resubmits it. Unknown exit is not falsely labelled confirmed OOM. |
| MEM-T09 Acknowledgement and draft recovery | MEM-09 | Distinguish volatile queued, durably queued, started, result available, durably committed and notified. Crash host/worker/webview and inject disk-full/checkpoint failure. Reopen and compare acknowledged project writes, audit, exact issued bytes and supported draft checkpoints. Nothing not durably accepted is claimed durable; draft recovery window is explicit. New schemas retain approved compatibility/migration semantics. |
| MEM-T10 Identity and snapshots | MEM-10 | While queued or retrying, switch account, revoke grants, change project copy/revision or destination. No cross-account disclosure/commit; stale results require review/new approval. Retry preserves approved input/seed/algorithm identity or becomes a new authorized operation. No transaction held across waiting or worker execution. |
| MEM-T11 Honest user recovery | MEM-11 | Observe accessible localized queued/blocked/running/retrying/cancelled/recovered/committed states with attempt/reason and actionable controls. Sanitize bounded diagnostics. Failed notification cannot turn a committed job into failure; post-restart explanation matches persisted evidence, with unknown cause stated honestly. |
| MEM-T12 Exactness and qualification | MEM-12 | Compare admitted/in-process/isolated/retried results to independent domain oracles, including deterministic Monte Carlo seeds/distributions/iterations/exact percentiles, row counts, money, format parity and PDF/A before final PAdES. Exercise approved hardware profiles and applicable 50k editable/100k stress objectives. No approximation, silent cap, missing record or mock-only installed-platform claim. |

For each authorized slice, assign a test command/fixture and independent oracle
before code. Record applicability per path/profile, exact source/build/tool and
data hashes, OS/installed package, settings and approved budgets, peak memory,
queue/interactive/cancel/recovery timing, expected/observed state and residual
findings. Use PASS, FAIL, BLOCKED or NOT RUN; N/A requires path/profile reasoning
and reviewer agreement. The reviewer for independent sign-off is neither the
implementation author nor the expected-oracle author (RILD Q8).

Use deterministic reservation/cancellation tests first, controlled bounded worker
faults next, then installed pressure/recovery and domain-scale fixtures. Do not
exhaust the user's machine to test OOM. Prove a fault by the intended failed oracle,
not a compiler error or unrelated timeout. Distinguish actual OS-confirmed OOM,
allocation failure, resource rejection, generic termination and unknown cause.
Polling and synthetic process exits cannot establish hard OS enforcement.

MR-1 admission, cancellation and accounting evidence must be available before its
corresponding delivery claim, independently of future G1 workers. MR-2 containment
and MR-3 durable queue/receipt evidence gate their stronger claims. Repeat applicable
MEM cases with each affected RILD/PS/ARC/EXT delivery; MR-5 is not permission to
defer safety tests to the end. A failed or unrun mandatory case blocks that claim.
These specifications add no executed evidence or implementation authorization.

## Slice records

### Time-interval resource detection and levelling (2026-11-23)

Changes `internal/kernel` overallocation detection and resource levelling
(owner decisions D-TIME and D-CAL), levelled-start persistence and MSPDI
levelling delays, under the [development contract](../design/resource-resilience.md#mandatory-slice-handoff).

| Item | Record |
| --- | --- |
| Path and phase | Existing in-process host paths (chart layout, levelling, reports, export, import); MR-0 inventory entry. Not a new heavy path. |
| MEM IDs | MEM-02 (envelope: measured allocation below), MEM-12 (exactness: independent oracles in the [overallocation contract](resource-overallocation-contract.md)). Others N/A for this slice only because nothing queues, isolates, retries or persists anew; independent reviewer agreement NOT obtained. |
| Owners | Change approved by the owner's decisions; independent reviewer not assigned. |
| Input and output bounds | Schedules within `kernel.MaxScheduleDays` (18,250 working days) and the 4× allocation limit. Detection keeps the per-day profiles and adds per-day buffers reused across days. Levelling keeps a per-resource whole-day total and capacity slice to the booked extent, plus part-day pieces (at most two per assignment of each part-day task). The delay helpers grow one date table to the latest offset they read. |
| Profile and ceiling | None approved (DEC-HARDWARE, OWN-10): BLOCKED. |
| Peak and estimate | Allocation per call measured (contract, Benchmarks): detection −7% at T0–T3; whole-day levelling T1 17.0 MB against 21.4 MB; part-day levelling T1 19.6 MB. Peak RSS NOT RUN; no estimate made, so no estimation error. |
| Admission, queue, cancellation | None, as before: synchronous calls on the caller's goroutine. Not changed. |
| Cleanup and retry | Pure computation; nothing to clean up; a repeated call gives the same result. |
| Identity and durability | Results persist only through the existing chart save and import paths; no new acknowledgement or checkpoint. |
| MEM-T results | MEM-T02 and MEM-T12 NOT RUN as qualification cases; the unit, property and differential tests in the contract ran on linux/amd64, Go 1.27.4, at the commits named in the change. |
| Prohibited effects | No approximation, cap or silent truncation: records, participants and levelled starts are exact against hand and independent oracles. |
