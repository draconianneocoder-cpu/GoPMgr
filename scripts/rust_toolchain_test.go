// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type rustToolsFixture struct {
	root, bin, pinned, trace, launcher, version string
}

func writeRustToolFixture(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
}

func newRustToolsFixture(t *testing.T) rustToolsFixture {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX DF0 reproduction launcher fixture")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("Bash is required for DF0 reproduction launcher fixtures")
	}
	source := filepath.Join("..", "spikes", "adr012-datafusion")
	var pin struct {
		Toolchain struct{ Channel string }
	}
	b, err := os.ReadFile(filepath.Join(source, "rust", "rust-toolchain.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := toml.Unmarshal(b, &pin); err != nil || pin.Toolchain.Channel == "" {
		t.Fatalf("read real toolchain pin: %v", err)
	}
	f := rustToolsFixture{root: t.TempDir(), version: pin.Toolchain.Channel}
	f.bin = filepath.Join(f.root, "shadow tools")
	f.pinned = filepath.Join(f.root, "pinned tools", "bin")
	f.trace = filepath.Join(f.root, "trace")
	f.launcher = filepath.Join(f.root, "scripts", "with-rust-toolchain.sh")
	for _, dir := range []string{f.trace, filepath.Join(f.root, "rust", "codegen")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"with-rust-toolchain.sh", "run-df0b.sh", "run-df0c.sh", "generate-df0ipc.sh"} {
		b, err := os.ReadFile(filepath.Join(source, "scripts", name))
		if err != nil {
			t.Fatal(err)
		}
		writeRustToolFixture(t, filepath.Join(f.root, "scripts", name), string(b))
	}
	writeRustToolFixture(t, filepath.Join(f.bin, "rustup"), `#!/bin/bash
set -eu
printf '%s\n' "$*" >> "$TRACE/rustup"
case "${1:-}" in
  which)
    [ "$#" -eq 4 ] && [ "$2" = --toolchain ] && [ "$3" = "$EXPECTED_PIN" ] && [ "$4" = cargo ] || exit 71
    printf '%s/cargo\n' "$PINNED"
    exit "${LOOKUP_EXIT:-0}"
    ;;
  run)
    [ "$#" -ge 3 ] && [ "$2" = "$EXPECTED_PIN" ] || exit 71
    [ "${RUN_EXIT:-0}" -eq 0 ] || exit "$RUN_EXIT"
    printf '%s\n' run >> "$TRACE/rustup-run"
    shift 2
    exec "$@"
    ;;
  *) exit 71 ;;
esac
`)
	for _, name := range []string{"cargo", "rustc", "rustdoc"} {
		writeRustToolFixture(t, filepath.Join(f.bin, name), "#!/bin/bash\nprintf '%s\\n' '"+name+"' >> \"$TRACE/shadow\"\nexit 81\n")
	}
	for _, name := range []string{"rustc", "rustdoc"} {
		writeRustToolFixture(t, filepath.Join(f.pinned, name), "#!/bin/bash\nprintf '%s\\n' '"+name+" "+f.version+" fixture'\n")
	}
	invocationProbe := `#!/bin/bash
set -eu
printf '%s\0' "$@" >> "$TRACE/args"
printf '%s\n' "$PWD" "$RUSTUP_TOOLCHAIN" "$RUSTC" "$RUSTDOC" "$CARGO_TARGET_DIR" >> "$TRACE/meta"
"$RUSTC" --version > "$TRACE/rustc"
"$RUSTDOC" --version > "$TRACE/rustdoc"
rustc --version > "$TRACE/path-rustc"
exit "${CARGO_EXIT:-0}"
`
	writeRustToolFixture(t, filepath.Join(f.pinned, "cargo"), invocationProbe)
	writeRustToolFixture(t, filepath.Join(f.bin, "probe"), invocationProbe)
	writeRustToolFixture(t, filepath.Join(f.bin, "go"), `#!/bin/bash
set -eu
case "$*" in
  *cmd/df0-candidates*|*cmd/df0-ipc*)
    [ "$RUSTUP_TOOLCHAIN" = "$EXPECTED_PIN" ] || exit 72
    rustc --version > "$TRACE/collector-rustc"
    ;;
esac
`)
	for name, version := range map[string]string{"protoc": "libprotoc 25.9", "protoc-gen-go": "protoc-gen-go v1.36.12", "protoc-gen-go-grpc": "protoc-gen-go-grpc 1.6.2"} {
		writeRustToolFixture(t, filepath.Join(f.root, "tools", "bin", name), "#!/bin/bash\nif [ \"${1:-}\" = --version ]; then printf '%s\\n' '"+version+"'; fi\n")
	}
	return f
}

func (f rustToolsFixture) run(t *testing.T, script string, args []string, extra ...string) (int, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", append([]string{script}, args...)...)
	cmd.Dir = f.root
	cmd.Env = append(os.Environ(), "PATH="+f.bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"TRACE="+f.trace, "PINNED="+f.pinned, "EXPECTED_PIN="+f.version,
		"CARGO_TARGET_DIR="+filepath.Join(f.root, "target with spaces"), "TOOLS="+filepath.Join(f.root, "tools"),
		"RUSTUP_TOOLCHAIN=ambient-old", "RUSTC="+filepath.Join(f.bin, "rustc"), "RUSTDOC="+filepath.Join(f.bin, "rustdoc"))
	cmd.Env = append(cmd.Env, extra...)
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("fixture command timed out: %v\n%s", ctx.Err(), out)
	}
	if err == nil {
		return 0, string(out)
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), string(out)
	}
	t.Fatalf("execute fixture: %v", err)
	return -1, string(out)
}

func (f rustToolsFixture) assertNoShadow(t *testing.T) {
	t.Helper()
	if b, err := os.ReadFile(filepath.Join(f.trace, "shadow")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("shadow tool executed: %s, %v", b, err)
	}
}

func TestRustToolchainPreservesInvocationAndOverridesShadowTools(t *testing.T) {
	for _, c := range []struct {
		command string
		code    int
	}{{"cargo", 0}, {"cargo", 29}, {"probe", 0}, {"probe", 29}} {
		t.Run(c.command+"-exit-"+strconv.Itoa(c.code), func(t *testing.T) {
			code := c.code
			f := newRustToolsFixture(t)
			args := []string{"test", "--locked", "two words", "", "literal;$value"}
			setting := "CARGO_EXIT=0"
			if code != 0 {
				setting = "CARGO_EXIT=29"
			}
			got, out := f.run(t, f.launcher, append([]string{c.command}, args...), setting)
			if got != code {
				t.Fatalf("exit %d, want %d: %s", got, code, out)
			}
			b, err := os.ReadFile(filepath.Join(f.trace, "args"))
			if err != nil || !bytes.Equal(b, []byte(strings.Join(args, "\x00")+"\x00")) {
				t.Fatalf("argument boundaries changed: %q, %v", b, err)
			}
			b, err = os.ReadFile(filepath.Join(f.trace, "meta"))
			lines := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
			if err != nil || len(lines) != 5 {
				t.Fatalf("missing invocation metadata: %q, %v", b, err)
			}
			gotDir, err := os.Stat(lines[0])
			if err != nil {
				t.Fatal(err)
			}
			wantDir, err := os.Stat(f.root)
			if err != nil || !os.SameFile(gotDir, wantDir) {
				t.Fatalf("working directory changed: %q, %v", lines[0], err)
			}
			want := strings.Join([]string{f.version, filepath.Join(f.pinned, "rustc"), filepath.Join(f.pinned, "rustdoc"), filepath.Join(f.root, "target with spaces")}, "\n")
			if strings.Join(lines[1:], "\n") != want {
				t.Fatalf("toolchain/target changed: %q, want %q", b, want)
			}
			b, err = os.ReadFile(filepath.Join(f.trace, "rustup-run"))
			if err != nil || string(b) != "run\n" {
				t.Fatalf("rustup runtime environment bypassed: %q, %v", b, err)
			}
			for _, name := range []string{"rustc", "rustdoc", "path-rustc"} {
				b, err := os.ReadFile(filepath.Join(f.trace, name))
				if err != nil || !strings.Contains(string(b), f.version+" fixture") {
					t.Fatalf("wrong %s selected: %q, %v", name, b, err)
				}
			}
			f.assertNoShadow(t)
		})
	}
}

func TestRustToolchainFailsWithoutFallbackOrInstallation(t *testing.T) {
	for _, name := range []string{"lookup-partial-failure", "run-refusal", "missing-rustup", "missing-cargo", "missing-rustc", "missing-rustdoc", "no-command"} {
		t.Run(name, func(t *testing.T) {
			f := newRustToolsFixture(t)
			args := []string{"cargo", "test"}
			var env []string
			want := 1
			switch name {
			case "lookup-partial-failure":
				env, want = []string{"LOOKUP_EXIT=43"}, 43
			case "run-refusal":
				env, want = []string{"RUN_EXIT=49"}, 49
			case "missing-rustup":
				empty := filepath.Join(f.root, "empty")
				if err := os.Mkdir(empty, 0o700); err != nil {
					t.Fatal(err)
				}
				env, want = []string{"PATH=" + empty}, 127
			case "no-command":
				args, want = nil, 2
			default:
				if err := os.Remove(filepath.Join(f.pinned, strings.TrimPrefix(name, "missing-"))); err != nil {
					t.Fatal(err)
				}
			}
			got, out := f.run(t, f.launcher, args, env...)
			if got != want {
				t.Fatalf("exit %d, want %d: %s", got, want, out)
			}
			if _, err := os.Stat(filepath.Join(f.trace, "args")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("Cargo ran after failed setup: %v", err)
			}
			f.assertNoShadow(t)
		})
	}
}

func TestDF0EntrypointsUsePinnedRustForBuildsAndEvidence(t *testing.T) {
	for _, name := range []string{"run-df0b.sh", "run-df0c.sh", "generate-df0ipc.sh"} {
		t.Run(name, func(t *testing.T) {
			f := newRustToolsFixture(t)
			got, out := f.run(t, filepath.Join(f.root, "scripts", name), nil)
			if got != 0 {
				t.Fatalf("entrypoint failed: %d: %s", got, out)
			}
			var want []string
			calls := 2
			switch name {
			case "generate-df0ipc.sh":
				want = []string{"run", "--locked", "--release", "-q", "--", "../../proto", "../worker/src/pb"}
				calls = 1
			default:
				pkg := "df0-cli"
				if name == "run-df0c.sh" {
					pkg = "df0-worker"
				}
				want = []string{"test", "--locked", "--release", "build", "--locked", "--release", "-p", pkg}
			}
			b, err := os.ReadFile(filepath.Join(f.trace, "args"))
			if err != nil || !bytes.Equal(b, []byte(strings.Join(want, "\x00")+"\x00")) {
				t.Fatalf("Cargo invocations changed: %q, want %q, %v", b, want, err)
			}
			b, err = os.ReadFile(filepath.Join(f.trace, "meta"))
			if err != nil || bytes.Count(b, []byte("\n")) != 5*calls {
				t.Fatalf("Cargo invocation count changed: %q, %v", b, err)
			}
			runs := calls
			if name != "generate-df0ipc.sh" {
				runs++ // The Go evidence collector also needs rustup's environment.
			}
			b, err = os.ReadFile(filepath.Join(f.trace, "rustup-run"))
			if err != nil || string(b) != strings.Repeat("run\n", runs) {
				t.Fatalf("entrypoint bypassed rustup runtime setup: %q, %v", b, err)
			}
			if name != "generate-df0ipc.sh" {
				b, err := os.ReadFile(filepath.Join(f.trace, "collector-rustc"))
				if err != nil || !strings.Contains(string(b), f.version+" fixture") {
					t.Fatalf("evidence collector used wrong compiler probe: %q, %v", b, err)
				}
			}
			f.assertNoShadow(t)
		})
	}
}
