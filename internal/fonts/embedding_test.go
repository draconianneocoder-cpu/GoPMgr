// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package fonts

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// plexWithFsType returns IBM Plex Sans Regular with its OS/2 fsType set,
// so the tests exercise a real font's table layout.
func plexWithFsType(t *testing.T, fsType uint16) []byte {
	t.Helper()
	b, err := assetsFS.ReadFile("assets/IBMPlexSans-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	b = append([]byte(nil), b...)
	numTables := int(binary.BigEndian.Uint16(b[4:6]))
	for i := 0; i < numTables; i++ {
		rec := 12 + 16*i
		if string(b[rec:rec+4]) == "OS/2" {
			off := binary.BigEndian.Uint32(b[rec+8:])
			binary.BigEndian.PutUint16(b[off+8:], fsType)
			return b
		}
	}
	t.Fatal("IBM Plex Sans has no OS/2 table")
	return nil
}

func TestCheckEmbeddingAllowed(t *testing.T) {
	for _, tc := range []struct {
		name    string
		fsType  uint16
		allowed bool
		reason  string
	}{
		{"installable", 0x0000, true, ""},
		{"preview and print", fsTypePreviewPrint, true, ""},
		{"editable", fsTypeEditable, true, ""},
		{"restricted", fsTypeRestricted, false, "restricted-license"},
		{"restricted with a less restrictive bit", fsTypeRestricted | fsTypeEditable, true, ""},
		{"no subsetting", fsTypeNoSubsetting, false, "may not be subset"},
		{"editable but no subsetting", fsTypeEditable | fsTypeNoSubsetting, false, "may not be subset"},
		{"bitmap only", fsTypeBitmapOnly, false, "only bitmaps"},
	} {
		err := checkEmbeddingAllowed(plexWithFsType(t, tc.fsType))
		if tc.allowed {
			if err != nil {
				t.Errorf("%s (fsType %#04x): %v, want allowed", tc.name, tc.fsType, err)
			}
			continue
		}
		if !errors.Is(err, ErrEmbeddingNotAllowed) || !strings.Contains(err.Error(), tc.reason) {
			t.Errorf("%s (fsType %#04x): err = %v, want ErrEmbeddingNotAllowed mentioning %q", tc.name, tc.fsType, err, tc.reason)
		}
	}
}

func TestCheckEmbeddingAllowed_FontWithoutOS2TableIsAllowed(t *testing.T) {
	for _, b := range [][]byte{fakeTTF(), {0, 1, 0, 0}} {
		if err := checkEmbeddingAllowed(b); err != nil {
			t.Errorf("font without a readable OS/2 table: %v", err)
		}
	}
}

func TestCheckEmbeddingAllowed_TruncatedOS2TableIsAllowed(t *testing.T) {
	b := plexWithFsType(t, fsTypeRestricted)
	numTables := int(binary.BigEndian.Uint16(b[4:6]))
	for i := 0; i < numTables; i++ {
		rec := 12 + 16*i
		if string(b[rec:rec+4]) == "OS/2" {
			binary.BigEndian.PutUint32(b[rec+8:], uint32(len(b))) // offset past EOF
		}
	}
	if _, ok := os2FsType(b); ok {
		t.Fatal("read an fsType from an OS/2 table that starts past the end of the font")
	}
}

func TestBundledFontsAllowEmbedding(t *testing.T) {
	entries, err := assetsFS.ReadDir("assets")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".ttf") {
			continue
		}
		b, err := assetsFS.ReadFile("assets/" + e.Name())
		if err != nil {
			t.Fatal(err)
		}
		if err := checkEmbeddingAllowed(b); err != nil {
			t.Errorf("bundled %s: %v", e.Name(), err)
		}
	}
}

func TestImportFont_RejectsFontsThatForbidEmbedding(t *testing.T) {
	src := filepath.Join(t.TempDir(), "Locked-Regular.ttf")
	if err := os.WriteFile(src, plexWithFsType(t, fsTypeRestricted), 0o600); err != nil {
		t.Fatal(err)
	}
	userDir := t.TempDir()
	_, err := NewManager(userDir).ImportFont(src)
	if !errors.Is(err, ErrEmbeddingNotAllowed) {
		t.Fatalf("ImportFont err = %v, want ErrEmbeddingNotAllowed", err)
	}
	for _, want := range []string{`"Locked-Regular.ttf" can't be used in documents`, "choose a font that allows embedding"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not say %q", err, want)
		}
	}
	if entries, _ := os.ReadDir(userDir); len(entries) != 0 {
		t.Fatalf("a rejected font was copied into the user font directory: %v", entries)
	}
}

func TestImportFont_AcceptsFontsThatAllowEmbedding(t *testing.T) {
	src := filepath.Join(t.TempDir(), "Open-Regular.ttf")
	if err := os.WriteFile(src, plexWithFsType(t, fsTypeEditable), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewManager(t.TempDir()).ImportFont(src); err != nil {
		t.Fatalf("ImportFont: %v", err)
	}
}

// TestRegisterAs_SkipsUserFontsThatForbidEmbedding covers a restricted font
// that reached the user directory before ImportFont checked permissions.
func TestRegisterAs_SkipsUserFontsThatForbidEmbedding(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Locked-Regular.ttf"), plexWithFsType(t, fsTypeRestricted), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Locked-Bold.ttf"), plexWithFsType(t, 0), 0o600); err != nil {
		t.Fatal(err)
	}
	reg := &recordingRegistrar{}
	if err := NewManager(dir).RegisterAs(reg, "Locked", "Helvetica"); err != nil {
		t.Fatalf("RegisterAs: %v", err)
	}
	if len(reg.calls) != 1 || reg.calls[0].style != "B" {
		t.Fatalf("registered %+v, want only the Bold face, whose license allows embedding", reg.calls)
	}

	onlyLocked := t.TempDir()
	if err := os.WriteFile(filepath.Join(onlyLocked, "Locked-Regular.ttf"), plexWithFsType(t, fsTypeRestricted), 0o600); err != nil {
		t.Fatal(err)
	}
	err := NewManager(onlyLocked).RegisterAs(&recordingRegistrar{}, "Locked", "Helvetica")
	if err == nil || !strings.Contains(err.Error(), "allow embedding") {
		t.Fatalf("RegisterAs err = %v, want an error that names embedding", err)
	}
}

// os2Font builds the smallest font layout os2FsType reads: a table
// directory holding one OS/2 table with the given fsType.
func os2Font(fsType uint16) []byte {
	b := make([]byte, 12+16+10)
	binary.BigEndian.PutUint32(b[0:], 0x00010000)
	binary.BigEndian.PutUint16(b[4:], 1)
	copy(b[12:], "OS/2")
	binary.BigEndian.PutUint32(b[12+8:], 12+16)
	binary.BigEndian.PutUint32(b[12+12:], 10)
	binary.BigEndian.PutUint16(b[12+16+8:], fsType)
	return b
}

// checkEmbeddingAllowed reads the fsType of a font a user imports. For any
// bytes it must not panic, must never allow a restricted-license font, and
// refuses only with ErrEmbeddingNotAllowed.
func FuzzCheckEmbeddingAllowed(f *testing.F) {
	for _, fsType := range []uint16{0, fsTypeRestricted, fsTypeRestricted | fsTypeEditable, fsTypeNoSubsetting, fsTypeBitmapOnly, 0xffff} {
		f.Add(os2Font(fsType))
	}
	f.Add([]byte{})
	f.Add(fakeTTF())
	f.Add(os2Font(fsTypeRestricted)[:30])
	// A directory that claims a second table record past the end.
	truncated := os2Font(fsTypeRestricted)
	copy(truncated[12:], "head")
	binary.BigEndian.PutUint16(truncated[4:], 2)
	f.Add(truncated)
	f.Fuzz(func(t *testing.T, b []byte) {
		err := checkEmbeddingAllowed(b)
		if err != nil && !errors.Is(err, ErrEmbeddingNotAllowed) {
			t.Fatalf("error %v does not wrap ErrEmbeddingNotAllowed", err)
		}
		fsType, ok := os2FsType(b)
		if !ok {
			if err != nil {
				t.Fatalf("refused a font with no readable fsType: %v", err)
			}
			return
		}
		restricted := fsType&fsTypeRestricted != 0 && fsType&(fsTypePreviewPrint|fsTypeEditable) == 0
		if restricted && err == nil {
			t.Fatalf("allowed a restricted-license font (fsType %#04x)", fsType)
		}
	})
}
