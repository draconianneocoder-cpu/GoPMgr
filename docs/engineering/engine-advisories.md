<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Vendored engine advisories (SEC-06)

GoPMgr compiles SQLite and SQLCipher from `third_party/go-sqlcipher` through
cgo. `govulncheck` cannot see that C code. `tools/engine-advisories` checks it
against a reviewed record, `tools/engine-advisories/reviewed.json`. This is
step A4 of the [response plan](../review/evaluation-2026-11-10-response-plan.md).

## Two checks

| Command | Network | Fails when | Runs in |
| --- | --- | --- | --- |
| `make engine-advisories-check` | None | An engine file (`sqlite3.c`, `sqlite3.h`, `flags.go`, `gopmgr_rng.c`) differs from the reviewed hash, even at the same version; a version macro differs from the record; the SBOM's native entries name another version or CPE; the record breaks its own rules | `make test`, so `make verify` and CI (`TestVendoredEngineIsTheReviewedOne`) |
| `make engine-advisories` | `services.nvd.nist.gov` | The offline check fails; NVD lists a CVE that applies to a reviewed version without a `not_affected` disposition; or NVD gives no answer ("no verdict") | Only when someone runs it |

The online scan is not in CI or `make check-release`: a check that depends on
NVD answering would need an owner decision (ADR-013: hosted CI and blocking
status need authorization). **Until that decision, run it:**

- before tagging any release candidate, and record the output with the release
  evidence;
- whenever the engine changes (the offline check forces a new review then);
- at least monthly while the engine is unchanged.

## What the scan decides

For each component it first asks NVD how many CVEs it lists for the product
at any version; none means the CPE name is probably wrong, and the scan stops
with "no verdict" rather than pass for want of a match. It then asks for CVEs
matching the version's CPE (`virtualMatchString`) and reads every page. It
waits 7 s between requests and retries a 5xx response or a failed connection
up to three times. Any other failure ends with "no verdict": nothing was
decided. That includes NVD's 403 for a rate-limited client, a reply that is
not a CVE API 2.0 page (wrong `format` or `version`, missing counts or
timestamp, counts that do not add up), and cancellation.

It then re-decides each CVE from NVD's configurations, and fails closed:

- applies when a `vulnerable` match for the component covers the version
  (`versionStart`/`versionEnd`, Including or Excluding; versions compare as
  dotted numbers, missing parts as zero);
- does not apply when every match for the component is `vulnerable: false`
  (the engine as the platform of another product's bug);
- applies when there are no configurations (NVD has not analysed it), none
  name the component, a configuration is negated, a match omits
  `vulnerable`, or a value does not parse.

Warnings, which do not fail the scan: a disposition NVD no longer returns or
that no longer applies, and a dispositioned CVE NVD changed on or after the
review day (or whose change date does not parse).

## When an advisory appears

1. Read it, NVD's configurations and [SQLite's CVE page](https://www.sqlite.org/cves.html)
   (or SQLCipher's release notes).
2. If the vendored engine is affected, upgrade it
   ([GOPMGR.md](../../third_party/go-sqlcipher/GOPMGR.md), Maintenance). There
   is no "accepted" disposition.
3. If it is not, add a `not_affected` disposition with the reason and the
   evidence you checked, then set `reviewed_on`.
4. Also read SQLite's CVE page for CVEs published since the last review: NVD
   cannot match a CVE it has not analysed yet (see Limits).

## Evidence, 2026-11-13

Run from an Ubuntu container, Go 1.27.4, `main` at `d764e50` plus this change.

| Query | NVD returned | Result |
| --- | --- | --- |
| SQLite 3.54.1 | CVE-2022-31631, CVE-2026-119586 | CVE-2022-31631 (a PHP bug; SQLite only as platform) does not apply. CVE-2026-119586 (the CLI shell's `.recover`, through 3.54.1) applies by its range and is dispositioned `not_affected`: SQLite calls it "Not a bug in the core SQLite library", and the vendored `sqlite3.c` has no shell code |
| SQLCipher 4.20.0 | none | Pass |
| SQLCipher, any version (control) | CVE-2020-27207, CVE-2021-3119 | Shows the version filter narrows |
| SQLite 3.33.0 and SQLCipher 4.4.2 (negative control: the engine shipped until 2026-11-10, in a scratch tree) | 18 and 1 | Fails: 18 advisories without a disposition, including CVE-2021-20227 and CVE-2025-6965 |

`make engine-advisories` passed on the real tree. The SQLCipher control is
now built into the scan; the version-filter comparison above can be repeated
by hand to re-confirm the API behaviour after any NVD change.

## Limits

- NVD's API documentation could not be read from the build environment. The
  matching described above was observed, not confirmed from documentation.
- The scan sees only what NVD matches to the CPE. A CVE NVD has not yet
  analysed has no configurations and is not returned; step 4 above is the
  manual cover for that backlog. This is residual risk.
- Dispositions are keyed by CVE ID. If NVD widens a dispositioned CVE, the
  "changed after the review" warning is the only signal.
- No NVD API key is supported, so large result sets are slow. Advisories in
  other sources (GitHub, distributions) are not consulted.
