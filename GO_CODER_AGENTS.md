<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GFDL-1.3-or-later
-->

# Universal AI Agent Instructions for Go Repositories

This `AGENTS.md` file defines the expected behavior, constraints, and coding standards for any AI agent operating within this Go codebase. These rules are synthesized from Effective Go, Google's Go Style Guide, and standard cryptographic practices to ensure high-quality, idiomatic, and secure Go code.

## 1. Agent Execution & Environment

- **Universal Sandbox Awareness:** You operate in an isolated environment. Assume external network access is restricted or disabled unless explicitly informed otherwise. Do not attempt to bypass sandbox rules or modify environment variables related to your execution container.
- **Command Execution:** When running commands (e.g., `go build`, `go test`, `go mod tidy`), be patient. Dependency resolution and test execution take time; do not attempt to kill processes prematurely using their PID.
- **Formatting & Imports:** Always run `goimports` (or `gofmt`) automatically after modifying Go code. Never ask for approval to format code. Leave the imports grouped standardly: standard library first, then third-party, then first-party packages.

## 2. Package Architecture & Code Organization

- **Avoid Package Bloat:** Similar to avoiding a bloated "core" module, resist the urge to dump everything into a shared `common` or `util` package. Group code by domain or feature. If a file or package exceeds manageable limits, extract functionality into cohesive, narrowly focused packages.
- **Internal Packages:** Use `internal/` directories to hide code that should not be imported by other repositories or modules. Expose only what is absolutely necessary.
- **Synchronous APIs:** APIs should be synchronous by default. Leave concurrency (goroutines) to the caller. Do not start background goroutines in library functions unless explicitly documented and properly managed with a lifecycle (e.g., via `context.Context`).

## 3. Idiomatic Go Style & Conventions

- **Naming:** Use `MixedCaps` or `mixedCaps` for multi-word names, not underscores. Package names must be short, concise, and entirely lowercase (e.g., `http` rather than `http_client`).
- **Guard Clauses (Line of Sight):** Avoid deep nesting of `if` statements. Handle errors and edge cases early and return. Keep the "happy path" aligned to the left edge of the screen.
- **Boolean Traps:** Avoid ambiguous `bool` parameters in function signatures (e.g., `foo(true)`). Prefer custom boolean types or structural options. If passing a boolean literal is unavoidable, use an inline argument comment matching the parameter name exactly: `foo(/* forceReload= */ true)`.
- **Initialization:** Avoid package-level state and `init()` functions where possible. Prefer explicit initialization functions (e.g., `NewClient()`) that return an initialized struct and an error.
- **Receivers:** Be consistent with pointer vs. value receivers. If a struct is large or requires mutation, use a pointer receiver. Do not mix receiver types for the same struct.

## 4. Error Handling

- **Explicit Returns:** Return `error` as the last return value. Do not discard errors using `_`.
- **Error Context:** Wrap errors to add context using `fmt.Errorf("doing task: %w", err)`. This aids debugging up the call stack.
- **No Panics:** Never use `panic` for normal error handling or control flow. Reserve panics strictly for unrecoverable initialization errors (e.g., `regexp.MustCompile`).
- **Inspection:** Use `errors.Is` for checking specific error values and `errors.As` for extracting specific error types.

## 5. Testing

- **Table-Driven Tests:** Use the table-driven testing pattern (a slice of anonymous structs) for functions with multiple scenarios.
- **Assertions:** Use `github.com/google/go-cmp/cmp` for deep equality checks of complex structs (e.g., `cmp.Diff(want, got)`). Never use `reflect.DeepEqual`.
- **Test Cleanup:** Use `t.Cleanup(func())` for tearing down resources rather than `defer`, as it ensures cleanup even if a test calls `t.Fatal()`.
- **Golden Files / Snapshots:** For UI output, large JSON payloads, or generated data, use standard Go golden files in a `testdata/` directory.

## 6. Cryptography & Security

- **Standard Library Exclusivity:** Rely solely on `crypto/*` standard libraries (e.g., `crypto/aes`, `crypto/rand`, `crypto/sha256`) or `golang.org/x/crypto/`. Do not implement custom cryptographic algorithms.
- **Randomness:** Always use `crypto/rand` for cryptographically secure random numbers (keys, nonces, salts). Never use `math/rand` for security contexts. For **recoverable** code paths (ID generation, password salts, recovery codes — anything that returns an `error`), use `io.ReadFull(rand.Reader, buf)` rather than `rand.Read(buf)` directly. In Go 1.26, `rand.Read` fatals the process if the CSPRNG fails; `io.ReadFull` returns a contextual error instead. Signing APIs like `rsa.SignPKCS1v15(rand.Reader, ...)` already return errors on entropy failure and do not need this workaround.
- **Symmetric Encryption:** Use Authenticated Encryption with Associated Data (AEAD) via `crypto/cipher` with `AES-GCM`. Always generate a unique nonce using `crypto/rand` for every encryption.
- **Hashing:** Use SHA-256 or higher for data integrity. For password hashing, rely strictly on `golang.org/x/crypto/bcrypt` or `argon2`. Never use MD5 or SHA-1.

## 7. App-Server & API Structs

- **JSON Serialization:** This codebase uses `snake_case` on the wire (e.g. `json:"project_id"`, `json:"created_at"`). Use `json:"snake_case"` tags consistently. Do not use `camelCase` for new struct fields — the Wails TypeScript surface mirrors the Go tags directly and the existing frontend code references snake_case field names throughout.
- **OmitEmpty:** Use `omitempty` carefully. Remember that `false` and `0` are omitted. If you need to distinguish between "unset" and a zero-value, use pointers (e.g., `*bool` or `*int`).
- **Timestamps:** Use `time.Time` for standard RFC3339 serialization, or `int64` for UNIX epoch seconds. Name timestamp fields explicitly (e.g., `createdAt`, `updatedAt`).
