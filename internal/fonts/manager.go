// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package fonts

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// assetsFS embeds the bundled TrueType fonts. The directory always
// contains at least README.md (so the embed pattern matches even
// before scripts/fetch-fonts.sh has downloaded the binaries), and the
// Manager filters to *.ttf at load time. If no .ttf files are present,
// the Manager simply reports the bundled families as unavailable and
// callers fall back to gofpdf core fonts.
//
//go:embed assets
var assetsFS embed.FS

// FontRegistrar is the slice of gofpdf's API the Manager needs. It is
// satisfied by *gofpdf.Fpdf. Defining it as an interface keeps this
// package free of a direct gofpdf import, so it builds and unit-tests
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

// Manager loads bundled + user fonts and registers them with a gofpdf
// document on demand. It is safe to construct once and reuse; the
// embedded-font lookups are read-only and the user directory is
// re-scanned on each Available() / Register() call so freshly-imported
// fonts appear without a restart.
type Manager struct {
	userDir string
}

// NewManager constructs a Manager. userDir is the directory where
// user-imported .ttf files live; pass "" to disable user fonts. The
// directory is created lazily on the first ImportFont call.
func NewManager(userDir string) *Manager {
	return &Manager{userDir: userDir}
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
// them with the gofpdf document via AddUTF8FontFromBytes. After a
// successful call, the caller can SetFont(family, "B"/"I"/"BI"/"", size).
//
// Lookup order: bundled catalog first, then user fonts. Returns an
// error if the family is unknown or has no usable Regular face.
func (m *Manager) Register(r FontRegistrar, family string) error {
	return m.RegisterAs(r, family, "")
}

// RegisterAs is like Register but registers the font under aliasName
// instead of its real family name. This is how PMForge swaps the
// document font without touching renderer code: registering the chosen
// family under "Helvetica" makes every existing SetFont("Helvetica",
// ...) call use the embedded TrueType font, because gofpdf's
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
		for _, style := range AllStyles {
			ff, ok := fam.File(style)
			if !ok {
				continue
			}
			b, err := assetsFS.ReadFile(filepath.ToSlash(filepath.Join("assets", ff.FileName)))
			if err != nil {
				continue // file not fetched; skip this style
			}
			if err := validateTrueType(b); err != nil {
				continue
			}
			r.AddUTF8FontFromBytes(regName, style.GofpdfStyle(), b)
			registered++
		}
		if registered == 0 {
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
			b, err := os.ReadFile(path) // #nosec G304 -- paths come from scanning the configured user font directory.
			if err != nil {
				continue
			}
			if err := validateTrueType(b); err != nil {
				continue
			}
			r.AddUTF8FontFromBytes(regName, style.GofpdfStyle(), b)
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
// with a clear error, because gofpdf's UTF-8 font path parses
// TrueType tables only.
func (m *Manager) ImportFont(srcPath string) (FamilyInfo, error) {
	if m.userDir == "" {
		return FamilyInfo{}, fmt.Errorf("fonts: no user font directory configured")
	}
	ext := strings.ToLower(filepath.Ext(srcPath))
	if ext != ".ttf" {
		return FamilyInfo{}, fmt.Errorf("fonts: only .ttf files are supported (got %q); OpenType/CFF .otf and WOFF are not supported by the PDF engine", ext)
	}

	b, err := os.ReadFile(srcPath) // #nosec G304 -- user-selected font import path; extension and signature are validated before copy.
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
func (m *Manager) presentBundledStyles(fam FontFamily) []Style {
	var styles []Style
	for _, ff := range fam.Files {
		name := filepath.ToSlash(filepath.Join("assets", ff.FileName))
		if _, err := assetsFS.Open(name); err == nil {
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

// validateTrueType checks the font's signature. gofpdf's UTF-8 parser
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
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) // #nosec G304 -- tmp is derived from PMForge's configured user font destination.
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
