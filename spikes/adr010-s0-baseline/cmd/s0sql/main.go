// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Command s0sql records what the project database engine actually
// supports. It opens a fresh encrypted project database through
// db.InitEncryptedDB (the same path the app uses), reads the effective
// pragmas and compile options, and tries each SQL feature that ADR-011
// design work might want. The database lives in a temporary folder that
// is deleted afterwards. Nothing here touches a real project.
package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"gopmgr/internal/db"
)

type record struct {
	ID         string         `json:"id"`
	Status     string         `json:"status"`
	Metrics    map[string]any `json:"metrics"`
	Notes      []string       `json:"notes,omitempty"`
	Platform   string         `json:"platform"`
	SourceHead string         `json:"source_head"`
	StartedAt  string         `json:"started_at"`
}

// probe is one feature check: setup statements, then the statement
// whose success means the feature works.
type probe struct {
	name  string
	since string // SQLite release that introduced it, for the reader
	setup []string
	test  string
	query bool // test returns rows
}

var probes = []probe{
	{name: "upsert", since: "3.24.0", setup: []string{"CREATE TABLE p_up(a INTEGER PRIMARY KEY, b)"}, test: "INSERT INTO p_up VALUES(1,1) ON CONFLICT(a) DO UPDATE SET b = excluded.b"},
	{name: "rename_column", since: "3.25.0", setup: []string{"CREATE TABLE p_rc(a, b)"}, test: "ALTER TABLE p_rc RENAME COLUMN b TO c"},
	{name: "window_functions", since: "3.25.0", test: "SELECT row_number() OVER () FROM (SELECT 1)", query: true},
	{name: "generated_columns", since: "3.31.0", test: "CREATE TABLE p_gc(a, b AS (a * 2))"},
	{name: "returning", since: "3.35.0", setup: []string{"CREATE TABLE p_ret(a)"}, test: "INSERT INTO p_ret VALUES(1) RETURNING a", query: true},
	{name: "drop_column", since: "3.35.0", setup: []string{"CREATE TABLE p_dc(a, b)"}, test: "ALTER TABLE p_dc DROP COLUMN b"},
	{name: "math_functions", since: "3.35.0 (compile option)", test: "SELECT sqrt(4.0)", query: true},
	{name: "strict_tables", since: "3.37.0", test: "CREATE TABLE p_st(a INTEGER) STRICT"},
	{name: "json1", since: "3.9.0 (compile option before 3.38.0)", test: "SELECT json_extract('{\"a\":1}', '$.a')", query: true},
	{name: "json_arrow_operators", since: "3.38.0", test: "SELECT '{\"a\":1}' ->> '$.a'", query: true},
	{name: "fts4", since: "compile option", test: "CREATE VIRTUAL TABLE p_fts4 USING fts4(x)"},
	{name: "fts5", since: "compile option", test: "CREATE VIRTUAL TABLE p_fts5 USING fts5(x)"},
	{name: "rtree", since: "compile option", test: "CREATE VIRTUAL TABLE p_rt USING rtree(id, x0, x1)"},
	{name: "without_rowid", since: "3.8.2", test: "CREATE TABLE p_wr(a PRIMARY KEY, b) WITHOUT ROWID"},
	{name: "partial_index", since: "3.8.0", setup: []string{"CREATE TABLE p_pi(a, b)"}, test: "CREATE INDEX p_pi_a ON p_pi(a) WHERE b IS NOT NULL"},
	{name: "deferred_foreign_keys", since: "3.6.19", setup: []string{"CREATE TABLE p_fp(id INTEGER PRIMARY KEY)"}, test: "CREATE TABLE p_fc(pid REFERENCES p_fp(id) DEFERRABLE INITIALLY DEFERRED)"},
	{name: "cipher_integrity_check", since: "SQLCipher 4.0.0", test: "PRAGMA cipher_integrity_check", query: true},
}

var pragmas = []string{
	"journal_mode", "synchronous", "foreign_keys", "busy_timeout", "page_size",
	"cipher_page_size", "kdf_iter", "cipher_hmac_algorithm", "cipher_kdf_algorithm",
	"cipher_provider", "cipher_provider_version", "user_version",
}

func main() {
	out := flag.String("out", "s0-sql.jsonl", "evidence file to append to")
	head := flag.String("head", "unknown", "source commit the run was built from")
	flag.Parse()
	r := run()
	r.SourceHead = *head
	if err := appendRecord(*out, r); err != nil {
		fmt.Fprintln(os.Stderr, "s0sql:", err)
		os.Exit(1)
	}
	fmt.Printf("%-6s %s\n", r.Status, r.ID)
}

func run() *record {
	r := &record{
		ID:        runtime.GOOS + "/sql-features",
		Status:    "PASS",
		Metrics:   map[string]any{},
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
		StartedAt: time.Now().UTC().Format(time.RFC3339),
	}
	dir, err := os.MkdirTemp("", "gopmgr-s0sql-")
	if err != nil {
		return r.fail(err)
	}
	defer os.RemoveAll(dir)
	dek := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return r.fail(err)
	}
	d, err := db.InitEncryptedDB(filepath.Join(dir, "probe.gopmgr"), dek)
	if err != nil {
		return r.fail(fmt.Errorf("open: %w", err))
	}
	defer d.Close()
	conn := d.Conn

	var version string
	if err := conn.QueryRow("SELECT sqlite_version()").Scan(&version); err != nil {
		return r.fail(err)
	}
	r.Metrics["sqlite_version"] = version
	r.Metrics["cipher_version"] = scalar(conn, "PRAGMA cipher_version")

	eff := map[string]string{}
	for _, p := range pragmas {
		eff[p] = scalar(conn, "PRAGMA "+p)
	}
	r.Metrics["effective_pragmas"] = eff
	r.Metrics["compile_options"] = column(conn, "PRAGMA compile_options")

	feats := map[string]map[string]any{}
	for _, p := range probes {
		feats[p.name] = runProbe(conn, p)
	}
	r.Metrics["features"] = feats
	return r
}

func runProbe(conn *sql.DB, p probe) map[string]any {
	res := map[string]any{"since": p.since}
	for _, s := range p.setup {
		if _, err := conn.Exec(s); err != nil {
			res["supported"] = false
			res["error"] = "setup: " + err.Error()
			return res
		}
	}
	var err error
	if p.query {
		var rows *sql.Rows
		rows, err = conn.Query(p.test)
		if err == nil {
			for rows.Next() {
			}
			err = rows.Err()
			_ = rows.Close()
		}
	} else {
		_, err = conn.Exec(p.test)
	}
	res["supported"] = err == nil
	if err != nil {
		res["error"] = err.Error()
	}
	return res
}

// scalar reads one value as text, or the error text when it fails.
func scalar(conn *sql.DB, q string) string {
	var v sql.NullString
	if err := conn.QueryRow(q).Scan(&v); err != nil {
		return "error: " + err.Error()
	}
	if !v.Valid {
		return ""
	}
	return v.String
}

func column(conn *sql.DB, q string) []string {
	rows, err := conn.Query(q)
	if err != nil {
		return []string{"error: " + err.Error()}
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if rows.Scan(&s) == nil {
			out = append(out, s)
		}
	}
	return out
}

func (r *record) fail(err error) *record {
	r.Status = "FAIL"
	r.Notes = append(r.Notes, err.Error())
	return r
}

func appendRecord(path string, r *record) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}
