// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Command df0-fixtures writes the ADR-012 DF0 corpus and its manifest.
//
//	go run ./cmd/df0-fixtures -out DIR            # write files, print the manifest path
//	go run ./cmd/df0-fixtures -out DIR -write-manifest
//	go run ./cmd/df0-fixtures -out DIR -check     # fail if the bytes drift from fixtures/manifest.json
//
// -large also writes the performance-only cases (1,000,000 rows).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopmgr/spikes/adr012-datafusion/internal/fixtures"
)

func main() {
	out := flag.String("out", "", "directory to write the corpus into (required)")
	large := flag.Bool("large", false, "also write performance-only cases")
	manifestPath := flag.String("manifest", "fixtures/manifest.json", "committed manifest")
	writeManifest := flag.Bool("write-manifest", false, "rewrite the committed manifest")
	check := flag.Bool("check", false, "fail if the generated corpus differs from the committed manifest")
	flag.Parse()
	if err := run(*out, *large, *manifestPath, *writeManifest, *check); err != nil {
		fmt.Fprintln(os.Stderr, "df0-fixtures:", err)
		os.Exit(1)
	}
}

func run(out string, large bool, manifestPath string, writeManifest, check bool) error {
	if out == "" {
		return fmt.Errorf("-out is required")
	}
	m, err := fixtures.WriteCorpus(out, large)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if err := os.WriteFile(filepath.Join(out, "manifest.json"), b, 0o600); err != nil {
		return err
	}
	if writeManifest {
		if err := os.WriteFile(manifestPath, b, 0o644); err != nil { // #nosec G306 -- a public fixture manifest.
			return err
		}
	}
	if check {
		raw, err := os.ReadFile(manifestPath) // #nosec G304 -- the committed manifest.
		if err != nil {
			return err
		}
		var want fixtures.Manifest
		if err := json.Unmarshal(raw, &want); err != nil {
			return fmt.Errorf("read %s: %w", manifestPath, err)
		}
		if diffs := fixtures.Diff(m, want); len(diffs) > 0 {
			return fmt.Errorf("corpus differs from %s:\n  %s", manifestPath, strings.Join(diffs, "\n  "))
		}
		fmt.Printf("df0-fixtures: %d cases match %s\n", len(m.Entries), manifestPath)
		return nil
	}
	fmt.Printf("df0-fixtures: wrote %d cases to %s\n", len(m.Entries), out)
	return nil
}
