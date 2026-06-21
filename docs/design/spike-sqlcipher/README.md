<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# ADR-001 Phase 0 spike: SQLCipher binding evaluation

Scratch programs behind Appendix A of
`../ADR-001-database-encryption-at-rest.md`. Stored as `.go.txt` so
they stay OUT of the PMForge build and module graph (the spike pulls
a dependency the main module must not acquire before the ADR is
accepted).

## Reproduce on macOS (the remaining Phase 0 platform)

Set REPO to your checkout first (paste-safe; no placeholders):

```sh
REPO="$HOME/Documents/GitLab/PMForge - Go + Typescript"

# Encrypted candidate
mkdir -p /tmp/spike/sqlcipher && cd /tmp/spike/sqlcipher
cp "$REPO/docs/design/spike-sqlcipher/spike_sqlcipher.go.txt" main.go
go mod init spike-sqlcipher
go get github.com/mutecomm/go-sqlcipher/v4@v4.4.2
time go build -o spike . && ./spike && ./spike && ./spike

# Plaintext baseline
mkdir -p /tmp/spike/mattn && cd /tmp/spike/mattn
cp "$REPO/docs/design/spike-sqlcipher/spike_baseline.go.txt" main.go
go mod init spike-mattn
go get github.com/mattn/go-sqlite3@v1.14.22
go build -o spike . && ./spike && ./spike && ./spike
```

Record in Appendix A: `SPIKE PASS` / failures, build wall time,
binary sizes (`ls -la spike`), and the three timing lines from each.
Linux/arm64 numbers are already in the appendix for comparison.
