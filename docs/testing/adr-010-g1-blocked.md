<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-010 G1 — BLOCKED tests and resource plan

**Status:** live list, kept up to date during G1. Created 2026-10-26 under
the maintainer's [P04 authorization](../review/ADR-010-G1-P04-authorization-decision.md),
decision 6.

**Rules:**

- A row here is **BLOCKED** until its validation has run on the named
  target and passed. It is never reported as PASS, and no hosted proxy
  substitutes for it.
- A row leaves the list only with its evidence: commit, target, raw files
  and result.
- **No signing key, certificate password, notarization credential or
  other secret** goes into the repository, prompts, logs or fixtures.
  Credentials live only in the signing service or in a protected CI
  secret store.

## 1. BLOCKED list

| ID | Target | Blocks | Why it is blocked |
|---|---|---|---|
| BLK-01 | Windows 11 x64 **desktop** | TPT-003, TPT-004 on the advertised Windows target; design §8 platform row; any Windows desktop claim | No hosted standard runner has a Windows 11 x64 client image |
| BLK-02 | Windows 10 x64 (22H2) | The same, while Windows 10 stays supported (P04-D5) | No hosted image of any kind |
| BLK-03 | Elevated host and non-elevated same-user intruder, on BLK-01 and BLK-02 | TPT-026; P04-D3 privileged and unprivileged interactions | Needs a desktop session with UAC split tokens. Hosted runners run as an administrator without one |
| BLK-04 | macOS 13 arm64 (stated minimum) | TPT-002 on the minimum version | The oldest hosted image is macOS 14 |
| BLK-05 | macOS Developer ID test signing, notarization, Gatekeeper | TPT-002 signed-app clause; Team-ID library validation | Needs an Apple Developer ID identity in a secured signing service |
| BLK-06 | Windows Authenticode test signing and WDAC | Signed execution on Windows; design §8 signed row | Needs a test-signing service and a WDAC test machine |
| BLK-07 | Installed, signed application on every OS | TPT-002, the design §8 packaging and signed rows, the plan §2.2 offline package | Needs G1 packaging, which needs BLK-05 and BLK-06 |
| BLK-08 | Approved hardware profiles (OWN-10) | Final G, K and M (P04-D4); final Q3 selection; the 20 % threshold applied | Needs an owner decision; S0 (#142) proposes candidates |

## 2. Acquisition and validation plan

### BLK-01 and BLK-02: Windows 11 and Windows 10 x64 desktops

**Options, in order of preference:**

1. **Self-hosted, ephemeral runner on licensed VMs.** One Windows 11 Pro
   or Enterprise 24H2 VM and one Windows 10 22H2 VM, under Hyper-V,
   VirtualBox or VMware on an owner machine, or in Azure.
   - Licensing: each VM needs its own licensed Windows. Azure Windows 11
     and 10 client images need eligible licensing; the maintainer
     confirms it.
   - Registration: repository **Settings → Actions → Runners → New
     self-hosted runner**, with labels `self-hosted, windows, x64, win11`
     (or `win10`).
   - Run the runner with `--ephemeral`, and revert the VM to a clean
     snapshot after each job.
   - **Security:** this repository is public, so the job must not run for
     pull requests from forks. The workflow keeps the job to `push` and
     `workflow_dispatch` on this repository's branches, with a required
     environment approval. The runner account holds no other secret.
2. **GitHub larger runner with the "Windows 11 desktop" partner image.**
   It is paid. The documentation says it needs a credit card, but does not
   say which plans may use it; check plan eligibility for this account
   first. It covers BLK-01 only, not Windows 10.
3. **Owner device, manual run.** Use the same commands as CI, and commit
   the evidence files from that run. This is acceptable evidence when the
   run is recorded with its commit and OS build.

**Validation, on each OS:**

- the G0 probe now, and the G1 suite (plan §5) once it exists, on the
  jlink runtime;
- offline with the firewall rules;
- the race-detector run;
- for BLK-03, the elevated-host scenarios (TPT-026);
- later, the installed signed app (BLK-07).

**PASS criteria:** every design §8 row passes. The cancel-to-exit p99
stays within G with no escalation.

### BLK-03: privilege interactions

- On the BLK-01 and BLK-02 machines, add a second, standard same-user
  logon session, with UAC on.
- Start the host elevated. Start the intruder from the unelevated session:
  try to add a second instance, connect first, and impersonate the client.
  Then reverse the roles.
- Record what Windows allows at each integrity level.
- **PASS:** the intruder gets no instance, no connection and no session,
  and the worker accepts no command from it.

### BLK-04: macOS 13

- **Option 1:** an owner Apple-silicon Mac running macOS 13. Run manually,
  or register it as a self-hosted runner in a dedicated user account.
- **Option 2:** a macOS 13 VM on Apple hardware. Apple's license allows
  up to two macOS VMs per Mac; use Tart or UTM with a macOS 13 restore
  image.
- **Validation:** the main, signed and sandbox-offline runs, as on
  macOS 14 and 15.

### BLK-05: Developer ID signing and notarization

- **Resource:** membership of the Apple Developer Program, with a
  **Developer ID Application** certificate and an App Store Connect API
  key for `notarytool`.
- **Storage:** a signing service, or a GitHub **environment** with
  required reviewers and environment secrets. The private key never
  leaves that store. The agent never asks for it or handles it.
- **Validation:**
  - sign the jlink image and the worker with the hardened runtime and
    `allow-jit` only;
  - check Team-ID library validation, including a deliberately foreign
    library that must be refused;
  - notarize and staple;
  - `spctl --assess`;
  - run the G1 suite from the installed app.

### BLK-06: Authenticode and WDAC

- **Resource:** a test code-signing identity from one of:
  - a test-only internal CA, whose root is trusted only on the test VM;
  - a test profile in a managed signing service (for example Azure
    Trusted Signing).
- **Machine:** a Windows 11 test VM with a WDAC policy that allows only
  signed binaries.
- **Validation:**
  - sign GoPMgr's binaries and the installer;
  - confirm WDAC allows them and blocks an unsigned copy;
  - run the G1 suite.
- SmartScreen reputation cannot be tested with a test identity. It stays
  out of scope until production signing is authorized.

### BLK-07: installed signed application

This depends on the G1 packaging work, plus BLK-05 and BLK-06.

**Validation:** the G1 suite from the installed app, offline, on every
supported OS.

### BLK-08: hardware profiles

- **Resource:** the maintainer approves the profiles. S0 (#142) proposes
  the candidates.
- **Validation:** on each approved profile, re-run `cancel` (n ≥ 30),
  `tree-kill` (kill to reap) and the design §4.1 comparison.
- **Then:**
  - set G = max(1 s, 4 × worst p99) and K the same way;
  - investigate any comparison over 20 % (P04-D4), never waive it;
  - put the values to the maintainer.

## 3. What the agent can and cannot do

| Item | The agent can | The agent cannot |
|---|---|---|
| BLK-01 to BLK-04 | Write the workflow jobs and scripts, and analyze the evidence | Provide machines, licenses or runner registration |
| BLK-05, BLK-06 | Write signing steps that read credentials from a secured store | Hold, request or see private keys or passwords |
| BLK-08 | Run the measurements once profiles exist | Choose the profiles |
