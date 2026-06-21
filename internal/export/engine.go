// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package export converts PMForge's internal data models into the
// archival document formats required by the Administrative Pack:
// PDF/A, ODT, DOCX, XLSX, CSV, and MSPDI XML (Microsoft Project).
//
// The package exposes a single entry point: GenerateArchivalReport.
// Each format lives in its own file (pdf.go, mspdi.go, xlsx.go, ...)
// so a missing dependency doesn't break unrelated formats.
package export

import (
	"fmt"
	"log"
	"time"

	"pmforge/internal/crypto"
	"pmforge/internal/debug"
	"pmforge/internal/kernel"
)

// ExportFormat is the on-disk format selector.
type ExportFormat string

const (
	FormatPDF   ExportFormat = "pdf"
	FormatDOCX  ExportFormat = "docx"
	FormatODT   ExportFormat = "odt"
	FormatXLSX  ExportFormat = "xlsx"
	FormatCSV   ExportFormat = "csv"
	FormatHTML  ExportFormat = "html"
	FormatMSPDI ExportFormat = "mspdi"
)

// ExportOptions bundles every flag the export engine takes. Fields are
// purposefully flat so the Wails bridge can pass them as a single JSON
// object from the Settings GUI.
type ExportOptions struct {
	Format           ExportFormat
	Title            string
	Encrypted        bool
	Password         string
	DigitalSignature bool
	CertPath         string
	CertPassword     string
}

// ReportPayload is the kernel-level data the export engine renders.
// It carries CPM tasks (which include progress and cost fields) and,
// optionally, earned-value metrics computed at export time. EVM is
// nil when the project lacks a start date or the caller skipped it;
// renderers also suppress the section when BAC is zero (no cost data
// means the numbers would be noise). Future Packs can grow this
// struct with additional fields.
type ReportPayload struct {
	Tasks map[string]*kernel.Task
	EVM   *kernel.EVMetrics
}

// evmSummaryLines renders the earned-value block shared by the DOCX,
// ODT, and PDF schedule reports: one "label: value" line per metric.
// Returns nil when the section should be suppressed.
func evmSummaryLines(m *kernel.EVMetrics) []string {
	if m == nil || m.BAC <= 0 {
		return nil
	}
	idx := func(v float64) string {
		if v <= 0 {
			return "n/a"
		}
		return fmt.Sprintf("%.2f", v)
	}
	return []string{
		fmt.Sprintf("Budget at completion (BAC): %.2f", m.BAC),
		fmt.Sprintf("Planned value (PV): %.2f", m.PV),
		fmt.Sprintf("Earned value (EV): %.2f", m.EV),
		fmt.Sprintf("Actual cost (AC): %.2f", m.AC),
		fmt.Sprintf("Schedule variance (SV): %.2f", m.SV),
		fmt.Sprintf("Cost variance (CV): %.2f", m.CV),
		"Schedule performance index (SPI): " + idx(m.SPI),
		"Cost performance index (CPI): " + idx(m.CPI),
		fmt.Sprintf("Estimate at completion (EAC): %.2f", m.EAC),
		fmt.Sprintf("Estimate to complete (ETC): %.2f", m.ETC),
		fmt.Sprintf("Variance at completion (VAC): %.2f", m.VAC),
	}
}

// GenerateArchivalReport renders payload into the chosen format and
// returns the bytes ready to write to disk. Encryption (if requested)
// is applied AFTER format conversion.
//
// The function logs a single Printf line with the format and a
// nanosecond-precision RFC3339 timestamp — useful when correlating an
// export with the audit log.
func GenerateArchivalReport(payload ReportPayload, opts ExportOptions) ([]byte, error) {
	reportTime := time.Now().UTC().Format(time.RFC3339Nano)
	log.Printf("[export] %s report at %s", opts.Format, reportTime)

	var (
		raw []byte
		err error
	)
	switch opts.Format {
	case FormatPDF:
		raw, err = renderPDF(payload, opts)
	case FormatXLSX:
		raw, err = renderXLSX(payload, opts)
	case FormatMSPDI:
		raw, err = ToMSPDI(opts.Title, payload.Tasks)
	case FormatCSV:
		raw, err = renderCSV(payload, opts)
	case FormatHTML:
		raw, err = renderHTML(payload, opts)
	case FormatDOCX:
		raw, err = renderDocumentDOCX(payload, opts)
	case FormatODT:
		raw, err = renderDocumentODT(payload, opts)
	default:
		return nil, debug.Wrap(
			fmt.Errorf("unknown format %q", opts.Format),
			"EXPORT_FORMAT_UNKNOWN",
		).ToError()
	}

	if err != nil {
		return nil, err
	}

	if opts.Encrypted {
		return crypto.EncryptBuffer(raw, opts.Password)
	}
	return raw, nil
}
