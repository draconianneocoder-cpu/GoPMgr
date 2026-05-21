// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package pdfmeta

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// minimalPDF returns a syntactically-valid 3-object PDF byte stream
// that exercises the same shape as gofpdf output: header marker,
// Catalog object, Pages object, content object, xref, trailer,
// startxref, EOF.
//
// The Catalog (object 1) references Pages (object 2). Object 3 is a
// content stream stand-in.
func minimalPDF() []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")

	// Object 1: Catalog
	obj1Off := b.Len()
	b.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

	// Object 2: Pages
	obj2Off := b.Len()
	b.WriteString("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")

	// Object 3: a stub content stream so we can test that "1 0 obj"
	// inside arbitrary bytes doesn't fool findObjectBody.
	obj3Off := b.Len()
	const fakeContent = "stream-data: looks like 1 0 obj but isn't\n"
	b.WriteString(fmt.Sprintf(
		"3 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n",
		len(fakeContent), fakeContent))

	// xref
	xrefOff := b.Len()
	b.WriteString("xref\n")
	b.WriteString("0 4\n")
	b.WriteString("0000000000 65535 f \n")
	fmt.Fprintf(&b, "%010d 00000 n \n", obj1Off)
	fmt.Fprintf(&b, "%010d 00000 n \n", obj2Off)
	fmt.Fprintf(&b, "%010d 00000 n \n", obj3Off)

	// trailer + startxref + EOF
	b.WriteString("trailer\n<<\n/Size 4\n/Root 1 0 R\n>>\n")
	fmt.Fprintf(&b, "startxref\n%d\n%%%%EOF\n", xrefOff)
	return b.Bytes()
}

func TestFindLastStartxref(t *testing.T) {
	pdf := minimalPDF()
	off, err := findLastStartxref(pdf)
	if err != nil {
		t.Fatalf("findLastStartxref: %v", err)
	}
	// The offset should point to the literal "xref\n" subsequence.
	if !bytes.HasPrefix(pdf[off:], []byte("xref")) {
		t.Fatalf("startxref offset %d does not point at 'xref' (got %q)", off, string(pdf[off:off+8]))
	}
}

func TestFindLastStartxref_Empty(t *testing.T) {
	if _, err := findLastStartxref([]byte{}); err == nil {
		t.Fatal("expected error on empty input")
	}
	if _, err := findLastStartxref([]byte("no startxref keyword here")); err == nil {
		t.Fatal("expected error when keyword missing")
	}
}

func TestParseTrailerSizeAndRoot(t *testing.T) {
	pdf := minimalPDF()
	xrefOff, err := findLastStartxref(pdf)
	if err != nil {
		t.Fatalf("findLastStartxref: %v", err)
	}
	size, root, gen, err := parseTrailerSizeAndRoot(pdf, xrefOff)
	if err != nil {
		t.Fatalf("parseTrailerSizeAndRoot: %v", err)
	}
	if size != 4 {
		t.Errorf("/Size: got %d, want 4", size)
	}
	if root != 1 {
		t.Errorf("/Root id: got %d, want 1", root)
	}
	if gen != 0 {
		t.Errorf("/Root gen: got %d, want 0", gen)
	}
}

func TestFindObjectBody_FindsCatalog(t *testing.T) {
	pdf := minimalPDF()
	body, err := findObjectBody(pdf, 1, 0)
	if err != nil {
		t.Fatalf("findObjectBody(1, 0): %v", err)
	}
	if !bytes.Contains(body, []byte("/Type /Catalog")) {
		t.Errorf("Catalog body missing /Type /Catalog marker; got %q", string(body))
	}
}

// TestFindObjectBody_IgnoresStreamSubstring confirms that a sequence
// like "1 0 obj" appearing inside a content stream does not match —
// the start-of-line guard must require a newline (or start of file)
// before the marker.
func TestFindObjectBody_IgnoresStreamSubstring(t *testing.T) {
	pdf := minimalPDF()
	body, err := findObjectBody(pdf, 1, 0)
	if err != nil {
		t.Fatalf("findObjectBody(1, 0): %v", err)
	}
	if !bytes.Contains(body, []byte("Catalog")) {
		t.Errorf("expected real Catalog body, got %q", string(body))
	}
}

func TestInsertMetadataReference_InsertsWhenAbsent(t *testing.T) {
	orig := []byte("<< /Type /Catalog /Pages 2 0 R >>")
	got := insertMetadataReference(orig, 99)
	if !bytes.Contains(got, []byte("/Metadata 99 0 R")) {
		t.Fatalf("expected /Metadata insertion, got %q", string(got))
	}
	// Original entries should still be present.
	if !bytes.Contains(got, []byte("/Type /Catalog")) {
		t.Errorf("lost /Type entry: %q", string(got))
	}
	if !bytes.Contains(got, []byte("/Pages 2 0 R")) {
		t.Errorf("lost /Pages entry: %q", string(got))
	}
}

func TestInsertMetadataReference_ReplacesExisting(t *testing.T) {
	orig := []byte("<< /Type /Catalog /Metadata 7 0 R /Pages 2 0 R >>")
	got := insertMetadataReference(orig, 99)
	if !bytes.Contains(got, []byte("/Metadata 99 0 R")) {
		t.Errorf("expected /Metadata 99 0 R, got %q", string(got))
	}
	if bytes.Contains(got, []byte("/Metadata 7 0 R")) {
		t.Errorf("old /Metadata 7 0 R should have been replaced: %q", string(got))
	}
}

func TestInjectXMPStream_EndToEnd(t *testing.T) {
	pdf := minimalPDF()
	xmp := BuildXMPPacket(XMPSpec{
		Title:    "Test Doc",
		Author:   "PMForge Tests",
		Subject:  "unit test fixture",
		Keywords: []string{"alpha", "beta"},
	})

	out, err := InjectXMPStream(pdf, xmp)
	if err != nil {
		t.Fatalf("InjectXMPStream: %v", err)
	}

	// 1. Output should be strictly longer than input — we APPEND, never overwrite.
	if len(out) <= len(pdf) {
		t.Fatalf("output (%d bytes) not longer than input (%d bytes)", len(out), len(pdf))
	}

	// 2. Original input must appear verbatim at the start.
	if !bytes.HasPrefix(out, pdf) {
		t.Fatal("original PDF bytes were modified instead of appended-to")
	}

	// 3. Output must end with %%EOF.
	if !bytes.HasSuffix(bytes.TrimRight(out, "\n"), []byte("%%EOF")) {
		t.Errorf("output does not end with %%EOF: last 40 bytes = %q", string(out[len(out)-40:]))
	}

	// 4. The appended bytes must contain the XMP packet.
	appended := out[len(pdf):]
	if !bytes.Contains(appended, []byte("<?xpacket begin")) {
		t.Error("appended bytes missing XMP packet")
	}

	// 5. Appended bytes must include the new Metadata object (4 0 obj) and the rewritten Catalog (1 0 obj).
	if !bytes.Contains(appended, []byte("4 0 obj")) {
		t.Error("appended bytes missing new Metadata object header")
	}
	if !bytes.Contains(appended, []byte("1 0 obj")) {
		t.Error("appended bytes missing rewritten Catalog object")
	}
	if !bytes.Contains(appended, []byte("/Metadata 4 0 R")) {
		t.Error("appended bytes missing /Metadata reference in Catalog")
	}

	// 6. The new trailer must reference the previous xref via /Prev.
	if !bytes.Contains(appended, []byte("/Prev ")) {
		t.Error("appended trailer missing /Prev for incremental update")
	}

	// 7. New startxref offset must be parseable and point at the new xref.
	newXrefOff, err := findLastStartxref(out)
	if err != nil {
		t.Fatalf("findLastStartxref on output: %v", err)
	}
	if newXrefOff <= len(pdf) {
		t.Errorf("new startxref %d should be after original input (%d)", newXrefOff, len(pdf))
	}
	if !bytes.HasPrefix(out[newXrefOff:], []byte("xref")) {
		t.Errorf("new startxref %d does not point at 'xref'; got %q", newXrefOff, string(out[newXrefOff:newXrefOff+8]))
	}

	// 8. New /Size in the appended trailer must be old + 1 (= 5).
	size, root, _, err := parseTrailerSizeAndRoot(out, newXrefOff)
	if err != nil {
		t.Fatalf("parseTrailerSizeAndRoot on output: %v", err)
	}
	if size != 5 {
		t.Errorf("new /Size: got %d, want 5", size)
	}
	if root != 1 {
		t.Errorf("/Root id: got %d, want 1", root)
	}
}

func TestInjectXMPStream_RejectsEmpty(t *testing.T) {
	if _, err := InjectXMPStream(nil, []byte("xmp")); err == nil {
		t.Error("expected error on empty PDF")
	}
	if _, err := InjectXMPStream(minimalPDF(), nil); err == nil {
		t.Error("expected error on empty XMP")
	}
}

func TestBuildXMPPacket_ContainsRequiredFields(t *testing.T) {
	pkt := BuildXMPPacket(XMPSpec{
		Title:   "Sample",
		Author:  "Sam",
		Subject: "S",
	})
	s := string(pkt)
	for _, want := range []string{
		"<?xpacket begin",
		"<?xpacket end",
		"<x:xmpmeta",
		"<dc:title>",
		"<dc:creator>",
		"<pdfaid:part>3</pdfaid:part>",
		"<pdfaid:conformance>B</pdfaid:conformance>",
		"Sample",
		"Sam",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("BuildXMPPacket missing required substring %q", want)
		}
	}
}
