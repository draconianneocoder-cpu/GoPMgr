// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package fonts

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// assetsFS embeds the tracked Source Sans 3 PDF/A baseline plus any optional
// families downloaded before compilation. The Manager filters to *.ttf at
// load time and reports only families whose files are available. Supported
// release builds always include Source Sans 3; optional families may still
// fall back to that baseline when their downloaded files are absent.
//
//go:embed assets
var assetsFS embed.FS

// maxFontFileSize bounds a .ttf file GoPMgr will read into memory, whether
// during ImportFont's copy-in validation or when re-reading a previously
// imported file from the user font directory for registration. It's a var,
// not a const, so tests can shrink it and prove the LimitReader bound
// actually holds rather than only exercising the early os.Stat-based
// refusal. The value is set generously for a full-coverage CJK TrueType
// face (which can legitimately reach tens of MB), not for the bundled
// catalog's own baseline (the largest bundled family file is under 500 KB)
// -- the goal is bounding worst-case memory use on a malformed or
// oversized font, not fitting the bundled catalog's own sizes. Mirrors the
// size-capping pattern app_charts.go applies to MSPDI schedule imports
// (maxMSPDIImportSize).
var maxFontFileSize int64 = 64 << 20 // 64 MiB

// fontFileTooLargeErr is shared by every size guard in readFontFileBounded
// so a caller sees the same actionable message regardless of which of the
// two layers (the fast os.Stat check or the io.LimitReader-backed
// post-read check) caught it.
func fontFileTooLargeErr(path string) error {
	return fmt.Errorf(
		"font file %q exceeds GoPMgr's %d MiB font-file limit; this is far larger "+
			"than any real TrueType font and was refused rather than read into memory",
		path, maxFontFileSize>>20)
}

// readFontFileBounded reads path the same way os.ReadFile would, but
// refuses anything larger than maxFontFileSize before reading it fully
// into memory: an os.Stat fast-path plus an io.LimitReader-bounded actual
// read, matching the two-layer guard app_charts.go applies to MSPDI
// schedule imports. Shared by ImportFont (a fresh user file-picker
// selection) and RegisterAs (re-reading a file already inside the app's
// own user font directory) so both call sites use one audited guard
// instead of two hand-rolled ones; each site's existing error handling
// (hard-fail in ImportFont, skip-and-continue in RegisterAs) applies
// unchanged to the size error this returns, same as any other read error.
func readFontFileBounded(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("font file %q is not a regular file", path)
	}
	if info.Size() > maxFontFileSize {
		return nil, fontFileTooLargeErr(path)
	}
	f, err := os.Open(path) // #nosec G304 -- caller-selected font file path; size-checked above.
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxFontFileSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxFontFileSize {
		return nil, fontFileTooLargeErr(path)
	}
	return data, nil
}

// FontRegistrar is the slice of fpdf API the Manager needs. It is
// satisfied by *fpdf.Fpdf. Defining it as an interface keeps this
// package free of a direct fpdf import, so it builds and unit-tests
// without resolving the wider module dependency graph.
type FontRegistrar interface {
	AddUTF8FontFromBytes(familyStr, styleStr string, utf8Bytes []byte)
}

// Origin records where a font came from.
type Origin int

const (
	OriginBundled Origin = iota // shipped in the binary via go:embed
	OriginUser                  // imported by the user into the font dir
)

func (o Origin) String() string {
	if o == OriginUser {
		return "user"
	}
	return "bundled"
}

// FamilyInfo is the UI-facing summary of one available family.
type FamilyInfo struct {
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	Description string   `json:"description"`
	License     string   `json:"license"`
	Origin      string   `json:"origin"`
	Styles      []string `json:"styles"` // human-readable: "Regular", "Bold", ...
}

// Manager loads bundled + user fonts and registers them with a fpdf
// document on demand. It is safe to construct once and reuse; the
// embedded-font lookups are read-only and the user directory is
// re-scanned on each Available() / Register() call so freshly-imported
// fonts appear without a restart.
type Manager struct {
	userDir string

	// bundledFS is the source of bundled font bytes. NewManager always
	// binds it to the real assetsFS embed; only newManagerWithBundledFS
	// substitutes it, and that constructor is unexported, so no production
	// path and no caller outside this package can swap the asset source.
	// It exists so the bundled-font branches are testable against a known
	// asset set instead of against whichever optional families `make
	// fonts` happened to fetch on the machine running the tests.
	// Substituting it does not bypass validation: bundled bytes still go
	// through validateTrueType before reaching the registrar.
	bundledFS fs.FS
}

// NewManager constructs a Manager. userDir is the directory where
// user-imported .ttf files live; pass "" to disable user fonts. The
// directory is created lazily on the first ImportFont call.
func NewManager(userDir string) *Manager {
	return &Manager{userDir: userDir, bundledFS: assetsFS}
}

// newManagerWithBundledFS is NewManager with the bundled-asset source
// replaced. Test-only seam: it is unexported precisely so the substitution
// cannot reach production code, and it is what lets the bundled branches
// be exercised deterministically -- the real embed's contents vary with
// whether optional families were fetched, which previously forced
// TestRegister_BundledWithoutAssets to skip itself on a developer machine
// and made this package's measured coverage depend on the environment.
func newManagerWithBundledFS(userDir string, bundled fs.FS) *Manager {
	return &Manager{userDir: userDir, bundledFS: bundled}
}

// bundled returns the bundled-asset source, falling back to the real
// embed when the field is unset. Manager's fields are all unexported, so
// a caller outside this package can still write fonts.Manager{}; before
// bundledFS existed that zero value worked for bundled fonts, and reading
// a nil fs.FS directly would turn it into a panic instead. The fallback
// keeps the zero value usable rather than making correct construction a
// precondition callers cannot see.
func (m *Manager) bundled() fs.FS {
	if m.bundledFS == nil {
		return assetsFS
	}
	return m.bundledFS
}

// Available returns every font family the Manager can register: bundled
// families whose .ttf files are actually present in the embed, plus any
// user-imported families discovered in userDir. Sorted by origin
// (bundled first) then name.
func (m *Manager) Available() []FamilyInfo {
	var out []FamilyInfo

	// Bundled.
	for _, fam := range Catalog {
		styles := m.presentBundledStyles(fam)
		if len(styles) == 0 {
			continue // binaries not fetched for this family
		}
		out = append(out, FamilyInfo{
			Name:        fam.Name,
			Category:    fam.Category,
			Description: fam.Description,
			License:     fam.License,
			Origin:      OriginBundled.String(),
			Styles:      styleNames(styles),
		})
	}

	// User.
	for _, uf := range m.scanUserFonts() {
		out = append(out, FamilyInfo{
			Name:        uf.name,
			Category:    "user",
			Description: "User-imported font",
			License:     "user-supplied",
			Origin:      OriginUser.String(),
			Styles:      styleNames(uf.styleList()),
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Origin != out[j].Origin {
			return out[i].Origin == OriginBundled.String()
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Register loads all available styles of the named family and registers
// them with the fpdf document via AddUTF8FontFromBytes. After a
// successful call, the caller can SetFont(family, "B"/"I"/"BI"/"", size).
//
// Lookup order: bundled catalog first, then user fonts. Returns an
// error if the family is unknown or has no usable Regular face.
func (m *Manager) Register(r FontRegistrar, family string) error {
	return m.RegisterAs(r, family, "")
}

// RegisterAs is like Register but registers the font under aliasName
// instead of its real family name. This is how GoPMgr swaps the
// document font without touching renderer code: registering the chosen
// family under "Helvetica" makes every existing SetFont("Helvetica",
// ...) call use the embedded TrueType font, because fpdf
// AddUTF8FontFromBytes overrides a core-font family name when you pass
// it one.
//
// An empty aliasName registers under the font's real family name.
func (m *Manager) RegisterAs(r FontRegistrar, family, aliasName string) error {
	if r == nil {
		return fmt.Errorf("fonts: nil registrar")
	}

	// Bundled?
	if fam, ok := CatalogFamily(family); ok {
		regName := fam.Name
		if aliasName != "" {
			regName = aliasName
		}
		registered := 0
		// present is the set of distinct embedded files this family
		// resolved to, whether or not they turned out to be valid fonts.
		// It separates "files are there but unusable" from "files were
		// never fetched", which otherwise reported the same error and sent
		// the reader to the wrong problem.
		//
		// A set rather than a counter, and populated before validation
		// rather than after, for two reasons that are easy to get wrong:
		// the loop walks AllStyles while FontFamily.File falls back to the
		// Regular face for undefined styles, so a single-file family
		// resolves the same file once per style and a counter reported 4
		// files for 1; and membership here means "readable", since the
		// validity verdict is what registered tracks.
		present := make(map[string]struct{}, len(fam.Files))
		for _, style := range AllStyles {
			ff, ok := fam.File(style)
			if !ok {
				continue
			}
			b, err := fs.ReadFile(m.bundled(), filepath.ToSlash(filepath.Join("assets", ff.FileName)))
			if err != nil {
				continue // file not fetched; skip this style
			}
			present[ff.FileName] = struct{}{}
			if err := validateTrueType(b); err != nil {
				continue
			}
			r.AddUTF8FontFromBytes(regName, style.FpdfStyle(), b)
			registered++
		}
		if registered == 0 {
			if len(present) > 0 {
				// Not 'make fonts': that runs scripts/fetch-fonts.sh with no
				// flags, and the script skips any file already on disk unless
				// --force is passed. For assets that are present but corrupt
				// it would do nothing at all, which is the same dead-end this
				// branch exists to steer the reader away from.
				return fmt.Errorf("fonts: bundled family %q has %d embedded .ttf file(s) but none is a usable TrueType font; the embedded assets are corrupt, so overwrite them with 'scripts/fetch-fonts.sh --force' ('make fonts' alone skips files that already exist)", family, len(present))
			}
			return fmt.Errorf("fonts: bundled family %q has no fetched .ttf files (run 'make fonts')", family)
		}
		return nil
	}

	// User?
	if uf, ok := m.findUserFont(family); ok {
		regName := uf.name
		if aliasName != "" {
			regName = aliasName
		}
		registered := 0
		for style, path := range uf.styles {
			b, err := readFontFileBounded(path)
			if err != nil {
				continue
			}
			if err := validateTrueType(b); err != nil {
				continue
			}
			r.AddUTF8FontFromBytes(regName, style.FpdfStyle(), b)
			registered++
		}
		if registered == 0 {
			return fmt.Errorf("fonts: user family %q has no readable .ttf files", family)
		}
		return nil
	}

	return fmt.Errorf("fonts: unknown family %q", family)
}

// ImportFont validates a user-supplied .ttf file and copies it into the
// user font directory, making it available to subsequent Register
// calls. Returns the FamilyInfo for the imported font.
//
// Rejects non-TrueType files (OpenType/CFF .otf, WOFF, collections)
// with a clear error, because fpdf UTF-8 font path parses
// TrueType tables only.
func (m *Manager) ImportFont(srcPath string) (FamilyInfo, error) {
	if m.userDir == "" {
		return FamilyInfo{}, fmt.Errorf("fonts: no user font directory configured")
	}
	ext := strings.ToLower(filepath.Ext(srcPath))
	if ext != ".ttf" {
		return FamilyInfo{}, fmt.Errorf("fonts: only .ttf files are supported (got %q); OpenType/CFF .otf and WOFF are not supported by the PDF engine", ext)
	}

	b, err := readFontFileBounded(srcPath)
	if err != nil {
		return FamilyInfo{}, fmt.Errorf("fonts: read source: %w", err)
	}
	if err := validateTrueType(b); err != nil {
		return FamilyInfo{}, err
	}

	if err := ensurePrivateDir(m.userDir); err != nil {
		return FamilyInfo{}, fmt.Errorf("fonts: create font dir: %w", err)
	}

	dest := filepath.Join(m.userDir, filepath.Base(srcPath))
	if err := writeFileAtomic(dest, b); err != nil {
		return FamilyInfo{}, fmt.Errorf("fonts: write font: %w", err)
	}

	name, _ := deriveFamilyAndStyle(filepath.Base(srcPath))
	uf, ok := m.findUserFont(name)
	if !ok {
		// Shouldn't happen — we just wrote it — but report defensively.
		return FamilyInfo{}, fmt.Errorf("fonts: imported %q but could not re-discover it", name)
	}
	return FamilyInfo{
		Name:        uf.name,
		Category:    "user",
		Description: "User-imported font",
		License:     "user-supplied",
		Origin:      OriginUser.String(),
		Styles:      styleNames(uf.styleList()),
	}, nil
}

// presentBundledStyles returns the styles of a bundled family whose
// .ttf files are actually present in the embed.
//
// Presence only: it does not run validateTrueType over each file, so a
// family whose embedded files are all present-but-corrupt is still listed
// by Available() and then fails in RegisterAs. Validating here would mean
// reading every bundled font's bytes on every Available() call, which the
// font picker makes on each refresh, to defend against a state that
// cannot occur in a supported build -- the assets are embedded at compile
// time and make required-font-assets checks the tracked baseline.
// RegisterAs reports that case distinctly instead, so the failure is at
// least diagnosable when it does happen.
func (m *Manager) presentBundledStyles(fam FontFamily) []Style {
	var styles []Style
	for _, ff := range fam.Files {
		name := filepath.ToSlash(filepath.Join("assets", ff.FileName))
		if f, err := m.bundled().Open(name); err == nil {
			_ = f.Close()
			styles = append(styles, ff.Style)
		}
	}
	return styles
}

// userFont groups the per-style files of one user-imported family.
type userFont struct {
	name   string
	styles map[Style]string // style -> absolute file path
}

func (u userFont) styleList() []Style {
	var out []Style
	for _, s := range AllStyles {
		if _, ok := u.styles[s]; ok {
			out = append(out, s)
		}
	}
	return out
}

// scanUserFonts walks userDir for .ttf files and groups them into
// families by their derived base name.
func (m *Manager) scanUserFonts() []userFont {
	if m.userDir == "" {
		return nil
	}
	entries, err := os.ReadDir(m.userDir)
	if err != nil {
		return nil
	}
	byName := map[string]*userFont{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.ToLower(filepath.Ext(e.Name())) != ".ttf" {
			continue
		}
		name, style := deriveFamilyAndStyle(e.Name())
		uf, ok := byName[name]
		if !ok {
			uf = &userFont{name: name, styles: map[Style]string{}}
			byName[name] = uf
		}
		uf.styles[style] = filepath.Join(m.userDir, e.Name())
	}
	out := make([]userFont, 0, len(byName))
	for _, uf := range byName {
		out = append(out, *uf)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

func (m *Manager) findUserFont(name string) (userFont, bool) {
	for _, uf := range m.scanUserFonts() {
		if strings.EqualFold(uf.name, name) {
			return uf, true
		}
	}
	return userFont{}, false
}

// deriveFamilyAndStyle parses a font filename into a family name and a
// style. It recognises common style suffixes after a "-" or space:
// "Bold", "Italic"/"Oblique", "BoldItalic"/"BoldOblique". Everything
// else maps to Regular. The family name is the remainder with
// separators normalised to spaces.
//
// Examples:
//
//	"LiberationSans-Bold.ttf"     -> ("LiberationSans", Bold)
//	"My Font-BoldItalic.ttf"      -> ("My Font", BoldItalic)
//	"Roboto-Regular.ttf"          -> ("Roboto", Regular)
//	"CustomFont.ttf"              -> ("CustomFont", Regular)
func deriveFamilyAndStyle(filename string) (string, Style) {
	base := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Split on the last '-' to isolate a possible style suffix.
	style := Regular
	name := base
	if idx := strings.LastIndex(base, "-"); idx >= 0 {
		suffix := strings.ToLower(strings.TrimSpace(base[idx+1:]))
		detected, ok := matchStyleSuffix(suffix)
		if ok {
			style = detected
			name = strings.TrimSpace(base[:idx])
		}
	}
	if name == "" {
		name = base
	}
	return name, style
}

func matchStyleSuffix(suffix string) (Style, bool) {
	hasBold := strings.Contains(suffix, "bold")
	hasItalic := strings.Contains(suffix, "italic") || strings.Contains(suffix, "oblique")
	switch {
	case hasBold && hasItalic:
		return BoldItalic, true
	case hasBold:
		return Bold, true
	case hasItalic:
		return Italic, true
	case suffix == "regular" || suffix == "normal" || suffix == "roman" || suffix == "book":
		return Regular, true
	}
	return Regular, false
}

func styleNames(styles []Style) []string {
	out := make([]string, 0, len(styles))
	for _, s := range styles {
		out = append(out, s.String())
	}
	return out
}

// validateTrueType checks the font's signature. fpdf UTF-8 parser
// handles TrueType outlines only, so OpenType/CFF ("OTTO"), WOFF, and
// TrueType Collections ("ttcf") are rejected with actionable errors.
func validateTrueType(b []byte) error {
	if len(b) < 4 {
		return fmt.Errorf("fonts: file too small to be a font")
	}
	sig := b[:4]
	switch {
	case sig[0] == 0x00 && sig[1] == 0x01 && sig[2] == 0x00 && sig[3] == 0x00:
		return nil // TrueType outlines (sfnt 1.0)
	case string(sig) == "true":
		return nil // Apple TrueType
	case string(sig) == "OTTO":
		return fmt.Errorf("fonts: OpenType/CFF fonts are not supported; please supply a TrueType .ttf")
	case string(sig) == "ttcf":
		return fmt.Errorf("fonts: TrueType Collections (.ttc) are not supported; extract a single .ttf")
	case string(sig) == "wOFF" || string(sig) == "wOF2":
		return fmt.Errorf("fonts: WOFF fonts are not supported; please supply a TrueType .ttf")
	default:
		return fmt.Errorf("fonts: unrecognised font signature %x; expected TrueType .ttf", sig)
	}
}

// writeFileAtomic writes b to path via a temp file + rename so a
// partial write never leaves a corrupt font in place.
func writeFileAtomic(path string, b []byte) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) // #nosec G304 -- tmp is derived from GoPMgr's configured user font destination.
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		if closeErr := f.Close(); closeErr != nil {
			err = fmt.Errorf("%w; close: %v", err, closeErr)
		}
		if removeErr := os.Remove(tmp); removeErr != nil && !os.IsNotExist(removeErr) {
			err = fmt.Errorf("%w; remove: %v", err, removeErr)
		}
		return err
	}
	if err := f.Close(); err != nil {
		if removeErr := os.Remove(tmp); removeErr != nil && !os.IsNotExist(removeErr) {
			return fmt.Errorf("%w; remove: %v", err, removeErr)
		}
		return err
	}
	return os.Rename(tmp, path)
}

func ensurePrivateDir(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	return os.Chmod(path, 0o700) // #nosec G302 -- this is a private directory mode, not a file mode.
}

// ensure embed.FS satisfies fs.FS (compile-time guard; also keeps the
// io/fs import meaningful).
var _ fs.FS = assetsFS
