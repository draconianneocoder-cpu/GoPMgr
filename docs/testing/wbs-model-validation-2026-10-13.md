<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Unified WBS model assurance

Scope: the approved pure model/identity/revision/validation slice, based on
`5e97f75`. The two design documents already belonged to this initiative; no
unrelated source edits or staged changes existed at entry. No runtime path,
legacy parser, schema migration, dependency or chart registry changed.

## Requirements and evidence

| Requirement | Evidence |
| --- | --- |
| Preserve scope identity, order and definitions | Literal malformed-input fixtures and semantic round trips retain IDs, duplicate display numbers, dictionary text, classification and finite negative legacy effort |
| Fail closed on ambiguous writable data | Missing required fields/IDs, unknown or case-aliased members, escaped duplicate keys, nulls, wrong shapes, invalid UTF-8, unpaired surrogates and trailing values return errors without partial models |
| Bound resource use | Exact/over-limit payload, text, identifier, node, hierarchy and combined association cases; decoded Unicode/escaped strings; wide/deep malformed input; encoded escaping expansion |
| Reject invalid topology and assignments | Nil/cyclic/aliased nodes, repeated IDs, missing/nonleaf scope targets and repeated sources; distinct schedule/activity tuples remain distinct |
| Preserve exact revision/context semantics | Zero, above-JavaScript-range and maximum `uint64` tokens; noncanonical strings and overflow fail; each copy/project/chart/lifetime axis and expected revision checked separately |

Independent plan review required explicit required/optional fields, hierarchy
versus container-depth accounting and common validation of Go-created models.
These were resolved before implementation. Classification remains uninterpreted;
empty text is valid; unknown fields are rejected rather than silently stripped.

## Reproduction

Local macOS arm64, Go 1.27.2, no toolchain/dependency change:

- Baseline `go test ./internal/charts/dag ./internal/db` and
  `go test . ./internal/...`: passed.
- Final `go test . ./internal/...`: passed; unchanged packages reused the
  baseline cache, while the new package executed its tests.
- `go test -race -shuffle=20261013 -count=10 ./internal/wbs
  ./internal/charts/dag ./internal/db`: passed, zero observed failures.
- New-package repetition with seed `20261014` (10 runs) and final expanded
  boundary fixtures with seed `20261016` (3 runs), also race-enabled: passed.
- `go test ./internal/wbs -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s
  -parallel=2`: passed, 269,293 executions, zero observed failures.
- `go vet . ./internal/...`, `golangci-lint run ./internal/wbs/...`, `make lint-go` and
  `go build ./internal/wbs`: passed.
- `go test -count=1 -cover ./internal/wbs`: passed. Coverage is diagnostic,
  not proof of safety; tests comprise 15 test functions and one fuzz target.
- `make code-map-current coverage-ledger-current`, `make release-scope` and
  `reuse lint`: passed.
- `make config-check` and `make coverage-ledger-drift`: passed; the latter
  checked default builds and DuckDB-tagged analytics, with 46 matching package
  headings and all 49 ledger headings accounted for.

Five compile-valid fault seeds were detected by targeted tests: per-array instead of
combined association limits; disabled leaf eligibility; activity identity missing
its schedule chart; context matching only copy identity; ignored unknown members.
Targeted tests failed for each and passed after restoration. An initial leaf seed
failed compilation and was discarded, not counted as behavioral sensitivity.
Self-review removed a fixture helper that could obscure encoding failures.

The independent final reviewer found no material issues. Its fresh package
tests, race check, vet and additional dictionary/link, exact encoded-size and
branching-tree probes passed. Exact encoded 8 MiB and branching 10,000-node
counterexamples were retained in the regression tests. Extended repetition and
fuzz totals above are this task's evidence, not claimed independent reruns.

## Boundaries and lessons

Strict standard-library decoding avoids a second JSON grammar. Explicit null
rejection and presence/zero-value rules are still necessary. Encoded size can
exceed decoded-text size through escaping. Hierarchy levels are not JSON depth.
Association keys must be tuples, not concatenated strings or display codes.

At this model milestone the package had no production caller. The later
[dormant storage foundation](../design/wbs-storage-boundary.md) now uses it
internally, without production application activation. Pure context matching
alone is not authorization or transactional compare-and-swap. The storage
foundation now covers receipts and atomic audit writes; external source
existence/ownership, unresolved-link warnings, migration, old-binary protection,
file exchange and UI behavior remain unimplemented. Native Wails,
Linux/Windows runtime and comparative usability were not tested for this slice;
no integrated parity or release-readiness claim follows from these results.
