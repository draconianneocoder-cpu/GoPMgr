// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// os-open-scan applies the memory gate's existing textual Close heuristic.
// It is not a control-flow or resource-ownership analysis.
package main

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func run(roots []string, out, errOut io.Writer) int {
	if len(roots) == 0 {
		fmt.Fprintln(errOut, "os-open-scan: at least one source root is required")
		return 2
	}
	findings := false
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("symlink source input: %s", path)
			}
			if entry.IsDir() {
				if entry.Name() == "vendor" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			found, err := scanFile(path, out)
			findings = findings || found
			return err
		})
		if err != nil {
			fmt.Fprintf(errOut, "os-open-scan: %v\n", err)
			return 2
		}
	}
	if findings {
		return 1
	}
	return 0
}

func scanFile(path string, out io.Writer) (bool, error) {
	found := false
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	// Preserve the previous helper's token limit, but report incomplete scans.
	scanner.Buffer(make([]byte, 4096), 1<<20)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("scan %s: %w", path, err)
	}
	for i, line := range lines {
		if !strings.Contains(line, "os.Open(") || strings.Contains(line, "//") {
			continue
		}
		closeFound := false
		for j := i; j < len(lines) && j < i+10; j++ {
			if strings.Contains(lines[j], "Close()") {
				closeFound = true
				break
			}
		}
		if !closeFound {
			if _, err := fmt.Fprintf(out, "%s:%d: %s\n", path, i+1, strings.TrimSpace(line)); err != nil {
				return false, fmt.Errorf("write finding: %w", err)
			}
			found = true
		}
	}
	return found, nil
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
