# Encryption At Rest Completion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Finish ADR-001 by making per-user `.pmforge` project databases SQLCipher-encrypted at rest without breaking login recovery, repair, backup, or headless maintenance flows.

**Architecture:** Keep `system.db` plaintext and store only password hashes plus wrapped DEKs there. Use the already implemented per-user random DEK as the SQLCipher raw key for every project database owned by that user. Preserve `db.InitDB(path)` for intentionally plaintext test/system use, and add explicit encrypted open and migration paths so callers cannot accidentally open encrypted project data without a DEK.

**Tech Stack:** Go 1.26, `database/sql`, `github.com/mutecomm/go-sqlcipher/v4 v4.4.2`, SQLCipher raw keyspecs, Argon2id + AES-256-GCM key wrapping, SQLite `sqlcipher_export`, Wails/Svelte settings UI.

---

## Current State

- Done but still uncommitted in this worktree: ADR-001 spike/design, `internal/crypto/keywrap.go`, `internal/users/dek.go`, recovery-code DEK wrapping, and `App.dek` session memory.
- Verified during planning: `go test -count=1 ./internal/crypto ./internal/users` passes.
- App package compile check is blocked before encryption by missing generated frontend embed files: `cmd/pmforge/main.go:61:12: pattern all:frontend/dist: cannot embed directory frontend/dist: contains no embeddable files`.
- Still pending per ADR-001: migration tool + Settings opt-in, SQLCipher dependency and database open path, secondary openers, encrypted round-trip release gate, and docs/UI warning copy.
- Dirty-worktree warning: this checkout also contains unrelated scheduling/chart/export changes. Stage encryption files by path or hunk only.

## File Map

- Modify: `go.mod`, `go.sum` - replace `github.com/mattn/go-sqlite3` with `github.com/mutecomm/go-sqlcipher/v4 v4.4.2`.
- Create: `internal/sqlitedriver/driver.go` - central driver registration and driver-name constant.
- Modify: `internal/db/sqlite.go` - remove direct mattn import, share pragma setup, add encrypted open entrypoint.
- Create: `internal/db/encryption.go` - SQLCipher DSN/key validation, encrypted-file header check, `cipher_integrity_check`, plaintext-to-encrypted migration.
- Modify: `internal/db/repair.go` - make snapshot integrity and `SwapInSnapshot` accept the live DEK for encrypted projects.
- Modify: `internal/db/backup.go` - confirm archive snapshots preserve encrypted bytes and do not produce plaintext `.pmforge` inside `.pmba`.
- Modify: `cmd/pmforge/main.go` - route project create/open/launchpad/repair through encrypted DB functions with `a.dek`.
- Modify: `internal/cli/parser.go` - add explicit headless password input for encrypted maintenance commands.
- Modify: `frontend/src/wails-window.d.ts` and `frontend/src/lib/components/project/ProjectSettings.svelte` - add migration state/action and warning copy.
- Create: `scripts/validate-encrypted-db.sh` - deterministic release gate for encrypted create, wrong-key failure, migration, backup, and repair.
- Modify: `scripts/check-release.sh`, `Makefile`, `scripts/release-gate-scope-check.sh` - wire the new gate and stop stale docs.
- Modify: `README.md`, `AGENT.md`, `docs/design/ADR-001-database-encryption-at-rest.md`, `session-notes.md` - update implementation status and user-facing recovery semantics.

### Task 0: Preserve And Land Completed Key-Hierarchy Slice

**Files:**
- Modify: `docs/design/ADR-001-database-encryption-at-rest.md:208`
- Modify: `session-notes.md`
- Stage: `internal/crypto/keywrap.go`, `internal/crypto/keywrap_test.go`, `internal/users/dek.go`, `internal/users/dek_test.go`, `internal/users/recovery.go`, `internal/users/recovery_test.go`, `internal/users/store.go`, `cmd/pmforge/main.go` hunks related to `dek`

- [x] **Step 1: Update ADR action item 3 to done**

Change:

```markdown
3. [ ] Implement key hierarchy in `internal/users` + `internal/crypto`
```

to:

```markdown
3. [x] Implement key hierarchy in `internal/users` + `internal/crypto`
```

- [x] **Step 2: Run focused tests**

Run:

```bash
go test -count=1 ./internal/crypto ./internal/users
```

Expected: both packages pass.

- [ ] **Step 3: Stage only encryption key-hierarchy files**

Run:

```bash
git add internal/crypto/keywrap.go internal/crypto/keywrap_test.go \
  internal/users/dek.go internal/users/dek_test.go \
  internal/users/recovery.go internal/users/recovery_test.go \
  internal/users/store.go docs/design/ADR-001-database-encryption-at-rest.md
git add -p cmd/pmforge/main.go session-notes.md
```

Expected: only `App.dek`, login/create/logout/recovery-code hunks are staged from `cmd/pmforge/main.go`; unrelated scheduling/chart hunks stay unstaged.

2026-06-14 note: safe whole-file key-hierarchy files are staged:
`docs/design/ADR-001-database-encryption-at-rest.md`,
`internal/crypto/keywrap.go`, `internal/crypto/keywrap_test.go`,
`internal/users/dek.go`, `internal/users/dek_test.go`,
`internal/users/recovery.go`, `internal/users/recovery_test.go`, and
`internal/users/store.go`. This step remains unchecked because
`cmd/pmforge/main.go` and `session-notes.md` still need deliberate
hunk-level staging to avoid sweeping unrelated dirty changes.

### Task 1: Adopt SQLCipher Driver Behind A Central Registration Package

**Files:**
- Create: `internal/sqlitedriver/driver.go`
- Modify: `internal/db/sqlite.go`
- Modify: `internal/users/store.go`
- Modify: `go.mod`, `go.sum`

- [x] **Step 1: Add the SQLCipher dependency**

Run:

```bash
go get github.com/mutecomm/go-sqlcipher/v4@v4.4.2
go mod tidy
```

Expected: `go.mod` requires `github.com/mutecomm/go-sqlcipher/v4 v4.4.2` and no longer requires `github.com/mattn/go-sqlite3` directly.

- [x] **Step 2: Create central driver registration**

Create `internal/sqlitedriver/driver.go`:

```go
// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package sqlitedriver

import _ "github.com/mutecomm/go-sqlcipher/v4"

const Name = "sqlite3"
```

- [x] **Step 3: Use the central driver from project and system DB packages**

In `internal/db/sqlite.go` and `internal/users/store.go`, remove blank `github.com/mattn/go-sqlite3` imports, import `pmforge/internal/sqlitedriver`, and change `sql.Open("sqlite3", path)` to `sql.Open(sqlitedriver.Name, path)`.

- [x] **Step 4: Verify plaintext compatibility**

Run:

```bash
go test -count=1 ./internal/db ./internal/users
```

Expected: existing plaintext DB tests pass under the SQLCipher-capable driver.

### Task 2: Add Explicit Encrypted Project Database Openers

**Files:**
- Create: `internal/db/encryption.go`
- Create: `internal/db/encryption_test.go`
- Modify: `internal/db/sqlite.go`

- [x] **Step 1: Write failing encrypted-open tests**

Create tests named:

```go
func TestInitEncryptedDBCreatesEncryptedDatabase(t *testing.T)
func TestInitEncryptedDBRejectsWrongKey(t *testing.T)
func TestMigratePlaintextToEncryptedPreservesData(t *testing.T)
func TestOpenEncryptedDBRejectsBadDEKLength(t *testing.T)
```

Assertions: encrypted files do not start with `SQLite format 3\x00`, `PRAGMA cipher_version` is non-empty, `PRAGMA integrity_check` returns `ok`, `PRAGMA cipher_integrity_check` returns zero rows, wrong DEK fails before migration, and migrated row counts match.

- [x] **Step 2: Add encrypted DSN helpers**

Create `internal/db/encryption.go` with:

```go
func InitEncryptedDB(path string, dek []byte) (*Database, error)
func MigratePlaintextToEncrypted(path string, dek []byte) (backupPath string, err error)
func IsEncryptedFile(path string) (bool, error)
```

Use this DSN shape:

```go
func encryptedDSN(path string, dek []byte) (string, error) {
	hexKey, err := crypto.KeyspecHex(dek)
	if err != nil {
		return "", err
	}
	u := url.URL{Scheme: "file", Path: path}
	q := u.Query()
	q.Set("_pragma_key", "x'"+hexKey+"'")
	u.RawQuery = q.Encode()
	return u.String(), nil
}
```

- [x] **Step 3: Share common open logic**

Refactor `internal/db/sqlite.go` so `InitDB(path)` and `InitEncryptedDB(path, dek)` both call an internal `initDBWithDSN(path, dsn string)` that opens, applies WAL/foreign-key/temp-store pragmas, runs `Migrate`, and calls `ensurePrivateSQLiteFiles`.

- [x] **Step 4: Implement plaintext-to-encrypted migration**

Implement this order exactly: reject missing/non-regular/already-encrypted target; open plaintext; require `PRAGMA integrity_check = ok`; attach encrypted temp with `KEY "x'<hex>'"`; run `SELECT sqlcipher_export('encrypted')`; copy `PRAGMA user_version`; detach; open encrypted temp and run integrity checks; close handles; rename original to `<path>.pre-encryption.bak`; rename encrypted temp to original; chmod main/backup/WAL/SHM sidecars to `0600` where present.

- [x] **Step 5: Run db tests**

Run:

```bash
go test -count=1 ./internal/db
```

Expected: all db tests pass, including the four new encryption tests.

### Task 3: Thread DEK Through GUI Project Open/Create Paths

**Files:**
- Modify: `cmd/pmforge/main.go`
- Create: `cmd/pmforge/encryption_project_test.go`

- [x] **Step 1: Add an app helper that requires a session DEK**

Add:

```go
func (a *App) requireDEKLocked() ([]byte, error) {
	if len(a.dek) != crypto.DEKSize {
		return nil, errors.New("database key is locked; sign in again")
	}
	dek := make([]byte, len(a.dek))
	copy(dek, a.dek)
	return dek, nil
}
```

- [x] **Step 2: Use encrypted DBs for newly created projects**

In `CreateProject` and `CreateProjectFromLaunchpad`, read the locked DEK and replace `db.InitDB(path)` with `db.InitEncryptedDB(path, dek)`.

- [x] **Step 3: Use encrypted DBs for project open**

In `OpenProject`, call `a.requireDEKLocked()` before closing the old DB, then replace `db.InitDB(path)` with `db.InitEncryptedDB(path, dek)`. If encrypted open fails and `db.IsEncryptedFile(path)` is false, return a typed plaintext-migration-required error for the UI.

- [x] **Step 4: Test app-level create/open**

Add tests that create an account, create a project, close it, confirm the file header is encrypted, log out, log back in, and reopen the project successfully. Add a second test that a different account cannot open the file because its DEK differs.

### Task 4: Build The Plaintext Migration And Settings Opt-In Flow

**Files:**
- Modify: `cmd/pmforge/main.go`
- Modify: `frontend/src/wails-window.d.ts`
- Modify: `frontend/src/lib/components/project/ProjectSettings.svelte`
- Create: `cmd/pmforge/encryption_migration_test.go`

- [x] **Step 1: Add Wails methods**

Add:

```go
func (a *App) IsProjectEncrypted(path string) (bool, error)
func (a *App) EncryptProjectAtRest(path string) (string, error)
```

`EncryptProjectAtRest` must require a signed-in user and session DEK, call `db.MigratePlaintextToEncrypted(path, dek)`, and return the `.pre-encryption.bak` path.

- [x] **Step 2: Force recovery-code reissue before migration**

Before migration, check active recovery codes for the user. If any code has an empty `wrapped_dek`, return this error text:

```text
Reissue recovery codes before enabling database encryption. Old recovery codes cannot preserve encrypted projects during password reset.
```

- [x] **Step 3: Add frontend opt-in UI**

In `ProjectSettings.svelte`, show state `Plaintext` or `Encrypted`; when plaintext, show a button labeled `Encrypt database`; on success, display the returned backup path; if recovery-code reissue is required, route to the existing recovery-code issue flow.

- [x] **Step 4: Migration tests**

Test that migration preserves project metadata and at least one chart/document row, creates `.pre-encryption.bak`, rejects plaintext `db.InitDB(path)` after migration, and opens with `db.InitEncryptedDB(path, dek)`.

### Task 5: Cover Repair, Self-Heal, Backup, And Headless CLI

**Files:**
- Modify: `internal/db/repair.go`
- Modify: `internal/db/backup.go`
- Modify: `internal/cli/parser.go`
- Modify: `cmd/pmforge/main.go`
- Create: `internal/db/repair_encryption_test.go`
- Create: `internal/db/backup_encryption_test.go`
- Create: `cmd/pmforge/headless_encryption_test.go`

- [x] **Step 1: Make repair key-aware**

Add:

```go
func (db *Database) SwapInEncryptedSnapshot(livePath string, dek []byte) (*Database, error)
func CheckEncryptedSnapshotIntegrity(path string, dek []byte) error
```

Keep the existing plaintext functions for tests and legacy snapshots.

- [x] **Step 2: Ensure backup archives encrypted bytes**

Add a test that creates an encrypted DB, calls `CreateArchivalBundle`, extracts `project.pmforge`, and asserts its header is not `SQLite format 3\x00`.

- [x] **Step 3: Add headless password input**

Extend CLI parsing with:

```text
--username <name>
--password-env <ENV_VAR_NAME>
```

For `--check`, `--repair`, `--vacuum`, and `--export-audit`, if the file is encrypted, load `system.db`, authenticate the user, unlock the DEK, and open via `db.InitEncryptedDB`.

### Task 6: Add Release Gate And Publish-Safety Checks

**Files:**
- Create: `scripts/validate-encrypted-db.sh`
- Modify: `Makefile`
- Modify: `scripts/check-release.sh`
- Modify: `scripts/release-gate-scope-check.sh`

- [x] **Step 1: Add deterministic encrypted DB gate**

Create `scripts/validate-encrypted-db.sh` to create an encrypted project database, confirm encrypted file header, confirm wrong-key failure, migrate a plaintext fixture, run integrity plus `cipher_integrity_check`, create a `.pmba`, and confirm `project.pmforge` inside remains encrypted.

- [x] **Step 2: Wire Makefile and release gate**

Add:

```make
check-encrypted-db: ## Validate SQLCipher encrypted project DB create/open/migration/backup.
	@bash scripts/validate-encrypted-db.sh
```

Add `check-encrypted-db` to `.PHONY`, help list, and `scripts/check-release.sh` before PDF gates.

- [x] **Step 3: Update release-scope guard**

Fail if README still says native database encryption is deferred, or if `go.mod` does not contain `github.com/mutecomm/go-sqlcipher/v4`.

### Task 7: Final Documentation And Full Verification

**Files:**
- Modify: `README.md`
- Modify: `AGENT.md`
- Modify: `docs/design/ADR-001-database-encryption-at-rest.md`
- Modify: `session-notes.md`

- [x] **Step 1: Update docs**

State that per-user `.pmforge` project DBs are encrypted with SQLCipher after opt-in or for newly created V3 projects; `system.db` remains plaintext by design and stores only password hashes and wrapped DEKs; recovery codes must be reissued after enabling encryption; losing password and all valid wrapped recovery codes makes encrypted projects unrecoverable by design; `.pmba` backups contain encrypted `project.pmforge` bytes.

- [x] **Step 2: Full verification**

Run:

```bash
npm --prefix frontend run build
mkdir -p cmd/pmforge/frontend
rm -rf cmd/pmforge/frontend/dist
cp -R frontend/dist cmd/pmforge/frontend/dist
go test -count=1 ./cmd/... ./internal/...
go test -count=1 -race ./internal/crypto ./internal/users ./internal/db
make check-encrypted-db
make license-check
make release-scope
make check-release
```

Expected: every command passes.

## Self-Review

- Spec coverage: ADR steps 4-7 are covered by Tasks 2, 4, 5, 6, and 7. The completed step 3 is preserved in Task 0. SQLCipher dependency acceptance is covered in Task 1.
- Risk coverage: recovery reset data survival, legacy recovery-code migration, wrong-key failure, encrypted backups, self-heal, and headless maintenance are all explicitly tested.
- Known gap: Windows packaging remains deferred until a Windows target exists, matching ADR-001 Appendix A.
