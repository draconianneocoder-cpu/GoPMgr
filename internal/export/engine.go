// SPDX-FileCopyrightText: 2026 The PMForge Contributors
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
	"time"

	"pmforge/internal/crypto"
	"pmforge/internal/debug"
	"pmforge/internal/documents"
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
// Right now it carries CPM tasks; future Packs (Agile, EVM) can grow
// this struct with additional fields.
type ReportPayload struct {
	Tasks map[string]*kernel.Task
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
	fmt.Printf("[export] %s report at %s\n", opts.Format, reportTime)

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
