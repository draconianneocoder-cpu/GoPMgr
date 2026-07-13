// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"pmforge/internal/agile"
	"pmforge/internal/calendar"
	"pmforge/internal/documents"
	"pmforge/internal/export"
	"pmforge/internal/fonts"
	"pmforge/internal/timeline"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// =========================================================
// Fonts
// =========================================================

// fontManager returns a *fonts.Manager scoped to the signed-in user's
// font directory (<DataDir>/fonts). If no user is signed in, the
// manager still serves the bundled catalog but cannot import or list
// user fonts.
func (a *App) fontManager() *fonts.Manager {
	userDir := ""
	if u := a.requireUser(); u != nil {
		userDir = filepath.Join(u.DataDir, "fonts")
	}
	return fonts.NewManager(userDir)
}

// ListFonts returns every font family available for document export:
// the bundled families whose .ttf files are present in the build, plus
// any fonts the user has imported. Each entry reports its origin
// (bundled / user), category, license, and available styles.
func (a *App) ListFonts() []fonts.FamilyInfo {
	return a.fontManager().Available()
}

// ImportFont opens a native file picker for a TrueType (.ttf) font,
// validates it, and copies it into the user's font directory so it
// becomes available for document export. Returns the imported family's
// info. OpenType/CFF (.otf), WOFF, and TrueType Collections are
// rejected with a clear error because the PDF engine embeds TrueType
// outlines only.
func (a *App) ImportFont() (fonts.FamilyInfo, error) {
	if a.ctx == nil {
		return fonts.FamilyInfo{}, errors.New("no context (Wails not started)")
	}
	if a.requireUser() == nil {
		return fonts.FamilyInfo{}, errors.New("not signed in")
	}
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:            "Select a TrueType font (.ttf)",
		DefaultDirectory: a.userDir(),
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "TrueType fonts (*.ttf)", Pattern: "*.ttf"},
		},
	})
	if err != nil {
		return fonts.FamilyInfo{}, err
	}
	if path == "" {
		return fonts.FamilyInfo{}, errors.New("no file selected")
	}
	return a.fontManager().ImportFont(path)
}

// GetDefaultFont returns the document-export font family the user has
// chosen, or the catalog default when unset.
func (a *App) GetDefaultFont() (string, error) {
	d := a.requireDB()
	if d == nil {
		return fonts.DefaultFamily, nil
	}
	s, err := d.GetSettings()
	if err != nil {
		return fonts.DefaultFamily, err
	}
	if s.DefaultFont == "" {
		return fonts.DefaultFamily, nil
	}
	return s.DefaultFont, nil
}

// SetDefaultFont persists the document-export font family. The chosen
// family must be available (bundled-and-fetched or user-imported).
func (a *App) SetDefaultFont(family string) error {
	d := a.requireDB()
	if d == nil {
		return errors.New("no project open")
	}
	available := false
	for _, f := range a.fontManager().Available() {
		if f.Name == family {
			available = true
			break
		}
	}
	if !available {
		return fmt.Errorf("font %q is not available", family)
	}
	s, err := d.GetSettings()
	if err != nil {
		return err
	}
	s.DefaultFont = family
	if err := d.SaveSettings(s); err != nil {
		return err
	}
	// Apply immediately so the next export uses the new font.
	documents.UseFont(a.fontManager(), family)
	return nil
}

// ExportProjectICS writes a .ics file with the project's timeline +
// (optionally) the country's holidays to the user's exports/ folder.
// Returns the absolute path. The frontend should open the file in
// the user's default calendar app.
func (a *App) ExportProjectICS(includeHolidays bool) (string, error) {
	d := a.requireDB()
	u := a.requireUser()
	if d == nil || u == nil {
		return "", errors.New("not signed in or no project open")
	}
	p, err := d.GetProject()
	if err != nil {
		return "", err
	}
	store := agile.NewStore(d.Conn, p.ID)
	sprints, err := store.ListSprints()
	if err != nil {
		return "", err
	}
	deploys, err := store.ListDeployments(time.Time{})
	if err != nil {
		return "", err
	}
	entries := timeline.Build(p, sprints, deploys)

	events := make([]export.ICalEvent, 0, len(entries))
	for _, e := range entries {
		events = append(events, export.ICalEvent{
			UID:         e.SourceID + "-" + string(e.Kind),
			Summary:     e.Title,
			Description: e.Description,
			Start:       e.Date,
			End:         e.EndDate,
			Category:    string(e.Kind),
		})
	}

	spec := export.ICalSpec{
		CalendarName: p.Name,
		ProjectID:    p.ID,
		Events:       events,
	}
	if includeHolidays {
		cal := calendar.For(p.CountryCode)
		// Span the calendar over the project's window, or a default
		// of one year backward + one year forward when dates are
		// blank.
		from := time.Now().AddDate(-1, 0, 0)
		to := time.Now().AddDate(1, 0, 0)
		if t, ok := parseISODate(p.StartDate); ok {
			from = t
		}
		if t, ok := parseISODate(p.EndDate); ok {
			to = t
		}
		spec = export.AppendHolidayEvents(spec, cal, from, to)
	}

	bytes := export.ICalRender(spec)

	outDir := filepath.Join(u.DataDir, "exports")
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return "", err
	}
	stamp := time.Now().UTC().Format("20060102-150405")
	outPath := filepath.Join(outDir, fmt.Sprintf("%s-%s.ics", sanitizeFilename(p.Name), stamp))
	if err := os.WriteFile(outPath, bytes, 0o600); err != nil {
		return "", err
	}
	return outPath, nil
}

// helpers ---------------------------------------------------------

func countryCodeOrDefault(c string) string {
	if c == "" {
		return "US"
	}
	return c
}

func parseISODate(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
