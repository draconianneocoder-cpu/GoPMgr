// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"time"

	"gopmgr/internal/crypto"
	"gopmgr/internal/fonts"
	"gopmgr/internal/pdfdoc"
	"gopmgr/internal/pdfmeta"
	"gopmgr/internal/signing"
)

// renderPDF produces an archival-quality PDF report of the CPM schedule.
// Layout:
//
//	Page 1: Title block (project title, nanosecond-precision timestamp),
//	        followed by a tabular task list with ES/EF/LS/LF/Float and a
//	        critical-path marker.
//
// If opts.DigitalSignature is set, the function embeds a real PAdES Baseline B
// signature through the shared signing pipeline. A certificate or embedding
// failure returns an error and no bytes; nonstandard comment-marker signatures
// are deliberately unsupported.
//
// The generated PDF is made PDF/A-2u (pdfaid:part=2, conformance=U) by
// pdfmeta.TagPDFA: font repair, XMP identification, and an sRGB
// OutputIntent, before any signature.
func renderPDF(payload ReportPayload, opts ExportOptions) ([]byte, error) {
	return renderPDFWithSignerLoader(payload, opts, crypto.LoadCertificate)
}

// renderPDFWithSignerLoader keeps certificate loading injectable for isolated
// tests. Production always passes crypto.LoadCertificate through renderPDF;
// the seam exists solely to prove that signing failures cannot publish a
// superficially "signed" fallback document.
// registerPDFABaseline registers the embedded fonts.DefaultFamily (IBM Plex
// Sans) faces under the "Helvetica" name every renderer in this package calls
// SetFont with.
//
// The error is logged rather than returned, and neither half of that is
// incidental. IBM Plex Sans is the PDF/A baseline: if it fails to register,
// fpdf silently falls back to its built-in core Helvetica, which is NOT
// embedded, and the document still goes out labelled PDF/A -- a
// non-conformant file that looks correct. Dropping the error (`_ =`, as both
// call sites did until 2026-09-18) meant the one diagnostic that names this
// reached no user and no log.
//
// It stays non-fatal because a supported build cannot reach it: the faces are
// //go:embed-ed at compile time and check-required-font-assets.sh pins their
// SHA-256s, so a failure means a tampered or hand-built binary, where refusing
// to export at all is not obviously the kinder outcome. Promoting this to a
// hard error is a deliberate release-policy call for a maintainer to make, not
// a default to slip in.
func registerPDFABaseline(pdf *pdfdoc.PDF) {
	if err := fonts.NewManager("").RegisterAs(pdf, fonts.DefaultFamily, "Helvetica"); err != nil {
		log.Printf("[export] PDF/A baseline font did not register, falling back to a non-embedded core font: %v", err)
	}
}

func renderPDFWithSignerLoader(
	payload ReportPayload,
	opts ExportOptions,
	loadSigner func(path, password string) (*crypto.Signer, error),
) ([]byte, error) {
	pdf := pdfdoc.New("P", "mm", "A4", "")
	registerPDFABaseline(pdf)
	pdf.SetTitle(opts.Title, true)
	pdf.SetAuthor("GoPMgr", true)
	pdf.SetCreator("GoPMgr "+exportVersion(), true)
	pdf.AddPage()

	// Title
	pdf.SetFont("Helvetica", "B", 18)
	pdf.Cell(0, 12, opts.Title)
	pdf.Ln(14)

	// Timestamp
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(120, 120, 120)
	pdf.Cell(0, 6, "Generated "+time.Now().UTC().Format(time.RFC3339Nano))
	pdf.Ln(10)
	pdf.SetTextColor(0, 0, 0)

	// Header row
	pdf.SetFont("Helvetica", "B", 10)
	headers := []string{"ID", "Title", "Dur.", "ES", "EF", "LS", "LF", "Float", "Crit?"}
	widths := []float64{18, 60, 14, 14, 14, 14, 14, 16, 14}
	for i, h := range headers {
		pdf.CellFormat(widths[i], 7, h, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)

	// Task rows in stable ID order.
	pdf.SetFont("Helvetica", "", 9)
	ids := make([]string, 0, len(payload.Tasks))
	for id := range payload.Tasks {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		t := payload.Tasks[id]
		crit := ""
		if t.IsCritical {
			crit = "YES"
			pdf.SetTextColor(180, 30, 30)
		}
		row := []string{
			t.ID,
			truncate(t.Title, 40),
			fmt.Sprintf("%.1f", t.Duration),
			fmt.Sprintf("%.1f", t.ES),
			fmt.Sprintf("%.1f", t.EF),
			fmt.Sprintf("%.1f", t.LS),
			fmt.Sprintf("%.1f", t.LF),
			fmt.Sprintf("%.2f", t.Float),
			crit,
		}
		for i, cell := range row {
			pdf.CellFormat(widths[i], 6, cell, "1", 0, "L", false, 0, "")
		}
		pdf.SetTextColor(0, 0, 0)
		pdf.Ln(-1)
	}

	// Earned-value summary (suppressed without cost data).
	if lines := evmSummaryLines(payload.EVM); lines != nil {
		pdf.Ln(6)
		pdf.SetFont("Helvetica", "B", 12)
		pdf.Cell(0, 8, "Earned Value (status date: today)")
		pdf.Ln(9)
		pdf.SetFont("Helvetica", "", 9)
		for _, line := range lines {
			pdf.Cell(0, 5.5, line)
			pdf.Ln(5.5)
		}
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	out := buf.Bytes()

	// Make the file PDF/A-2u before any optional digital signature. PAdES
	// signs exact byte ranges, so signing must be the final incremental
	// update.
	out = tagPDFA(out, XMPSpec{
		Title:   opts.Title,
		Author:  "GoPMgr",
		Subject: "Critical Path Method Schedule Report",
	})

	// Optional Baseline B signature. The archive export API predates project
	// TSA settings, so it deliberately requests no timestamp here; application
	// document/report exports use the same pipeline with prepared PAdES-T
	// configuration.
	if opts.DigitalSignature {
		if loadSigner == nil {
			return nil, errors.New("export: PAdES certificate loader is required")
		}
		signer, err := loadSigner(opts.CertPath, opts.CertPassword)
		if err != nil {
			return nil, err
		}
		signedPDF, _, err := signing.ApplyPAdES(context.Background(), out, signer, nil)
		if err != nil {
			return nil, fmt.Errorf("export: apply PAdES Baseline B signature: %w", err)
		}
		out = signedPDF
	}

	return out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

// exportVersion is a small indirection so the PDF metadata block can
// learn the live app version without import cycles. The root main
// package wires this at startup.
var exportVersion = func() string { return "1.x" }

// SetVersion lets the application set the version string used in PDF
// metadata. Called once at startup from the root main.go.
func SetVersion(v string) { exportVersion = func() string { return v } }

// tagPDFA makes a rendered report PDF/A-2u. If tagging fails, the report
// still goes out, untagged, and the failure is logged: a readable report is
// more useful than none, and the file never claims PDF/A it lacks.
func tagPDFA(raw []byte, spec XMPSpec) []byte {
	tagged, err := pdfmeta.TagPDFA(raw, spec)
	if err != nil {
		log.Printf("[export] PDF/A tagging failed, exporting an untagged PDF: %v", err)
	}
	return tagged
}
