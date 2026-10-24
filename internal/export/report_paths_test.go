// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/rand/v2"
	"strings"
	"testing"
	"unicode/utf8"

	"gopmgr/internal/crypto"
	"gopmgr/internal/kernel"
)

func scheduleTasks() map[string]*kernel.Task {
	tasks := map[string]*kernel.Task{
		"A": {ID: "A", Title: "Design", Duration: 2},
		"B": {ID: "B", Title: "Build", Duration: 3, Precedents: []string{"A"}},
	}
	kernel.CalculateCPM(tasks)
	return tasks
}

// GenerateArchivalReport hands each format to its renderer and refuses
// one it does not know.
func TestGenerateArchivalReportDispatchesEveryFormat(t *testing.T) {
	payload := ReportPayload{Tasks: scheduleTasks()}

	xlsx, err := GenerateArchivalReport(payload, ExportOptions{Format: FormatXLSX, Title: "Demo"})
	if err != nil {
		t.Fatalf("XLSX: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(xlsx), int64(len(xlsx)))
	if err != nil {
		t.Fatalf("XLSX is not a zip package: %v", err)
	}
	hasWorkbook := false
	for _, f := range zr.File {
		hasWorkbook = hasWorkbook || f.Name == "xl/workbook.xml"
	}
	if !hasWorkbook {
		t.Error("XLSX has no xl/workbook.xml")
	}

	mspdi, err := GenerateArchivalReport(payload, ExportOptions{Format: FormatMSPDI, Title: "Demo"})
	if err != nil {
		t.Fatalf("MSPDI: %v", err)
	}
	if !bytes.HasPrefix(mspdi, []byte("<?xml")) || !bytes.Contains(mspdi, []byte("<Title>Demo</Title>")) || !bytes.Contains(mspdi, []byte("<Name>Build</Name>")) {
		t.Errorf("MSPDI export is not the project's XML:\n%s", mspdi)
	}

	if _, err := GenerateArchivalReport(payload, ExportOptions{Format: "bogus"}); err == nil || !strings.Contains(err.Error(), `unknown format "bogus"`) || !strings.Contains(err.Error(), "EXPORT_FORMAT_UNKNOWN") {
		t.Errorf("unknown format error = %v", err)
	}
}

// An encrypted export decrypts, with its password, to exactly the plain
// export, and not with another password.
func TestGenerateArchivalReportEncryptsWithThePassword(t *testing.T) {
	payload := ReportPayload{Tasks: scheduleTasks()}
	plain, err := GenerateArchivalReport(payload, ExportOptions{Format: FormatCSV})
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := GenerateArchivalReport(payload, ExportOptions{Format: FormatCSV, Encrypted: true, Password: "correct horse"})
	if err != nil {
		t.Fatalf("encrypted export: %v", err)
	}
	if bytes.Contains(sealed, []byte("Design")) {
		t.Fatal("encrypted export holds the plain text")
	}
	opened, err := crypto.DecryptBuffer(sealed, "correct horse")
	if err != nil || !bytes.Equal(opened, plain) {
		t.Fatalf("decrypted export = %q, %v; want the plain CSV", opened, err)
	}
	if _, err := crypto.DecryptBuffer(sealed, "wrong"); err == nil {
		t.Error("the export decrypts with the wrong password")
	}
}

// An HTML schedule with no title or tasks still reads sensibly.
func TestHTMLScheduleWithoutTitleOrTasks(t *testing.T) {
	out, err := GenerateArchivalReport(ReportPayload{}, ExportOptions{Format: FormatHTML})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<title>Project Schedule — Schedule</title>", "<h1>Project Schedule</h1>", "No scheduled tasks."} {
		if !bytes.Contains(out, []byte(want)) {
			t.Errorf("HTML is missing %q", want)
		}
	}
}

// Asking for a signature without a way to load the certificate fails
// rather than exporting an unsigned PDF.
func TestSignedPDFNeedsACertificateLoader(t *testing.T) {
	out, err := renderPDFWithSignerLoader(ReportPayload{Tasks: scheduleTasks()}, ExportOptions{Format: FormatPDF, Title: "Demo", DigitalSignature: true}, nil)
	if err == nil || out != nil || !strings.Contains(err.Error(), "certificate loader is required") {
		t.Fatalf("signed PDF without a loader = %d bytes, %v", len(out), err)
	}
}

// truncate counts characters, so a Japanese or Korean title is shortened
// between characters. It used to count bytes, cutting a character in half
// (which then printed as a replacement mark) and shortening CJK titles
// at a third of their allowed length.
func TestTruncateCountsCharacters(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"Design", 10, "Design"},
		{strings.Repeat("a", 10), 10, strings.Repeat("a", 10)},
		{strings.Repeat("a", 11), 10, strings.Repeat("a", 9) + "…"},
		{strings.Repeat("検", 10), 10, strings.Repeat("検", 10)},
		{strings.Repeat("검수", 20), 16, strings.Repeat("검수", 7) + "검…"},
	}
	for _, c := range cases {
		got := truncate(c.in, c.n)
		if got != c.want || !utf8.ValidString(got) || utf8.RuneCountInString(got) > c.n {
			t.Errorf("truncate(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
		}
	}

	// The schedule PDF's task column uses it.
	tasks := map[string]*kernel.Task{"A": {ID: "A", Title: strings.Repeat("検収レビュー", 10), Duration: 1}}
	kernel.CalculateCPM(tasks)
	pdf, err := GenerateArchivalReport(ReportPayload{Tasks: tasks}, ExportOptions{Format: FormatPDF, Title: "Demo"})
	if err != nil {
		t.Fatal(err)
	}
	text := sigmaPDFText(t, pdf)
	if strings.ContainsRune(text, utf8.RuneError) {
		t.Error("schedule PDF prints a replacement mark for a cut character")
	}
	if !strings.Contains(text, strings.Repeat("検収レビュー", 6)+"検収レ…") {
		t.Errorf("schedule PDF does not show the title cut at 40 characters: %q", text)
	}
}

// failingWriter refuses every write, like a full or removed disk.
type failingWriter struct{}

var errDiskFull = errors.New("disk full")

func (failingWriter) Write([]byte) (int, error) { return 0, errDiskFull }

// A destination that cannot be written fails the attachments export at
// the step that first reaches it, never silently. The zip writer buffers
// about 4 KiB and compresses up to 64 KiB before writing, so the attachment
// sizes below move the first real write through each step. The data is
// random so compression cannot shrink it.
func TestWriteAttachmentsZIPReportsAFailingDestination(t *testing.T) {
	random := func(n int) []byte {
		b := make([]byte, n)
		r := rand.New(rand.NewChaCha8([32]byte{1}))
		for i := range b {
			b[i] = byte(r.Uint32())
		}
		return b
	}
	source := func(name string, data []byte) AttachmentZIPSource {
		digest := sha256.Sum256(data)
		row := AttachmentManifestRow{ZipEntryName: name, Filename: name, SizeBytes: int64(len(data)), SHA256: hex.EncodeToString(digest[:])}
		return AttachmentZIPSource{ZipEntryName: name, Manifest: row, Fetch: func() ([]byte, error) { return data, nil }}
	}
	cases := []struct {
		name    string
		sources []AttachmentZIPSource
		step    string
	}{
		{"a large attachment", []AttachmentZIPSource{source("attachments/a.bin", random(70_000))}, `write zip entry "attachments/a.bin"`},
		{"two medium attachments", []AttachmentZIPSource{source("attachments/a.bin", random(5_000)), source("attachments/b.bin", random(5_000))}, `create zip entry "attachments/b.bin"`},
		{"one medium attachment", []AttachmentZIPSource{source("attachments/a.bin", random(5_000))}, "create manifest.json entry"},
		{"one small attachment", []AttachmentZIPSource{source("attachments/a.bin", random(10))}, "finalize attachments zip"},
	}
	for _, c := range cases {
		err := WriteAttachmentsZIP(failingWriter{}, c.sources)
		if !errors.Is(err, errDiskFull) || !strings.HasPrefix(err.Error(), c.step) {
			t.Errorf("%s: error = %v, want %q wrapping the write failure", c.name, err, c.step)
		}
	}
}
