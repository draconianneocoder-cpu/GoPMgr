// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package sqlitedriver

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const compatibilityKey = "x'4242424242424242424242424242424242424242424242424242424242424242'"

func TestEncryptedFilesInteroperateWithUnmodifiedDriver(t *testing.T) {
	dir := t.TempDir()
	pristineDir := filepath.Join(dir, "pristine")
	copyPristineSQLCipherModule(t, pristineDir)
	// A separate module/process prevents duplicate sqlite3 registrations. Its
	// local replacement is the hash-verified exact v4.4.2 runtime, not the
	// patched module selected by the parent repository.
	mod := "module compatibility\n\ngo 1.27.2\n\nrequire github.com/mutecomm/go-sqlcipher/v4 v4.4.2\n\nreplace github.com/mutecomm/go-sqlcipher/v4 => ./pristine\n"
	for name, data := range map[string][]byte{"go.mod": []byte(mod), "main.go": []byte(legacyCompatibilityHelper)} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	env := []string{}
	for _, value := range os.Environ() {
		if strings.HasPrefix(value, "GOWORK=") || strings.HasPrefix(value, "GOFLAGS=") {
			continue
		}
		env = append(env, value)
	}
	env = append(env, "GOWORK=off", "GOFLAGS=")
	legacy := func(args ...string) string {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir, cmd.Env = dir, env
		out, err := cmd.CombinedOutput()
		if err != nil {
			if ctx.Err() != nil {
				t.Fatalf("legacy helper timed out: %v\n%s", ctx.Err(), out)
			}
			t.Fatalf("legacy helper: %v\n%s", err, out)
		}
		return string(out)
	}
	var resolved struct {
		Version, Dir string
		Replace      *struct{ Dir string }
	}
	if err := json.Unmarshal([]byte(legacy("list", "-mod=readonly", "-m", "-json", "github.com/mutecomm/go-sqlcipher/v4")), &resolved); err != nil {
		t.Fatal(err)
	}
	resolvedDir, err := filepath.Abs(resolved.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Version != "v4.4.2" || resolved.Replace == nil || !sameFile(t, resolvedDir, pristineDir) {
		t.Fatalf("legacy module is not pristine: %+v", resolved)
	}
	if !sameFile(t, resolved.Replace.Dir, pristineDir) {
		t.Fatalf("legacy replacement = %s, want %s", resolved.Replace.Dir, pristineDir)
	}
	legacyCore, err := os.ReadFile(filepath.Join(resolvedDir, "sqlite3.c"))
	if err != nil {
		t.Fatal(err)
	}
	sumCore := sha256.Sum256(legacyCore)
	if got := hex.EncodeToString(sumCore[:]); got != pristineSQLCipherAmalgamationSHA256 {
		t.Fatalf("legacy amalgamation hash = %s, want %s", got, pristineSQLCipherAmalgamationSHA256)
	}
	binary := filepath.Join(dir, "legacy")
	legacy("build", "-mod=readonly", "-o", binary, ".")
	runOld := func(mode, path string) { t.Helper(); runNativeCommand(t, dir, binary, mode, path) }
	for _, oldCreates := range []bool{true, false} {
		path := filepath.Join(dir, "new-first.db")
		if oldCreates {
			path = filepath.Join(dir, "old-first.db")
			runOld("create", path)
		}
		conn, err := sql.Open(Name, compatibilityDSN(path, compatibilityKey))
		if err != nil {
			t.Fatal(err)
		}
		openedInitial := conn
		t.Cleanup(func() { _ = openedInitial.Close() })
		if !oldCreates {
			if _, err := conn.Exec("CREATE TABLE sample(id INTEGER PRIMARY KEY,v TEXT); INSERT INTO sample VALUES(1,'old café')"); err != nil {
				t.Fatal(err)
			}
		} else {
			checkCompatibilityValue(t, conn, 1, "old café")
		}
		if oldCreates {
			if _, err := conn.Exec("INSERT INTO sample VALUES(2,'new 東京')"); err != nil {
				t.Fatal(err)
			}
		}
		if err := conn.Close(); err != nil {
			t.Fatal(err)
		}
		if !oldCreates {
			runOld("append", path)
		}
		runOld("check", path)
		runOld("wrong-key", path)
		conn, err = sql.Open(Name, compatibilityDSN(path, compatibilityKey))
		if err != nil {
			t.Fatal(err)
		}
		reopened := conn
		t.Cleanup(func() { _ = reopened.Close() })
		checkCompatibilityValue(t, conn, 1, "old café")
		checkCompatibilityValue(t, conn, 2, "new 東京")
		var count int
		if err := conn.QueryRow("SELECT count(*) FROM sample").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 2 {
			t.Fatalf("interchange row count = %d, want 2", count)
		}
		rows, err := conn.Query("PRAGMA cipher_integrity_check")
		if err != nil {
			t.Fatal(err)
		}
		if rows.Next() {
			_ = rows.Close()
			t.Fatal("cipher_integrity_check reported a failure")
		}
		if err := rows.Close(); err != nil {
			t.Fatal(err)
		}
		if err := conn.Close(); err != nil {
			t.Fatal(err)
		}
		wrong, err := sql.Open(Name, compatibilityDSN(path, "wrong-key"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = wrong.Close() })
		err = wrong.QueryRow("SELECT count(*) FROM sample").Scan(&count)
		if closeErr := wrong.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		if err == nil {
			t.Fatal("patched driver accepted wrong key")
		}
		header, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(header) < 16 || string(header[:16]) == "SQLite format 3\x00" {
			t.Fatal("interchange file not encrypted")
		}
	}
}

func sameFile(t *testing.T, first, second string) bool {
	t.Helper()
	firstInfo, err := os.Stat(first)
	if err != nil {
		t.Fatal(err)
	}
	secondInfo, err := os.Stat(second)
	if err != nil {
		t.Fatal(err)
	}
	return os.SameFile(firstInfo, secondInfo)
}

func copyPristineSQLCipherModule(t *testing.T, destination string) {
	t.Helper()
	if err := os.Mkdir(destination, 0700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join("..", "..", "third_party", "go-sqlcipher")
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatal(err)
	}
	pristine := sha256.New()
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "GOPMGR.md" || entry.Name() == "THIRD_PARTY_NOTICES.txt" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(source, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if entry.Name() == "sqlite3.c" {
			data = []byte(restorePristineWALCheckpoint(t, string(data)))
		}
		if entry.Name() == "sqlite3.go" {
			data = []byte(restorePristineCommitCleanup(t, string(data)))
		}
		writeInventoryLine(pristine, entry.Name(), data)
		if err := os.WriteFile(filepath.Join(destination, entry.Name()), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if got := hex.EncodeToString(pristine.Sum(nil)); got != pristineSQLCipherRuntimeTreeSHA256 {
		t.Fatalf("compatibility runtime tree = %s, want %s", got, pristineSQLCipherRuntimeTreeSHA256)
	}
}

func compatibilityDSN(path, key string) string {
	return path + "?_pragma_key=" + url.QueryEscape(key) + "&_journal_mode=WAL&_pragma_cipher_page_size=4096"
}
func checkCompatibilityValue(t *testing.T, conn *sql.DB, id int, want string) {
	t.Helper()
	var got string
	if err := conn.QueryRow("SELECT v FROM sample WHERE id=?", id).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("row %d=%q, want %q", id, got, want)
	}
}

const legacyCompatibilityHelper = `package main
import("database/sql";"fmt";"net/url";"os";_ "github.com/mutecomm/go-sqlcipher/v4")
func main(){
 mode,path:=os.Args[1],os.Args[2]
 key:="x'4242424242424242424242424242424242424242424242424242424242424242'"
 if mode=="wrong-key"{key="wrong-key"}
 db,err:=sql.Open("sqlite3",path+"?_pragma_key="+url.QueryEscape(key)+"&_journal_mode=WAL&_pragma_cipher_page_size=4096");must(err)
 defer db.Close()
 switch mode {
 case "create": _,err=db.Exec("CREATE TABLE sample(id INTEGER PRIMARY KEY,v TEXT); INSERT INTO sample VALUES(1,'old café')");must(err)
 case "append": _,err=db.Exec("INSERT INTO sample VALUES(2,'new 東京')");must(err)
 case "check":
 for id,want:=range []string{"old café","new 東京"}{var got string;must(db.QueryRow("SELECT v FROM sample WHERE id=?",id+1).Scan(&got));if got!=want{panic("interchange content mismatch")}}
 var count int;must(db.QueryRow("SELECT count(*) FROM sample").Scan(&count));if count!=2{panic("interchange row count mismatch")}
 var integrity string;must(db.QueryRow("PRAGMA integrity_check").Scan(&integrity));if integrity!="ok"{panic(integrity)}
 case "wrong-key":var n int;if db.QueryRow("SELECT count(*) FROM sample").Scan(&n)==nil{panic("legacy accepted wrong key")}
 default:panic("invalid mode")
 }
 must(db.Close())
 fmt.Println("compatibility passed")
}
func must(err error){if err!=nil{panic(err)}}
`
