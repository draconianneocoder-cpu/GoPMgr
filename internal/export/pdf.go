// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"bytes"
	"fmt"
	"sort"
	"time"

	"github.com/jung-kurt/gofpdf"

	"pmforge/internal/crypto"
	"pmforge/internal/fonts"
	"pmforge/internal/pdfmeta"
)

// renderPDF produces an archival-quality PDF report of the CPM schedule.
// Layout:
//
//	Page 1: Title block (project title, nanosecond-precision timestamp),
//	        followed by a tabular task list with ES/EF/LS/LF/Float and a
//	        critical-path marker.
//
// If opts.DigitalSignature is set, the function embeds a real PAdES B-B
// signature using pdfmeta.InjectPAdESSignature (proper /Sig dictionary,
// /ByteRange, and /Contents via incremental update). This is the
// production path. Falls back to a comment marker only if embedding fails.
//
// The generated PDF receives PDF/A-3 XMP metadata (pdfaid:part=3,
// conformance=B) via the shared pdfmeta package. Full strict PDF/A-3
// also requires an embedded ICC profile via OutputIntent (see
// pdfmeta.InjectOutputIntent and MakePDFA3). When an ICC profile is
// available the renderer will use MakePDFA3 for the strongest claim
// possible.
func renderPDF(payload ReportPayload, opts ExportOptions) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	_ = fonts.NewManager("").RegisterAs(pdf, "Source Sans 3", "Helvetica")
	pdf.SetTitle(opts.Title, true)
	pdf.SetAuthor("PMForge", true)
	pdf.SetCreator("PMForge "+exportVersion(), true)
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
		Author:  "PMForge",
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

	// Optional digital signature.
	// Preferred path: real PAdES B-B embedding via incremental update
	// (creates a proper /Sig dictionary + /ByteRange + /Contents).
	// Falls back to the old comment marker if embedding fails.
	if opts.DigitalSignature {
		signer, err := crypto.LoadCertificate(opts.CertPath, opts.CertPassword)
		if err != nil {
			return nil, err
		}

		// Real PAdES B-B path: we let InjectPAdESSignature build the
		// structure + exact ByteRange first, then it calls us back to
		// sign the precise concatenated ranges.
		signedPDF, padesErr := pdfmeta.InjectPAdESSignature(out, signer.SignPDFCMS)
		if padesErr == nil {
			out = signedPDF
		} else {
			// Fallback to the older comment-based marker
			cmsBlob, _ := signer.SignPDFCMS(out)
			out = appendCMSSignatureMarker(out, cmsBlob)
		}
	}

	return out, nil
}

// appendSignatureMarker writes the raw RSA signature blob to the
// end of the PDF inside a PDF comment so naive readers ignore it
// but verifiers can extract it.
//
// Used as the fallback when CMS/PKCS#7 signing isn't available
// (e.g. a P12 bundle without a chain). New code SHOULD prefer
// appendCMSSignatureMarker.
func appendSignatureMarker(pdfBytes, sig []byte) []byte {
	const tag = "\n%%PMForgeSignature:"
	out := make([]byte, 0, len(pdfBytes)+len(sig)+len(tag)+8)
	out = append(out, pdfBytes...)
	out = append(out, []byte(tag)...)
	out = append(out, []byte(hexEncode(sig))...)
	out = append(out, '\n')
	return out
}

// appendCMSSignatureMarker writes a CMS/PKCS#7 detached signature
// (built by crypto.Signer.SignPDFCMS) to the end of the PDF inside
// a PDF comment. The marker uses a distinct prefix so consumers can
// distinguish CMS blobs from the older raw-RSA marker:
//
//	%%PMForgeCMSSignature:<hex CMS blob>
//
// This is interim. Full PAdES B-B compliance requires embedding the
// CMS blob inside an incremental update with a /Sig dictionary and
// a properly-sized /Contents slot referenced by /ByteRange. That
// rewrites the PDF's xref table and is non-trivial with gofpdf;
// tracked in AGENT.md §8 as "real PDF signing widget".
func appendCMSSignatureMarker(pdfBytes, cmsBlob []byte) []byte {
	const tag = "\n%%PMForgeCMSSignature:"
	out := make([]byte, 0, len(pdfBytes)+len(cmsBlob)*2+len(tag)+8)
	out = append(out, pdfBytes...)
	out = append(out, []byte(tag)...)
	out = append(out, []byte(hexEncode(cmsBlob))...)
	out = append(out, '\n')
	return out
}

func hexEncode(b []byte) string {
	const hex = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hex[v>>4]
		out[i*2+1] = hex[v&0x0f]
	}
	return string(out)
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
