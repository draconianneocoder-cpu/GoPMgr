// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Command engine-advisories checks the SQLite and SQLCipher engine vendored
// in third_party/go-sqlcipher against the review recorded in reviewed.json
// (SEC-06). govulncheck cannot see this C code. See
// docs/engineering/engine-advisories.md.
//
// From the repository root:
//
//	go run ./tools/engine-advisories check   # offline: the engine is the reviewed one
//	go run ./tools/engine-advisories scan    # online: NVD has no advisory without a disposition
//
// check fails when an engine file, a version or the SBOM differs from the
// review. scan runs check, then asks NVD for the CVEs matching each
// component's version, and fails on any that applies without a
// not_affected disposition, or when NVD gives no answer.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout, newNVDClient(), time.Now); err != nil {
		fmt.Fprintln(os.Stderr, "engine-advisories:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out io.Writer, nvd *nvdClient, now func() time.Time) error {
	fs := flag.NewFlagSet("engine-advisories", flag.ContinueOnError)
	fs.SetOutput(out)
	root := fs.String("root", ".", "repository root")
	if len(args) == 0 {
		return errors.New("usage: engine-advisories check|scan [-root dir]")
	}
	cmd := args[0]
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	r, err := loadRecord(*root)
	if err != nil {
		return err
	}
	if err := r.validate(); err != nil {
		return err
	}
	if err := r.checkAgainstTree(*root); err != nil {
		return err
	}
	switch cmd {
	case "check":
		_, _ = fmt.Fprintf(out, "engine-advisories: the vendored engine is the one reviewed on %s\n", r.ReviewedOn)
		return nil
	case "scan":
		return scan(ctx, r, nvd, out, now)
	default:
		return fmt.Errorf("unknown command %q; use check or scan", cmd)
	}
}

// scan reports every CVE NVD returns and fails on any that applies without
// a disposition. Warnings never fail it: a disposition NVD no longer
// returns, and a dispositioned CVE NVD changed after the review.
func scan(ctx context.Context, r record, nvd *nvdClient, out io.Writer, now func() time.Time) error {
	reviewed, err := r.reviewDate()
	if err != nil {
		return err
	}
	days := int(now().UTC().Sub(reviewed).Hours() / 24)
	_, _ = fmt.Fprintf(out, "Review of %s, %d days ago.\n", r.ReviewedOn, days)

	byID := map[string]disposition{}
	for _, d := range r.Dispositions {
		byID[d.ID] = d
	}
	returned := map[string]bool{}
	var undisposed, warnings []string
	for _, c := range r.Components {
		q, err := nvd.cvesFor(ctx, c.cpe())
		for _, u := range q.URLs {
			_, _ = fmt.Fprintf(out, "GET %s\n", u)
		}
		if err != nil {
			return fmt.Errorf("%s %s: %w", c.Name, c.Version, err)
		}
		_, _ = fmt.Fprintf(out, "%s %s: NVD returned %d CVEs (NVD time %s)\n", c.Name, c.Version, len(q.CVEs), q.Timestamp)
		for _, cve := range q.CVEs {
			applicable, why := applies(cve, c)
			d, disposed := byID[cve.ID]
			disposed = disposed && d.Component == c.Name
			if disposed {
				returned[cve.ID] = true
			}
			switch {
			case !applicable:
				_, _ = fmt.Fprintf(out, "  %s (%s): does not apply: %s\n", cve.ID, cve.VulnStatus, why)
			case disposed:
				_, _ = fmt.Fprintf(out, "  %s (%s): applies (%s); disposition %s\n", cve.ID, cve.VulnStatus, why, d.Status)
				if modified, err := time.Parse("2006-01-02T15:04:05", trimFraction(cve.LastModified)); err == nil && modified.After(reviewed.Add(24*time.Hour)) {
					warnings = append(warnings, fmt.Sprintf("%s changed at NVD on %s, after the review; check its disposition still holds", cve.ID, cve.LastModified))
				}
			default:
				_, _ = fmt.Fprintf(out, "  %s (%s): APPLIES (%s) and has no disposition\n", cve.ID, cve.VulnStatus, why)
				undisposed = append(undisposed, c.Name+" "+cve.ID)
			}
		}
	}
	for _, d := range r.Dispositions {
		if !returned[d.ID] {
			warnings = append(warnings, fmt.Sprintf("NVD no longer returns %s for %s; remove its disposition at the next review", d.ID, d.Component))
		}
	}
	for _, w := range warnings {
		_, _ = fmt.Fprintf(out, "warning: %s\n", w)
	}
	if len(undisposed) > 0 {
		return fmt.Errorf("%d advisories apply to the vendored engine without a disposition: %v; see docs/engineering/engine-advisories.md", len(undisposed), undisposed)
	}
	_, _ = fmt.Fprintln(out, "No advisory applies without a disposition.")
	return nil
}

// trimFraction drops NVD's fractional seconds ("2026-06-17T04:45:50.960").
func trimFraction(s string) string {
	if i := len("2006-01-02T15:04:05"); len(s) > i && s[i] == '.' {
		return s[:i]
	}
	return s
}
