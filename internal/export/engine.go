// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package export converts GoPMgr's internal data models into the
// archival document formats required by the Administrative Pack:
// PDF/A, ODT, DOCX, XLSX, CSV, HTML, and MSPDI XML (Microsoft Project).
//
// The package exposes a single entry point: GenerateArchivalReport.
// Each format lives in its own file (pdf.go, mspdi.go, xlsx.go, ...)
// so a missing dependency doesn't break unrelated formats.
package export

import (
	"fmt"
	"log"
	"time"

	"gopmgr/internal/crypto"
	"gopmgr/internal/debug"
	"gopmgr/internal/documents"
	"gopmgr/internal/kernel"
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
// means the numbers would be noise). Currency is the project's reporting
// currency, in which the earned-value amounts print. Future Packs can grow
// this struct with additional fields.
type ReportPayload struct {
	Tasks    map[string]*kernel.Task
	EVM      *kernel.EVMetrics
	Currency string
}

// evmSummaryLines renders the earned-value block shared by the DOCX, ODT,
// HTML, and PDF schedule reports, with amounts in the payload's reporting
// currency, as combined reports print them. It returns nil when the section
// is suppressed, and an error for a malformed currency, as documents do.
func evmSummaryLines(payload ReportPayload) ([]string, error) {
	lines := documents.EVMSummaryLines(payload.EVM, payload.Currency)
	if lines == nil {
		return nil, nil
	}
	if !documents.IsCurrencyCode(payload.Currency) {
		return nil, fmt.Errorf("export: invalid reporting currency %q for the earned-value summary", payload.Currency)
	}
	return lines, nil
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
