<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Wails v3 / GTK 4 migration evidence

Status: implementation begun on 2026-11-03; **core native shell gate passed
on the available hosts**. Application, platform and cutover qualification
remain incomplete. The main application has not been switched to v3. Its existing
runtime is the staging baseline, not an approved fallback. The owner requires
forward fixes and backup-based data recovery, with no v2/GTK 3 fallback.

Baseline: `e193e993c1dd6972ed9b72a4c1758d06580f1a3c`; branch
`migration/wails-v3-gtk4`. Isolated shell evidence is not a frozen release candidate. Scope is `spikes/wails-v3-shell`; the root Go dependency
graph, production frontend and application API are unchanged.

## Verified baseline and provenance

- `make verify`, `make race`, `make license-check` and
  `make check-encrypted-db`: passed on the current macOS host before conversion.
- Wails library v3.1.0, upstream commit
  `4b454e443f6f11ccb64679c5c3d3f3bf5e6fb440`, module sum
  `h1:ld0AXSicXiTTsIvmTq3qNn9FzCP8vIHRSJdkRLgaAo8=`.
- macOS arm64, Go 1.27.3, CGO; native execution host reports macOS 27.2.
  Compiler C/C++/linker flags and `MACOSX_DEPLOYMENT_TARGET` are set to 13.0;
  `vtool` confirms the probe's Mach-O minimum 13.0. This is not execution on
  macOS 13. The temporary bundle is ad-hoc signed without added entitlements;
  no Developer ID, notarization or installed-app trust is claimed.
- Local Ubuntu 26.04.1 x86_64: GTK 4.22.4 and WebKitGTK 2.52.6 through
  `gtk4`/`webkitgtk-6.0`. ELF inspection confirms direct linkage to
  `libgtk-4.so.1`, `libwebkitgtk-6.0.so.4` and `libjavascriptcoregtk-6.0.so.1`.
  Global Go 1.26.8 was left unchanged; the task uses
  `GOTOOLCHAIN=go1.27.3`. Native runs use Xvfb with GLX disabled and a private
  D-Bus session. They do not qualify accelerated rendering, Wayland,
  accepted file selections or the minimum Ubuntu 24.04 target.

## Informational v2 baseline

A fresh build of the unchanged production source used `desktop,production`,
`-ldflags '-s -w'`, Go 1.27.3, the macOS 13 deployment floor and
`-framework UniformTypeIdentifiers`. Three launches used the existing native
isolation harness, fresh disposable data roots and explicit sandbox denials
for real account/project roots. All three quit normally through Cmd+Q.
Time from harness launch to the first Accessibility window was 1.127, 0.588
and 0.651 seconds. Main-process RSS two seconds later was 139552, 139744 and
140368 KiB; the executable was 92690688 bytes. These are observations, not
budgets or evidence of a speedup. Window existence does not prove completed
frontend startup; main-process RSS excludes WebKit helpers. The minimal ad-hoc
bundle is not an installer/package-size baseline. Full package and comparable
v3 application measurements remain pending.

## Test sensitivity and findings

1. A compiling permissive close gate failed dirty-close, invalid-approval and
   failed-save assertions before the mutex coordinator was implemented.
2. A real native permissive callback allowed window destruction without the
   prompt/approval trace; the native regression rejected it before wiring
   the coordinator into both hooks.
3. An immediate-close resource owner failed admission/draining assertions
   before the lock/WaitGroup/once owner was implemented. `synctest` removes
   the scheduler-dependent assumption that a goroutine has reached its wait.
4. Independent review found unordered/count-insensitive traces, save/discard
   labels without actual saves, startup-error misclassification and missing
   host revocation. Corrections use exact trace sequences, real file writes
   and reopen checks, an `errors.Is` sentinel and an inside-startup marker,
   and an injected pre-dispatch failure with host revocation and retry.
5. Reusing a custom-event name for recovery request and response caused an
   observed feedback loop: v3 Go listeners receive host emissions too.
   Distinct request/response names fix it; exact trace counts catch it.
6. Five compiling overlay faults were detected by their intended assertions:
   duplicate prompts, unrevoked approval, file close before draining,
   unrelated startup errors, and omitted host revocation. Fault sources and
   binaries were disposable; no production source mutation was left behind.
7. An initial test link warned about inconsistent macOS deployment targets.
   C++ flags and the deployment environment now match C/linker flags.

The twelve-scenario programmatic/bridge/signal matrix passed on macOS and Ubuntu:
window Close, application Quit, Cancel/re-request, failed-save retry, Discard,
bootstrap close, actual service-startup failure, dispatch-failure recovery,
SIGINT, SIGTERM, a guarded updater-style Quit through a real bound service,
and cancellation/draining of an in-flight bridge call before file closure.
Both separately launched probe binaries and the Go harness
and unit suites were built/run with the race detector.
`govulncheck ./...` in the isolated module reported no vulnerabilities.
The task-local CLI reports v3.1.0; the global v2 CLI is unchanged.
Scoped `go vet ./...`, REUSE licensing and coverage-ledger currency checks
also passed. Independent plan and intermediate reviews produced the corrections
above. After an initial reviewer usage-limit failure, independent final source review
became available and approved the core shell gate on the available hosts.
Its documentation findings are corrected; this approval does not qualify the production application.

## Native macOS UI result and remaining qualification

- **macOS OS close button / Cmd+Q / menu Quit: passed after unlocking.**
  The race-instrumented ad-hoc bundle and harness passed all seventeen cases,
  including the three Accessibility-driven close actions and two actual
  native dialog cancellations on the owned process.
  Exact traces verified prompt, save, approval, shutdown and resource closure;
  reopened files contained the saved draft. Earlier locked-screen failures
  remain environment failures, not Wails defects. The preflight now requires
  an explicitly unlocked console and the current user as its logged-in owner;
  a missing per-session lock property alone does not establish that state.
- The owner confirmed no macOS 13 or Ubuntu 24.04 test hosts are currently
  available. The owner approved moving their execution checks from the pre-conversion
  gate to a mandatory release gate on 2026-11-03. Development may continue on
  the available hosts; older-version compatibility is not yet qualified.
- Minimum macOS 13 execution, Ubuntu 24.04, Windows, Wayland, HiDPI,
  accessibility, accepted file selections and package lifecycle remain unqualified.
- Ubuntu passed fifteen cases with the race-instrumented binary and harness:
  the twelve common cases, two actual portal-mediated dialog cancellations
  and the native Ctrl+Q menu accelerator. Initial dialog searches failed
  because the portal owned the window; inspection confirmed its unique probe
  title, and the private-display driver now matches that title. Both the
  probe and the outer D-Bus/portal session use disposable HOME/XDG roots.
- Dialog acceptance, the production updater and production account-store
  draining remain pending. The model and real probe bridge cancellation tests
  do not establish the production App's lifecycle behavior.
- Production App service registration, all 209 generated binding contracts,
  real editor recovery, SQLCipher lifecycle/draining, packaging, notices/SBOM
  refresh and final cutover checks have not run for v3.

Do not broaden conversion on the strength of mock, programmatic-close or
locked-screen results. See the [approved design](../design/wails-v3-migration-design.md)
and the isolated shell README for reproduction commands and the next gate.
