// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package fonts

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
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

// TestNewManager_BindsRealEmbeddedAssets guards the seam itself. Every
// other bundled-path test injects a fake bundle, so if NewManager stopped
// binding the real assetsFS embed -- or bound a nil fs.FS -- nothing else
// in this file would notice, and production would ship a Manager that can
// register no bundled font at all.
//
// It asserts only on Source Sans 3, which is committed to the repository
// as the PDF/A baseline (the other families are gitignored and fetched by
// `make fonts`), so this stays deterministic on any checkout.
func TestNewManager_BindsRealEmbeddedAssets(t *testing.T) {
	mgr := NewManager(t.TempDir())
	reg := &recordingRegistrar{}

	if err := mgr.Register(reg, "Source Sans 3"); err != nil {
		t.Fatalf("Register the committed Source Sans 3 baseline: %v", err)
	}
	if len(reg.calls) == 0 {
		t.Fatal("NewManager must bind the real embed; no style was registered")
	}
	for _, call := range reg.calls {
		if call.n == 0 {
			t.Errorf("style %q registered with 0 bytes; the embed is not being read", call.style)
		}
	}
}

// TestZeroValueManager_FallsBackToRealEmbed guards Manager.bundled()'s
// nil fallback. Manager's fields are all unexported, so code outside this
// package can legally write fonts.Manager{}; that zero value worked for
// bundled fonts before bundledFS was introduced, and reading the field
// directly would have turned it into a nil-interface panic. Without this
// test, collapsing bundled() back into a plain field read would reinstate
// that panic silently.
func TestZeroValueManager_FallsBackToRealEmbed(t *testing.T) {
	var mgr Manager // deliberately not NewManager

	got := mgr.Available()
	if len(got) == 0 {
		t.Fatal("zero-value Manager should still report the committed Source Sans 3 baseline")
	}

	reg := &recordingRegistrar{}
	if err := mgr.Register(reg, "Source Sans 3"); err != nil {
		t.Fatalf("zero-value Manager should register the committed baseline: %v", err)
	}
	if len(reg.calls) == 0 {
		t.Error("zero-value Manager registered no styles")
	}
}

// bundledAsset is the embed-relative path of one bundled style, matching
// what Manager builds from the catalog. Kept next to the tests that
// construct fake bundles so a catalog rename shows up here as a compile
// or lookup failure rather than as a silently empty fake FS.
func bundledAsset(fileName string) string { return "assets/" + fileName }

// TestRegister_BundledWithoutAssets confirms the actionable error when a
// bundled family's binaries haven't been fetched.
//
// This drives the Manager against an empty in-memory bundle rather than
// the real embed. The real embed's contents depend on whether `make
// fonts` fetched the optional families -- Liberation Sans is gitignored
// -- so the previous version of this test skipped itself whenever a
// developer had run `make fonts`, and, worse, made this package's
// measured coverage depend on the machine it ran on: with assets present
// Register covered its whole success path (83.8%), without them it
// returned early (83.3%). That is why the drift ledger recorded a number
// no CI runner could reproduce. Injecting the bundle removes the
// environmental dependency entirely: the no-assets branch is now always
// the branch under test.
func TestRegister_BundledWithoutAssets(t *testing.T) {
	mgr := newManagerWithBundledFS(t.TempDir(), fstest.MapFS{})
	reg := &recordingRegistrar{}

	err := mgr.Register(reg, "Liberation Sans")
	if err == nil {
		t.Fatal("Register with an empty bundle should fail, got nil")
	}
	if !contains(err.Error(), "make fonts") {
		t.Errorf("error %q should guide the user to 'make fonts'", err.Error())
	}
	if !contains(err.Error(), "no fetched") {
		t.Errorf("error %q should report the files as unfetched", err.Error())
	}
	if len(reg.calls) != 0 {
		t.Errorf("nothing should be registered from an empty bundle, got %d calls", len(reg.calls))
	}
}

// TestRegister_BundledAssetsPresentButCorrupt covers the case the old
// skip-based test could never reach: the embedded files exist but none is
// a usable TrueType font. This previously reported the same "no fetched
// .ttf files (run 'make fonts')" error as an empty bundle, which points
// the reader at fetching -- advice that cannot fix a file that is already
// there and corrupt. The two cases must report distinctly.
func TestRegister_BundledAssetsPresentButCorrupt(t *testing.T) {
	fam, ok := CatalogFamily("Liberation Sans")
	if !ok {
		t.Fatal("Liberation Sans missing from the catalog")
	}
	bundle := fstest.MapFS{}
	for _, ff := range fam.Files {
		// "OTTO" is a real signature validateTrueType rejects, so this is
		// a plausible corruption (an OpenType/CFF file named .ttf) rather
		// than arbitrary bytes.
		bundle[bundledAsset(ff.FileName)] = &fstest.MapFile{Data: []byte("OTTO____")}
	}

	mgr := newManagerWithBundledFS(t.TempDir(), bundle)
	reg := &recordingRegistrar{}

	err := mgr.Register(reg, "Liberation Sans")
	if err == nil {
		t.Fatal("Register over a corrupt bundle should fail, got nil")
	}
	if contains(err.Error(), "no fetched") {
		t.Errorf("error %q misreports present-but-corrupt files as unfetched", err.Error())
	}
	if !contains(err.Error(), "corrupt") {
		t.Errorf("error %q should say the embedded assets are corrupt", err.Error())
	}
	if len(reg.calls) != 0 {
		t.Errorf("a font failing validation must never reach the registrar, got %d calls", len(reg.calls))
	}
}

// TestRegister_BundledFromInjectedBundle proves the seam registers real
// styles when the bundle holds valid TrueType data, so the two failure
// tests above are asserting a genuine failure rather than a bundle the
// Manager could never read in the first place. It also pins the
// alias-registration contract RegisterAs exists for.
func TestRegister_BundledFromInjectedBundle(t *testing.T) {
	fam, ok := CatalogFamily("Liberation Sans")
	if !ok {
		t.Fatal("Liberation Sans missing from the catalog")
	}
	bundle := fstest.MapFS{}
	for _, ff := range fam.Files {
		bundle[bundledAsset(ff.FileName)] = &fstest.MapFile{Data: fakeTTF()}
	}

	mgr := newManagerWithBundledFS(t.TempDir(), bundle)
	reg := &recordingRegistrar{}

	if err := mgr.RegisterAs(reg, "Liberation Sans", "Helvetica"); err != nil {
		t.Fatalf("RegisterAs over a valid bundle: %v", err)
	}
	if len(reg.calls) != len(fam.Files) {
		t.Fatalf("expected %d registrar calls, got %d", len(fam.Files), len(reg.calls))
	}
	for _, call := range reg.calls {
		if call.family != "Helvetica" {
			t.Errorf("RegisterAs should register under the alias, got %q", call.family)
		}
	}
}

// TestAvailable_SortsBundledBeforeUserThenByName pins the order the font
// picker renders. Available() documents "sorted by origin (bundled first)
// then name", and nothing asserted it.
//
// It also removes the last environmental dependency in this package's
// coverage. The comparator's name tiebreak is only reached when two
// entries share an origin, and against the real embed that needs two
// bundled families -- i.e. optional families fetched by `make fonts`. On a
// bare checkout only the committed Source Sans 3 baseline is bundled, so
// the tiebreak went uncovered and this package measured 0.5 points lower
// than on a developer machine. Injecting two bundled families covers it on
// every checkout.
func TestAvailable_SortsBundledBeforeUserThenByName(t *testing.T) {
	// Three families, deliberately not two: Catalog's own order is
	// "Liberation Sans", "Liberation Serif", "Liberation Mono", so the
	// first *two* already happen to be in name order and would pass even
	// against a comparator that never reordered anything. Taking three
	// makes catalog order and name order genuinely differ, so the
	// assertion below can only pass if the sort actually ran.
	const want = 3
	var picked []FontFamily
	bundle := fstest.MapFS{}
	for _, fam := range Catalog {
		ff, ok := fam.File(Regular)
		if !ok {
			continue
		}
		picked = append(picked, fam)
		bundle[bundledAsset(ff.FileName)] = &fstest.MapFile{Data: fakeTTF()}
		if len(picked) == want {
			break
		}
	}
	if len(picked) != want {
		t.Fatalf("need %d bundled families with a Regular style, catalog gave %d", want, len(picked))
	}

	catalogOrder := make([]string, len(picked))
	for i, fam := range picked {
		catalogOrder[i] = fam.Name
	}
	nameOrder := append([]string(nil), catalogOrder...)
	sort.Strings(nameOrder)
	if reflect.DeepEqual(catalogOrder, nameOrder) {
		// Not a pass: it means this test can no longer detect a broken
		// comparator, which is exactly the pass-by-omission this suite
		// treats as a failure elsewhere.
		t.Fatalf("catalog order %v already equals name order; pick families whose orders differ or this test proves nothing", catalogOrder)
	}

	userDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(userDir, "Acme-Regular.ttf"), fakeTTF(), 0o600); err != nil {
		t.Fatalf("write user font: %v", err)
	}

	mgr := newManagerWithBundledFS(userDir, bundle)
	got := mgr.Available()
	if len(got) != want+1 {
		t.Fatalf("expected %d bundled + 1 user family, got %d: %+v", want, len(got), got)
	}

	var gotBundled []string
	for i, info := range got {
		if info.Origin == OriginBundled.String() {
			if i >= want {
				t.Errorf("bundled family %q at index %d should precede every user font", info.Name, i)
			}
			gotBundled = append(gotBundled, info.Name)
			continue
		}
		if i != want {
			t.Errorf("user font %q at index %d should sort after all bundled families", info.Name, i)
		}
	}
	if !reflect.DeepEqual(gotBundled, nameOrder) {
		t.Errorf("bundled families = %v, want ascending by name %v", gotBundled, nameOrder)
	}
}

// TestAvailable_ReportsOnlyBundledStylesPresent pins Available() to the
// injected bundle too, so the family list the UI renders is asserted
// against a known asset set instead of whichever optional families the
// developer happened to fetch.
func TestAvailable_ReportsOnlyBundledStylesPresent(t *testing.T) {
	fam, ok := CatalogFamily("Liberation Sans")
	if !ok {
		t.Fatal("Liberation Sans missing from the catalog")
	}
	regularFile, ok := fam.File(Regular)
	if !ok {
		t.Fatal("Liberation Sans has no Regular style in the catalog")
	}
	// Only the Regular face is present; the other three are unfetched.
	bundle := fstest.MapFS{
		bundledAsset(regularFile.FileName): &fstest.MapFile{Data: fakeTTF()},
	}

	mgr := newManagerWithBundledFS("", bundle)

	var got *FamilyInfo
	for i, info := range mgr.Available() {
		if info.Name == "Liberation Sans" {
			got = &mgr.Available()[i]
			break
		}
	}
	if got == nil {
		t.Fatal("Liberation Sans should be available when its Regular face is present")
	}
	if len(got.Styles) != 1 || got.Styles[0] != Regular.String() {
		t.Errorf("expected only the Regular style, got %v", got.Styles)
	}
	if got.Origin != OriginBundled.String() {
		t.Errorf("expected a bundled origin, got %q", got.Origin)
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
