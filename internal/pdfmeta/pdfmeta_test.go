// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package pdfmeta

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/hex"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/digitorus/pkcs7"

	pmcrypto "pmforge/internal/crypto"
)

// minimalPDF returns a syntactically-valid 3-object PDF byte stream
// that exercises the same shape as gofpdf output: header marker,
// Catalog object, Pages object, content object, xref, trailer,
// startxref, EOF.
//
// The Catalog (object 1) references Pages (object 2). Object 3 is a
// content stream stand-in.
func minimalPDF() []byte {
	return minimalPDFWithCatalog("<< /Type /Catalog /Pages 2 0 R >>")
}

func minimalPDFWithoutBinaryComment() []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")

	obj1Off := b.Len()
	b.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")
	obj2Off := b.Len()
	b.WriteString("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")
	obj3Off := b.Len()
	const fakeContent = "stream-data\n"
	fmt.Fprintf(&b, "3 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n", len(fakeContent), fakeContent)

	xrefOff := b.Len()
	b.WriteString("xref\n0 4\n")
	b.WriteString("0000000000 65535 f \n")
	fmt.Fprintf(&b, "%010d 00000 n \n", obj1Off)
	fmt.Fprintf(&b, "%010d 00000 n \n", obj2Off)
	fmt.Fprintf(&b, "%010d 00000 n \n", obj3Off)
	fmt.Fprintf(&b, "trailer\n<<\n/Size 4\n/Root 1 0 R\n>>\nstartxref\n%d\n%%%%EOF\n", xrefOff)
	return b.Bytes()
}

type testPDFObject struct {
	id   int
	gen  int
	body string
}

func minimalPDFWithCatalog(catalogBody string, extraObjects ...testPDFObject) []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")

	offsets := map[int]int{}
	gens := map[int]int{}

	// Object 1: Catalog
	offsets[1] = b.Len()
	gens[1] = 0
	fmt.Fprintf(&b, "1 0 obj\n%s\nendobj\n", catalogBody)

	// Object 2: Pages
	offsets[2] = b.Len()
	gens[2] = 0
	b.WriteString("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")

	// Object 3: a stub content stream so we can test that "1 0 obj"
	// inside arbitrary bytes doesn't fool findObjectBody.
	offsets[3] = b.Len()
	gens[3] = 0
	const fakeContent = "stream-data: looks like 1 0 obj but isn't\n"
	b.WriteString(fmt.Sprintf(
		"3 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n",
		len(fakeContent), fakeContent))

	maxID := 3
	for _, obj := range extraObjects {
		if obj.id > maxID {
			maxID = obj.id
		}
		offsets[obj.id] = b.Len()
		gens[obj.id] = obj.gen
		fmt.Fprintf(&b, "%d %d obj\n%s\nendobj\n", obj.id, obj.gen, obj.body)
	}

	// xref
	xrefOff := b.Len()
	b.WriteString("xref\n")
	fmt.Fprintf(&b, "0 %d\n", maxID+1)
	b.WriteString("0000000000 65535 f \n")
	for id := 1; id <= maxID; id++ {
		off, ok := offsets[id]
		if !ok {
			b.WriteString("0000000000 65535 f \n")
			continue
		}
		fmt.Fprintf(&b, "%010d %05d n \n", off, gens[id])
	}

	// trailer + startxref + EOF
	fmt.Fprintf(&b, "trailer\n<<\n/Size %d\n/Root 1 0 R\n>>\n", maxID+1)
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

func TestFindObjectBody_ReturnsLatestIncrementalRevision(t *testing.T) {
	pdf := append([]byte(nil), minimalPDF()...)
	pdf = append(pdf, []byte("\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R /Metadata 4 0 R >>\nendobj\n")...)

	body, err := findObjectBody(pdf, 1, 0)
	if err != nil {
		t.Fatalf("findObjectBody(1, 0): %v", err)
	}
	if !bytes.Contains(body, []byte("/Metadata 4 0 R")) {
		t.Fatalf("expected latest Catalog revision with /Metadata, got %q", string(body))
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

func TestMakePDFA3PreservesMetadataAndOutputIntentInLatestCatalog(t *testing.T) {
	pdf := minimalPDFWithoutBinaryComment()
	out, err := MakePDFA3(pdf, XMPSpec{Title: "PDF/A sample", Author: "PMForge"}, []byte("fake-icc-profile"))
	if err != nil {
		t.Fatalf("MakePDFA3: %v", err)
	}

	headerEnd := bytes.IndexByte(out, '\n')
	if headerEnd < 0 || !hasBinaryHeaderComment(out[headerEnd+1:]) {
		t.Fatalf("PDF/A output missing binary header comment: %q", out[:min(len(out), 32)])
	}

	xrefOff, err := findLastStartxref(out)
	if err != nil {
		t.Fatalf("findLastStartxref: %v", err)
	}
	_, root, gen, err := parseTrailerSizeAndRoot(out, xrefOff)
	if err != nil {
		t.Fatalf("parseTrailerSizeAndRoot: %v", err)
	}
	catalog, err := findObjectBody(out, root, gen)
	if err != nil {
		t.Fatalf("findObjectBody(root): %v", err)
	}
	if !bytes.Contains(catalog, []byte("/Metadata ")) {
		t.Fatalf("latest Catalog missing /Metadata: %q", string(catalog))
	}
	if !bytes.Contains(catalog, []byte("/OutputIntents ")) {
		t.Fatalf("latest Catalog missing /OutputIntents: %q", string(catalog))
	}
	if _, ok := readTrailerIDValue(out, xrefOff); !ok {
		t.Fatal("final PDF/A trailer missing /ID")
	}
	if !bytes.HasPrefix(out[xrefOff:], []byte("xref")) {
		t.Fatalf("startxref does not point at xref after binary header insertion")
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

func TestInjectPAdESSignature_SignsDeclaredByteRange(t *testing.T) {
	pdf := minimalPDF()
	var signedInput []byte

	out, err := InjectPAdESSignature(pdf, func(b []byte) ([]byte, error) {
		signedInput = append([]byte(nil), b...)
		return []byte{0xde, 0xad, 0xbe, 0xef}, nil
	})
	if err != nil {
		t.Fatalf("InjectPAdESSignature: %v", err)
	}
	if len(signedInput) == 0 {
		t.Fatal("signing callback was not invoked")
	}

	br := parseByteRangeForTest(t, out)
	if br[0] != 0 {
		t.Fatalf("ByteRange starts at %d, want 0", br[0])
	}
	if br[1] <= 0 || br[2] <= br[1] || br[3] <= 0 {
		t.Fatalf("ByteRange values are not plausible: %v", br)
	}
	if br[2]+br[3] != len(out) {
		t.Fatalf("ByteRange does not reach EOF: %v over %d-byte PDF", br, len(out))
	}

	declared := make([]byte, 0, br[1]+br[3])
	declared = append(declared, out[br[0]:br[0]+br[1]]...)
	declared = append(declared, out[br[2]:br[2]+br[3]]...)
	if !bytes.Equal(signedInput, declared) {
		t.Fatalf("signed bytes do not match declared ByteRange: signed %d bytes, declared %d bytes", len(signedInput), len(declared))
	}

	contents := out[br[1]:br[2]]
	if !bytes.HasPrefix(contents, []byte("<deadbeef")) {
		t.Fatalf("Contents range does not start with encoded CMS signature: %.32q", contents)
	}
	if !bytes.HasSuffix(contents, []byte(">")) {
		t.Fatalf("Contents range does not end at closing hex delimiter: %.32q", contents[len(contents)-32:])
	}
}

func TestInjectPAdESSignature_IncludesSignedModificationTime(t *testing.T) {
	out, err := InjectPAdESSignature(minimalPDF(), func([]byte) ([]byte, error) {
		return []byte{0xca, 0xfe}, nil
	})
	if err != nil {
		t.Fatalf("InjectPAdESSignature: %v", err)
	}

	modTime := regexp.MustCompile(`/M \(D:\d{14}Z\)`).Find(out)
	if modTime == nil {
		t.Fatalf("signed PDF missing PAdES signature dictionary /M timestamp:\n%s", string(out))
	}

	br := parseByteRangeForTest(t, out)
	if !bytes.Contains(byteRangeBytesForTest(t, out, br), modTime) {
		t.Fatalf("/M timestamp %q is not covered by the declared ByteRange", string(modTime))
	}
}

func TestInjectPAdESSignature_EmbeddedCMSVerifiesDeclaredByteRange(t *testing.T) {
	pdf := minimalPDFWithCatalog(
		"<< /Type /Catalog /Pages 2 0 R >>",
		testPDFObject{id: 4, body: "<< /Type /Example /Contents <00112233> /ByteRange [1 2 3 4] >>"},
	)
	signer := newTestPAdESSigner(t, "PMForge PAdES Integration Signer")

	out, err := InjectPAdESSignature(pdf, signer.SignPDFCMS)
	if err != nil {
		t.Fatalf("InjectPAdESSignature: %v", err)
	}

	br := parseByteRangeForTest(t, out)
	p7 := parsePAdESSignedDataForTest(t, out, br)

	p7.Content = byteRangeBytesForTest(t, out, br)
	if err := p7.Verify(); err != nil {
		t.Fatalf("embedded CMS does not verify declared ByteRange: %v", err)
	}

	tampered := append([]byte(nil), out...)
	tampered[0] ^= 0x01
	p7.Content = byteRangeBytesForTest(t, tampered, br)
	if err := p7.Verify(); err == nil {
		t.Fatal("expected embedded CMS verification to fail after tampering with signed bytes")
	}
}

func TestInjectPAdESSignature_EmbedsInvisibleSignatureWidget(t *testing.T) {
	out, err := InjectPAdESSignature(minimalPDF(), func([]byte) ([]byte, error) {
		return []byte{0xca, 0xfe}, nil
	})
	if err != nil {
		t.Fatalf("InjectPAdESSignature: %v", err)
	}

	for _, want := range [][]byte{
		[]byte("/Type /Annot"),
		[]byte("/Subtype /Widget"),
		[]byte("/FT /Sig"),
		[]byte("/Rect [0 0 0 0]"),
		[]byte("/V 4 0 R"),
		[]byte("/AcroForm << /Fields [ 5 0 R ] /SigFlags 3 >>"),
		[]byte("/Name (PMForge Digital Signature)"),
	} {
		if !bytes.Contains(out, want) {
			t.Fatalf("signed PDF missing %q", string(want))
		}
	}
}

func TestInjectPAdESSignature_AppendsExistingInlineAcroFormFields(t *testing.T) {
	pdf := minimalPDFWithCatalog("<< /Type /Catalog /Pages 2 0 R /AcroForm << /Fields [ 3 0 R ] /SigFlags 1 >> >>")
	out, err := InjectPAdESSignature(pdf, func([]byte) ([]byte, error) {
		return []byte{0xca, 0xfe}, nil
	})
	if err != nil {
		t.Fatalf("InjectPAdESSignature: %v", err)
	}

	appended := out[len(pdf):]
	for _, want := range [][]byte{
		[]byte("/AcroForm << /Fields [ 3 0 R 5 0 R ] /SigFlags 3 >>"),
		[]byte("/Subtype /Widget"),
		[]byte("/V 4 0 R"),
	} {
		if !bytes.Contains(appended, want) {
			t.Fatalf("signed PDF missing %q in appended update:\n%s", string(want), string(appended))
		}
	}
}

func TestInjectPAdESSignature_AppendsExistingIndirectAcroFormFields(t *testing.T) {
	pdf := minimalPDFWithCatalog(
		"<< /Type /Catalog /Pages 2 0 R /AcroForm 4 0 R >>",
		testPDFObject{id: 4, body: "<< /Fields [ 3 0 R ] /SigFlags 1 >>"},
	)
	out, err := InjectPAdESSignature(pdf, func([]byte) ([]byte, error) {
		return []byte{0xca, 0xfe}, nil
	})
	if err != nil {
		t.Fatalf("InjectPAdESSignature: %v", err)
	}

	appended := out[len(pdf):]
	for _, want := range [][]byte{
		[]byte("4 0 obj\n<< /Fields [ 3 0 R 6 0 R ] /SigFlags 3 >>"),
		[]byte("4 1\n"),
		[]byte("/Subtype /Widget"),
		[]byte("/V 5 0 R"),
	} {
		if !bytes.Contains(appended, want) {
			t.Fatalf("signed PDF missing %q in appended update:\n%s", string(want), string(appended))
		}
	}
}

func TestInjectPAdESSignature_RejectsUnsupportedIndirectFieldsArray(t *testing.T) {
	pdf := minimalPDFWithCatalog(
		"<< /Type /Catalog /Pages 2 0 R /AcroForm 4 0 R >>",
		testPDFObject{id: 4, body: "<< /Fields 7 0 R /SigFlags 1 >>"},
	)
	called := false
	_, err := InjectPAdESSignature(pdf, func([]byte) ([]byte, error) {
		called = true
		return []byte{0xca, 0xfe}, nil
	})
	if err == nil {
		t.Fatal("expected unsupported indirect /Fields array to fail")
	}
	if !strings.Contains(err.Error(), "AcroForm /Fields is not a direct array") {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Fatal("signing callback should not run after an unsupported AcroForm merge")
	}
}

func parseByteRangeForTest(t *testing.T, pdf []byte) [4]int {
	t.Helper()
	const marker = "/ByteRange ["
	idx := bytes.LastIndex(pdf, []byte(marker))
	if idx < 0 {
		t.Fatal("PDF missing /ByteRange")
	}
	start := idx + len(marker)
	endRel := bytes.IndexByte(pdf[start:], ']')
	if endRel < 0 {
		t.Fatal("PDF missing closing ] for /ByteRange")
	}
	fields := strings.Fields(string(pdf[start : start+endRel]))
	if len(fields) != 4 {
		t.Fatalf("ByteRange field count: got %d fields %v, want 4", len(fields), fields)
	}
	var out [4]int
	for i, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil {
			t.Fatalf("ByteRange field %q is not an integer: %v", f, err)
		}
		out[i] = n
	}
	return out
}

func byteRangeBytesForTest(t *testing.T, pdf []byte, br [4]int) []byte {
	t.Helper()
	if br[0] < 0 || br[1] < 0 || br[2] < 0 || br[3] < 0 {
		t.Fatalf("ByteRange contains negative values: %v", br)
	}
	if br[0]+br[1] > len(pdf) || br[2]+br[3] > len(pdf) {
		t.Fatalf("ByteRange %v extends past %d-byte PDF", br, len(pdf))
	}
	out := make([]byte, 0, br[1]+br[3])
	out = append(out, pdf[br[0]:br[0]+br[1]]...)
	out = append(out, pdf[br[2]:br[2]+br[3]]...)
	return out
}

func parsePAdESSignedDataForTest(t *testing.T, pdf []byte, br [4]int) *pkcs7.PKCS7 {
	t.Helper()
	if br[1] >= br[2] || br[1] < 0 || br[2] > len(pdf) {
		t.Fatalf("ByteRange does not enclose /Contents: %v over %d-byte PDF", br, len(pdf))
	}
	if pdf[br[1]] != '<' || pdf[br[2]-1] != '>' {
		t.Fatalf("ByteRange gap is not a hex string: prefix=%q suffix=%q", pdf[br[1]], pdf[br[2]-1])
	}

	contentsHex := pdf[br[1]+1 : br[2]-1]
	contents := make([]byte, hex.DecodedLen(len(contentsHex)))
	n, err := hex.Decode(contents, contentsHex)
	if err != nil {
		t.Fatalf("decode /Contents hex: %v", err)
	}
	contents = contents[:n]

	var raw asn1.RawValue
	rest, err := asn1.Unmarshal(contents, &raw)
	if err != nil {
		t.Fatalf("decode CMS DER from padded /Contents: %v", err)
	}
	for _, b := range rest {
		if b != 0 {
			t.Fatalf("non-zero data after CMS DER in padded /Contents")
		}
	}

	p7, err := pkcs7.Parse(raw.FullBytes)
	if err != nil {
		t.Fatalf("parse embedded CMS: %v", err)
	}
	if len(p7.Content) != 0 {
		t.Fatalf("embedded CMS must be detached; parsed content length = %d", len(p7.Content))
	}
	return p7
}

func newTestPAdESSigner(t *testing.T, commonName string) *pmcrypto.Signer {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}

	now := time.Now().UTC()
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(now.UnixNano()),
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}

	return &pmcrypto.Signer{
		Cert:       cert,
		PrivateKey: key,
	}
}
