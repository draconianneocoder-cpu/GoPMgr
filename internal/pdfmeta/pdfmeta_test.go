// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
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
	"unicode/utf16"

	"github.com/digitorus/pkcs7"
	"github.com/go-pdf/fpdf"

	pmcrypto "gopmgr/internal/crypto"
)

// minimalPDF returns a syntactically-valid 3-object PDF byte stream
// that exercises the same shape as fpdf output: header marker,
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
	fmt.Fprintf(&b,
		"3 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n",
		len(fakeContent), fakeContent)

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

// TestInsertOutputIntentsReference_TrimsLeadingWhitespace covers the
// leading-whitespace trim loop's body, which the package's other tests
// never exercise (their fixture catalogs start with "<<" directly, no
// leading whitespace).
func TestInsertOutputIntentsReference_TrimsLeadingWhitespace(t *testing.T) {
	orig := []byte("  \n\t<< /Type /Catalog /Pages 2 0 R >>")
	got := insertOutputIntentsReference(orig, 99)
	if !bytes.Contains(got, []byte("/OutputIntents [ 99 0 R ]")) {
		t.Fatalf("expected /OutputIntents insertion, got %q", string(got))
	}
	if !bytes.Contains(got, []byte("/Pages 2 0 R")) {
		t.Errorf("lost /Pages entry: %q", string(got))
	}
}

// TestInsertOutputIntentsReference_WrapsNonDictBody covers the "malformed
// input" branch: catalogBody doesn't start with "<<", so the function
// wraps it in a fresh dictionary rather than trying to edit it in place.
func TestInsertOutputIntentsReference_WrapsNonDictBody(t *testing.T) {
	orig := []byte("/Type /Catalog")
	got := insertOutputIntentsReference(orig, 42)
	if !bytes.HasPrefix(got, []byte("<<")) || !bytes.HasSuffix(got, []byte(">>")) {
		t.Fatalf("expected the result to be wrapped in a fresh dict, got %q", string(got))
	}
	if !bytes.Contains(got, []byte("/OutputIntents [ 42 0 R ]")) {
		t.Errorf("expected /OutputIntents insertion, got %q", string(got))
	}
	if !bytes.Contains(got, orig) {
		t.Errorf("original body should be preserved inside the wrapper, got %q", string(got))
	}
}

// TestInsertOutputIntentsReference_ReplacesExistingArrayValue covers the
// "already has an /OutputIntents array" branch's array-skipping loop: it
// must track bracket depth to find the array's closing "]", not just the
// first one (real OutputIntents arrays are one level deep, but the depth
// counter exists precisely so a "]" belonging to something else in a
// deeper structure can't be mistaken for the end).
func TestInsertOutputIntentsReference_ReplacesExistingArrayValue(t *testing.T) {
	orig := []byte("<< /Type /Catalog /OutputIntents [ 3 0 R ] /Pages 2 0 R >>")
	got := insertOutputIntentsReference(orig, 7)
	if n := bytes.Count(got, []byte("/OutputIntents")); n != 1 {
		t.Fatalf("expected exactly one /OutputIntents key, got %d in %q", n, string(got))
	}
	if !bytes.Contains(got, []byte("/OutputIntents [ 7 0 R ]")) {
		t.Errorf("expected the new reference, got %q", string(got))
	}
	if bytes.Contains(got, []byte("[ 3 0 R ]")) {
		t.Errorf("old array value should have been replaced, got %q", string(got))
	}
	if !bytes.Contains(got, []byte("/Pages 2 0 R")) {
		t.Errorf("lost /Pages entry: %q", string(got))
	}
}

// TestInsertOutputIntentsReference_ReplacesExistingIndirectRefValue covers
// the array-skipping loop's OTHER exit: an /OutputIntents value that is a
// bare indirect reference ("3 0 R"), not an array, so the loop never sees
// a "[" and must instead stop at the next key's leading "/" — the
// depth == 0 && trimmed[end] == '/' arm, distinct from the "]" arm above.
func TestInsertOutputIntentsReference_ReplacesExistingIndirectRefValue(t *testing.T) {
	orig := []byte("<< /Type /Catalog /OutputIntents 3 0 R /Pages 2 0 R >>")
	got := insertOutputIntentsReference(orig, 11)
	if n := bytes.Count(got, []byte("/OutputIntents")); n != 1 {
		t.Fatalf("expected exactly one /OutputIntents key, got %d in %q", n, string(got))
	}
	if !bytes.Contains(got, []byte("/OutputIntents [ 11 0 R ]")) {
		t.Errorf("expected the new reference, got %q", string(got))
	}
	if bytes.Contains(got, []byte("OutputIntents 3 0 R")) {
		t.Errorf("old indirect-ref value should have been replaced, got %q", string(got))
	}
	if !bytes.Contains(got, []byte("/Pages 2 0 R")) {
		t.Errorf("lost /Pages entry: %q", string(got))
	}
}

// TestInsertOutputIntentsReference_ReplacesValueEndingAtDictClose covers
// the array-skipping loop's third exit arm: an /OutputIntents value
// immediately followed by the dict's closing ">>" with no further key —
// depth == 0 && trimmed[end] == '>' && the next char is also '>'.
func TestInsertOutputIntentsReference_ReplacesValueEndingAtDictClose(t *testing.T) {
	orig := []byte("<< /Type /Catalog /OutputIntents 3 0 R>>")
	got := insertOutputIntentsReference(orig, 5)
	if !bytes.Contains(got, []byte("/OutputIntents [ 5 0 R ]")) {
		t.Fatalf("expected the new reference, got %q", string(got))
	}
	if !bytes.HasSuffix(bytes.TrimSpace(got), []byte(">>")) {
		t.Errorf("expected the dict to still close properly, got %q", string(got))
	}
}

// TestInsertOutputIntentsReference_HandlesUnterminatedArray pins defined
// behavior for a malformed catalog whose /OutputIntents array never
// closes: the array-skipping loop must exit via its "end < len(trimmed)"
// condition going false, not via any of its three break arms, and the
// function must still return without panicking.
func TestInsertOutputIntentsReference_HandlesUnterminatedArray(t *testing.T) {
	orig := []byte("<< /Type /Catalog /OutputIntents [ 3 0 R")
	got := insertOutputIntentsReference(orig, 13)
	if !bytes.Contains(got, []byte("/OutputIntents [ 13 0 R ]")) {
		t.Fatalf("expected the new reference even for malformed input, got %q", string(got))
	}
}

// TestInsertOutputIntentsReference_InsertsFreshEntry covers the "no
// existing /OutputIntents key" branch (the common case, already exercised
// indirectly by TestMakePDFA3PreservesMetadataAndOutputIntentInLatestCatalog
// below). This is a coverage test, not a guard-presence one: the branch is
// this function's final statement with no following code, so deleting it
// is a "missing return" compile error — there's no mutation to run.
func TestInsertOutputIntentsReference_InsertsFreshEntry(t *testing.T) {
	orig := []byte("<< /Type /Catalog /Pages 2 0 R >>")
	got := insertOutputIntentsReference(orig, 9)
	if !bytes.Contains(got, []byte("/OutputIntents [ 9 0 R ]")) {
		t.Fatalf("expected /OutputIntents insertion, got %q", string(got))
	}
	if !bytes.Contains(got, []byte("/Pages 2 0 R")) {
		t.Errorf("lost /Pages entry: %q", string(got))
	}
}

func TestInjectXMPStream_EndToEnd(t *testing.T) {
	pdf := minimalPDF()
	xmp := BuildXMPPacket(XMPSpec{
		Title:    "Test Doc",
		Author:   "GoPMgr Tests",
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
	out, err := MakePDFA3(pdf, XMPSpec{Title: "PDF/A sample", Author: "GoPMgr"}, []byte("fake-icc-profile"))
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

func TestInjectPAdESSignature_ReservesRoomForTimestampedCMS(t *testing.T) {
	t.Parallel()

	const representativeTimestampedCMSBytes = 24 * 1024
	out, err := InjectPAdESSignature(minimalPDF(), func([]byte) ([]byte, error) {
		// Real TSA chains vary substantially in size. This regression is larger
		// than the former 8 KiB slot and proves the reserved PAdES-T capacity is
		// available before the ByteRange is signed.
		return bytes.Repeat([]byte{0xa5}, representativeTimestampedCMSBytes), nil
	})
	if err != nil {
		t.Fatalf("InjectPAdESSignature() error = %v", err)
	}

	byteRange := parseByteRangeForTest(t, out)
	if got := byteRange[2] - byteRange[1] - 2; got < representativeTimestampedCMSBytes*2 {
		t.Fatalf("hex Contents capacity = %d, want at least %d", got, representativeTimestampedCMSBytes*2)
	}
}

func TestInjectPAdESSignature_RejectsCMSLargerThanReservedCapacity(t *testing.T) {
	t.Parallel()

	_, err := InjectPAdESSignature(minimalPDF(), func([]byte) ([]byte, error) {
		return make([]byte, 32*1024+1), nil
	})
	if err == nil || !strings.Contains(err.Error(), "too large for placeholder") {
		t.Fatalf("InjectPAdESSignature() error = %v, want capacity rejection", err)
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
	signer := newTestPAdESSigner(t, "GoPMgr PAdES Integration Signer")

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
		[]byte("/Name (GoPMgr Digital Signature)"),
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

// ----- xmlEscape -----

func TestXmlEscape_Empty(t *testing.T) {
	if got := xmlEscape(""); got != "" {
		t.Errorf("xmlEscape(\"\") = %q, want empty", got)
	}
}

func TestXmlEscape_AllSpecialChars(t *testing.T) {
	got := xmlEscape(`&<>"'`)
	want := "&amp;&lt;&gt;&quot;&apos;"
	if got != want {
		t.Errorf("xmlEscape = %q, want %q", got, want)
	}
}

func TestXmlEscape_NoSpecialChars(t *testing.T) {
	in := "hello world 123"
	if got := xmlEscape(in); got != in {
		t.Errorf("xmlEscape(%q) = %q, want passthrough", in, got)
	}
}

func TestXmlEscape_Mixed(t *testing.T) {
	got := xmlEscape("a&b<c")
	want := "a&amp;b&lt;c"
	if got != want {
		t.Errorf("xmlEscape = %q, want %q", got, want)
	}
}

// ----- DefaultICCProfile / HasDefaultICC -----

func TestDefaultICCProfile_NonNil(t *testing.T) {
	got := DefaultICCProfile()
	if len(got) == 0 {
		t.Error("DefaultICCProfile() returned nil or empty slice")
	}
}

func TestDefaultICCProfile_ReturnsCopy(t *testing.T) {
	a := DefaultICCProfile()
	a[0] ^= 0xFF
	b := DefaultICCProfile()
	if b[0] == a[0] {
		t.Error("DefaultICCProfile() returned a reference to the embedded slice, not a copy")
	}
}

func TestHasDefaultICC_ReturnsTrue(t *testing.T) {
	if !HasDefaultICC() {
		t.Error("HasDefaultICC() = false, want true")
	}
}

// utf16beString encodes s as UTF-16BE with a leading BOM (0xFEFF), the
// exact form fpdf.SetTitle/SetAuthor/etc. store internally when called
// with isUTF8=true (see go-pdf/fpdf's utf8toutf16). ApplyPDFAMetadata
// always passes true, and fpdf's own putinfo() writes each field as
// "/Key (<those bytes>)" with the value immediately adjacent to the key —
// confirmed by direct inspection of fpdf@v0.9.0's putinfo/textstring
// before relying on it — so "/Key (" + utf16beString(want) is a reliable,
// specific discriminator: a bare bytes.Contains(output, utf16beString(x))
// would also match if x appears as some OTHER field's value.
func utf16beString(s string) []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0xFE, 0xFF})
	for _, r := range utf16.Encode([]rune(s)) {
		buf.WriteByte(byte(r >> 8))
		buf.WriteByte(byte(r))
	}
	return buf.Bytes()
}

// renderMinimalPDF applies fn to a fresh single-page fpdf document and
// returns the rendered bytes, for tests that need to inspect what
// ApplyPDFAMetadata actually wrote into the PDF's /Info dictionary.
func renderMinimalPDF(t *testing.T, fn func(*fpdf.Fpdf)) []byte {
	t.Helper()
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	fn(pdf)
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		t.Fatalf("render PDF: %v", err)
	}
	return buf.Bytes()
}

// TestApplyPDFAMetadata_NilPDFIsNoop covers the pdf == nil guard. Unlike
// this package's error-returning functions, this guard break-verifies via
// a PANIC (a nil-pointer method call), not differing error text: deleting
// it and calling pdf.SetTitle on a nil *fpdf.Fpdf crashes the test.
func TestApplyPDFAMetadata_NilPDFIsNoop(t *testing.T) {
	ApplyPDFAMetadata(nil, XMPSpec{Title: "should not panic"})
}

// TestApplyPDFAMetadata_SetsAllFieldsWhenProvided pins that every
// caller-supplied field lands in the PDF's /Info dictionary. Creator is
// always "GoPMgr" regardless of spec — ApplyPDFAMetadata hardcodes it and
// ignores XMPSpec.CreatorTool entirely, unlike BuildXMPPacket (see
// pdfmeta.go), which does honor CreatorTool for the XMP packet's
// <xmp:CreatorTool> element. This is an observed inconsistency between
// the two metadata surfaces, not verified as intentional; flagged for a
// follow-up rather than silently pinned as if it were the specified
// contract.
func TestApplyPDFAMetadata_SetsAllFieldsWhenProvided(t *testing.T) {
	b := renderMinimalPDF(t, func(pdf *fpdf.Fpdf) {
		ApplyPDFAMetadata(pdf, XMPSpec{
			Title:    "My Title",
			Subject:  "My Subject",
			Author:   "Alice",
			Keywords: []string{"one", "two", "three"},
		})
	})
	for key, want := range map[string]string{
		"/Title":    "My Title",
		"/Subject":  "My Subject",
		"/Author":   "Alice",
		"/Creator":  "GoPMgr",
		"/Keywords": "one, two, three",
	} {
		needle := append([]byte(key+" ("), utf16beString(want)...)
		if !bytes.Contains(b, needle) {
			t.Errorf("%s: want value %q adjacent to the key, not found", key, want)
		}
	}
}

// TestApplyPDFAMetadata_DefaultsAuthorWhenEmpty covers the
// "spec.Author == ”" default branch. Asserting a bare
// bytes.Contains(b, utf16beString("GoPMgr")) would be masked: Creator is
// unconditionally set to "GoPMgr" a few lines later regardless of this
// guard, so that substring is always present either way. The key-adjacent
// needle ("/Author (" + the encoded bytes) discriminates: with the guard
// present, /Author's value encodes "GoPMgr"; with it deleted, SetAuthor("")
// still emits an /Author key (fpdf's UTF-16 encoding of "" is a 2-byte BOM,
// which the field-non-empty check on the fpdf side still treats as
// present) but with an empty value — confirmed by direct inspection of
// fpdf's putinfo/SetAuthor before relying on it, not assumed.
func TestApplyPDFAMetadata_DefaultsAuthorWhenEmpty(t *testing.T) {
	b := renderMinimalPDF(t, func(pdf *fpdf.Fpdf) {
		ApplyPDFAMetadata(pdf, XMPSpec{})
	})
	needle := append([]byte("/Author ("), utf16beString("GoPMgr")...)
	if !bytes.Contains(b, needle) {
		t.Error("expected /Author to default to \"GoPMgr\" when unset")
	}
}

// TestApplyPDFAMetadata_OmitsTitleAndSubjectWhenEmpty covers the
// "spec.Title/Subject != ”" guards. Both break-verify via key absence,
// not value comparison: SetTitle("")/SetSubject("") would still each
// leave a non-empty (2-byte BOM) internal string on the fpdf side, so
// putinfo would still emit an empty "/Title ()"/"/Subject ()" key if these
// guards were deleted — only the key's total absence distinguishes
// "guard ran and skipped the call" from "guard was deleted."
func TestApplyPDFAMetadata_OmitsTitleAndSubjectWhenEmpty(t *testing.T) {
	b := renderMinimalPDF(t, func(pdf *fpdf.Fpdf) {
		ApplyPDFAMetadata(pdf, XMPSpec{})
	})
	if bytes.Contains(b, []byte("/Title (")) {
		t.Error("expected no /Title key when spec.Title is empty")
	}
	if bytes.Contains(b, []byte("/Subject (")) {
		t.Error("expected no /Subject key when spec.Subject is empty")
	}
}

// TestApplyPDFAMetadata_SingleKeywordSkipsJoinLoop pins that a
// single-element Keywords slice doesn't pick up a spurious leading
// separator from the join loop's zero-iteration case
// (spec.Keywords[1:] on a 1-element slice is empty).
func TestApplyPDFAMetadata_SingleKeywordSkipsJoinLoop(t *testing.T) {
	b := renderMinimalPDF(t, func(pdf *fpdf.Fpdf) {
		ApplyPDFAMetadata(pdf, XMPSpec{Keywords: []string{"only"}})
	})
	needle := append([]byte("/Keywords ("), utf16beString("only")...)
	if !bytes.Contains(b, needle) {
		t.Error("expected /Keywords to contain the single keyword unmodified")
	}
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
