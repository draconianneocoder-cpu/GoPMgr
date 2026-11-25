<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Agent Operating Guide

This repository is GoPMgr, a local-first desktop project controls
application built with a Go backend, Wails v3, and a Svelte 5 frontend.
Treat this file as the first project-local instruction source for
automated engineering work.

## Session Spin-Up

1. Confirm the active checkout with `pwd` and `git status --short`.
   Older handoffs may mention stale clone paths; verify before running
   release or build commands.
2. Read `docs/STATUS.md` for what is implemented, open, blocked or
   unverified today. Where an older document disagrees with it about
   status, it wins; update it in the same change that alters a status.
   When present, also read local `session-notes.md` and the relevant files
   in `.agent_memory/` before choosing backlog work or changing behavior.
3. Inspect existing source before editing. Do not read `DEVELOPER_HANDBOOK.md` in
   full unless required; it is a large developer handbook. Prefer targeted
   `rg` searches and small snippets.
4. Assume the worktree can contain unrelated user or agent changes. Do
   not revert, restage, or flatten changes you did not make.

## Work Rules

For build-tool provisioning or new shell/workflow assurance gates, read
[ADR-013](docs/design/ADR-013-engineering-build-assurance.md) and its
[baseline](docs/engineering/adr-013-baseline.md). Make remains canonical;
Wails' internal Taskfiles are subordinate build details. mise is approved for
staged auxiliary provisioning only. P0 checkout/performance characterization
is complete on the recorded macOS/Ubuntu hosts. P1 opt-in provisioning is
implemented; see [setup and proof limits](docs/engineering/adr-013-provisioning.md).
P2's [local ShellCheck inventory and adjudication](docs/engineering/adr-013-shellcheck.md)
is implemented. P3's [local shfmt style and read-only check](docs/engineering/adr-013-shfmt.md)
is implemented with separately reviewed normalization. Maintainer acceptance
precedes mandatory lint adoption. P4's [local workflow correctness check](docs/engineering/adr-013-actionlint.md)
is implemented. P5's [local workflow security policy](docs/engineering/adr-013-zizmor.md)
is implemented under the approved regular-persona contract; its documented
credential-detection gap is covered by the separately owner-approved
[P6 configuration guard](docs/engineering/adr-013-checkout-guard-plan.md).
P6-B makes [licensing validation read-only](docs/engineering/adr-013-license-check.md)
with explicit Finder cleanup separate. P6-D requires
[successful licensing at every release entry point](docs/engineering/adr-013-release-license-plan.md),
with no skip mode. P6-C [pins the CI vulnerability scanner](docs/engineering/adr-013-govulncheck.md).
P6-E [native Make environments](docs/engineering/adr-013-native-make-environment.md)
align test/race and three coverage targets with macOS 26. The
[standalone release/validator slice](docs/engineering/adr-013-release-native-environment.md)
also guards release race execution, encrypted-database tests and PDF generators.
The [memory-scanner repair](docs/engineering/memory-scan-integrity-plan.md)
also guards analysis commands and the explicit-root helper, and its grep stages
fail on operational errors. Standalone coverage callers remain separate. Other P6
repairs and P7–P9 remain separate.
Do not infer new runtime, hosted-CI or release authority from this direction.

- Plan first for any multi-file change. Keep the plan scoped to a
  reviewable slice.
- For ADR-010 or ADR-011 work (plugins, MPXJ, worker IPC, large
  schedules, or AI/MCP), load
  `docs/agent/ADR-010-architecture-preservation-agent-prompt.md` first
  and follow it. The architecture was accepted on 2026-10-24; only the
  work slices authorized in
  `docs/review/ADR-010-v2.2-ACCEPTED-OWNER-DECISIONS.md` may proceed.
- For ADR-012 work (`internal/analytics`, DataFusion, or the Rust
  analytics worker), load
  `docs/agent/adr-012-coding-agent-instructions.md` first and follow it.
  On 2026-11-02 the maintainer retired DuckDB in favour of a Go-native engine
  for both analytics operations (see
  `docs/review/ADR-012-DUCKDB-RETIREMENT-OWNER-DECISION.md`). On 2026-11-09
  the DataFusion spike was concluded without adoption
  (`docs/testing/adr-012-df0-g-report.md`) and archived at the tag
  `archive/adr012-datafusion-spike`; its evidence stays in `docs/testing/`.
  No DataFusion or Rust-worker work is authorized without a new owner decision.
- Use repository patterns instead of inventing new architecture.
- Read existing files before editing. Do not guess APIs, versions,
  flags, commit SHAs, or package names; verify by reading code or
  documentation first.
- For behavior changes, inspect existing coverage and apply the testing rules
  below. Documentation-only changes still need `git diff --check`.
- Every file under `docs/` has an entry in `docs/ledger.json` (rules:
  `docs/design/docs-reorganization-plan.md` §6). Add, move, merge or remove a
  document together with its entry, then run `make docs-ledger`.
  `make verify` (and so CI) runs `make docs-ledger-check`; before a PR, also
  run `make docs-ledger-history`, which needs full history. CI's Verify job
  runs it too.
- Preserve REUSE/SPDX compliance. New tracked files need SPDX metadata
  directly or a `REUSE.toml` annotation when inline metadata is not
  possible.
- Keep `session-notes.md`, `.agent_memory/`, and other private handoff
  material out of public release scope unless the user explicitly asks
  otherwise.
- Stage intentionally by path or hunk. Avoid broad `git add .` in a
  dirty worktree.
- Before opening or updating a pull request, inspect the affected coverage
  evidence and
  `docs/TEST_COVERAGE_LEDGER.md`, and run `make coverage-ledger-current`
  and `make coverage-ledger-drift` on the exact source to be submitted. Repeat
  affected checks after subsequent changes. `make verify` alone does not run
  the measured-coverage drift check. Correct stale entries against measured
  results; never weaken assertions or add tests merely to match a percentage.
  Record the commands, source revision and platform; if the required coverage
  baseline cannot be verified, report the blocker before submitting the PR.
- Pull request titles and bodies do not address the owner by personal name.
- Be concise in public documentation. Summarize long generated output
  and point to the command or artifact that reproduces it.

| Rule | What enforces it |
| --- | --- |
| Pull request titles and bodies do not address the owner by personal name. | `scripts/check-pr-text.sh` reads that name from this file's SPDX copyright line. `make verify` runs the script's self-test. The Verify job runs the script on the pull request title and body. |

## Testing rules (anti-slop)

Do not add tests by default or to increase a test count or coverage percentage.
Add or extend a test only when all of these are true:

1. It protects an observable behavior, invariant or contract against a specific
   failure that existing tests do not already detect. Inspect those tests first;
   prefer extending the primary owner test over creating another one.
2. It would fail for a credible regression, not merely a harmless refactor,
   rename or formatting change. Name the regression; when practical, demonstrate
   that the test fails with that defect present and passes after the fix.
3. It exercises the strongest practical real boundary for that contract. Give
   each contract one primary owner test; do not replay the same assertion across
   layers. Additional layers must expose a distinct failure, such as incorrect
   wiring, serialization, persistence or platform behavior.
4. Its oracle is independent of the implementation and its mocks. Hardcode
   known expected values or use independently reviewed fixtures. Property-based,
   fuzz and differential tests may use independently specified invariants or
   reference implementations; never reproduce the production algorithm to
   calculate its own expected answer.

Reject and do not write:

- Self-comparisons, tautologies or probes that only assert "does not throw".
- Implementation-shape or source-grep assertions used as substitutes for
  behavior. Static inspection is appropriate only when the inspected structure
  is itself an explicit contract, such as a build or security policy; identify
  that contract and the bypass being prevented.
- Duplicate contracts already covered elsewhere without a distinct failure mode.
- Test-only production seams, flags or injection hooks with no real caller.
- Mocks that supply the behavior being asserted. Mock external boundaries only;
  keep the decision or transformation under test real.
- Change-detector tests or snapshots that break on expected data updates without
  protecting a semantic contract. Never regenerate expected output blindly.

When changing tests, prefer consolidating or deleting demonstrated low-value
tests over adding more. Preserve every distinct useful failure detector; do not
delete or weaken tests merely to make a change pass. A behavior-preserving
refactor should leave contract assertions unchanged. Adapt necessary imports or
setup without rewriting expectations to match the implementation.

If unsure whether an additional test adds value, skip it and state why in the
summary. An uncovered critical requirement is a verification gap to resolve or
report as blocked, not permission to claim completion. These rules govern test
selection; they do not waive existing required verification or release gates.

## Go Engineering Rules

- Keep package boundaries domain-oriented. Do not add broad `common` or
  `util` packages unless a narrow existing pattern already supports it.
- Prefer synchronous APIs. Start background goroutines only when the
  owner, cancellation path, and cleanup lifecycle are explicit.
- Use guard clauses for error and edge handling. Keep the normal path
  shallow and readable.
- Return `error` as the last value, wrap it with operation context, and
  use `errors.Is` / `errors.As` for inspection.
- Run `gofmt` or `goimports` after Go edits. Keep imports grouped as
  standard library, third-party, then first-party packages.
- Use table-driven tests for multi-case logic. Use `t.Cleanup` for test
  cleanup that must run after `t.Fatal`.
- For complex comparisons, use `reflect.DeepEqual` (the project's current
  practice — no comparison-diff dependency such as `go-cmp` is vendored;
  don't add one without a deliberate dependency-addition decision, per
  DEPENDENCIES.md).
- Use `crypto/rand` for security-sensitive randomness. In recoverable
  paths such as IDs, salts, or recovery codes, use
  `io.ReadFull(rand.Reader, buf)` so entropy failures return errors
  instead of terminating the process.
- Use `snake_case` JSON tags on Wails-bound structs; the TypeScript
  surface and existing frontend code expect those wire names.

## Project Invariants

- `go.mod` requires Go 1.27.4 or later and pins Wails v3.1.0. CGO is required.
- The root `main.go` embeds repo-root `frontend/dist` through `go:embed` and
  calls `desktop.Main`. The Wails service `App` lives in `internal/desktop`;
  its 212 exported methods are frontend API, listed in `code-map/app-api.json`.
  `frontend/src/lib/desktop-api.ts` exports the generated App namespace from
  `frontend/bindings/gopmgr/internal/desktop/index.ts`, turning a Go method's
  `RuntimeError` into a `BackendError` that reads as its message. Generate real bindings
  before frontend checks; do not restore ambient `window.go` declarations.
  `domain-types.d.ts` contains frontend editor views, and `desktop-dtos.ts`
  explicitly normalizes nullable collections for those views. Raw generated
  calls retain wire null/empty distinctions. `desktop-runtime.ts` adapts
  `Events.On` and preserves listener-before-guard readiness and unsubscribe.
- `make build` calls `scripts/wails-build.sh` and the reviewed v3 Taskfiles.
  Use the task-local `.tmp/wails-v3/bin/wails3` CLI pinned to v3.1.0 and npm
  `@wailsio/runtime` 3.1.0. Product metadata is `build/config.yml` `info.version`.
  CGO Linux builds use GTK 4 and WebKitGTK 6.0 without legacy backend tags.
- The migration was approved on 2026-11-03; package/release qualification is
  separate. macOS 26 Tahoe and Ubuntu 26.04 execution remain mandatory release gates
  (Ubuntu 26.04 is the Linux minimum since 2026-11-09; the macOS 26 host is
  currently unavailable). Windows NSIS qualification remains pending.
  Run CI-equivalent checks on local macOS/local Ubuntu; do not launch hosted CI.
- `system.db` stores local account metadata. Per-project `.gopmgr`
  databases hold project data and are SQLCipher-capable through
  `internal/sqlitedriver`.
- PDF/A and PAdES are release-critical features. Preserve the order:
  render PDF content, apply PDF/A metadata and output intent, then apply
  PAdES as the final PDF mutation.
- Frontend changes need more than type checking. The runtime smoke gate
  exists because `svelte-check` and Vite build can miss load-time Svelte
  rune misuse.

## Useful Commands

```sh
go test . ./internal/...
go test -race . ./internal/...
npm --prefix frontend run check
npm --prefix frontend run build
make frontend-smoke
make check-pades
make check-pades-external
make check-pdfa
make license-check
make release-scope
make check-release
git diff --check && git diff --cached --check
```

Use focused commands during development, then broaden verification before
claiming completion. Do not claim a gate passes unless it was run in the
current session.

## Completion

Before ending a meaningful task:

1. Summarize changed files and verification evidence.
2. Record durable project state in `.agent_memory/` when the work changes
   architecture, release gates, security posture, or future handoff steps.
3. Leave unresolved issues explicit, especially when a command could not
   be run because of missing tools or existing unrelated failures.

## Companion website

`website/` is an isolated Hugo module. Use only opt-in `website-*` Make
targets; never add it to application or release prerequisites. Set `GOWORK=off`.
Keep generated output in ignored fresh temporary directories. Website sources
and assets must stay outside `frontend/dist` and application packaging inputs.
No hosted builds, pushes, Pages settings or publication are authorized by a
local website build. The owner-approved hosted website job runs checker tests
and `make website-ci`, including the pinned Linux Hugo production build and
output checks. It does not publish or authorize deployment. Follow
`website/README.md` for evidence and blockers.

## Reporting and extension plan work

For reporting/lifecycle modernization or new extension capabilities, start with
`docs/design/reporting-extension/implementation-plan.md`, ADR-014, ADR-015 and
`docs/testing/reporting-extension-assurance.md`. Rebase the requirement registers
against actual code; retain every original reporting/EXT identifier and explicit
applicability. These detailed plans are proposed, not implementation authority.
Existing authorized ADR-010/G1 slices retain their conditions. Host-native reports
need not await Wasm; plugin writes cannot bypass ADR-011/format/audit/receipt gates.
Distinguish RILD evidence levels from EXT trust profiles, preserve imported facts,
exact calculations, historical audit bytes and original issued artifacts. Record
missing decisions and unrun qualification honestly; do not reduce report scope or
weaken tests to make a slice appear complete.

## Resource resilience

For new or materially expanded memory-intensive work, and the inventory of existing
heavy paths, read `docs/design/resource-resilience.md` and
`docs/testing/resource-resilience-assurance.md`. Apply MEM-01–12 and the per-slice
handoff before completion; missing mandatory evidence blocks the affected claim.
MR-1 estimation/admission/cancellation protects existing host paths without waiting
for G1. MR-2 isolation reuses its approved supervisor; MR-3 new durable schemas keep
format/audit/retention gates. No numeric budget, runtime mechanism or implementation
is authorized by a planning link. Preserve exact results, source facts, critical
save capacity and honest queued/committed/checkpoint states; never claim fatal-OOM
containment or every-keystroke durability from input caps or a green unit test.
