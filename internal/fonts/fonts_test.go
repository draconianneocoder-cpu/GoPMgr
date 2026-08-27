// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package fonts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStyleFpdfStyle(t *testing.T) {
	cases := map[Style]string{
		Regular:    "",
		Bold:       "B",
		Italic:     "I",
		BoldItalic: "BI",
	}
	for style, want := range cases {
		if got := style.FpdfStyle(); got != want {
			t.Errorf("Style(%d).FpdfStyle() = %q, want %q", style, got, want)
		}
	}
}

func TestFontFamilyFile_FallsBackToRegular(t *testing.T) {
	fam := FontFamily{
		Name: "Test",
		Files: []FontFile{
			{Regular, "Test-Regular.ttf"},
			{Bold, "Test-Bold.ttf"},
		},
	}
	// Present style returns itself.
	if ff, ok := fam.File(Bold); !ok || ff.FileName != "Test-Bold.ttf" {
		t.Errorf("File(Bold) = %+v, %v", ff, ok)
	}
	// Missing style (Italic) falls back to Regular.
	ff, ok := fam.File(Italic)
	if !ok || ff.FileName != "Test-Regular.ttf" {
		t.Errorf("File(Italic) fallback = %+v, %v; want Test-Regular.ttf", ff, ok)
	}
}

func TestFontFamilyFile_NoRegularNoMatch(t *testing.T) {
	fam := FontFamily{
		Name:  "OnlyBold",
		Files: []FontFile{{Bold, "OnlyBold-Bold.ttf"}},
	}
	if _, ok := fam.File(Italic); ok {
		t.Error("File(Italic) should fail when neither Italic nor Regular present")
	}
}

func TestCatalogFamily(t *testing.T) {
	if _, ok := CatalogFamily("Liberation Sans"); !ok {
		t.Error("expected to find 'Liberation Sans'")
	}
	// Case-insensitive.
	if _, ok := CatalogFamily("liberation sans"); !ok {
		t.Error("CatalogFamily should be case-insensitive")
	}
	if _, ok := CatalogFamily("Nonexistent Font"); ok {
		t.Error("did not expect to find 'Nonexistent Font'")
	}
}

func TestDeriveFamilyAndStyle(t *testing.T) {
	cases := []struct {
		filename  string
		wantName  string
		wantStyle Style
	}{
		{"LiberationSans-Regular.ttf", "LiberationSans", Regular},
		{"LiberationSans-Bold.ttf", "LiberationSans", Bold},
		{"LiberationSans-Italic.ttf", "LiberationSans", Italic},
		{"LiberationSans-BoldItalic.ttf", "LiberationSans", BoldItalic},
		{"DejaVuSans-Oblique.ttf", "DejaVuSans", Italic},
		{"DejaVuSans-BoldOblique.ttf", "DejaVuSans", BoldItalic},
		{"CustomFont.ttf", "CustomFont", Regular},
		{"My Font-BoldItalic.ttf", "My Font", BoldItalic},
		// A hyphen that isn't a style suffix stays part of the name.
		{"Foo-Bar.ttf", "Foo-Bar", Regular},
	}
	for _, c := range cases {
		name, style := deriveFamilyAndStyle(c.filename)
		if name != c.wantName || style != c.wantStyle {
			t.Errorf("deriveFamilyAndStyle(%q) = (%q, %v), want (%q, %v)",
				c.filename, name, style, c.wantName, c.wantStyle)
		}
	}
}

func TestValidateTrueType(t *testing.T) {
	tests := []struct {
		name    string
		sig     []byte
		wantErr bool
	}{
		{"sfnt 1.0", []byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x00}, false},
		{"apple true", []byte("true----"), false},
		{"opentype cff", []byte("OTTO----"), true},
		{"collection", []byte("ttcf----"), true},
		{"woff", []byte("wOFF----"), true},
		{"garbage", []byte("%PDF----"), true},
		{"too short", []byte{0x00, 0x01}, true},
	}
	for _, tc := range tests {
		err := validateTrueType(tc.sig)
		if (err != nil) != tc.wantErr {
			t.Errorf("validateTrueType(%s) err=%v, wantErr=%v", tc.name, err, tc.wantErr)
		}
	}
}

// fakeTTF returns a byte slice with a valid TrueType signature padded
// to a usable length. It is NOT a parseable font — only validateTrueType
// and the byte-handling paths accept it; never pass it to a real
// fpdf registrar.
func fakeTTF() []byte {
	b := make([]byte, 64)
	b[0], b[1], b[2], b[3] = 0x00, 0x01, 0x00, 0x00
	return b
}

// recordingRegistrar implements FontRegistrar by recording calls.
type recordingRegistrar struct {
	calls []struct {
		family string
		style  string
		n      int
	}
}

func (r *recordingRegistrar) AddUTF8FontFromBytes(family, style string, b []byte) {
	r.calls = append(r.calls, struct {
		family string
		style  string
		n      int
	}{family, style, len(b)})
}

func TestImportFontAndRegister(t *testing.T) {
	dir := t.TempDir()

	// Create a fake source font to import.
	src := filepath.Join(t.TempDir(), "Acme-Bold.ttf")
	if err := os.WriteFile(src, fakeTTF(), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}

	mgr := NewManager(dir)
	info, err := mgr.ImportFont(src)
	if err != nil {
		t.Fatalf("ImportFont: %v", err)
	}
	if info.Name != "Acme" {
		t.Errorf("imported family name = %q, want Acme", info.Name)
	}
	if info.Origin != "user" {
		t.Errorf("origin = %q, want user", info.Origin)
	}

	// The file should now be in the user dir.
	if _, err := os.Stat(filepath.Join(dir, "Acme-Bold.ttf")); err != nil {
		t.Errorf("imported font not found in user dir: %v", err)
	}

	// Available should report the user font.
	found := false
	for _, fam := range mgr.Available() {
		if fam.Name == "Acme" && fam.Origin == "user" {
			found = true
		}
	}
	if !found {
		t.Error("Available() did not include the imported Acme font")
	}

	// Register should call the registrar for the Bold style.
	reg := &recordingRegistrar{}
	if err := mgr.Register(reg, "Acme"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if len(reg.calls) != 1 {
		t.Fatalf("expected 1 registrar call, got %d", len(reg.calls))
	}
	if reg.calls[0].family != "Acme" || reg.calls[0].style != "B" {
		t.Errorf("registrar call = %+v, want family=Acme style=B", reg.calls[0])
	}
}

func TestImportFontTightensExistingUserDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "fonts")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir font dir: %v", err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("chmod broad font dir: %v", err)
	}
	src := filepath.Join(t.TempDir(), "Acme-Regular.ttf")
	if err := os.WriteFile(src, fakeTTF(), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}

	mgr := NewManager(dir)
	if _, err := mgr.ImportFont(src); err != nil {
		t.Fatalf("ImportFont: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat font dir: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o700 {
		t.Fatalf("font dir mode = %o, want 700", mode)
	}
}

func TestImportFont_RejectsNonTTF(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	// Wrong extension.
	otf := filepath.Join(t.TempDir(), "Bad.otf")
	if err := os.WriteFile(otf, []byte("OTTO1234"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := mgr.ImportFont(otf); err == nil {
		t.Error("expected error importing .otf")
	}

	// Right extension but OpenType/CFF signature.
	fakeOtf := filepath.Join(t.TempDir(), "Sneaky.ttf")
	if err := os.WriteFile(fakeOtf, []byte("OTTO1234"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := mgr.ImportFont(fakeOtf); err == nil {
		t.Error("expected error importing OTTO-signed .ttf")
	}
}

// TestImportFont_RejectsOversizedFile proves the early os.Stat-based
// refusal: a file reported larger than maxFontFileSize is refused before
// any read is attempted. The file is sparse (Truncate, no real bytes
// written) since only its reported size matters for this branch.
func TestImportFont_RejectsOversizedFile(t *testing.T) {
	src := filepath.Join(t.TempDir(), "huge.ttf")
	f, err := os.Create(src) // #nosec G304 -- test-owned temp path.
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if err := f.Truncate(maxFontFileSize + 1); err != nil {
		t.Fatalf("truncate temp file: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}

	mgr := NewManager(t.TempDir())
	if _, err := mgr.ImportFont(src); err == nil {
		t.Fatal("expected an error for an oversized font file, got nil")
	} else if !strings.Contains(err.Error(), "font-file limit") {
		t.Errorf("error %q should mention the font-file limit", err.Error())
	}
}

// TestImportFont_RejectsOversizedRealFileAtShrunkCap proves oversized
// refusal at a shrunk cap using a real (non-sparse) file, complementing the
// sparse-file fast-path test above. It does NOT prove independence from
// the os.Stat fast path -- this file's real byte count matches what Stat
// reports, both exceed the cap, so the shipped os.Stat check catches it
// first; instrumentation confirmed the second-layer (io.LimitReader +
// post-read length check) code is never reached by either test. What the
// second layer alone proves rests on separate os.Stat-disabled
// fault-seeding, not on either test in this file: with Stat neutered, the
// second layer still rejects both correctly; disabling only the post-read
// length check (keeping io.LimitReader) makes both FAIL, and does so worse
// here than in internal/crypto: io.LimitReader alone silently truncates a
// 64-byte fakeTTF to the shrunk cap, and since validateTrueType only
// inspects the leading 4 signature bytes, the truncated file passes
// validation and imports successfully with NO error at all -- silent
// truncate-and-accept, not even a confusing downstream error. The
// post-read length check, not LimitReader, is what produces the actionable
// "too large" error and prevents that silent truncation. Disabling only
// LimitReader (keeping the post-read check) still passes, so no test
// distinguishes a properly memory-bounded read from an unbounded one
// that's rejected afterward. The os.Stat-vs-actual-size TOCTOU scenario
// named in the guard's code comment is accordingly unproven by any test
// here; it rests on io.LimitReader's documented stdlib contract, not
// independent evidence.
func TestImportFont_RejectsOversizedRealFileAtShrunkCap(t *testing.T) {
	original := maxFontFileSize
	maxFontFileSize = 16
	t.Cleanup(func() { maxFontFileSize = original })

	src := filepath.Join(t.TempDir(), "small-but-over-cap.ttf")
	if err := os.WriteFile(src, fakeTTF(), 0o600); err != nil { // fakeTTF is 64 bytes, well over the shrunk cap
		t.Fatalf("write temp file: %v", err)
	}

	mgr := NewManager(t.TempDir())
	if _, err := mgr.ImportFont(src); err == nil {
		t.Fatal("expected an error once the file exceeds the (shrunk) import limit, got nil")
	} else if !strings.Contains(err.Error(), "font-file limit") {
		t.Errorf("error %q should mention the font-file limit", err.Error())
	}
}

// TestImportFont_AcceptsFileExactlyAtCap proves the boundary is `>`, not
// `>=`: a file exactly maxFontFileSize bytes must pass the guard. This is
// the one plausible mutation (`>` -> `>=`) no refusal-only test above would
// catch.
func TestImportFont_AcceptsFileExactlyAtCap(t *testing.T) {
	original := maxFontFileSize
	maxFontFileSize = 16
	t.Cleanup(func() { maxFontFileSize = original })

	src := filepath.Join(t.TempDir(), "Cap-Regular.ttf")
	content := fakeTTF()[:maxFontFileSize] // exactly at the cap, not over; still a valid TrueType signature
	if err := os.WriteFile(src, content, 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	mgr := NewManager(t.TempDir())
	if _, err := mgr.ImportFont(src); err != nil {
		t.Fatalf("a file exactly at the cap must not be rejected: %v", err)
	}
}

// TestRegisterAs_SkipsOversizedStyleButRegistersSiblings proves RegisterAs's
// existing skip-and-continue behavior (already relied on for unreadable or
// invalid-signature style files) extends to an oversized one: a single
// oversized style file must not abort registration of the family's other,
// well-sized styles. The files are written directly into the user font
// directory (bypassing ImportFont, which would itself refuse the oversized
// one at import time) to model the RegisterAs threat this guards against --
// a file already on disk growing out-of-band, not a fresh user import.
func TestRegisterAs_SkipsOversizedStyleButRegistersSiblings(t *testing.T) {
	original := maxFontFileSize
	maxFontFileSize = 16
	t.Cleanup(func() { maxFontFileSize = original })

	dir := t.TempDir()
	oversized := append(fakeTTF(), make([]byte, 16)...)
	if err := os.WriteFile(filepath.Join(dir, "Acme-Regular.ttf"), fakeTTF()[:8], 0o600); err != nil {
		t.Fatalf("write Regular: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Acme-Bold.ttf"), oversized, 0o600); err != nil {
		t.Fatalf("write Bold: %v", err)
	}

	mgr := NewManager(dir)
	reg := &recordingRegistrar{}
	if err := mgr.Register(reg, "Acme"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if len(reg.calls) != 1 {
		t.Fatalf("expected exactly 1 registrar call (Regular only), got %d: %+v", len(reg.calls), reg.calls)
	}
	if reg.calls[0].style != "" {
		t.Errorf("registered style = %q, want Regular (empty FpdfStyle)", reg.calls[0].style)
	}
}

func TestRegister_UnknownFamily(t *testing.T) {
	mgr := NewManager(t.TempDir())
	reg := &recordingRegistrar{}
	if err := mgr.Register(reg, "No Such Family"); err == nil {
		t.Error("expected error registering unknown family")
	}
}

func TestRegister_NilRegistrar(t *testing.T) {
	mgr := NewManager(t.TempDir())
	if err := mgr.Register(nil, "Liberation Sans"); err == nil {
		t.Error("expected error with nil registrar")
	}
}

// TestRegister_BundledWithoutAssets confirms the actionable error when
// a bundled family's binaries haven't been fetched. assets/ always
// carries README.md plus the always-tracked Source Sans 3 baseline;
// Liberation Sans (like other optional families) is gitignored and
// fetched via `make fonts`, so a checkout without it reproduces the
// no-assets state this test targets. Skips if fonts happen to be
// present already (e.g. after `make fonts` ran locally).
func TestRegister_BundledWithoutAssets(t *testing.T) {
	mgr := NewManager(t.TempDir())
	reg := &recordingRegistrar{}
	err := mgr.Register(reg, "Liberation Sans")
	if err == nil {
		t.Skip("Liberation Sans assets present (fonts were fetched); skipping no-assets check")
	}
	// Error should mention running 'make fonts'.
	if !contains(err.Error(), "make fonts") {
		t.Errorf("error %q should guide the user to 'make fonts'", err.Error())
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
