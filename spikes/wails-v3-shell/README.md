<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Wails v3 native lifecycle qualification shell

Isolated migration evidence, not a second application runtime or fallback.
Wails v3.1.0/Go 1.27.3/CGO are pinned in this module. It uses an embedded
plain JavaScript test frontend, actual native windows/events, and a disposable
file to model resource ownership. It never opens GoPMgr account/project data.
It does not qualify SQLCipher, production editors or the 209-method bridge.

The controlled scenarios cover programmatic window/application close,
Save/Discard/Cancel, failed-save retry, bootstrap close, injected service startup
failure, pre-dispatch failure with revoked approval, SIGINT/SIGTERM,
a guarded updater-style Quit through a bound service, and cancellation/draining
of an in-flight bridge call before file closure.
Expected event order/counts and independently read saved-file contents must
match. macOS OS-driven close button/Cmd+Q/menu scenarios are opt-in and require
an unlocked GUI session and existing Accessibility authorization. The opt-in
also drives actual Open/Save panels through Cancel and checks no data/export
mutation before the normal guarded close.

## macOS checks

From this directory, use the same deployment settings for build and tests:

```sh
export MACOSX_DEPLOYMENT_TARGET=13.0
export CGO_CFLAGS=-mmacosx-version-min=13.0
export CGO_CXXFLAGS=-mmacosx-version-min=13.0
export CGO_LDFLAGS=-mmacosx-version-min=13.0

go test -race -count=1 -timeout=45s ./...
mkdir -p ../../.tmp/wails-v3
go build -race -o ../../.tmp/wails-v3/shell-race .
WAILS_SHELL_BIN="$(pwd)/../../.tmp/wails-v3/shell-race"   go test -race -count=1 -v -timeout=180s ./...
go vet ./...
govulncheck ./...
```

Without `WAILS_SHELL_BIN`, the native test explicitly skips; that is not a
native PASS. Both harness and binary need `-race` for instrumented application
code. Native subprocesses receive private HOME/data directories and a
25-second bound; only owned child processes are signalled or killed.

For OS-driven checks, create an ad-hoc test bundle, keeping it outside release
scope. This does not establish Developer ID or notarization trust:

```sh
python3 - <<'PYBUILD'
from pathlib import Path
import plistlib, shutil
app = Path('../../.tmp/wails-v3/GoPMgrProbe.app')
exe = app / 'Contents/MacOS/shell'
exe.parent.mkdir(parents=True, exist_ok=True)
shutil.copy2('../../.tmp/wails-v3/shell-race', exe)
(app / 'Contents/Info.plist').write_bytes(plistlib.dumps({
    'CFBundleExecutable': 'shell',
    'CFBundleIdentifier': 'org.gopmgr.wails3.qualification',
    'CFBundleName': 'GoPMgrProbe', 'CFBundlePackageType': 'APPL',
    'LSMinimumSystemVersion': '13.0', 'NSHighResolutionCapable': True,
    'NSPrincipalClass': 'NSApplication',
}))
PYBUILD
codesign --force --sign - ../../.tmp/wails-v3/GoPMgrProbe.app
WAILS_SHELL_OS_UI=1   WAILS_SHELL_BIN="$(pwd)/../../.tmp/wails-v3/GoPMgrProbe.app/Contents/MacOS/shell"   go test -race -count=1 -v -timeout=180s -run '^TestNativeShellCloseCycle$' .
```

Do not run keyboard/menu automation on a locked desktop. A rendered WebView
or a process exit is insufficient proof of a visible window or user-driven
close routing. Current blocked checks and platform limits are in the
[migration evidence](../../docs/testing/wails-v3-migration-evidence.md).

## Linux checks

On a prepared local Linux target, verify `pkg-config --modversion gtk4 webkitgtk-6.0` first. Use Go 1.27.3 (a task-specific `GOTOOLCHAIN=go1.27.3`
can retain the host's installed compiler), then:

```sh
go test -race -count=1 -timeout=45s ./...
probe_output=$(mktemp -d "${TMPDIR:-/tmp}/gopmgr-wails-v3.XXXXXX")
go build -race -o "$probe_output/shell" .
mkdir -p "$probe_output/home" "$probe_output/runtime"
chmod 700 "$probe_output/home" "$probe_output/runtime"
env GOPATH="$(go env GOPATH)" GOCACHE="$(go env GOCACHE)" \
  HOME="$probe_output/home" XDG_RUNTIME_DIR="$probe_output/runtime" \
  XDG_CONFIG_HOME="$probe_output/home/config" \
  XDG_CACHE_HOME="$probe_output/home/cache" \
  XDG_DATA_HOME="$probe_output/home/data" \
  WAILS_SHELL_BIN="$probe_output/shell" \
  xvfb-run -a -s '-screen 0 1280x800x24 -extension GLX' \
  dbus-run-session -- go test -race -count=1 -v -timeout=180s ./...
```

The command above runs twelve programmatic/signal cases. To add the two
native dialog cancellations and the native Ctrl+Q menu accelerator, supply
these additional environment assignments to that same private-Xvfb command:

```sh
WAILS_SHELL_PRIVATE_X11=1 \
WAILS_SHELL_X11_TOOL=/absolute/task/path/xdotool
```

Use a verified task-local or already installed `xdotool`; if it was extracted
with its library into a task directory, also supply that library directory
through `LD_LIBRARY_PATH`. The tested tool version was 3.20160805.1, from
Ubuntu package `1:3.20160805.1-5.1build1`. No global installation is required.
The private-display flag is an operator assertion; it does not create isolation.
Set HOME/XDG variables before starting D-Bus so its portal services are private
too. GTK portals own their dialog processes, so the driver matches the unique
probe-PID title rather than requiring the dialog's PID to match the probe.
Do not substitute the active desktop's display.

Remove only the retained `probe_output` directory after inspection and after
all owned child processes have exited.

The GLX setting is specific to the current Xvfb host. Keep a private,
mode-0700 `XDG_RUNTIME_DIR` when the host needs one. Preserve renderer and
portal warnings in the evidence. Xvfb does not qualify Wayland, accelerated
rendering, desktop accessibility, accepted file selections or installed
packages. The opt-in cases qualify dialog cancellation only. There
is no GTK 3 backend selection or fallback in this shell.
