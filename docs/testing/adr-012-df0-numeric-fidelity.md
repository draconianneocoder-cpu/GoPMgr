<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# DF0-E2: typed tabular numeric fidelity on macOS

**Partial feasibility evidence, not production UI integration.** This slice
uses the existing corpus `Cell {k,v}` representation, whose values are exact
text. It does not introduce or approve a production tabular DTO. Production
portfolio money already passes through `PortfolioSummaryWire` decimal strings;
its internal numeric summary is not the Wails return type and is unchanged.

## Native path and acceptance

Five existing literal-golden Parquet fixtures run through both `parquet_range`
and `parquet_stream`: scalar values, nullable scalars, Decimal128/256,
timestamps/dates, and nested collections. Each file is written by the existing
Go corpus generator. Its expected rows come from literal fixture values, not
the worker's output. The native Rust/DataFusion worker reads only the granted
source and emits Arrow IPC inside protobuf over the authenticated Unix socket.
The Go host decodes and checks the batches, then serializes the typed-cell
result with `encoding/json`. A separate Node process runs `JSON.parse`, checks
strict equality against the golden table, requires string payloads and
round-trips integer text through `BigInt`.

The [raw observations](../../spikes/adr012-datafusion/evidence/df0-fidelity-macos/native.json)
record 10 successful jobs, 60 rows and 30 batches. Each two-row batch is below
the negotiated 64 KiB frame cap. Every job requires verified source/completion
digests, complete ordered rows, a normal worker exit and endpoint cleanup
before invoking the JavaScript checker. The source SHA-256 and observed Arrow
types accompany each table. The
[environment and hashes](../../spikes/adr012-datafusion/evidence/df0-fidelity-macos/environment.json)
identify the source, fixture generator, native worker and tools.

| Risk | Executed check |
| --- | --- |
| Integer rounding/narrowing | Exact signed int64 extremes, `2^53+1`, unsigned uint64 maximum; primitive widths and signedness checked |
| Decimal value/scale loss | Decimal128(38,4), Decimal128(9,2), Decimal256(60,10), including huge, zero and negative values; precision/scale types and exact trailing zeros checked |
| Timestamp loss | UTC microseconds, naive nanoseconds including int64 endpoints, New York milliseconds and year 1; units/timezone types and exact date/time text checked |
| Null/text or collection loss | Null scalar rows, empty text/binary/lists/maps, Unicode, int64 maximum nested in a list; literal ordered values checked |
| Provisional result mistaken for success | Host completion, row counts and digests checked before the JavaScript step; no failed worker result is qualified |
| A green checker that no longer compares values | Eight corrupted JSON payloads rejected, plus comparator-removal fault detected in a disposable checkout |

Nested values remain **opaque JSON text** after the outer result is decoded.
This preserves their digits but does not qualify reparsing them with JavaScript
numbers. Safe semantic access to nested numeric values, production tabular
encoding, native Wails/WebView use, other schemas/decimal precisions and the
production portfolio pipeline remain unqualified by this slice. No installed
app, minimum-macOS or browser compatibility claim follows from Node 24.18.0.
Full DF-065 remains partial; DF0-E performance is NOT-RUN.

## Defects exposed and corrections

The first native decimal run failed because Arrow-Go's display `ValueStr`
removed a trailing zero from a Decimal256 value. The test-side adapter now
uses the value and declared scale explicitly with `ToString(scale)` for both
decimal widths. The literal goldens were not changed. This corrects the new
adapter's mistaken display-format assumption, not an alleged DataFusion value
corruption or a patch to the dependency. Duplicate map keys are rejected rather
than silently collapsing entries into the corpus's object representation.

Eight explicit bad payloads fail with `WIRE_MISMATCH`: integer-as-number,
rounded integer text, decimal scale loss, millisecond timestamp truncation,
null-as-text, rounded nested JSON text, missing row and column reorder.
A passing baseline and four compiling faults in a disposable worktree prove
sensitivity to a wrong decimal scale, timestamp truncation, omitted JavaScript
comparison and omitted duplicate-map guard. Each failed for its intended
assertion; no build error or unrelated timeout counted as detection. The
worktree and fault changes were removed.

## Reproduce

From the spike directory on native macOS, using the reviewed native worker:

```sh
DF0_WORKER=/absolute/path/to/df0-worker \
DF0_FIDELITY_EVIDENCE=/absolute/path/to/native.json \
  go test -race -count=1 -v ./internal/ipchost \
  -run '^(TestNativeNumericWireFidelity|TestNumericWireJavaScriptRejectsLoss|TestNumericWireRejectsDuplicateMapKeys)$'
go vet ./...
```

`DF0_WORKER` is required for native jobs; absence is explicitly a skip. The
JavaScript negative controls require the existing Node executable and fail if
it is missing. The optional evidence file is written only if the native test
passes. The adapter is limited to the selected fixture types and rejects
unqualified types; it is not a hardened general Arrow parser.

The full spike Go race suite passed with the 10 new native jobs and 60 earlier
Rust-worker scenarios. Vet, Linux host-package cross-compilation and the
focused JavaScript/duplicate-map controls passed. The root
`TestPortfolioSummaryWire` race regression also passed separately; it does not
join the candidate worker to that production boundary. Rust source and dependencies
are unchanged; the worker is hash-verified against its earlier reviewed build,
not rebuilt here. No native Linux/Windows, root PDF, packaging, installed-app,
frontend component or hosted checks were performed for this isolated slice.
Structured plan, mid-work, adversarial and final self-reviews are the disclosed
fallback while independent reviewer capacity is unavailable. No independent
review or security clearance is claimed.

[The roadmap](../ROADMAP.md) retains resource containment, independent OS
observation, remaining numeric/format cases, fair three-candidate performance,
packaging/licensing and a separate production decision before DuckDB removal.
No production Go API, SQLCipher schema, build tag, dependency, installer or
accepted containment contract changes in this slice.
