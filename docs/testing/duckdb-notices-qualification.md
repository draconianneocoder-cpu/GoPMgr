<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# DuckDB native notices qualification

This is an interim release safeguard for the existing DuckDB dependency.
It does not upgrade DuckDB, authorize redistribution, or qualify its replacement.
[The exclusion decision](../review/TPC-benchmark-exclusion-decision.md) and
[ADR-012 acceptance](../review/ADR-012-v0.2-ACCEPTED-OWNER-DECISIONS.md)
remain separate gates. The Debian build hook rejects a vendored tree with
excluded archives. It does not intercept source-only archive creation; publishing
such archives remains prohibited and needs a separately qualified workflow.
SBOM delivery is not implemented by this change.

## Acceptance and evidence

- The complete reviewed file inventories of both cgo modules and all five
  platform modules must match `tools/duckdb-notices/inputs.go` before parsing
  their inputs. Added files, changed headers, renamed files and symlinks fail.
- Every required linked archive must exist. Link flags outside the reviewed
  grammar, including conditional directives and forced-loading forms, fail.
  Explicit system-library flags are allowed only without local archive shadows.
- Unknown objects, missing symbol tables, empty excluded symbol evidence and
  malformed archive bounds or padding fail. Go AST selectors detect cgo names
  even with whitespace or comments. Full-inventory pins cover preambles,
  headers and other native inputs the selector analysis does not interpret.
- Offline notice checks compare the entire file's SHA-256, not only its header.
  Full regeneration requires pinned source commits and clean tracked working
  files and index. Review changed texts before updating the approved digest.
- Binary/tree marker checks fail on malformed lists, unreadable inputs,
  traversal errors and tool failures, including failures after partial output.
  Real excluded archives are positive controls; clean synthetic files are
  negative controls. No failure may emit a clean-result claim for that input.
- Packaging must deliver the reviewed native notices beside the Go notices and
  check the binary it actually packages. For DuckDB-linked binaries it also
  invokes the authoritative native input/digest check, so matching coverage
  headers alone cannot qualify a notice file. That path requires the reviewed
  modules in the builder cache (downloaded if missing). Host and artifact
  evidence must be reported separately; a test using a fake build is not a native package test.

## Reproduce reviewed input digests

Download the versions selected by this checkout without changing `go.mod`, then
run `go mod verify`. Verification checks module-cache integrity, not licensing
or upstream binary provenance. Independently reproduce the digest with Python
3 from the repository root:

```sh
go mod download
go mod verify
python3 - <<'PY'
import hashlib, json, pathlib, stat, struct, subprocess
base = "github.com/duckdb/duckdb-go-bindings"
modules = [base, "github.com/duckdb/duckdb-go/v2"] + [
    base + "/lib/" + platform for platform in
    ("darwin-amd64", "darwin-arm64", "linux-amd64", "linux-arm64", "windows-amd64")
]
for module in modules:
    meta = json.loads(subprocess.check_output(["go", "mod", "download", "-json", module]))
    root = pathlib.Path(meta["Dir"])
    paths = []
    for path in root.rglob("*"):
        mode = path.lstat().st_mode
        if stat.S_ISDIR(mode):
            continue
        if not stat.S_ISREG(mode):
            raise SystemExit(f"nonregular input: {path}")
        paths.append(path.relative_to(root).as_posix())
    total = hashlib.sha256()
    for name in sorted(paths):
        data = name.encode("utf-8")
        body = hashlib.sha256()
        with (root / name).open("rb") as stream:
            for block in iter(lambda: stream.read(1024 * 1024), b""):
                body.update(block)
        total.update(struct.pack(">Q", len(data)))
        total.update(data)
        total.update(body.digest())
    print(module, total.hexdigest())
PY
```

Each record is the eight-byte big-endian UTF-8 filename length, slash-relative
filename bytes, and raw 32-byte file SHA-256. Sort complete filenames, not
walk traversal order. Timestamps and permission bits are not hashed.
Never replace approved pins merely because this command reports a mismatch:
review the selected versions, link flags, archives and native sources first.
Concurrent hostile replacement of the module cache is outside this gate.

## Commands

```sh
go test -count=1 ./tools/duckdb-notices
go test -race ./tools/duckdb-notices
bash scripts/verify-excluded-native-code_test.sh
make duckdb-notices-check
# Network required; compares with verified pinned source checkouts:
go run ./tools/duckdb-notices -fetch -check
make module-notices-tests package-tarball-tests
make verify
make coverage-ledger-drift
make license-check release-scope
make build
bash scripts/verify-module-notices.sh build/bin/gopmgr.app/Contents/MacOS/gopmgr
```

Use the actual platform's build output, as configured in `wails.json`; the last
command illustrates macOS. Source regeneration can also use clean local
checkouts through `-duckdb DIR -robin-hood DIR`. A version bump requires a new
review, full regeneration and inspection of all changed license bodies; it is
not automatic maintenance of an engine scheduled for retirement.

## Limits and lessons

Fixed-input symbol checks and listed-marker scans are complementary bounded
evidence. They do not prove arbitrary code absent, interpret every linker
behavior, provide a legal conclusion, or replace an SBOM or exact-candidate
native checks. Unsupported inputs stop the gate rather than being silently
ignored. In particular, a missing archive or symbol table must not look like
zero references, and a failed `grep` must not look like no marker matches.
A clean working copy alone does not establish a clean Git index.

The original PR's historical binary measurements remain attributed in its
decision record. Do not promote those measurements into current release claims.
