<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Wails v3 / GTK 4 migration evidence

Status: production conversion is implemented in the migration working tree;
local integration validation is recorded below; release/cutover qualification remains open. This is not release
approval. The owner requires forward fixes and backup-based recovery, with no
v2/GTK 3 fallback. macOS 13 and Ubuntu 24.04 execution remain mandatory before
release; the owner approved continuing development because those hosts are
unavailable.

Baseline: `e193e993c1dd6972ed9b72a4c1758d06580f1a3c`; branch
`migration/wails-v3-gtk4`. The original isolated-shell qualification was
committed separately. The production dependency, frontend and build conversion
below is a later working-tree change, not a frozen published release candidate.

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
- At the isolated-shell milestone, dialog acceptance, the production updater and production account-store
  draining remain pending. The model and real probe bridge cancellation tests
  do not establish the production App's lifecycle behavior.
- At that milestone, production App service registration, all 209 generated binding contracts,
  real editor recovery, SQLCipher lifecycle/draining, packaging, notices/SBOM
  refresh and final cutover checks have not run for v3.

Do not claim native qualification from mock, programmatic-close or
locked-screen results. See the [approved design](../design/wails-v3-migration-design.md)
and the isolated shell README for reproduction commands and the next gate.


## Production conversion and evidence (2026-11-03 to 2026-11-04)

The root module and CLI are pinned to Wails v3.1.0; the frontend adds exactly
`@wailsio/runtime` 3.1.0. There is no legacy bridge or GTK 3 build branch.
The 209 exported App method names, parameters and return signatures match the
pre-migration inventory. Generation uses interfaces, name-based calls and
string timestamps. Independent clean generation produces identical output.
The generation runner canonicalizes upstream trailing whitespace and EOFs,
with a failing-before/passing-after regression; CI definitions use that same
runner. Staged-diff checks inspect newly added files as well as tracked edits.
Frontend calls import `desktopApi` from the generated service; domain/editor
view types do not replace the generated transport types.

The standard transport does not join running bound calls during shutdown.
GoPMgr therefore counts complete `/wails/runtime` handlers, closes admission,
cancels its owned domain context, and drains responses before requesting native
quit. The native event loop remains available during drainage. Joining those
handlers inside native `OnShutdown` would deadlock GTK response delivery.
Only after quiescence does `OnShutdown` clear session keys and close stores.
Unexpected late native-loop return with outstanding RPC or UI work refuses
store cleanup. No new transport protocol, header-authentication guarantee,
manual cgo/unsafe implementation, updater or database format was introduced.

Native About is asynchronous. Its UI lease lasts until dismissal rather than
until `Show` returns. The owner approved a separate Cancel button after native
experiments showed one button could support Return or Escape, but not both,
through the pinned public API. OK is the default and Cancel handles Escape;
both menu entry points use the owned dialog. The approval remains terminal
across both application/window hooks and late listener registration.

Passed evidence so far:

- Full desktop Go suite and runtimehost suite; focused race checks of shutdown,
  dialog admission and menu construction.
- Real production App native bridge on macOS 27.2 and Ubuntu 26.04, with race
  instrumentation and disposable account roots: fresh booleans, null account,
  domain error rejection, account creation/login/logout, exact budget
  `90071992547409.93`, encrypted project persistence/reopen, nil versus explicit
  empty collections. A test-only service additionally checks safe integer and
  text integer boundaries, byte/base64 encoding, nanosecond timestamps, empty
  maps and multiple return values. It is absent from the production service.
- Native macOS About OK, Cancel, Return and Escape: quit refusal preserves the
  usable session, actual dismissal releases admission, and retry exits cleanly.
- Production shutdown assertions start with a live user, project and DEK. They
  verify the original key backing array is zeroed and the store is closed.
  macOS normal termination exits the process, so these assertions run in
  `PostShutdown`; a successful exit alone is never accepted as proof.
- Three compiling native fault overlays are detected: changed large money,
  omitted session cleanup, and failed completion after guard enablement. A
  browser assertion failure reports promptly and exits nonzero.
- Frontend type checking: zero errors/warnings. The real-binding frontend
  suite passed 742 tests in the final aggregate verification, including the
  updated help copy.
- Native macOS production build: strict ad-hoc signature verification, retained
  `dev.gopmgr.GoPMgr` identity, lowercase `gopmgr.app`, Mach-O minimum 13.0 and
  bundle minimum 13.0.0. No Developer ID/notarization claim.
- Actual NSIS syntax compilation with the pinned v3-generated macros and
  harmless fixture executables. This is not native Windows lifecycle proof.
- PDF/A-2u, local PAdES-T and encrypted-database validation passed after the
  production conversion. External certificate trust is a separate gate.

Build failures cannot be rescued by stale bundles. Output publication uses
fresh staging; signing and signature verification errors propagate. The DMG
consumes the exact expected bundle, preserving unrelated build outputs.
Version/metadata authority is `build/config.yml`; the v2 config is removed.
The development URL and Vite readiness both use port 5173.

License checks found two new shipped-code attribution requirements: Rolldown's
injected interoperability runtime and the Wails npm runtime, whose package
omits its license file. Notices include Rolldown's MIT/derived-code notices and
the verbatim checksum-pinned Wails v3.1.0 upstream MIT license. Unknown virtual
modules still fail closed. Baseline/current npm audits both report the same
12 low/moderate development-tool advisories; no new advisory packages appeared.

Reproduce generated production bridge checks on macOS with the deployment
floor exported for every native Go command:

```sh
export MACOSX_DEPLOYMENT_TARGET=13.0
export CGO_CFLAGS=-mmacosx-version-min=13.0
export CGO_CXXFLAGS=-mmacosx-version-min=13.0
export CGO_LDFLAGS=-mmacosx-version-min=13.0
bash scripts/wails-v3-qualification.sh check
(cd frontend && node_modules/.bin/vite build ../internal/desktop/testdata --outDir "$PWD/../.tmp/wails-v3/native-wire-assets" --emptyOutDir)
GOPMGR_NATIVE_WIRE_ASSETS="$PWD/.tmp/wails-v3/native-wire-assets" go test -race ./internal/desktop -run '^TestNativeProductionBridge$' -count=1 -v
```

For macOS physical About checks, explicitly set `GOPMGR_NATIVE_OS_UI=1` and run
`TestNativeAboutDismissalAndQuitRetry` with the same assets in an unlocked
session. Linux UI checks require the explicitly opted-in private X11 driver;
they are distinct from the common bridge round trips. Ordinary unit runs skip
native subprocess execution when these inputs are absent.

## Final local integration checks and review disposition (2026-11-04)

- `make verify`, `make race`, Go/frontend lint, coverage-ledger drift (all 50
  measured package headings), release-scope, REUSE, PDF/A, local PAdES-T and
  encrypted-database gates passed. The final deployment-floor verification
  command and its result are recorded in the private execution log; no
  minimum-OS execution is inferred from compiler flags.
- macOS and GTK 4 native Open and Save selection passed through actual browser
  RPC, each dispatched once. Literal Unicode and apostrophes in directory and
  filenames survived. Input bytes remained unchanged and Save selection alone
  created no output. Quit while the panel was open refused safely; dismissal
  allowed retry and shutdown. GTK tests use an inspected fixed panel geometry,
  not proof across themes, locales or display scales.
- GTK About OK, Cancel, Return, Escape and window-manager close passed; macOS
  OK/Cancel/Return/Escape passed. A negative missing-dialog probe failed
  promptly. Both application-menu and Help About use the owned UI lease.
- The complete production macOS Svelte login screen rendered from bundled
  assets in an isolated home with sandbox denials for real account roots.
  Screenshot inspection confirmed visible fields/focus and no clipped login
  controls. An owned native About sheet showed both buttons; Escape followed by
  Cmd+Q exited zero. These checks do not qualify all editor flows or screen-reader
  behavior. Screenshots and process-identified traces remain private test output.
- Actual macOS PKG and DMG builds passed. Expanded PKG and mounted DMG app copies
  passed strict signature verification. The DMG's bundled frontend launched and
  Cmd+Q exited zero without a development server. No system installation,
  upgrade/removal, Developer ID or notarization is claimed.
- Package inspection exposed an existing DMG defect: adding license files after
  the original app signature invalidated the resource seal. Actual mounted
  verification failed on all five new notice files. A regression failed before
  the fix; default ad-hoc re-signing and strict verification now occur after
  adding notices. Injected signing and verification failures prevent packaging.
  The rebuilt mounted app passes strict verification. Release SBOM inventory
  and mounted-DMG selection also now use exact expected bundle paths; an
  unrelated earlier-sorting bundle causes the old selection regression to fail.
- Actual Ubuntu production .deb/.rpm builds and extracted metadata, payload,
  notices and ELF inspection passed. Package installation was not performed.
  With disposable HOME/XDG paths, the extracted Debian app rendered the full
  login screen (screenshot inspected) and Ctrl+Q exited zero, without a Vite
  server. This native run did not enforce network isolation.
  An additional outer bubblewrap/offline GUI experiment failed because the
  host denied WebKit's nested namespace/dbus-proxy launch; the app aborted.
  No WebKit security feature was disabled to obtain a pass. This experiment
  does not qualify packaged Linux GUI execution under that containment; earlier
  real-App GTK bridge/dialog checks remain separate successful evidence.

Adversarial review found and corrected native-thread response-drain deadlock,
asynchronous About lease release, incomplete native fixture preconditions and
shutdown assertions, Windows CLI extension handling, stale-bundle selection,
Bash 3 exit-status handling and package-signature invalidation. Compiling faults
in large-money delivery, session cleanup and guarded completion failed for the
intended reason. Bindings regenerated from equivalent clean inputs match and
209 public App signatures remain unchanged.

Earlier plan and intermediate reviews were independent. Both delegated agents
hit their usage limit before final integration review. The owner explicitly
approved a disclosed self-review fallback for the local commit. Final readiness
therefore uses that fallback and does not claim independent final approval.

Existing security observations remain: the full production vulnerability scan
found no reachable/imported vulnerable package, but the unused OpenPGP module
advisory GO-2026-5932 remains; npm retains the same 12 low/moderate development
advisories. The optional memory scan passed its configured gate while reporting
14 pre-existing gosec warnings; standalone staticcheck was absent, while Go
lint's configured staticcheck passed. These are not a claim of zero findings.
GTK X11 deprecation and private-Xvfb graphics/portal warnings remain disclosed.

One aggregate attempt correctly refused to run reset fixtures while the owned
GUI was active; the app was quit and verification rerun without bypassing the
protection. A test invocation without deployment-floor environment produced
linker target warnings; production builds explicitly set the 13.0 flags and the
final aggregate run uses the same environment. Warnings are not proof of older
OS compatibility.

Still open before release: frozen-candidate release/tag preflight, complete
installed-package lifecycle, real v2-data backup/restore qualification, minimum
macOS/Ubuntu execution, Windows native qualification, development-mode/HMR
execution, comparative startup/package measurements, Wayland/HiDPI/IME/
accessibility and installed application trust. Native shell tests do not replace
production editor/recovery workflow qualification. No hosted CI, push, PR,
publication or release ran for this task.
