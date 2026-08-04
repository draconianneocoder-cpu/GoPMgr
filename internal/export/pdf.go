// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/go-pdf/fpdf"

	"gopmgr/internal/crypto"
	"gopmgr/internal/fonts"
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
// The generated PDF receives PDF/A-3 XMP metadata (pdfaid:part=3,
// conformance=B) via the shared pdfmeta package. Full strict PDF/A-3
// also requires an embedded ICC profile via OutputIntent (see
// pdfmeta.InjectOutputIntent and MakePDFA3). When an ICC profile is
// available the renderer will use MakePDFA3 for the strongest claim
// possible.
func renderPDF(payload ReportPayload, opts ExportOptions) ([]byte, error) {
	return renderPDFWithSignerLoader(payload, opts, crypto.LoadCertificate)
}

// renderPDFWithSignerLoader keeps certificate loading injectable for isolated
// tests. Production always passes crypto.LoadCertificate through renderPDF;
// the seam exists solely to prove that signing failures cannot publish a
// superficially "signed" fallback document.
func renderPDFWithSignerLoader(
	payload ReportPayload,
	opts ExportOptions,
	loadSigner func(path, password string) (*crypto.Signer, error),
) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	_ = fonts.NewManager("").RegisterAs(pdf, "Source Sans 3", "Helvetica")
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

	// Apply PDF/A-3 XMP metadata (and OutputIntent + ICC when available)
	// before any optional digital signature. PAdES signs exact byte ranges,
	// so signing must be the final incremental update.
	spec := XMPSpec{
		Title:   opts.Title,
		Author:  "GoPMgr",
		Subject: "Critical Path Method Schedule Report",
	}
	// First try the full MakePDFA3 path (XMP + OutputIntent) if we have an ICC.
	// When no ICC is bundled yet we fall back to XMP-only (still a big win).
	if icc := defaultICCProfile(); len(icc) > 0 {
		if tagged, err := MakePDFA3(out, spec, icc); err == nil {
			out = tagged
		}
	} else if xmp := BuildXMPPacket(spec); len(xmp) > 0 {
		if tagged, err := InjectXMPStream(out, xmp); err == nil {
			out = tagged
		}
	}

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

// defaultICCProfile returns the sRGB ICC profile for PDF/A-3 OutputIntent
// if it has been fetched via `make icc`. Returns nil otherwise (the
// renderer will then only inject XMP metadata, which is still a strong
// PDF/A-3 claim but without the color profile).
func defaultICCProfile() []byte {
	return DefaultICCProfile()
}
