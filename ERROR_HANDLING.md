<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Error Handling

How PMForge represents, wraps, surfaces, and tests errors across the Go
backend and the Svelte frontend. See [STYLE.md](STYLE.md) for general code
style and [TESTING.md](TESTING.md) for verification gates.

## Go: sentinel errors

Every package that has recoverable, callable-specific failure modes
declares typed sentinel errors with `errors.New`, scoped by package-name
prefix, e.g.:

```go
var ErrNoProject = errors.New("db: no project initialised in this file")
var ErrBadDEK = errors.New("crypto: DEK must be exactly 32 bytes")
var ErrCycle = errors.New("dag: graph contains a cycle")
```

Callers check them with `errors.Is`, never with string comparison on
`err.Error()`. Sentinels currently exist in `internal/agile`, `internal/auth`,
`internal/charts/{dag,flow}`, `internal/crypto`, `internal/db` (one per
entity: `ErrNoChart`, `ErrNoDocument`, `ErrNoBaseline`, `ErrNoScenario`, ...),
`internal/documents`, `internal/update`, `internal/users`, `internal/analytics`
(`ErrAnalyticsUnavailable`), `internal/charts` (`ErrEngineNotImplemented`),
and `main.go` (`ErrProjectRequiresEncryptionMigration`,
`ErrRecoveryCodesRequireReissue`). See
[code-map/symbols.json](code-map/symbols.json) for the full current list.

**Wrapped sentinels must still match `errors.Is`.** When a sentinel crosses
a package boundary, wrap it with `%w`, not `%v`:

```go
return fmt.Errorf("open project: %w", db.ErrNoProject)
```

A regression test should assert the wrapped case specifically (see
`TestIsEngineNotImpl` in `internal/charts/pdfrender` for the pattern) —
a string-based check would pass against both correct and broken code and
prove nothing.

## Go: error wrapping for context

Standard path: `fmt.Errorf("context: %w", err)`. Every returned error should
read as a sentence fragment describing the failed operation, lower-case,
no trailing punctuation, per Go convention.

## Go: structured reports for self-heal / diagnostics

Recoverable paths that the UI needs to introspect (not just display) use
`internal/debug.Wrap(err, "CONTEXT_TAG")`, which captures a timestamp,
call-site `file:line`, and a stack trace into an `ErrorReport`, then
`.ToError()` converts it back into a plain `error` for normal plumbing.
The frontend recovers the structured report via `errors.As` on the Go side
before it crosses the Wails bridge (the `ErrorReport` struct has `json`
tags for direct serialization). `context` is a short upper-case tag
(`SNAPSHOT_FAILED`, `CERT_BUNDLING_FAILED`, ...) the UI can match on to
show a specific recovery hint rather than a generic failure message.
Every `Wrap` call also logs one line to the persistent log file, so a
self-heal failure is diagnosable from `logs/` even without reproducing it
in the UI.

Use `debug.Wrap` for: database repair/snapshot paths, anything the user
might need to send a log excerpt about. Use plain `fmt.Errorf` wrapping for
everything else — most of the codebase.

## Go: fail-soft vs fail-hard

PMForge distinguishes operations where a failure should degrade gracefully
from operations where it must abort:

- **Fail-soft** (log and continue): PDF XMP-metadata tagging. If
  `InjectXMPStream` errors, `documents.Render()` returns the valid-but-
  untagged PDF rather than failing the whole export — a desktop user
  should never lose a document export because a metadata step hiccupped.
- **Fail-hard** (abort and surface): anything touching the encryption
  migration path (`MigratePlaintextToEncrypted`), path confinement
  (`projectPathFor`), or account/session state. These reject early and
  return a typed error rather than attempting a partial operation.

When adding a new operation, default to fail-hard; only make a path
fail-soft when a partial/degraded result is genuinely more useful to the
user than an error, and say so in a comment (see `documents.Render` for
the existing example).

## Go: user-facing error string convention (ST1005 exception)

`.golangci.yml` disables `staticcheck`'s `ST1005` (Go convention: error
strings should not be capitalized) for exactly the error strings meant to
reach the UI verbatim, e.g. `ErrRecoveryCodesRequireReissue`
(`"Reissue recovery codes before enabling database encryption. ..."`).
These are a **contract** — at least one is matched verbatim by frontend
code, so don't reword an existing user-facing error string without
grepping the frontend for a literal match first.

## Go: what NOT to do

- No `panic`/`recover` as control flow. `panic` is reserved for programmer
  errors (nil dereference, index out of range) that `go vet`/tests should
  catch before release, not for expected failure modes.
- No swallowing errors with a bare `_ = err` outside of `defer`-cleanup
  contexts (`defer file.Close()` is the one broadly accepted exception;
  `errcheck`'s `_test.go` exclusion covers test cleanup similarly).
- No comparing errors with `==` or matching on `err.Error()` substrings;
  always `errors.Is`/`errors.As` against a sentinel.

## Frontend (Svelte): async error handling

Every Wails call happens inside `try/catch` in an `onMount(async () => ...)`
or an event handler; there is no global unhandled-rejection handler this
relies on. Two patterns, chosen by whether the failure is expected:

- **User-visible failure** — catch, then call the shared `showToast(message,
  'error')` helper so the user sees what happened. Used for anything the
  user directly triggered (save, delete, create, export).
- **Expected/optional-feature absence** — catch and silently fall back
  (e.g. an older backend binary without a newer Wails method bound):
  ```ts
  try {
    agileEnabled = await window.go.main.App.AgileEnabled();
  } catch {
    // Older binary without the Agile bindings — feature stays hidden.
    agileEnabled = false;
  }
  ```
  Always leave a comment explaining *why* the catch is silent — a bare
  `catch {}` with no comment is a code-review flag, not a pattern to copy.

User-facing errors accumulated during a form/editor session are held in
`let error = $state('')` and rendered inline, per the Svelte conventions in
[AGENT.md](AGENT.md) §4.

## Testing error paths

- Sentinel errors get a table-driven test asserting the wrapped case
  (`fmt.Errorf("...: %w", sentinel)` still satisfies `errors.Is`), not just
  the bare sentinel — see §"Go: sentinel errors" above.
- `internal/exportsafe` and similar defensive helpers have a unit test per
  dangerous input class, not just the happy path.
- Confinement/validation errors (`projectPathFor`, `encryptedDSN`'s `?`/`#`
  rejection) get an explicit regression test asserting the rejection, not
  just a manual check — see `project_path_confinement_test.go` and
  `TestEncryptedDSNRejectsAmbiguousPath`.

## See also

- [SECURITY.md](SECURITY.md) — secrets must never appear in an error string
  or log line (DEKs, passwords, recovery codes, SQLCipher keys).
- [code-map/public-api-map.json](code-map/public-api-map.json) — every
  Wails `App` method's return signature, most of which end in `error`.
