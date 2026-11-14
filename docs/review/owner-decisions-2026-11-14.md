<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Owner decisions, 2026-11-14

**Status:** decided by the maintainer, James L. Burns, on 2026-11-14.
**How:** in a working session, the maintainer chose one option for each
decision from a list of options with their trade-offs. The chosen option and
the text shown with it are recorded below; the options not chosen are not.
**Repository state when decided:** `main` at `0952c8f` (PR 214).
**Context:** the decisions listed as waiting in [STATUS.md](../STATUS.md), the
SEC-04 [triage](sec-04-triage.md) and the
[engine advisory procedure](../engineering/engine-advisories.md).

Each decision authorizes only what it says. Implementation follows the
response plan's rules: one reviewable slice at a time, every new guard
fault-seeded, and nothing activates a schema or migrates a file unless its own
decision says so.

## Security and release

| Decision | Chosen option | What it means |
| --- | --- | --- |
| DEC-ADVISORY-SCAN | **Release gate + 90-day** | `make engine-advisories` becomes required in `check-release` before every tag, and the scan fails if `reviewed_on` is more than 90 days old. `main` does not go red on its own. NVD must be reachable from the release host. |
| SEC-04 item 1, recovery codes on the clipboard | **Auto-clear after 60 s** | After Copy, clear the clipboard after 60 seconds only if it still holds the codes, and say so beside the button. The clearing must read before it clears, so it never wipes something else the user copied. |
| SEC-04 item 2, plaintext copy after encryption | **Offer to delete after open** | Once the encrypted project has opened successfully, offer a confirmed "Delete the unencrypted copy" action. SECURITY.md changes from "retain" to "kept until the user deletes it". |
| SEC-04 items 3 and 4, sign-in attempts and timing | **Add a short delay** | An increasing delay after repeated failed sign-ins, up to a few seconds, with no lock-out. Unknown-username timing is not equalized; it stays recorded as residual risk. |
| DEC-WINDOWS | **Unsupported preview** | Keep building the Windows `.exe`, labelled unverified preview in release notes, until a real Windows host qualifies it. No Windows support is advertised. SEC-05 stays unverified. |

### Amendment, same day

After choosing "Add a short delay", the maintainer asked for the timing of
unknown usernames to be increased as well. Sign-in now does the same
Argon2id work for a username with no account as for a wrong password, and
both count toward the same delay. The recovery-code reset on the sign-in
screen does the same: a full set of checks for any name, and the same
delay. This replaces "Unknown-username timing is
not equalized" above; usernames remain readable locally, so this closes a
timing difference, not a disclosure.

## Architecture

| Decision | Chosen option | What it authorizes |
| --- | --- | --- |
| DEC-AUDIT (ADR-011 DEC-04) | **Approve bridge design** | A design slice for a versioned, append-only compact audit event that joins the existing hash chain, never rewrites, resequences or recanonicalizes old events, and keeps `VerifyAuditChain` passing on mixed history. Implementation needs a separate approval. |
| DEC-FORMAT | **Finish design, then ratify** | Completing ADR-011's open items (O-1, O-6 to O-10) into a full schema, units, migration and recovery specification for the owner's ratification. No file format change until ratified. The layered protection decided on 2026-10-27 (O-13) stands. |
| DEC-HARDWARE | **Two profiles** | A mid-range laptop (about 8 GB RAM, 4 cores, SSD) as the minimum and a current Apple Silicon Mac as the reference. Budgets are set on the minimum. Fixtures are licensed or generated from real anonymized schedules. Numeric budgets still need approval once measured. |
| DEC-INTEROP | **Authorize G1 only** | The production worker supervisor and bridge with the bundled MPXJ worker, inactive behind a capability flag, with its production dependencies (grpc-go, Protobuf, go-winio) reviewed. G2 and G3 wait for G1's evidence and their own approval. Work follows the ADR-010 preservation prompt. |

## Still open

- **SEC-04 items 14 and 15:** a design decision held privately with the
  maintainer.
- **Numeric performance budgets:** set after measurement on the two profiles.
- **G2, G3, and the audit bridge's implementation:** each needs its own
  approval after the slices authorized here.
