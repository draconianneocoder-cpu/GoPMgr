// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestPortableArchiveSegment(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"backslash traversal", `..\..\evil.exe`, ".._.._evil.exe"},
		{"colon (drive or alternate stream)", "C:evil.txt", "C_evil.txt"},
		{"every reserved character", `a<b>c:d"e/f\g|h?i*j.pdf`, "a_b_c_d_e_f_g_h_i_j.pdf"},
		{"control characters", "a\x00b\x1fc.txt", "a_b_c.txt"},
		{"invalid UTF-8", "\xffreport.pdf", "_report.pdf"},
		{"reserved device name", "CON", "_CON"},
		{"reserved name with extension", "con.txt", "_con.txt"},
		{"reserved name with double extension", "NUL.tar.gz", "_NUL.tar.gz"},
		{"reserved superscript port", "LPT¹.log", "_LPT¹.log"},
		{"reserved name before a space", "AUX .txt", "_AUX .txt"},
		{"not reserved: longer port number", "COM10.txt", "COM10.txt"},
		{"not reserved: name merely starts with one", "CONTRACT.pdf", "CONTRACT.pdf"},
		{"trailing dots and spaces", "report. . ", "report"},
		{"surrounding spaces", "  quote.pdf  ", "quote.pdf"},
		{"dot-dot", "..", "attachment"},
		{"single dot", ".", "attachment"},
		{"empty", "", "attachment"},
		{"leading dot is allowed", ".env", ".env"},
		{"Unicode is preserved", "工程 見積もり.pdf", "工程 見積もり.pdf"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PortableArchiveSegment(tc.in); got != tc.want {
				t.Fatalf("PortableArchiveSegment(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// A long name is cut on a rune boundary within the budget and keeps its
// extension, so the file still opens with the right application.
func TestPortableArchiveSegmentBoundsLongNamesKeepingExtension(t *testing.T) {
	// 305 bytes. The leading "a" puts the byte limit inside a 3-byte rune, so
	// a cut that ignored rune boundaries would leave invalid UTF-8.
	got := PortableArchiveSegment("a" + strings.Repeat("工", 100) + ".pdf")
	if len(got) > portableSegmentBudget || !utf8.ValidString(got) || !strings.HasSuffix(got, ".pdf") {
		t.Fatalf("PortableArchiveSegment(long) = %q (%d bytes), want valid UTF-8 ending .pdf within %d bytes", got, len(got), portableSegmentBudget)
	}
	// An over-long "extension" is treated as part of the name, not kept whole.
	got = PortableArchiveSegment("a." + strings.Repeat("x", 300))
	if len(got) > portableSegmentBudget {
		t.Fatalf("PortableArchiveSegment(long extension) is %d bytes, want at most %d", len(got), portableSegmentBudget)
	}
}

func TestWriteAttachmentsZIPRefusesUnsafeEntryNamesBeforeFetching(t *testing.T) {
	cases := map[string][]string{
		"parent segment":              {"attachments/../evil.txt"},
		"leading parent segment":      {"../evil.txt"},
		"absolute path":               {"/etc/evil.txt"},
		"empty segment":               {"attachments//evil.txt"},
		"trailing slash":              {"attachments/"},
		"backslash":                   {`attachments/..\evil.txt`},
		"colon":                       {"attachments/C:evil.txt"},
		"control character":           {"attachments/a\x01.txt"},
		"invalid UTF-8":               {"attachments/\xff.txt"},
		"reserved device name":        {"attachments/CON.txt"},
		"trailing dot":                {"attachments/report."},
		"trailing space":              {"attachments/report "},
		"over-long segment":           {"attachments/" + strings.Repeat("a", maxSegmentBytes+1)},
		"manifest name":               {"manifest.json"},
		"manifest name, other case":   {"MANIFEST.JSON"},
		"duplicate differing by case": {"attachments/Invoice.pdf", "attachments/invoice.pdf"},
	}
	for name, entryNames := range cases {
		t.Run(name, func(t *testing.T) {
			fetched := false
			sources := make([]AttachmentZIPSource, 0, len(entryNames))
			for _, entryName := range entryNames {
				data := []byte("bytes")
				sources = append(sources, AttachmentZIPSource{
					ZipEntryName: entryName,
					Manifest:     attachmentManifest(entryName, "original.txt", data),
					Fetch: func() ([]byte, error) {
						fetched = true
						return data, nil
					},
				})
			}
			var buf bytes.Buffer
			err := WriteAttachmentsZIP(&buf, sources)
			if !errors.Is(err, ErrUnsafeArchiveName) {
				t.Fatalf("WriteAttachmentsZIP(%q) error = %v, want ErrUnsafeArchiveName", entryNames, err)
			}
			if fetched || buf.Len() != 0 {
				t.Fatalf("WriteAttachmentsZIP(%q) fetched=%v wrote %d bytes, want the archive refused before any work", entryNames, fetched, buf.Len())
			}
		})
	}
}

func TestWriteAttachmentsZIPAcceptsPortableEntryNames(t *testing.T) {
	names := []string{
		"attachments/Invoice 2026.pdf",
		"attachments/工程.pdf",
		"attachments/.env",
		"attachments/" + strings.Repeat("a", maxSegmentBytes),
	}
	sources := make([]AttachmentZIPSource, 0, len(names))
	for _, name := range names {
		data := []byte(name)
		sources = append(sources, AttachmentZIPSource{
			ZipEntryName: name,
			Manifest:     attachmentManifest(name, name, data),
			Fetch:        func() ([]byte, error) { return data, nil },
		})
	}
	var buf bytes.Buffer
	if err := WriteAttachmentsZIP(&buf, sources); err != nil {
		t.Fatalf("WriteAttachmentsZIP(portable names): %v", err)
	}
}

// Whatever a stored filename contains, the portable segment is always a
// valid archive entry under attachments/, both as-is and with the ID suffix
// the app layer appends to disambiguate a collision.
func FuzzPortableArchiveSegment(f *testing.F) {
	for _, seed := range []string{
		"", ".", "..", `..\..\x.exe`, "C:x", "CON", "nul.txt", "LPT³", "a. . ",
		"\xff\xfe", "工程.pdf", strings.Repeat("工", 100) + ".pdf", "a." + strings.Repeat("x", 300),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, in string) {
		got := PortableArchiveSegment(in)
		if len(got) > portableSegmentBudget || !utf8.ValidString(got) {
			t.Fatalf("PortableArchiveSegment(%q) = %q (%d bytes), want valid UTF-8 within %d bytes", in, got, len(got), portableSegmentBudget)
		}
		if err := validateArchiveEntryName("attachments/" + got); err != nil {
			t.Fatalf("PortableArchiveSegment(%q) = %q, rejected: %v", in, got, err)
		}
		if err := validateArchiveEntryName("attachments/" + got + "-attachment_0123abcd-2"); err != nil {
			t.Fatalf("PortableArchiveSegment(%q) = %q, rejected with an ID suffix: %v", in, got, err)
		}
	})
}
