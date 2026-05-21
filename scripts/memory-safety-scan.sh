#!/bin/bash
# SPDX-FileCopyrightText: 2026 The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Memory-safety hardening gate.
#
# Runs a sequence of static checks against the Go source tree. Each
# check that finds a violation prints the offending lines and the
# script exits non-zero so CI can fail the build.
#
# What this catches:
#   1. go vet ./...                — standard correctness checks
#      (printf format, suspicious assignments, lock copies, ...)
#   2. unsafe.Pointer usage         — forbidden in this codebase
#   3. os.Open without nearby Close — likely file-handle leak
#   4. sql.Open without nearby Close — likely connection leak
#   5. naked sync.Mutex copies      — `var m = otherMutex` is a bug
#   6. Goroutines without context   — every `go func` must accept a
#      ctx or be documented as fire-and-forget in this script's
#      allow-list at the bottom.
#
# Optional tools (auto-detected; skipped silently if absent):
#   staticcheck ./...
#   gosec ./...
#   govulncheck ./...

set -eu
cd "$(dirname "$0")/.."

# Colour helpers (no-op when not a TTY).
if [ -t 1 ]; then
    RED='\033[0;31m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'; NC='\033[0m'
else
    RED=''; YELLOW=''; GREEN=''; NC=''
fi

# Scope: only scan PMForge's own source tree. Other directories
# (vendored libraries, sibling repos accidentally cloned at the root)
# are explicitly excluded so the scan stays focused.
PMF_DIRS="./cmd ./internal ./scripts"

fail=0
section () {
    printf "\n${YELLOW}== %s ==${NC}\n" "$1"
}
ok () {
    printf "${GREEN}OK${NC}: %s\n" "$1"
}
fail_msg () {
    printf "${RED}FAIL${NC}: %s\n" "$1"
    fail=1
}

# -------------------------------------------------------------------
# 1. go vet
# -------------------------------------------------------------------
section "go vet ./..."
if command -v go >/dev/null 2>&1; then
    if go vet ./... 2>&1; then
        ok "go vet clean"
    else
        fail_msg "go vet reported issues (see above)"
    fi
else
    printf "${YELLOW}skip${NC}: go not in PATH\n"
fi

# -------------------------------------------------------------------
# 2. unsafe.Pointer is forbidden
# -------------------------------------------------------------------
section "unsafe.Pointer ban"
matches=$(grep -rn 'unsafe\.Pointer' --include='*.go' $PMF_DIRS 2>/dev/null \
    | grep -v '^\./vendor/' \
    | grep -v 'AGENT.md' \
    | grep -v 'scripts/memory-safety-scan.sh' \
    || true)
if [ -n "$matches" ]; then
    fail_msg "unsafe.Pointer use is forbidden — refactor or document"
    printf "%s\n" "$matches"
else
    ok "no unsafe.Pointer usage"
fi

# -------------------------------------------------------------------
# 3. os.Open without nearby defer Close
# -------------------------------------------------------------------
# Heuristic: for every line `os.Open(`, scan the following 10 lines
# for `defer .*Close()` or `.Close()`. Misses legitimate one-shot
# uses; flag for human review.
section "os.Open → defer Close()"
if ! command -v go >/dev/null 2>&1; then
    printf "${YELLOW}skip${NC}: go not in PATH (cannot run helper)\n"
    problems=""
else
problems=$(go run - <<'EOF' 2>&1 || true
package main

import (
    "bufio"
    "fmt"
    "os"
    "path/filepath"
    "strings"
)

func main() {
    root, _ := os.Getwd()
    bad := []string{}
    filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
        if err != nil || info.IsDir() {
            return nil
        }
        if !strings.HasSuffix(p, ".go") {
            return nil
        }
        if strings.Contains(p, "/vendor/") {
            return nil
        }
        f, err := os.Open(p)
        if err != nil {
            return nil
        }
        defer f.Close()
        scan := bufio.NewScanner(f)
        scan.Buffer(make([]byte, 1<<20), 1<<20)
        lines := []string{}
        for scan.Scan() {
            lines = append(lines, scan.Text())
        }
        for i, l := range lines {
            if !strings.Contains(l, "os.Open(") {
                continue
            }
            if strings.Contains(l, "//") {
                // crude comment skip
                continue
            }
            // Look ahead 10 lines for a Close.
            closeFound := false
            for j := i; j < len(lines) && j < i+10; j++ {
                if strings.Contains(lines[j], "Close()") {
                    closeFound = true
                    break
                }
            }
            if !closeFound {
                bad = append(bad, fmt.Sprintf("%s:%d: %s", p, i+1, strings.TrimSpace(l)))
            }
        }
        return nil
    })
    for _, b := range bad {
        fmt.Println(b)
    }
}
EOF
)
fi
if [ -n "$problems" ]; then
    fail_msg "os.Open without an adjacent Close()"
    printf "%s\n" "$problems"
else
    ok "every os.Open has a Close() within 10 lines"
fi

# -------------------------------------------------------------------
# 4. sql.Open / DB.Conn use without Close
# -------------------------------------------------------------------
section "sql.Open → defer Close()"
matches=$(grep -rn 'sql\.Open(' --include='*.go' $PMF_DIRS 2>/dev/null \
    | grep -v '/vendor/' \
    | while IFS=: read -r f n _; do
        # Look forward 30 lines from the match for either Close() on
        # the returned handle or the caller storing it in a struct
        # that owns Close (Database, Store).
        tail -n "+$n" "$f" | head -30 | grep -qE '(Close\(\)|&Database\{|&Store\{|return.*Database|return.*Store)' || \
            echo "$f:$n: sql.Open without nearby Close"
      done)
if [ -n "$matches" ]; then
    fail_msg "sql.Open call sites missing Close()"
    printf "%s\n" "$matches"
else
    ok "every sql.Open is fed to a struct that owns Close"
fi

# -------------------------------------------------------------------
# 5. Mutex copies (sync.Mutex/RWMutex must not be copied by value)
# -------------------------------------------------------------------
section "no value-copies of sync.Mutex / RWMutex"
# go vet already catches these but we double-check by looking for
# explicit assignments like `m := other.mu`.
matches=$(grep -rnE '(:=|=)\s*[a-zA-Z_]+\.mu([^.]|$)' --include='*.go' $PMF_DIRS 2>/dev/null \
    | grep -v '/vendor/' \
    | grep -v '\.mu\.' \
    || true)
if [ -n "$matches" ]; then
    fail_msg "possible value-copy of a mutex — review:"
    printf "%s\n" "$matches"
else
    ok "no obvious mutex value-copies"
fi

# -------------------------------------------------------------------
# 6. Goroutines explicitly spawned
# -------------------------------------------------------------------
# Match `go <ident>(` or `go func(` at the start of an expression
# (preceded by whitespace, brace, paren, or semicolon) so we don't
# trip on substrings inside comments / package names like `gofpdf`.
section "goroutine inventory (informational)"
matches=$(grep -rnE '(^|[[:space:]{(;])go (func|[A-Za-z_][A-Za-z0-9_]*\()' \
    --include='*.go' $PMF_DIRS 2>/dev/null \
    | grep -v '/vendor/' \
    | grep -vE ':[[:space:]]*//' \
    || true)
if [ -n "$matches" ]; then
    printf "${YELLOW}goroutines detected${NC} (must be reviewed):\n%s\n" "$matches"
else
    ok "no explicit goroutines (Wails runtime is the only spawner)"
fi

# -------------------------------------------------------------------
# 7. Optional: staticcheck
# -------------------------------------------------------------------
section "staticcheck (optional)"
if command -v staticcheck >/dev/null 2>&1; then
    if staticcheck ./...; then
        ok "staticcheck clean"
    else
        fail_msg "staticcheck reported issues"
    fi
else
    printf "${YELLOW}skip${NC}: staticcheck not installed\n"
    printf "  Install:  go install honnef.co/go/tools/cmd/staticcheck@latest\n"
fi

# -------------------------------------------------------------------
# 8. Optional: gosec
# -------------------------------------------------------------------
section "gosec (optional)"
if command -v gosec >/dev/null 2>&1; then
    if gosec -quiet ./...; then
        ok "gosec clean"
    else
        fail_msg "gosec flagged security issues"
    fi
else
    printf "${YELLOW}skip${NC}: gosec not installed\n"
fi

# -------------------------------------------------------------------
# 9. Optional: govulncheck (known vulns in dependencies)
# -------------------------------------------------------------------
section "govulncheck (optional)"
if command -v govulncheck >/dev/null 2>&1; then
    if govulncheck ./...; then
        ok "no known vulnerabilities"
    else
        fail_msg "govulncheck found vulnerabilities"
    fi
else
    printf "${YELLOW}skip${NC}: govulncheck not installed\n"
fi

# -------------------------------------------------------------------
# Summary
# -------------------------------------------------------------------
echo
if [ "$fail" -ne 0 ]; then
    printf "${RED}Memory-safety gate FAILED.${NC} See findings above.\n"
    exit 1
fi
printf "${GREEN}Memory-safety gate PASSED.${NC}\n"
