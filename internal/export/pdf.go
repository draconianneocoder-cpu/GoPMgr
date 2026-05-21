// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"bytes"
	"fmt"
	"sort"
	"time"

	"github.com/jung-kurt/gofpdf"

	"pmforge/internal/crypto"
)

// renderPDF produces an archival-quality PDF report of the CPM schedule.
// Layout:
//
//	Page 1: Title block (project title, nanosecond-precision timestamp),
//	        followed by a tabular task list with ES/EF/LS/LF/Float and a
//	        critical-path marker.
//
// If opts.DigitalSignature is set, the function ALSO appends a
// SHA-256 + RSA signature blob to the document via crypto.Signer.
//
// NOTE: This produces a standard PDF 1.5, not a strict PDF/A-3. For
// true PDF/A compliance you must embed every font subset, set the XMP
// metadata stream, and pass through veraPDF for validation. That is a
// V1.2 milestone.
func renderPDF(payload ReportPayload, opts ExportOptions) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
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

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	out := buf.Bytes()

	// Optional digital signature. Appended as a trailing PDF comment
	// block containing the base64 signature — NOT a fully-embedded
	// CMS/PKCS#7 signature (see crypto/pdf_sign.go for the upgrade
	// path). Verifiers should treat the trailing block as a proof of
	// integrity, not as an Adobe-Reader-recognised signature widget.
	if opts.DigitalSignature {
		signer, err := crypto.LoadCertificate(opts.CertPath, opts.CertPassword)
		if err != nil {
			return nil, err
		}
		// Prefer CMS/PKCS#7 detached signatures (PAdES B-B basics)
		// for archival quality. Falls back to raw-RSA-in-comment if
		// the CMS path errors out — shipping a less-conformant
		// signature is still better than no signature for an
		// "audit log" PDF.
		cmsBlob, cmsErr := signer.SignPDFCMS(out)
		if cmsErr == nil && len(cmsBlob) > 0 {
			out = appendCMSSignatureMarker(out, cmsBlob)
		} else {
			sig, err := signer.SignPDFHash(out)
			if err != nil {
				return nil, err
			}
			out = appendSignatureMarker(out, sig)
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
// learn the live app version without import cycles. The cmd/pmforge
// main package wires this at startup.
var exportVersion = func() string { return "1.x" }

// SetVersion lets the application set the version string used in PDF
// metadata. Called once at startup from cmd/pmforge/main.go.
func SetVersion(v string) { exportVersion = func() string { return v } }
