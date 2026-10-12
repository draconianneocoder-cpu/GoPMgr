<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Native editor Cmd+Q evidence, 2026-10-12

This closes a macOS verification gap, not the complete P0 editor-data-loss item.
No production code changed. Source: `3ac7f5b37a3d6bbd3d92e4a76a4da8566394817f`.
Host: macOS 27.0.1 (26A434), arm64. `make build` produced an ad-hoc-signed local
Wails app; this is not notarization, trusted distribution, or installer evidence.
Executable SHA-256:
`fd1963a4f8552aebadfbbac065828420d75eac5d42cebe11228d3c555caa5f8c`.

## Observed outcomes

Each test used an existing saved record in a synthetic encrypted project and
OS-level injected Cmd+Q directed at the verified owned application process,
not a DOM event or a direct call to the close controller.

| Editor | Cancel | Save refusal | Successful guarded save |
| --- | --- | --- | --- |
| `StakeholderManager` | Exact unsaved name retained; process alive | Whitespace required name rejected; draft and quit dialog retained | Process exited with status 0; exact final name recovered in the same record after relaunch |
| `WorkItemEditor` | Exact unsaved title retained; process alive | Whitespace required title rejected; draft and quit dialog retained | Process exited with status 0; exact final title recovered in the same record after relaunch |

The refused save was cancelled, the required field repaired, and Cmd+Q repeated
before the successful save. This also exercised guard re-arming after refusal.
Record counts remained unchanged across relaunch.

## Isolation and reproduction

1. Run `bash scripts/launch-isolated-native.sh` with the freshly built app.
   Use only a disposable account, project, and saved records.
2. Verify the owned PID and executable before each native shortcut. Create a
   baseline record, change its required field, then exercise the outcomes above.
3. For persistence, retain the generated sandbox profile and reuse the exact
   profile, deny parameters, HOME, XDG_DATA_HOME, TMPDIR, and
   CFFIXED_USER_HOME. The launcher creates a new root on each invocation, so
   running it again alone does not test persistence.
4. Check retained-path ownership, permissions, canonical paths and absence of
   symlinks; verify the executable digest and rerun the generated sandbox
   preflight before re-entry. Authenticate and reopen the same saved record.

The preflights passed on initial launch and both re-entries. Normal and legacy
GoPMgr/PMForge application-data and Documents roots were sandbox-denied.
This is bounded project-root isolation, not proof of zero macOS/WebKit cache
writes elsewhere. No real account or project was opened.

Native automation must use fresh accessibility indices after each UI snapshot.
For Svelte fields, actual paste/input events were needed: accessibility-only
value assignment could change the visible text without updating bound state.
One setup navigation attempt using a stale index was cancelled and excluded;
the intended native-close test was repeated with fresh indices.

## Supporting checks and limits

- `go test -race . -run 'Test(NativeCloseGuard|CompleteNativeClose)' -count=1`: passed.
- `npm --prefix frontend test -- src/lib/native-close.test.ts src/lib/components/project/StakeholderManager.test.ts src/lib/components/agile/WorkItemEditor.test.ts`: 48 tests passed.
- `bash scripts/launch-isolated-native_test.sh` and `make build`: passed.

The build emitted the existing macOS linker deployment-target warning. No
support-policy change or older-macOS compatibility claim follows from this run.
Not exercised: Cmd+Q Discard in these editors, native disk/backend failure,
native mid-save edits, new-record Cmd+Q, crash/power loss, Linux/Windows GUI,
assistive technology, package lifecycle, signing, or notarization. Whitespace
refusal proves validation handling, not backend failure handling. Automated
tests and historical window-close evidence do not substitute for these cases.
