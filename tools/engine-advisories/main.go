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

// maxReviewAgeDays is the oldest review the scan accepts (owner decision
// DEC-ADVISORY-SCAN, 2026-11-14): one more than 90 days old fails it. Only
// the online scan applies it, so the offline check run by make test never
// fails because time has passed.
const maxReviewAgeDays = 90

// errReviewAge marks a review too old for the scan, or dated in the future.
var errReviewAge = errors.New("the advisory review is out of date")

// reviewAgeDays is the number of whole UTC calendar days from reviewed (a
// UTC date at midnight) to now's UTC date. It is negative for a review
// dated after today.
func reviewAgeDays(reviewed, now time.Time) int {
	n := now.UTC()
	today := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
	return int(today.Sub(reviewed) / (24 * time.Hour))
}

// checkReviewAge fails a review older than maxReviewAgeDays, or dated
// after today.
func checkReviewAge(reviewedOn string, days int) error {
	switch {
	case days < 0:
		return fmt.Errorf("%w: reviewed_on %s is after today's UTC date; reviewed_on is a UTC date", errReviewAge, reviewedOn)
	case days > maxReviewAgeDays:
		return fmt.Errorf("%w: reviewed on %s, %d days ago, more than the %d days allowed; review the advisories above and set reviewed_on (docs/engineering/engine-advisories.md)",
			errReviewAge, reviewedOn, days, maxReviewAgeDays)
	}
	return nil
}

// scan reports every CVE NVD returns and fails on any that applies without
// a disposition, when NVD gives no answer, or when the review is out of
// date (checkReviewAge). An out-of-date review still queries NVD, so the
// output is what the next review needs; its error is joined with any other.
// Warnings never fail it: a disposition NVD no longer returns, and a
// dispositioned CVE NVD changed after the review.
func scan(ctx context.Context, r record, nvd *nvdClient, out io.Writer, now func() time.Time) error {
	reviewed, err := r.reviewDate()
	if err != nil {
		return err
	}
	days := reviewAgeDays(reviewed, now())
	_, _ = fmt.Fprintf(out, "Review of %s, %d days ago (UTC dates); the scan allows %d.\n", r.ReviewedOn, days, maxReviewAgeDays)
	ageErr := checkReviewAge(r.ReviewedOn, days)
	if err := errors.Join(scanNVD(ctx, r, reviewed, nvd, out), ageErr); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(out, "No advisory applies without a disposition.")
	return nil
}

// scanNVD queries NVD for each component in turn and fails on the first
// component NVD gives no answer for, or, once all are read, on any
// applicable advisory without a disposition.
func scanNVD(ctx context.Context, r record, reviewed time.Time, nvd *nvdClient, out io.Writer) error {
	byID := map[string]disposition{}
	for _, d := range r.Dispositions {
		byID[d.ID] = d
	}
	returned := map[string]bool{}
	var undisposed, warnings []string
	for _, c := range r.Components {
		// Control: NVD must know the product at all, or a wrong CPE name
		// would pass for want of anything to match.
		u, total, err := nvd.known(ctx, c.CPEVendor, c.CPEProduct)
		_, _ = fmt.Fprintf(out, "GET %s\n", u)
		if err != nil {
			return fmt.Errorf("%s: %w", c.Name, err)
		}
		if total == 0 {
			return fmt.Errorf("%s: %w: NVD lists no CVE for %s:%s at any version; check the CPE name", c.Name, errNoVerdict, c.CPEVendor, c.CPEProduct)
		}
		_, _ = fmt.Fprintf(out, "%s:%s: NVD lists %d CVEs at any version\n", c.CPEVendor, c.CPEProduct, total)
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
				if disposed {
					warnings = append(warnings, fmt.Sprintf("%s no longer applies to %s %s; remove its disposition at the next review", cve.ID, c.Name, c.Version))
				}
			case disposed:
				_, _ = fmt.Fprintf(out, "  %s (%s): applies (%s); disposition %s\n", cve.ID, cve.VulnStatus, why, d.Status)
				// NVD's times are UTC. A change on the review day itself is
				// reported too: the review may have read the earlier entry.
				modified, err := time.Parse("2006-01-02T15:04:05", trimFraction(cve.LastModified))
				switch {
				case err != nil:
					warnings = append(warnings, fmt.Sprintf("%s has an unreadable lastModified %q; check its disposition still holds", cve.ID, cve.LastModified))
				case !modified.Before(reviewed):
					warnings = append(warnings, fmt.Sprintf("%s changed at NVD on %s, on or after the review; check its disposition still holds", cve.ID, cve.LastModified))
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
	return nil
}

// trimFraction drops NVD's fractional seconds ("2026-06-17T04:45:50.960").
func trimFraction(s string) string {
	if i := len("2006-01-02T15:04:05"); len(s) > i && s[i] == '.' {
		return s[:i]
	}
	return s
}
