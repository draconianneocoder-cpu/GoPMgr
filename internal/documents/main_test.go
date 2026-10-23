// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import (
	"os"
	"testing"
)

// TestMain runs this package's tests on fpdf's core Helvetica. Many
// assertions here are absolute byte-size floors and ceilings measured
// without an embedded font (see project_overview_test.go and
// wbs_document_test.go); the production default embeds IBM Plex Sans, which
// adds tens of kilobytes and would hide the differences those tests measure.
// The tests of the embedded default in fonts_test.go restore it explicitly
// with withDefaultDocumentFont.
func TestMain(m *testing.M) {
	SetFontApplier(nil)
	os.Exit(m.Run())
}
