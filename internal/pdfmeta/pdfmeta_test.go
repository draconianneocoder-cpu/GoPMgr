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
	"math"
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

// TestParseDigits_WithinRange covers the ordinary path: a digit run
// well within int's range accumulates to the expected value.
func TestParseDigits_WithinRange(t *testing.T) {
	n, err := parseDigits([]byte("42"))
	if err != nil {
		t.Fatalf("parseDigits: %v", err)
	}
	if n != 42 {
		t.Errorf("got %d, want 42", n)
	}
}

// TestParseDigits_OverflowReturnsError covers parseDigits' only
// failure mode. Every digit-run accumulator in this file used to be a
// bare `n = n*10 + int(c-'0')` loop with no bounds check, so a long
// enough digit run would silently wrap to a wrong (often negative)
// value instead of erroring — found while covering
// ensureSignatureFieldFlags's /SigFlags parsing (confirmed directly:
// `ensureSignatureFieldFlags([]byte("<< /SigFlags
// 99999999999999999999 >>"))` returned err=nil and wrote
// "/SigFlags 7766279631452241919" before this fix) and swept to every
// accumulation site in the file (findLastStartxref, readDictInt,
// readDictRef, parsePositiveDecimal, ensureSignatureFieldFlags,
// readRefAt), not just the one found first. Confirmed by mutation:
// reverting to the raw loop makes this fixture return (n, nil) with a
// wrapped value instead of an error.
func TestParseDigits_OverflowReturnsError(t *testing.T) {
	_, err := parseDigits([]byte("99999999999999999999"))
	if err == nil {
		t.Fatal("expected an error for a digit run that overflows int")
	}
	if !strings.Contains(err.Error(), "overflows") {
		t.Errorf("expected the guard's own error text, got %q", err)
	}
}

// TestParseDigits_BoundaryValues pins the overflow guard's off-by-one
// behavior at math.MaxInt (9223372036854775807 on a 64-bit build): the
// guard's division-based check (`n > (math.MaxInt-d)/10`) is easy to
// get wrong by one at exactly this boundary, and the other overflow
// test's fixture (20 nines) overflows by orders of magnitude — it
// would not catch an off-by-one here. MaxInt itself must parse
// cleanly; MaxInt+1 must error.
func TestParseDigits_BoundaryValues(t *testing.T) {
	n, err := parseDigits([]byte("9223372036854775807"))
	if err != nil {
		t.Fatalf("parseDigits(MaxInt): %v", err)
	}
	if n != 9223372036854775807 {
		t.Errorf("got %d, want math.MaxInt", n)
	}
	if _, err := parseDigits([]byte("9223372036854775808")); err == nil {
		t.Fatal("expected an error for MaxInt+1")
	}
}

func TestWriteClassicXrefEntry_WritesFixedWidthEntry(t *testing.T) {
	var buf bytes.Buffer
	if err := writeClassicXrefEntry(&buf, 7, 1234, 5); err != nil {
		t.Fatalf("writeClassicXrefEntry: %v", err)
	}
	want := "7 1\n0000001234 00005 n \n"
	if buf.String() != want {
		t.Fatalf("got %q, want %q", buf.String(), want)
	}
}

// TestWriteClassicXrefEntry_AcceptsMaxWidthValues pins the guards' exact
// off-by-one boundary: the largest offset/gen that still fit the 10- and
// 5-digit fields (9999999999, 99999) must succeed, not just fail one past
// it. Without this, the overflow tests alone (which fixture values far
// past the limit) can't catch a `>` vs `>=` mistake in either guard — the
// same gap TestParseDigits_BoundaryValues closed for parseDigits in
// increment 4a. Confirmed by mutation: changing either guard to `>=`
// leaves the whole suite green without this test.
func TestWriteClassicXrefEntry_AcceptsMaxWidthValues(t *testing.T) {
	var buf bytes.Buffer
	if err := writeClassicXrefEntry(&buf, 1, 9999999999, 99999); err != nil {
		t.Fatalf("writeClassicXrefEntry at the max field width: %v", err)
	}
	want := "1 1\n9999999999 99999 n \n"
	if buf.String() != want {
		t.Fatalf("got %q, want %q", buf.String(), want)
	}
}

// TestWriteClassicXrefEntry_OffsetOverflowReturnsError covers the half of
// the guard that is not reachable through the public Inject* functions with
// a realistic fixture (it would take a ~9.3GB input PDF to produce an
// offset this large), so it is exercised directly here instead. Confirmed
// by mutation: removing the offset check lets this fixture through with
// err == nil and a widened, mis-aligned entry.
func TestWriteClassicXrefEntry_OffsetOverflowReturnsError(t *testing.T) {
	var buf bytes.Buffer
	err := writeClassicXrefEntry(&buf, 1, 10000000000, 0)
	if err == nil {
		t.Fatal("expected an error for an offset that overflows the 10-digit field")
	}
	if !strings.Contains(err.Error(), "10-digit field width") {
		t.Errorf("expected the guard's own error text, got %q", err)
	}
}

// TestWriteClassicXrefEntry_GenOverflowReturnsError covers the guard half
// that IS reachable through a realistic fixture — see
// TestInjectXMPStream_RootGenOverflowReturnsError,
// TestInjectOutputIntent_RootGenOverflowReturnsError, and
// TestInjectPAdESSignature_RootGenOverflowReturnsError for the integration
// paths that exercise this via a crafted /Root generation number. Confirmed
// by mutation: removing the gen check lets this fixture through with
// err == nil and a widened, mis-aligned entry.
func TestWriteClassicXrefEntry_GenOverflowReturnsError(t *testing.T) {
	var buf bytes.Buffer
	err := writeClassicXrefEntry(&buf, 1, 0, 100000)
	if err == nil {
		t.Fatal("expected an error for a generation that overflows the 5-digit field")
	}
	if !strings.Contains(err.Error(), "5-digit field width") {
		t.Errorf("expected the guard's own error text, got %q", err)
	}
}

// rootGenOverflowPDF returns a hand-built, single-object PDF whose Catalog
// has generation number 100000 — over the classic xref format's 5-digit
// field width. This is the only realistic (not multi-gigabyte) way to
// reach writeClassicXrefEntry's generation-overflow guard through
// InjectXMPStream/InjectOutputIntent/InjectPAdESSignature: catalogGen is
// parsed straight out of the trailer's /Root entry by
// parseTrailerSizeAndRoot and flows unchanged into every xref entry these
// functions write for the rewritten Catalog. Modeled on swappedIDPDF
// above. Note this fixture IS spec-invalid on its own terms (the PDF spec
// caps generation numbers at 65535, matching the free-list entry's own
// "65535 f" three lines above); writeClassicXrefEntry's guard is a
// format-width check (99999), not a spec-conformance check, deliberately
// looser than the spec so a spec-invalid-but-format-safe generation
// (e.g. 70000) still formats correctly instead of being rejected here.
func rootGenOverflowPDF() []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	b.WriteString("1 100000 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")
	xrefOff := b.Len()
	b.WriteString("xref\n0 1\n0000000000 65535 f \n")
	b.WriteString("trailer\n<<\n/Size 2\n/Root 1 100000 R\n>>\n")
	fmt.Fprintf(&b, "startxref\n%d\n%%%%EOF\n", xrefOff)
	return b.Bytes()
}

// TestInjectXMPStream_RootGenOverflowReturnsError covers the "xmp xref: %w"
// wrap around writeClassicXrefEntry's generation guard. See
// TestWriteClassicXrefEntry_GenOverflowReturnsError for the guard's own
// direct test and rootGenOverflowPDF's docstring for why this fixture
// reaches it without a multi-gigabyte input. Confirmed by mutation:
// reverting the call site to the raw Fprintf lets this fixture through
// with err == nil and a widened, mis-aligned xref entry.
func TestInjectXMPStream_RootGenOverflowReturnsError(t *testing.T) {
	_, err := InjectXMPStream(rootGenOverflowPDF(), []byte("<xmp/>"))
	if err == nil {
		t.Fatal("expected an error for a /Root generation that overflows the xref format's 5-digit field")
	}
	if !strings.Contains(err.Error(), "5-digit field width") {
		t.Errorf("expected the guard's own error text, got %q", err)
	}
}

// TestInjectOutputIntent_RootGenOverflowReturnsError covers the
// "output intent xref: %w" wrap around the Catalog re-write entry's
// generation guard (the ICC-stream and OutputIntent-dict entries are
// always written with gen 0, so only the Catalog entry can trigger this).
// See TestInjectXMPStream_RootGenOverflowReturnsError above for the
// fixture rationale. Confirmed by mutation: reverting the call site to
// the raw Fprintf lets this fixture through with err == nil.
func TestInjectOutputIntent_RootGenOverflowReturnsError(t *testing.T) {
	_, err := InjectOutputIntent(rootGenOverflowPDF(), []byte("fake-icc-profile"))
	if err == nil {
		t.Fatal("expected an error for a /Root generation that overflows the xref format's 5-digit field")
	}
	if !strings.Contains(err.Error(), "5-digit field width") {
		t.Errorf("expected the guard's own error text, got %q", err)
	}
}

// TestInjectPAdESSignature_RootGenOverflowReturnsError covers the
// "signature xref: %w" wrap around the loop that writes one xref entry
// per rewritten/new object — gens[0] is catalogGen, so it triggers on the
// loop's first iteration. See TestInjectXMPStream_RootGenOverflowReturnsError
// above for the fixture rationale. Confirmed by mutation: reverting the
// call site to the raw Fprintf lets this fixture through with err == nil.
func TestInjectPAdESSignature_RootGenOverflowReturnsError(t *testing.T) {
	_, err := InjectPAdESSignature(rootGenOverflowPDF(), func(b []byte) ([]byte, error) {
		return []byte{0xde, 0xad, 0xbe, 0xef}, nil
	})
	if err == nil {
		t.Fatal("expected an error for a /Root generation that overflows the xref format's 5-digit field")
	}
	if !strings.Contains(err.Error(), "5-digit field width") {
		t.Errorf("expected the guard's own error text, got %q", err)
	}
}

// TestCheckTrailerSizeRoom_BoundaryValues pins checkTrailerSizeRoom's
// off-by-one behavior at the exact math.MaxInt boundary, the same shape
// as TestParseDigits_BoundaryValues did for parseDigits in increment
// 4a. This guard closes the increment-5a-disclosed, 5b-fixed bug:
// InjectOutputIntent/InjectPAdESSignature each allocate two IDs off
// trailerSize and then write trailerSize+2 as /Size, so a trailerSize
// within 2 of math.MaxInt — not just exactly math.MaxInt, which is as
// far as parseDigits' own guard reaches — silently wraps to a negative
// value downstream. newObjects=1 (InjectXMPStream's shape) and
// newObjects=2 (InjectOutputIntent/InjectPAdESSignature's shape) are
// each checked at their own exact boundary: the largest trailerSize
// that must still succeed, and one past it that must fail.
func TestCheckTrailerSizeRoom_BoundaryValues(t *testing.T) {
	if err := checkTrailerSizeRoom(math.MaxInt-1, 1); err != nil {
		t.Errorf("newObjects=1 at the boundary (MaxInt-1): unexpected error: %v", err)
	}
	if err := checkTrailerSizeRoom(math.MaxInt, 1); err == nil {
		t.Error("newObjects=1 one past the boundary (MaxInt): expected an error")
	}
	if err := checkTrailerSizeRoom(math.MaxInt-2, 2); err != nil {
		t.Errorf("newObjects=2 at the boundary (MaxInt-2): unexpected error: %v", err)
	}
	if err := checkTrailerSizeRoom(math.MaxInt-1, 2); err == nil {
		t.Error("newObjects=2 one past the boundary (MaxInt-1): expected an error")
	}
}

// trailerSizeOverflowPDF returns a hand-built, single-object PDF whose
// trailer /Size is the given literal decimal string. Modeled on
// rootGenOverflowPDF: a real Catalog body is required because all
// three Inject* functions call findObjectBody(catalogID, catalogGen)
// and reach checkTrailerSizeRoom only after that lookup succeeds.
func trailerSizeOverflowPDF(size string) []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	b.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")
	xrefOff := b.Len()
	b.WriteString("xref\n0 1\n0000000000 65535 f \n")
	fmt.Fprintf(&b, "trailer\n<<\n/Size %s\n/Root 1 0 R\n>>\n", size)
	fmt.Fprintf(&b, "startxref\n%d\n%%%%EOF\n", xrefOff)
	return b.Bytes()
}

// TestInjectXMPStream_TrailerSizeOverflowReturnsError covers
// checkTrailerSizeRoom's newObjects=1 call site. A /Size of literal
// "9223372036854775807" (math.MaxInt) parses cleanly through
// parseDigits (which only rejects a value that would exceed
// math.MaxInt, not math.MaxInt itself) — this is the exact value
// increment 5a confirmed, by direct execution, silently corrupts
// InjectOutputIntent's output; checkTrailerSizeRoom now rejects it
// before any object ID or /Size arithmetic runs. Confirmed by
// mutation: removing the checkTrailerSizeRoom call in InjectXMPStream
// lets this fixture through with err == nil.
func TestInjectXMPStream_TrailerSizeOverflowReturnsError(t *testing.T) {
	_, err := InjectXMPStream(trailerSizeOverflowPDF("9223372036854775807"), []byte("<xmp/>"))
	if err == nil {
		t.Fatal("expected an error for a trailer /Size of math.MaxInt")
	}
	if !strings.Contains(err.Error(), "no room to allocate") {
		t.Errorf("expected the guard's own error text, got %q", err)
	}
}

// TestInjectOutputIntent_TrailerSizeOverflowReturnsError uses
// math.MaxInt-1, not math.MaxInt, deliberately: InjectOutputIntent
// allocates two IDs (trailerSize, trailerSize+1) and writes
// trailerSize+2 as /Size, so its correct bound rejects anything above
// math.MaxInt-2 — one lower than InjectXMPStream's bound. A fixture at
// math.MaxInt alone would pass just as well under an off-by-one guard
// written for InjectXMPStream's shape (trailerSize >= math.MaxInt)
// mistakenly reused here; math.MaxInt-1 is the value that
// discriminates the two (it must still error under the correct
// newObjects=2 bound, but would wrongly succeed under the
// newObjects=1 bound). Confirmed by mutation: reverting the call site
// to `checkTrailerSizeRoom(trailerSize, 1)` lets this fixture through
// with err == nil.
func TestInjectOutputIntent_TrailerSizeOverflowReturnsError(t *testing.T) {
	_, err := InjectOutputIntent(trailerSizeOverflowPDF("9223372036854775806"), []byte("fake-icc-profile"))
	if err == nil {
		t.Fatal("expected an error for a trailer /Size of math.MaxInt-1")
	}
	if !strings.Contains(err.Error(), "no room to allocate") {
		t.Errorf("expected the guard's own error text, got %q", err)
	}
}

// TestInjectPAdESSignature_TrailerSizeOverflowReturnsError mirrors
// TestInjectOutputIntent_TrailerSizeOverflowReturnsError —
// InjectPAdESSignature has the identical trailerSize/trailerSize+1/
// trailerSize+2 shape (sigID, fieldID, /Size). See that test's
// docstring for why math.MaxInt-1, not math.MaxInt, is the
// discriminating fixture value. Confirmed by mutation: reverting the
// call site to `checkTrailerSizeRoom(trailerSize, 1)` lets this
// fixture through with err == nil.
func TestInjectPAdESSignature_TrailerSizeOverflowReturnsError(t *testing.T) {
	_, err := InjectPAdESSignature(trailerSizeOverflowPDF("9223372036854775806"), func(b []byte) ([]byte, error) {
		return []byte{0xde, 0xad, 0xbe, 0xef}, nil
	})
	if err == nil {
		t.Fatal("expected an error for a trailer /Size of math.MaxInt-1")
	}
	if !strings.Contains(err.Error(), "no room to allocate") {
		t.Errorf("expected the guard's own error text, got %q", err)
	}
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

// TestFindLastStartxref_OffsetOutOfRange covers the "offset <= 0 ||
// offset >= len(b)" range check with both operands independently: a
// parsed offset of 0 (the "<= 0" side) and a parsed offset past the end
// of the buffer (the ">= len(b)" side). Verified independently, not
// just predicted: mutating the guard to keep only "offset >= len(b)"
// fails just the "zero offset" case; keeping only "offset <= 0" fails
// just "offset too large" — each subcase discriminates its own operand.
func TestFindLastStartxref_OffsetOutOfRange(t *testing.T) {
	cases := map[string][]byte{
		"zero offset":      []byte("startxref\n0\n%%EOF"),
		"offset too large": []byte("startxref\n999999\n%%EOF"),
	}
	for name, input := range cases {
		if _, err := findLastStartxref(input); err == nil {
			t.Errorf("%s: expected an out-of-range error, got nil", name)
		}
	}
}

// TestFindLastStartxref_OffsetOverflowReturnsError covers the new
// "startxref offset: %w" wrap around parseDigits. Asserts the wrap
// prefix specifically, not a bare err != nil: an overflowing offset
// could in principle wrap to a value that also fails the very next
// "out of range" guard, so a bare non-nil check wouldn't distinguish
// this guard firing from that one.
func TestFindLastStartxref_OffsetOverflowReturnsError(t *testing.T) {
	_, err := findLastStartxref([]byte("startxref\n99999999999999999999\n%%EOF"))
	if err == nil {
		t.Fatal("expected an error for a startxref offset that overflows int")
	}
	if !strings.Contains(err.Error(), "startxref offset:") {
		t.Errorf("expected the wrap prefix, got %q", err)
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

// TestParseTrailerSizeAndRoot_XrefOffsetOutOfRange covers the
// "xrefOffset < 0 || xrefOffset >= len(b)" guard with three subcases
// covering both operands. The "exactly len(b)" case is deliberately
// included because it's the boundary where a naive assumption fails:
// b[len(b):] is a legal empty slice in Go, so if only the guard's "< 0"
// half survived a mutation, this exact value wouldn't panic — it would
// fall through and cascade into the next guard ("trailer keyword not
// found"), a different but still non-nil error. All three subcases
// therefore assert the guard's own "out of range" text rather than a
// bare err != nil check, so a cascade would be caught rather than
// masked.
func TestParseTrailerSizeAndRoot_XrefOffsetOutOfRange(t *testing.T) {
	b := minimalPDF()
	// A slice, not a map: map iteration order is randomized per run, and
	// the "negative" subcase panics under a mutation that strips only
	// the "< 0" half of the guard (see the break-verification note
	// below) — a randomized order would make which subcases actually
	// execute before the panic non-deterministic across runs.
	cases := []struct {
		name   string
		offset int
	}{
		{"negative", -1},
		{"exactly len(b)", len(b)},
		{"past len(b)", len(b) + 100},
	}
	for _, c := range cases {
		_, _, _, err := parseTrailerSizeAndRoot(b, c.offset)
		if err == nil {
			t.Errorf("%s: expected an error", c.name)
			continue
		}
		if !strings.Contains(err.Error(), "out of range") {
			t.Errorf("%s: expected an \"out of range\" error, got %q", c.name, err)
		}
	}
}

// TestParseTrailerSizeAndRoot_TrailerKeywordNotFound covers the
// "trailer keyword not found after xref" guard. Uses xrefOffset > 0
// (not 0): deleting the guard leaves trailerIdx == -1, and
// absTrailer := xrefOffset + trailerIdx becomes xrefOffset - 1 — a
// valid non-negative index only when xrefOffset > 0. With this
// fixture (no "/Size" text present either), a deleted guard cascades
// into readDictInt's "/Size not present" error rather than panicking,
// so the assertion checks the guard's own text, not just err != nil.
func TestParseTrailerSizeAndRoot_TrailerKeywordNotFound(t *testing.T) {
	b := []byte("%PDF-1.4\nnothing relevant appears in this buffer\n")
	_, _, _, err := parseTrailerSizeAndRoot(b, 1)
	if err == nil {
		t.Fatal("expected an error when no trailer keyword follows xrefOffset")
	}
	if !strings.Contains(err.Error(), "trailer keyword not found") {
		t.Errorf("expected a \"trailer keyword not found\" error, got %q", err)
	}
}

// TestParseTrailerSizeAndRoot_StartxrefNotFoundAfterTrailer covers the
// "startxref keyword not found after trailer" guard: the trailer
// keyword is present, but nothing after it contains "startxref".
func TestParseTrailerSizeAndRoot_StartxrefNotFoundAfterTrailer(t *testing.T) {
	b := []byte("%PDF-1.4\ntrailer\n<< /Size 4 /Root 1 0 R >>\nnothing relevant follows\n")
	_, _, _, err := parseTrailerSizeAndRoot(b, 0)
	if err == nil {
		t.Fatal("expected an error when no startxref follows trailer")
	}
	if !strings.Contains(err.Error(), "startxref keyword not found after trailer") {
		t.Errorf("expected a \"startxref keyword not found after trailer\" error, got %q", err)
	}
}

// TestParseTrailerSizeAndRoot_SizeReadError covers the "/Size: %w"
// wrap: the trailer block is otherwise well-formed but has no /Size
// key, so readDictInt's own error propagates wrapped.
func TestParseTrailerSizeAndRoot_SizeReadError(t *testing.T) {
	b := []byte("%PDF-1.4\ntrailer\n<< /Root 1 0 R >>\nstartxref\n5\n%%EOF\n")
	_, _, _, err := parseTrailerSizeAndRoot(b, 0)
	if err == nil {
		t.Fatal("expected an error when /Size is absent")
	}
	if !strings.Contains(err.Error(), "/Size:") {
		t.Errorf("expected the /Size wrap prefix, got %q", err)
	}
}

// TestParseTrailerSizeAndRoot_RootReadError covers the "/Root: %w"
// wrap: /Size parses fine but /Root is absent, so readDictRef's own
// error propagates wrapped.
func TestParseTrailerSizeAndRoot_RootReadError(t *testing.T) {
	b := []byte("%PDF-1.4\ntrailer\n<< /Size 4 >>\nstartxref\n5\n%%EOF\n")
	_, _, _, err := parseTrailerSizeAndRoot(b, 0)
	if err == nil {
		t.Fatal("expected an error when /Root is absent")
	}
	if !strings.Contains(err.Error(), "/Root:") {
		t.Errorf("expected the /Root wrap prefix, got %q", err)
	}
}

// TestReadDictInt_KeyNotPresent covers readDictInt's "key not present"
// guard. Asserts the guard's own error text ("not present"), not a bare
// non-nil check: deleting the guard leaves idx == -1, and
// i := idx + len(key) then lands on some byte a few positions into the
// block by coincidence — for this fixture that byte happens to be
// non-digit, so the *next* guard ("no integer value") fires instead and
// a bare err == nil check would stay green under the mutation.
// Confirmed by direct mutation before finalizing.
func TestReadDictInt_KeyNotPresent(t *testing.T) {
	_, err := readDictInt([]byte("<< /Foo 1 >>"), "/Size")
	if err == nil {
		t.Fatal("expected an error when the key is absent")
	}
	if !strings.Contains(err.Error(), "not present") {
		t.Errorf("expected a \"not present\" error, got %q", err)
	}
}

// TestReadDictInt_NoIntegerValue covers the "no digits after the key"
// guard: the key is present but is followed by non-digit bytes.
func TestReadDictInt_NoIntegerValue(t *testing.T) {
	if _, err := readDictInt([]byte("/Size abc"), "/Size"); err == nil {
		t.Fatal("expected an error when no digits follow the key")
	}
}

// TestReadDictInt_ValueOverflowReturnsError covers the wrapped
// parseDigits error: a /Size (or similar) value long enough to
// overflow int. Confirmed by mutation: reverting to the pre-fix raw
// accumulation loop returns a wrapped, silently-wrong value instead of
// an error.
func TestReadDictInt_ValueOverflowReturnsError(t *testing.T) {
	_, err := readDictInt([]byte("/Size 99999999999999999999"), "/Size")
	if err == nil {
		t.Fatal("expected an error when the value overflows int")
	}
	if !strings.Contains(err.Error(), "overflows") {
		t.Errorf("expected the guard's own error text, got %q", err)
	}
}

// TestReadDictRef_KeyNotPresent covers readDictRef's "key not present"
// guard. Asserts the guard's own error text, for the same reason as
// TestReadDictInt_KeyNotPresent: idx == -1 can still land i on a
// coincidentally non-digit byte, masking the guard's deletion behind
// the "no id digit" guard's error instead. Confirmed by direct
// mutation before finalizing.
func TestReadDictRef_KeyNotPresent(t *testing.T) {
	_, _, err := readDictRef([]byte("<< /Foo 1 0 R >>"), "/Root")
	if err == nil {
		t.Fatal("expected an error when the key is absent")
	}
	if !strings.Contains(err.Error(), "not present") {
		t.Errorf("expected a \"not present\" error, got %q", err)
	}
}

// TestReadDictRef_NoIdDigit covers the "no id digit" guard. Asserts the
// guard's own error text: this guard is structurally masked by the
// very next guard ("no gen digit") for every possible input, not just
// this fixture — the leading-whitespace skip before the id-digit scan
// has already consumed every space/tab/newline/CR, so whatever
// non-digit byte blocks the id-digit scan is guaranteed to also be
// non-whitespace, which means the second (space/tab-only) skip before
// the gen-digit scan can't move past it either — the gen-digit scan
// starts at the exact same blocking byte and fails the same way.
// Deleting this guard therefore falls through to "no gen digit" firing
// immediately after, not to success. Confirmed by direct mutation
// before finalizing.
func TestReadDictRef_NoIdDigit(t *testing.T) {
	_, _, err := readDictRef([]byte("/Root R"), "/Root")
	if err == nil {
		t.Fatal("expected an error when no id digits follow the key")
	}
	if !strings.Contains(err.Error(), "no id digit") {
		t.Errorf("expected a \"no id digit\" error, got %q", err)
	}
}

// TestReadDictRef_NoGenDigit covers the "no gen digit" guard: the id
// parses fine, but no digits follow it for the generation number.
func TestReadDictRef_NoGenDigit(t *testing.T) {
	if _, _, err := readDictRef([]byte("/Root 1 R"), "/Root"); err == nil {
		t.Fatal("expected an error when no gen digits follow the id")
	}
}

// TestReadDictRef_ExpectedRAfterGen covers the "expected R after gen"
// guard: id and gen both parse fine, but the trailing "R" marker is
// something else.
func TestReadDictRef_ExpectedRAfterGen(t *testing.T) {
	if _, _, err := readDictRef([]byte("/Root 1 0 X"), "/Root"); err == nil {
		t.Fatal("expected an error when the R marker is missing")
	}
}

// TestReadDictRef_IdOverflowReturnsError and
// TestReadDictRef_GenOverflowReturnsError cover the wrapped
// parseDigits errors for /Root's id and generation numbers
// respectively. Confirmed by mutation.
func TestReadDictRef_IdOverflowReturnsError(t *testing.T) {
	_, _, err := readDictRef([]byte("/Root 99999999999999999999 0 R"), "/Root")
	if err == nil {
		t.Fatal("expected an error when the id overflows int")
	}
	if !strings.Contains(err.Error(), "id") || !strings.Contains(err.Error(), "overflows") {
		t.Errorf("expected an id-overflow error, got %q", err)
	}
}

func TestReadDictRef_GenOverflowReturnsError(t *testing.T) {
	_, _, err := readDictRef([]byte("/Root 1 99999999999999999999 R"), "/Root")
	if err == nil {
		t.Fatal("expected an error when the gen overflows int")
	}
	if !strings.Contains(err.Error(), "gen") || !strings.Contains(err.Error(), "overflows") {
		t.Errorf("expected a gen-overflow error, got %q", err)
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

// TestFindObjectBody_ObjectNotFound covers the "object not found" guard:
// the requested id/gen never appears as an "<id> <gen> obj" marker.
func TestFindObjectBody_ObjectNotFound(t *testing.T) {
	if _, err := findObjectBody(minimalPDF(), 99, 0); err == nil {
		t.Fatal("expected an error for a nonexistent object id")
	}
}

// TestFindObjectBody_EndobjNotFound covers the "endobj not found" guard:
// the object marker is present but its matching "endobj" never appears.
func TestFindObjectBody_EndobjNotFound(t *testing.T) {
	if _, err := findObjectBody([]byte("1 0 obj\n<< /Type /Catalog >>\n"), 1, 0); err == nil {
		t.Fatal("expected an error when endobj is missing")
	}
}

// TestInsertMetadataReference_TrimsLeadingWhitespace covers the
// leading-whitespace trim loop's body, mirroring
// TestInsertOutputIntentsReference_TrimsLeadingWhitespace for the sibling
// function: this package's other insertMetadataReference tests all pass
// fixtures whose catalog body starts with "<<" directly, so without this
// test the trim loop itself would never execute.
func TestInsertMetadataReference_TrimsLeadingWhitespace(t *testing.T) {
	orig := []byte("  \n\t<< /Type /Catalog /Pages 2 0 R >>")
	got := insertMetadataReference(orig, 99)
	if !bytes.Contains(got, []byte("/Metadata 99 0 R")) {
		t.Fatalf("expected /Metadata insertion, got %q", string(got))
	}
	if !bytes.Contains(got, []byte("/Pages 2 0 R")) {
		t.Errorf("lost /Pages entry: %q", string(got))
	}
	// Deleting the trim loop is masked by the "not a dict" guard below
	// it: an untrimmed body starts with a whitespace byte instead of
	// "<<", so the guard's own malformed-input check fires and the
	// function falls back to wrapping the whole (still-untrimmed) body
	// in a brand-new "<< ... >>" shell — which still contains both
	// substrings above, so a bare Contains check would stay green under
	// that mutation. The wrap fallback nests the original body's own
	// "<<" inside the new wrapper's "<<", producing two occurrences
	// instead of one; the correct in-place-edit path (trim loop intact)
	// reuses the original dict's single "<<" as-is. Confirmed by direct
	// mutation before finalizing.
	if n := bytes.Count(got, []byte("<<")); n != 1 {
		t.Errorf("expected the original dict to be edited in place (one \"<<\"), got %d in %q", n, string(got))
	}
}

// TestInsertMetadataReference_WrapsNonDictBody covers the "malformed
// input" branch. Uses the same HasPrefix("<<")/HasSuffix(">>") pattern as
// TestInsertOutputIntentsReference_WrapsNonDictBody rather than a bare
// bytes.Contains: deleting this guard falls through to the "already has
// <<" insert-in-place logic, which for a non-"<<"-prefixed body like
// "/Type /Catalog" splices "/Metadata 42 0 R" into the middle of the
// literal bytes ("/T" + the insertion + "ype /Catalog") — a result that
// still contains the "/Metadata 42 0 R" substring (so a bare Contains
// check would stay green under that mutation) but does not start with
// "<<" or end with ">>". Confirmed by direct mutation before finalizing:
// deleting the guard produced exactly "/T\n/Metadata 42 0 Rype /Catalog".
func TestInsertMetadataReference_WrapsNonDictBody(t *testing.T) {
	orig := []byte("/Type /Catalog")
	got := insertMetadataReference(orig, 42)
	if !bytes.HasPrefix(got, []byte("<<")) || !bytes.HasSuffix(got, []byte(">>")) {
		t.Fatalf("expected the result to be wrapped in a fresh dict, got %q", string(got))
	}
	if !bytes.Contains(got, []byte("/Metadata 42 0 R")) {
		t.Errorf("expected /Metadata insertion, got %q", string(got))
	}
	if !bytes.Contains(got, orig) {
		t.Errorf("original body should be preserved inside the wrapper, got %q", string(got))
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

// TestInjectXMPStream_RejectsEmpty's first case asserts the guard's own
// error text, not a bare err != nil: nil pdfBytes also fails
// findLastStartxref a few lines later with a different, still-non-nil
// error, so a bare check would pass even with this guard deleted
// (found and confirmed by mutation while covering the analogous guard
// in InjectPAdESSignature — see TestInjectPAdESSignature_RejectsEmpty).
// The second case doesn't have this risk: minimalPDF() is valid, so
// nothing else in the call chain can fail before the XMP-packet guard.
func TestInjectXMPStream_RejectsEmpty(t *testing.T) {
	_, err := InjectXMPStream(nil, []byte("xmp"))
	if err == nil {
		t.Fatal("expected error on empty PDF")
	}
	if !strings.Contains(err.Error(), "empty PDF input") {
		t.Errorf("expected the guard's own error text, got %q", err)
	}
	if _, err := InjectXMPStream(minimalPDF(), nil); err == nil {
		t.Error("expected error on empty XMP")
	}
}

// TestInjectXMPStream_PropagatesLocateStartxrefError covers the
// "pdfmeta: locate startxref: %w" wrap. Asserts the stage-specific
// wrap prefix, not a bare err != nil: InjectXMPStream calls three
// functions in sequence, each wrapped with its own distinct prefix, so
// a fixture crafted to fail at the wrong stage would still produce a
// non-nil error and a bare check wouldn't catch the mismatch.
func TestInjectXMPStream_PropagatesLocateStartxrefError(t *testing.T) {
	_, err := InjectXMPStream([]byte("%PDF-1.4\nno relevant marker here\n"), []byte("<xmp/>"))
	if err == nil {
		t.Fatal("expected an error when startxref is absent")
	}
	if !strings.Contains(err.Error(), "pdfmeta: locate startxref:") {
		t.Errorf("expected the locate-startxref wrap prefix, got %q", err)
	}
}

// TestInjectXMPStream_PropagatesParseTrailerError covers the
// "pdfmeta: parse trailer: %w" wrap. The fixture has a valid, in-range
// startxref (so stage 1 succeeds) but no trailer keyword anywhere, so
// stage 2 fails specifically, not stage 1 or stage 3.
func TestInjectXMPStream_PropagatesParseTrailerError(t *testing.T) {
	_, err := InjectXMPStream([]byte("%PDF-1.4\nxxxx\nstartxref\n5\n%%EOF\n"), []byte("<xmp/>"))
	if err == nil {
		t.Fatal("expected an error when the trailer keyword is absent")
	}
	if !strings.Contains(err.Error(), "pdfmeta: parse trailer:") {
		t.Errorf("expected the parse-trailer wrap prefix, got %q", err)
	}
}

// TestInjectXMPStream_PropagatesLocateCatalogError covers the
// "pdfmeta: locate Catalog object: %w" wrap. The fixture has a valid
// startxref, trailer, /Size, and /Root (so stages 1 and 2 both
// succeed), but the referenced Catalog object body is never actually
// present in the byte stream, so only stage 3 fails.
func TestInjectXMPStream_PropagatesLocateCatalogError(t *testing.T) {
	b := []byte("%PDF-1.4\ntrailer\n<< /Size 4 /Root 1 0 R >>\nstartxref\n5\n%%EOF\n")
	_, err := InjectXMPStream(b, []byte("<xmp/>"))
	if err == nil {
		t.Fatal("expected an error when the Catalog object is absent")
	}
	if !strings.Contains(err.Error(), "pdfmeta: locate Catalog object:") {
		t.Errorf("expected the locate-Catalog wrap prefix, got %q", err)
	}
}

// TestInjectXMPStream_AddsTrailingNewlineWhenMissing covers the
// "pdfBytes[len(pdfBytes)-1] != '\n'" guard. minimalPDF() already ends
// in '\n', so the fixture trims it to force the guard to fire.
// Discriminator: out[len(pdfBytes)] must be the inserted '\n' — with
// the guard deleted (mutated to `if false`), the appended block's
// first byte is instead the leading digit of the metadata object's
// "<id> 0 obj" line, observed gluing directly onto the trimmed
// fixture's last byte with no separator: out[len(pdfBytes):] reads
// "4 0 obj\n<<\n/Type /Me" (confirmed by mutation).
func TestInjectXMPStream_AddsTrailingNewlineWhenMissing(t *testing.T) {
	pdfBytes := bytes.TrimRight(minimalPDF(), "\n")
	out, err := InjectXMPStream(pdfBytes, []byte("<xmp/>"))
	if err != nil {
		t.Fatalf("InjectXMPStream: %v", err)
	}
	if out[len(pdfBytes)] != '\n' {
		t.Errorf("expected a newline inserted immediately after the original bytes, got %q", out[len(pdfBytes):min(len(out), len(pdfBytes)+20)])
	}
}

// swappedIDPDF returns a hand-built PDF whose /Root object ID (9) is
// deliberately larger than its /Size (4) — a spec-violating structure
// (PDF/Size is defined as one greater than the highest object number in
// use), but the only way to reach InjectXMPStream's "first > second"
// xref-entry-swap branch: metaID is derived from /Size, and for any
// spec-valid PDF /Size is already larger than every existing object
// ID, so metaID > catalogID always holds and the swap never triggers.
// This exists to cover defensive code, not to model a realistic input.
func swappedIDPDF() []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	b.WriteString("9 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")
	xrefOff := b.Len()
	b.WriteString("xref\n0 1\n0000000000 65535 f \n")
	b.WriteString("trailer\n<<\n/Size 4\n/Root 9 0 R\n>>\n")
	fmt.Fprintf(&b, "startxref\n%d\n%%%%EOF\n", xrefOff)
	return b.Bytes()
}

// TestInjectXMPStream_SwapsXrefEntriesWhenCatalogIDGreaterThanMetaID
// covers the "first > second" branch using swappedIDPDF (see its
// docstring for why the fixture is deliberately spec-violating).
// Discriminator: the xref subsection immediately following the
// appended incremental update's "0000000000 65535 f " free-list entry
// must list the lower ID (metaID, 4) before the higher one (catalogID,
// 9) — ascending order proves the swap ran. Searches only within
// out[len(pdfBytes):] (the newly appended bytes), not the whole
// output, because the fixture's own original xref table also contains
// the literal "0000000000 65535 f " text and would otherwise match
// the wrong occurrence.
func TestInjectXMPStream_SwapsXrefEntriesWhenCatalogIDGreaterThanMetaID(t *testing.T) {
	pdfBytes := swappedIDPDF()
	out, err := InjectXMPStream(pdfBytes, []byte("<xmp/>"))
	if err != nil {
		t.Fatalf("InjectXMPStream: %v", err)
	}
	appended := out[len(pdfBytes):]
	const freeEntry = "0000000000 65535 f \n"
	_, rest, ok := bytes.Cut(appended, []byte(freeEntry))
	if !ok {
		t.Fatalf("expected the appended xref's free-list entry, got %q", appended)
	}
	if !bytes.HasPrefix(rest, []byte("4 1\n")) {
		t.Errorf("expected the lower ID (metaID=4) listed first after the swap, got %q", rest[:min(len(rest), 20)])
	}
}

// swappedIDGenOverflowPDF combines swappedIDPDF's ID-swap trick with a
// generation number that overflows the xref format's 5-digit field: after
// the "first > second" swap, the Catalog's huge generation lands in
// secondGen, not firstGen — the only way to reach the *second*
// writeClassicXrefEntry call's error-wrap in InjectXMPStream, since a
// non-swapped fixture always fails on the first call already (see
// rootGenOverflowPDF's docstring). Deliberately spec-violating on two
// independent counts, neither of which this codebase's own guards reject:
// the Catalog ID exceeding /Size (same as swappedIDPDF), and the
// generation number exceeding the spec's 65535 cap (see
// rootGenOverflowPDF's docstring for why writeClassicXrefEntry's guard is
// a looser format-width check, not a spec-conformance check). Covers
// defensive code, not realistic input.
func swappedIDGenOverflowPDF() []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	b.WriteString("9 100000 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")
	xrefOff := b.Len()
	b.WriteString("xref\n0 1\n0000000000 65535 f \n")
	b.WriteString("trailer\n<<\n/Size 4\n/Root 9 100000 R\n>>\n")
	fmt.Fprintf(&b, "startxref\n%d\n%%%%EOF\n", xrefOff)
	return b.Bytes()
}

// TestInjectXMPStream_SecondEntryRootGenOverflowReturnsError covers the
// second writeClassicXrefEntry call's error wrap — see
// swappedIDGenOverflowPDF's docstring for why the first call alone can't
// reach it. Confirmed by mutation: reverting the second call site to the
// raw Fprintf lets this fixture through with err == nil.
func TestInjectXMPStream_SecondEntryRootGenOverflowReturnsError(t *testing.T) {
	_, err := InjectXMPStream(swappedIDGenOverflowPDF(), []byte("<xmp/>"))
	if err == nil {
		t.Fatal("expected an error for a /Root generation that overflows the xref format's 5-digit field")
	}
	if !strings.Contains(err.Error(), "5-digit field width") {
		t.Errorf("expected the guard's own error text, got %q", err)
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

// TestMakePDFA3_PropagatesBinaryHeaderCommentError covers the
// "pdfmeta: binary header comment: %w" wrap around the first stage.
func TestMakePDFA3_PropagatesBinaryHeaderCommentError(t *testing.T) {
	_, err := MakePDFA3([]byte("not a pdf"), XMPSpec{}, []byte("icc"))
	if err == nil {
		t.Fatal("expected an error when the input has no PDF header")
	}
	if !strings.Contains(err.Error(), "pdfmeta: binary header comment:") {
		t.Errorf("expected the binary-header-comment wrap prefix, got %q", err)
	}
}

// TestMakePDFA3_PropagatesXMPInjectionError covers the
// "pdfmeta: xmp injection: %w" wrap around the second stage. The fixture
// already carries a valid header and binary comment (so stage 1
// succeeds via ensureBinaryHeaderComment's early-return path) but has no
// "startxref" anywhere, so InjectXMPStream's own findLastStartxref call
// fails. Filler text reused from
// TestEnsureBinaryHeaderComment_PropagatesFindLastStartxrefError, not
// invented fresh: an earlier draft of this fixture used filler
// containing the literal word "startxref", which made findLastStartxref
// succeed and the fixture fail for an unrelated reason (a parse error
// downstream) while still passing this test's wrap-prefix assertion --
// caught before finalizing by reusing the file's own known-safe filler
// rather than writing new prose.
func TestMakePDFA3_PropagatesXMPInjectionError(t *testing.T) {
	pdf := []byte("%PDF-1.4\n%\xe2\xe3\xcf\xd3\nnothing relevant appears in this buffer\n")
	_, err := MakePDFA3(pdf, XMPSpec{}, []byte("icc"))
	if err == nil {
		t.Fatal("expected an error when the input has no startxref")
	}
	if !strings.Contains(err.Error(), "pdfmeta: xmp injection:") {
		t.Errorf("expected the xmp-injection wrap prefix, got %q", err)
	}
}

// TestMakePDFA3_PropagatesOutputIntentInjectionError covers the
// "pdfmeta: outputintent injection: %w" wrap around the third stage.
// minimalPDF() lets stages 1 and 2 succeed; a nil iccProfile fails only
// InjectOutputIntent's own "empty ICC profile" guard.
func TestMakePDFA3_PropagatesOutputIntentInjectionError(t *testing.T) {
	_, err := MakePDFA3(minimalPDF(), XMPSpec{}, nil)
	if err == nil {
		t.Fatal("expected an error for a nil ICC profile")
	}
	if !strings.Contains(err.Error(), "pdfmeta: outputintent injection:") {
		t.Errorf("expected the outputintent-injection wrap prefix, got %q", err)
	}
}

// TestStreamPayload_EmptyInputReturnsUnchanged covers the "len(data) ==
// 0" guard. Without it, streamPayload panics on data[len(data)-1] since
// len(data)-1 == -1 for an empty slice -- a real crash, not a value
// mismatch, which is why this guard exists before the two newline-check
// branches below it can index into data at all.
//
// NOTE ON THE OTHER TWO BRANCHES (not tested here, see
// `git log --all -p -- DEVELOPER_HANDBOOK.md`, trimmed from the public
// handbook in commit ebfd971): streamPayload's third branch
// (data ending in '\n' or '\r') returns a byte-for-byte copy of data,
// not a trailing-EOL-stripped version despite the name and the
// newline-conditioned structure suggesting that was the intent. Both
// call sites (InjectXMPStream, InjectOutputIntent) write an unconditional
// extra '\n' after the returned bytes regardless, and
// BuildXMPPacket's real output always ends in '\n' (fmt.Fprintln's
// trailing newline on `<?xpacket end="w"?>`), so this branch is the one
// actually exercised by every real InjectXMPStream call today -- not a
// rare edge case. Confirmed not a functional bug (PDF readers tolerate
// extra whitespace before `endstream`, and the extra byte isn't counted
// in /Length either way), but the copy is not load-bearing: it is
// observably identical in content to the "return data" branch, and this
// test does not assert otherwise.
func TestStreamPayload_EmptyInputReturnsUnchanged(t *testing.T) {
	if got := streamPayload(nil); len(got) != 0 {
		t.Errorf("expected an empty result for nil input, got %q", got)
	}
	if got := streamPayload([]byte{}); len(got) != 0 {
		t.Errorf("expected an empty result for empty input, got %q", got)
	}
}

// readTrailerIDValue is tested directly rather than through
// InjectXMPStream/trailerIDEntry: its happy path is already exercised
// indirectly via TestMakePDFA3PreservesMetadataAndOutputIntentInLatestCatalog,
// but its five failure guards need malformed trailer text no PDF this
// app generates would ever produce -- the same rationale as
// parseTrailerSizeAndRoot and replaceStartxrefValue's direct tests in
// earlier increments.

// TestReadTrailerIDValue_XrefOffsetOutOfRange covers both operands of
// "xrefOffset < 0 || xrefOffset >= len(b)". Table is a slice, not a map,
// for deterministic execution order -- same reasoning as
// TestParseTrailerSizeAndRoot_XrefOffsetOutOfRange, whose precedent this
// mirrors exactly: the "negative" subcase panics on b[xrefOffset:] with
// a negative index when the guard is deleted; the "exactly len(b)"
// subcase does not panic (b[len(b):] is a legal empty slice) and
// cascades into the very next guard (trailerIdx < 0) with the identical
// ("", false) result either way, so it does not independently
// discriminate the mutation -- accepted as the same non-discriminating-
// boundary-subcase shape already established for that precedent, not
// treated as a masking bug to fix.
func TestReadTrailerIDValue_XrefOffsetOutOfRange(t *testing.T) {
	b := []byte("trailer\n<< /ID [<abc>] >>\nstartxref\n0\n%%EOF\n")
	cases := []struct {
		name   string
		offset int
	}{
		{"negative", -1},
		{"exactly len(b)", len(b)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, ok := readTrailerIDValue(b, c.offset); ok {
				t.Errorf("expected ok=false for an out-of-range offset %d", c.offset)
			}
		})
	}
}

// TestReadTrailerIDValue_TrailerKeywordNotFound covers the "trailerIdx <
// 0" guard with xrefOffset == 0, chosen deliberately: with the guard
// deleted, trailerIdx stays -1, absTrailer becomes 0 + (-1) == -1, and
// the next line's b[absTrailer:] slices at a negative index and panics
// -- confirmed by mutation, not just traced.
func TestReadTrailerIDValue_TrailerKeywordNotFound(t *testing.T) {
	b := []byte("xref\n0 1\n0000000000 65535 f \nno keyword\nstartxref\n0\n%%EOF\n")
	if _, ok := readTrailerIDValue(b, 0); ok {
		t.Error("expected ok=false when the trailer keyword is absent")
	}
}

// TestReadTrailerIDValue_StartxrefNotFoundAfterTrailer covers the
// "endIdx < 0" guard. With the guard deleted, endIdx stays -1 and
// block := b[absTrailer : absTrailer+endIdx] becomes a high < low slice
// expression, which panics -- confirmed by mutation.
func TestReadTrailerIDValue_StartxrefNotFoundAfterTrailer(t *testing.T) {
	b := []byte("trailer\n<< /Size 4 /Root 1 0 R >>\nno marker here\n")
	if _, ok := readTrailerIDValue(b, 0); ok {
		t.Error("expected ok=false when no startxref follows the trailer keyword")
	}
}

// TestReadTrailerIDValue_IDNotFollowedByArray covers the "block[i] !=
// '['" guard. A bare bytes.Contains/ok==false check on a fixture like
// "/ID (not an array)" would NOT discriminate this guard from the very
// next one (unterminated array): with this guard deleted, the scan
// falls into the ']'-search loop, finds no ']', and the next guard
// returns the identical ("", false) -- masked. This fixture instead
// gives the fallthrough a ']' to find: "/ID (abc) /Other [1 2]" is not
// a valid /ID array, but with the guard deleted the scan starts at '('
// and the first ']' it finds is the one inside "[1 2]", returning
// ("(abc) /Other [1 2]", true) -- a wrong, non-empty value with ok=true,
// confirmed by mutation (not just traced) to be exactly that string, and
// therefore genuinely distinguishable from the guard-present ("",
// false) result.
func TestReadTrailerIDValue_IDNotFollowedByArray(t *testing.T) {
	b := []byte("trailer\n<< /ID (abc) /Other [1 2] >>\nstartxref\n0\n%%EOF\n")
	id, ok := readTrailerIDValue(b, 0)
	if ok {
		t.Errorf("expected ok=false when /ID is not followed by '[', got %q", id)
	}
	if id != "" {
		t.Errorf("expected an empty id, got %q", id)
	}
}

// TestReadTrailerIDValue_UnterminatedIDArray covers the "i >=
// len(block)" guard (no closing ']' before the block ends at
// "startxref"). With the guard deleted, the loop exits at i ==
// len(block), and block[start:i+1] does NOT panic -- Go's slice
// capacity for `block` (itself a sub-slice of b) extends past its own
// length to the end of b's backing array, so the expression legally
// reads one byte beyond block's logical end. Confirmed by mutation, not
// predicted: the result is ("[<abc><def>\ns", true) -- a wrong,
// non-empty value with ok=true, ending in the first byte of the
// following "startxref" keyword. An earlier draft of this docstring
// assumed a panic by analogy with the other four guards in this
// function before actually running the mutation; corrected to the
// observed behavior.
func TestReadTrailerIDValue_UnterminatedIDArray(t *testing.T) {
	b := []byte("trailer\n<< /ID [<abc><def>\nstartxref\n0\n%%EOF\n")
	if _, ok := readTrailerIDValue(b, 0); ok {
		t.Error("expected ok=false for an unterminated /ID array")
	}
}

// TestEnsureBinaryHeaderComment_MissingHeaderReturnsError covers the
// "missing PDF header" guard: input that doesn't start with "%PDF-".
// Confirmed by mutation: with the guard deleted, the fixture cascades
// into the next guard's "unterminated" text instead (this input has
// no newline either), so the assertion on this guard's own text still
// catches the mutation.
func TestEnsureBinaryHeaderComment_MissingHeaderReturnsError(t *testing.T) {
	_, err := ensureBinaryHeaderComment([]byte("not a pdf"))
	if err == nil {
		t.Fatal("expected an error for input missing the %PDF- header")
	}
	if !strings.Contains(err.Error(), "missing PDF header") {
		t.Errorf("expected a \"missing PDF header\" error, got %q", err)
	}
}

// TestEnsureBinaryHeaderComment_UnterminatedHeaderReturnsError covers
// the "PDF header line is unterminated" guard: a header with no
// newline anywhere in the buffer. Confirmed by mutation: with the
// guard deleted, headerEnd stays -1 and nextLine becomes the whole
// buffer via pdfBytes[0:], which cascades into
// findLastStartxref's "startxref keyword not found" instead — a
// different, non-"unterminated" error the assertion catches.
func TestEnsureBinaryHeaderComment_UnterminatedHeaderReturnsError(t *testing.T) {
	_, err := ensureBinaryHeaderComment([]byte("%PDF-1.4"))
	if err == nil {
		t.Fatal("expected an error for an unterminated header line")
	}
	if !strings.Contains(err.Error(), "unterminated") {
		t.Errorf("expected an \"unterminated\" error, got %q", err)
	}
}

// TestEnsureBinaryHeaderComment_AlreadyHasCommentReturnsInputUnchanged
// covers the early-return success path when the binary header comment
// is already present. minimalPDF() already carries the comment (see
// minimalPDFWithCatalog). Asserts pointer identity, not just byte
// equality: this path returns pdfBytes itself with no copy, and a
// future "defensive copy" refactor changing that allocation behavior
// should be a deliberate, reviewed decision rather than an invisible
// side effect. Confirmed by mutation: with the early return deleted,
// the output gains a second, duplicate binary-header comment.
func TestEnsureBinaryHeaderComment_AlreadyHasCommentReturnsInputUnchanged(t *testing.T) {
	pdf := minimalPDF()
	out, err := ensureBinaryHeaderComment(pdf)
	if err != nil {
		t.Fatalf("ensureBinaryHeaderComment: %v", err)
	}
	if !bytes.Equal(out, pdf) {
		t.Fatalf("expected the input returned unchanged, got %q", string(out))
	}
	if len(pdf) > 0 && &out[0] != &pdf[0] {
		t.Error("expected the early-return path to return the input slice itself, not a copy")
	}
}

// TestEnsureBinaryHeaderComment_PropagatesFindLastStartxrefError
// covers the bare (unwrapped) propagation of findLastStartxref's own
// error at the "xrefOffset, err := findLastStartxref(pdfBytes)" call.
// The fixture's second line intentionally isn't a binary comment (so
// the early-return guard above doesn't fire first) and the buffer has
// no "startxref" anywhere. Confirmed by mutation: with the guard
// neutralized, xrefOffset stays 0 and execution proceeds into
// shiftClassicXrefOffsets, which fails with its own, different error
// text — the assertion on findLastStartxref's exact text catches it.
func TestEnsureBinaryHeaderComment_PropagatesFindLastStartxrefError(t *testing.T) {
	_, err := ensureBinaryHeaderComment([]byte("%PDF-1.4\nnothing relevant appears in this buffer\n"))
	if err == nil {
		t.Fatal("expected findLastStartxref's error to propagate")
	}
	if !strings.Contains(err.Error(), "startxref keyword not found") {
		t.Errorf("expected findLastStartxref's exact error text, got %q", err)
	}
}

// TestEnsureBinaryHeaderComment_PropagatesShiftClassicXrefOffsetsError
// covers the bare propagation of shiftClassicXrefOffsets's error. The
// fixture's startxref points at plain text, not a "xref" keyword, so
// shiftClassicXrefOffsets's own first guard fires. Confirmed by
// mutation.
func TestEnsureBinaryHeaderComment_PropagatesShiftClassicXrefOffsetsError(t *testing.T) {
	_, err := ensureBinaryHeaderComment([]byte("%PDF-1.4\nsome content here\nstartxref\n9\n%%EOF\n"))
	if err == nil {
		t.Fatal("expected shiftClassicXrefOffsets's error to propagate")
	}
	if !strings.Contains(err.Error(), "does not point to classic xref") {
		t.Errorf("expected shiftClassicXrefOffsets's exact error text, got %q", err)
	}
}

// TestEnsureBinaryHeaderComment_InsertsCommentAndShiftsOffsets is the
// direct positive-path test (the indirect happy path is already
// exercised via TestMakePDFA3PreservesMetadataAndOutputIntentInLatestCatalog,
// but that only proves the full MakePDFA3 chain works, not this
// function in isolation).
//
// Deliberately does NOT rely on findObjectBody to validate the shift:
// findObjectBody locates objects by a textual "<id> <gen> obj" scan,
// never by dereferencing the xref table's recorded byte offsets — so
// it would report success even if every offset in the table were
// wrong, as long as the object text itself is still present somewhere
// in the buffer. That would make an off-by-len(comment) arithmetic
// bug in shiftClassicXrefOffsets invisible to a findObjectBody-based
// round-trip (confirmed directly: mutating the shift by one byte still
// passed a findObjectBody-based check, since a real xref-offset-aware
// reader is the only thing that would notice). This test instead
// parses minimalPDFWithoutBinaryComment()'s known xref structure
// directly — subsection "0 4", four fixed-width 20-byte entries, one
// per object in ID order — and reads the shifted offset for object 1
// out of the table itself, then asserts pdfBytes at exactly that
// offset starts with "1 0 obj". That is the assertion an actual
// xref-offset-consuming PDF reader depends on, and the one that would
// actually catch a wrong shift.
func TestEnsureBinaryHeaderComment_InsertsCommentAndShiftsOffsets(t *testing.T) {
	pdf := minimalPDFWithoutBinaryComment()
	out, err := ensureBinaryHeaderComment(pdf)
	if err != nil {
		t.Fatalf("ensureBinaryHeaderComment: %v", err)
	}
	_, afterHeader, ok := bytes.Cut(out, []byte("\n"))
	if !ok || !hasBinaryHeaderComment(afterHeader) {
		t.Fatalf("output missing binary header comment: %q", out[:min(len(out), 32)])
	}
	xrefOff, err := findLastStartxref(out)
	if err != nil {
		t.Fatalf("findLastStartxref on shifted output: %v", err)
	}
	const subsectionHeader = "xref\n0 4\n"
	if !bytes.HasPrefix(out[xrefOff:], []byte(subsectionHeader)) {
		t.Fatalf("unexpected xref subsection shape: %q", out[xrefOff:min(xrefOff+len(subsectionHeader)+10, len(out))])
	}
	// Entry 0 is the free-list head; entry 1 is object 1 (Catalog).
	entriesStart := xrefOff + len(subsectionHeader)
	obj1Entry := out[entriesStart+20 : entriesStart+40]
	obj1Off, err := parsePositiveDecimal(obj1Entry[:10])
	if err != nil {
		t.Fatalf("parsing object 1's shifted xref entry: %v", err)
	}
	if !bytes.HasPrefix(out[obj1Off:], []byte("1 0 obj")) {
		t.Errorf("object 1's shifted xref entry points at %q, not \"1 0 obj\"", out[obj1Off:min(obj1Off+20, len(out))])
	}

	// Independently confirm the Catalog is also reachable via the
	// textual path (parseTrailerSizeAndRoot + findObjectBody), the
	// same shape TestMakePDFA3PreservesMetadataAndOutputIntentInLatestCatalog
	// checks — this proves the trailer's /Root reference and the
	// object body itself are intact, complementary to (not a
	// substitute for) the offset check above.
	_, root, gen, err := parseTrailerSizeAndRoot(out, xrefOff)
	if err != nil {
		t.Fatalf("parseTrailerSizeAndRoot on shifted output: %v", err)
	}
	catalog, err := findObjectBody(out, root, gen)
	if err != nil {
		t.Fatalf("findObjectBody(root) on shifted output: %v", err)
	}
	if !bytes.Contains(catalog, []byte("/Type /Catalog")) {
		t.Fatalf("Root object is not a Catalog: %q", string(catalog))
	}
}

// TestHasBinaryHeaderComment_NonBinaryCommentReturnsFalse covers the
// "line[i] <= 127" loop body — a %-prefixed line that is NOT the
// binary marker, e.g. an ordinary ASCII PDF comment. Deleting this
// check would make the function return true for any 6+-byte line
// starting with '%' whose 6th byte is a line terminator, regardless
// of what's in between — the fixture below is chosen to pass the
// preceding len/prefix guard cleanly (6 bytes, starts with '%') so a
// mutation of *this* guard specifically is what the test would catch,
// not an accidental trip of the guard before it. Confirmed by
// mutation.
func TestHasBinaryHeaderComment_NonBinaryCommentReturnsFalse(t *testing.T) {
	if hasBinaryHeaderComment([]byte("%ABCD\n")) {
		t.Error("expected an ordinary ASCII comment line to not be recognized as the binary marker")
	}
}

// TestShiftClassicXrefOffsets_InvalidXrefOffsetReturnsError covers the
// combined "xrefOffset < 0 || xrefOffset >= len(pdfBytes) ||
// !bytes.HasPrefix(...)" guard across all three of its conditions. A
// slice, not a map, for deterministic execution order (see the same
// rationale in TestParseTrailerSizeAndRoot_XrefOffsetOutOfRange).
// Confirmed by mutation: with the whole guard deleted, all four
// subcases fail (none produce the guard's "does not point to classic
// xref" text), each for a different downstream reason — "negative"
// and "wrong prefix" both cascade into "malformed xref subsection
// header" (parsing "f" or the entry line's leading digits as a
// 1-field subsection header), "exactly len(buf)" and "past len(buf)"
// both cascade into "xref trailer not found" (an empty post-offset
// slice never contains a "trailer" line).
//
// Each subcase gets its own fresh buffer, not a buffer shared across
// the loop: shiftClassicXrefOffsets mutates its input in place as it
// rewrites xref entries, so a subcase that (under some other mutation)
// reaches the entry-rewrite loop before erroring would otherwise leak
// its partial rewrite into the next subcase's input — the same
// order-dependence risk the map-to-slice conversion above addresses
// for iteration order, but for shared mutable state instead.
func TestShiftClassicXrefOffsets_InvalidXrefOffsetReturnsError(t *testing.T) {
	const fixture = "xref\n0 1\n0000000000 65535 f \ntrailer\n"
	cases := []struct {
		name   string
		offset int
	}{
		{"negative", -1},
		{"exactly len(buf)", len(fixture)},
		{"past len(buf)", len(fixture) + 100},
		{"in range but wrong prefix", 5},
	}
	for _, c := range cases {
		buf := []byte(fixture)
		err := shiftClassicXrefOffsets(buf, c.offset, 6)
		if err == nil {
			t.Errorf("%s: expected an error", c.name)
			continue
		}
		if !strings.Contains(err.Error(), "does not point to classic xref") {
			t.Errorf("%s: expected the guard's own error text, got %q", c.name, err)
		}
	}
}

// TestShiftClassicXrefOffsets_UnterminatedXrefLineReturnsError covers
// the first "unterminated xref line" guard: a subsection header line
// with no trailing newline anywhere in the buffer. Break-verifies via
// panic (deleting the guard leaves lineEnd == -1, and the subsequent
// pdfBytes[lineStart:lineEnd] slice expression panics with a
// slice-bounds-out-of-range error), the same category as increment
// 2a's TestFindObjectBody_EndobjNotFound. Confirmed by direct mutation.
func TestShiftClassicXrefOffsets_UnterminatedXrefLineReturnsError(t *testing.T) {
	err := shiftClassicXrefOffsets([]byte("xref\n0 1"), 0, 6)
	if err == nil {
		t.Fatal("expected an error for an unterminated xref line")
	}
	if !strings.Contains(err.Error(), "unterminated xref line") {
		t.Errorf("expected the guard's own error text, got %q", err)
	}
}

// TestShiftClassicXrefOffsets_MalformedSubsectionHeaderReturnsError
// covers the "len(fields) != 2" guard: a subsection header line that
// doesn't parse into exactly a start-ID and a count. Break-verifies
// via panic: deleting the guard leaves fields with only 1 element
// ("badheader" has no count field), and the subsequent fields[1]
// access panics with an index-out-of-range error. Confirmed by direct
// mutation.
func TestShiftClassicXrefOffsets_MalformedSubsectionHeaderReturnsError(t *testing.T) {
	err := shiftClassicXrefOffsets([]byte("xref\nbadheader\ntrailer\n"), 0, 6)
	if err == nil {
		t.Fatal("expected an error for a malformed subsection header")
	}
	if !strings.Contains(err.Error(), "malformed xref subsection header") {
		t.Errorf("expected the guard's own error text, got %q", err)
	}
}

// TestShiftClassicXrefOffsets_InvalidSubsectionCountReturnsError
// covers the "xref subsection count: %w" wrap around
// parsePositiveDecimal's own error. Confirmed by mutation: with the
// guard neutralized, count stays 0 (the zero value on parse failure),
// so the entry loop never runs and control falls straight back to the
// outer loop's next line — which is "trailer", so the mutated function
// silently returns nil instead of erroring.
func TestShiftClassicXrefOffsets_InvalidSubsectionCountReturnsError(t *testing.T) {
	err := shiftClassicXrefOffsets([]byte("xref\n0 abc\ntrailer\n"), 0, 6)
	if err == nil {
		t.Fatal("expected an error for a non-numeric subsection count")
	}
	if !strings.Contains(err.Error(), "xref subsection count:") {
		t.Errorf("expected the wrap prefix, got %q", err)
	}
}

// TestShiftClassicXrefOffsets_UnterminatedEntryReturnsError covers the
// second "unterminated xref entry" guard: the subsection header
// promises one entry but the entry line has no trailing newline.
// Break-verifies via panic, same shape as the xref-line guard above.
// Confirmed by direct mutation.
func TestShiftClassicXrefOffsets_UnterminatedEntryReturnsError(t *testing.T) {
	err := shiftClassicXrefOffsets([]byte("xref\n0 1\n0000000000 65535 f "), 0, 6)
	if err == nil {
		t.Fatal("expected an error for an unterminated xref entry")
	}
	if !strings.Contains(err.Error(), "unterminated xref entry") {
		t.Errorf("expected the guard's own error text, got %q", err)
	}
}

// TestShiftClassicXrefOffsets_InvalidEntryOffsetReturnsError covers
// the "xref entry offset: %w" wrap around parsePositiveDecimal's own
// error when the entry's 10-digit offset field isn't numeric.
// Confirmed by mutation: with the guard neutralized, oldOff stays 0
// (the zero value on parse failure) and the function silently writes
// a corrupted-but-plausible-looking offset into the entry instead of
// erroring.
func TestShiftClassicXrefOffsets_InvalidEntryOffsetReturnsError(t *testing.T) {
	err := shiftClassicXrefOffsets([]byte("xref\n0 1\nabcdefghij 65535 n \ntrailer\n"), 0, 6)
	if err == nil {
		t.Fatal("expected an error for a non-numeric entry offset")
	}
	if !strings.Contains(err.Error(), "xref entry offset:") {
		t.Errorf("expected the wrap prefix, got %q", err)
	}
}

// TestShiftClassicXrefOffsets_OffsetOverflowReturnsError covers a real
// bug found and fixed during this increment's coverage work, not just
// a pre-existing guard: copy(pdfBytes[entryStart:entryStart+10], ...)
// into the fixed-width 10-digit offset field would silently truncate
// (rather than error) if the shifted offset ever needed 11+ digits —
// a source-slice-longer-than-destination copy() truncates instead of
// panicking. Only reachable in practice on a PDF whose existing xref
// offset is within `delta` of 9999999999 (~9.3GB), astronomically
// unlikely for this application's generated PDFs, but the guard
// prevents silent offset corruption rather than requiring that
// unlikelihood to hold forever. Directly exercises the arithmetic
// without needing an actual multi-gigabyte fixture: the function
// never dereferences pdfBytes at the parsed offset, only parses and
// rewrites the digit text. Confirmed by mutation: with the guard
// deleted, this test correctly goes red. The exact corrupted value the
// deleted guard would otherwise let through silently: copy() takes
// only the first 10 bytes of the 11-digit %010d result, so a true
// offset of 10000000004 gets written as 1000000000 — nearly 10x
// smaller than correct, with no error raised.
func TestShiftClassicXrefOffsets_OffsetOverflowReturnsError(t *testing.T) {
	err := shiftClassicXrefOffsets([]byte("xref\n0 1\n9999999998 65535 n \ntrailer\n"), 0, 6)
	if err == nil {
		t.Fatal("expected an error when the shifted offset exceeds the 10-digit field width")
	}
	if !strings.Contains(err.Error(), "exceeds the classic xref format's 10-digit field width") {
		t.Errorf("expected the overflow guard's own error text, got %q", err)
	}
}

// TestShiftClassicXrefOffsets_TrailerNotFoundReturnsError covers the
// terminal "xref trailer not found" fallthrough (only reachable if the
// loop exits via reaching end-of-buffer, since every other exit is an
// inner return). The fixture's one xref entry is engineered to consume
// exactly to the end of the buffer with no following "trailer" line —
// the entry's flag byte is 'f' (free), not 'n', to bypass the
// offset-shift arithmetic entirely and isolate this guard. Coverage
// gap here is only reachable via a compile error, not a behavior
// mutation: this is the function's final statement, so deleting it
// leaves a code path with no return — a "missing return" compile
// error, not something a mutate/run/restore cycle can verify. Same
// category as increment 2a's TestInsertOutputIntentsReference_InsertsFreshEntry.
func TestShiftClassicXrefOffsets_TrailerNotFoundReturnsError(t *testing.T) {
	err := shiftClassicXrefOffsets([]byte("xref\n0 1\n0000000000 65535 f \n"), 0, 6)
	if err == nil {
		t.Fatal("expected an error when no trailer line follows the xref subsections")
	}
	if !strings.Contains(err.Error(), "xref trailer not found") {
		t.Errorf("expected the guard's own error text, got %q", err)
	}
}

// TestParsePositiveDecimal_EmptyReturnsError covers the
// "len(b) == 0" guard (after TrimSpace, so whitespace-only input hits
// it too). Confirmed by mutation: with the guard deleted, the empty
// range loop below executes zero times and silently returns (0, nil).
func TestParsePositiveDecimal_EmptyReturnsError(t *testing.T) {
	_, err := parsePositiveDecimal([]byte("   "))
	if err == nil {
		t.Fatal("expected an error for empty/whitespace-only input")
	}
	if !strings.Contains(err.Error(), "empty decimal") {
		t.Errorf("expected the guard's own error text, got %q", err)
	}
}

// TestParsePositiveDecimal_NonDigitReturnsError covers the
// "c < '0' || c > '9'" guard. Confirmed by mutation.
func TestParsePositiveDecimal_NonDigitReturnsError(t *testing.T) {
	_, err := parsePositiveDecimal([]byte("12a3"))
	if err == nil {
		t.Fatal("expected an error for a non-digit byte")
	}
	if !strings.Contains(err.Error(), "invalid decimal") {
		t.Errorf("expected the guard's own error text, got %q", err)
	}
}

// TestParsePositiveDecimal_OverflowReturnsError covers the wrapped
// parseDigits error. parsePositiveDecimal's only current caller
// (shiftClassicXrefOffsets) always passes exactly 10 digits, so this
// path isn't reachable through that call chain — parsePositiveDecimal
// is a general-purpose helper tested directly, same rationale as its
// other two guards. Confirmed by mutation.
func TestParsePositiveDecimal_OverflowReturnsError(t *testing.T) {
	_, err := parsePositiveDecimal([]byte("99999999999999999999"))
	if err == nil {
		t.Fatal("expected an error for a decimal that overflows int")
	}
	if !strings.Contains(err.Error(), "overflows") {
		t.Errorf("expected the guard's own error text, got %q", err)
	}
}

// TestReplaceStartxrefValue_KeywordNotFoundReturnsError and
// TestReplaceStartxrefValue_ValueMissingReturnsError cover
// replaceStartxrefValue's two guards directly rather than through
// ensureBinaryHeaderComment, its only caller: both guards are
// structurally unreachable through that call chain. findLastStartxref
// already validates that "startxref" followed by digits exists in the
// original bytes before ensureBinaryHeaderComment ever calls
// shiftClassicXrefOffsets or replaceStartxrefValue, the binary-comment
// insertion happens only near the file's start and never touches the
// trailer/startxref region, and both functions search for the literal
// "startxref" text via bytes.LastIndex — so if findLastStartxref found
// it, replaceStartxrefValue searching the (differently-offset but
// content-identical-in-that-region) post-insertion buffer always finds
// the same text. Verified by reasoning about the call sequence, not
// assumed: no fixture reaching ensureBinaryHeaderComment can trip
// either guard without also tripping findLastStartxref's identical
// checks first. Tested directly since replaceStartxrefValue is a
// general-purpose helper whose own guards deserve unit coverage
// regardless of what its one current caller can reach. Both guards
// confirmed by mutation: deleting the "keyword not found" guard
// cascades into the "value missing" guard's text (idx stays -1, and
// the subsequent skip/scan lands past the fixture's short buffer with
// no digits found); deleting the "value missing" guard silently
// splices the replacement offset into a zero-length gap instead of
// erroring.
func TestReplaceStartxrefValue_KeywordNotFoundReturnsError(t *testing.T) {
	_, err := replaceStartxrefValue([]byte("no keyword here"), 5)
	if err == nil {
		t.Fatal("expected an error when \"startxref\" is absent")
	}
	if !strings.Contains(err.Error(), "startxref keyword not found") {
		t.Errorf("expected the guard's own error text, got %q", err)
	}
}

func TestReplaceStartxrefValue_ValueMissingReturnsError(t *testing.T) {
	_, err := replaceStartxrefValue([]byte("startxref\nno digits\n"), 5)
	if err == nil {
		t.Fatal("expected an error when no digits follow \"startxref\"")
	}
	if !strings.Contains(err.Error(), "startxref value missing") {
		t.Errorf("expected the guard's own error text, got %q", err)
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

// TestBuildXMPPacket_DefaultsTitleWhenEmpty covers the
// "spec.Title == ”" default branch. Not masked: deleting the guard
// leaves an empty <rdf:li></rdf:li> inside <dc:title>, so
// "GoPMgr document" is genuinely absent from the output rather than
// just relocated — a bare bytes.Contains would already catch this.
// Uses the tag-adjacent needle anyway for symmetry with the Author
// test below, which does need it. Confirmed by mutation.
func TestBuildXMPPacket_DefaultsTitleWhenEmpty(t *testing.T) {
	pkt := BuildXMPPacket(XMPSpec{})
	needle := []byte(`<dc:title><rdf:Alt><rdf:li xml:lang="x-default">GoPMgr document</rdf:li>`)
	if !bytes.Contains(pkt, needle) {
		t.Errorf("expected /Title to default to \"GoPMgr document\" when unset, got %q", string(pkt))
	}
}

// TestBuildXMPPacket_DefaultsAuthorWhenEmpty covers the
// "spec.Author == ”" default branch. Masked three ways by a bare
// bytes.Contains(pkt, []byte("GoPMgr")): the packet unconditionally
// contains "GoPMgr" from x:xmptk="GoPMgr", <pdf:Producer>GoPMgr
// </pdf:Producer>, and (when spec.CreatorTool is also unset, as here)
// <xmp:CreatorTool>GoPMgr</xmp:CreatorTool> — none of which depend on
// this guard. The tag-adjacent needle isolates <dc:creator>'s own
// value. Confirmed by mutation: with the guard deleted, the observed
// output was an empty <dc:creator><rdf:Seq><rdf:li></rdf:li> — the
// three unconditional "GoPMgr" occurrences remained present elsewhere
// in the packet exactly as predicted, and the needle caught the
// difference where a bare bytes.Contains(pkt, []byte("GoPMgr")) would
// not have.
func TestBuildXMPPacket_DefaultsAuthorWhenEmpty(t *testing.T) {
	pkt := BuildXMPPacket(XMPSpec{})
	needle := []byte(`<dc:creator><rdf:Seq><rdf:li>GoPMgr</rdf:li>`)
	if !bytes.Contains(pkt, needle) {
		t.Errorf("expected /Author to default to \"GoPMgr\" when unset, got %q", string(pkt))
	}
}

// TestBuildXMPPacket_IncludesDescriptionWhenSet covers the
// "spec.Description != ”" guard — the one uncovered branch in this
// function not accounted for in this increment's original plan (only
// the Title and Author defaults were scoped in; this one surfaced
// during the coverage check after those two landed). Break-verifies
// via key absence: with the guard deleted, no <dc:description> element
// is emitted at all regardless of spec.Description, so its presence in
// the output is itself the guard-ran signal. Confirmed by mutation.
func TestBuildXMPPacket_IncludesDescriptionWhenSet(t *testing.T) {
	pkt := BuildXMPPacket(XMPSpec{Description: "a test description"})
	needle := []byte(`<dc:description><rdf:Alt><rdf:li xml:lang="x-default">a test description</rdf:li>`)
	if !bytes.Contains(pkt, needle) {
		t.Errorf("expected /Description to appear when set, got %q", string(pkt))
	}
}

// TestInjectPAdESSignature_RejectsEmpty asserts the guard's own error
// text, not a bare err != nil: a nil pdfBytes also fails findLastStartxref
// a few lines later with a different, still-non-nil error, so a bare
// check would pass even with this guard deleted (confirmed by mutation).
func TestInjectPAdESSignature_RejectsEmpty(t *testing.T) {
	_, err := InjectPAdESSignature(nil, func(b []byte) ([]byte, error) { return b, nil })
	if err == nil {
		t.Fatal("expected error on empty PDF")
	}
	if !strings.Contains(err.Error(), "empty PDF for signing") {
		t.Errorf("expected the guard's own error text, got %q", err)
	}
}

// Break-verified with a mutation-testing note: with this guard disabled,
// the fixture doesn't fail cleanly — it panics on the nil call a few
// lines later (invalid memory address). That panic still fails the test
// (Go's test runner reports it as a failure), so it's a valid red signal,
// just not a clean error return.
func TestInjectPAdESSignature_RejectsNilSignRanges(t *testing.T) {
	_, err := InjectPAdESSignature(minimalPDF(), nil)
	if err == nil {
		t.Fatal("expected error on nil signRanges callback")
	}
	if !strings.Contains(err.Error(), "signRanges callback required") {
		t.Errorf("expected the guard's own error text, got %q", err)
	}
}

// TestInjectPAdESSignature_PropagatesLocateStartxrefError covers the
// "pdfmeta: locate startxref: %w" wrap — same fixture pattern as
// TestInjectXMPStream_PropagatesLocateStartxrefError.
func TestInjectPAdESSignature_PropagatesLocateStartxrefError(t *testing.T) {
	_, err := InjectPAdESSignature([]byte("%PDF-1.4\nno relevant marker here\n"), func(b []byte) ([]byte, error) { return b, nil })
	if err == nil {
		t.Fatal("expected an error when startxref is absent")
	}
	if !strings.Contains(err.Error(), "pdfmeta: locate startxref:") {
		t.Errorf("expected the locate-startxref wrap prefix, got %q", err)
	}
}

// TestInjectPAdESSignature_PropagatesParseTrailerError covers the
// "pdfmeta: parse trailer: %w" wrap — same fixture pattern as
// TestInjectXMPStream_PropagatesParseTrailerError.
func TestInjectPAdESSignature_PropagatesParseTrailerError(t *testing.T) {
	_, err := InjectPAdESSignature([]byte("%PDF-1.4\nxxxx\nstartxref\n5\n%%EOF\n"), func(b []byte) ([]byte, error) { return b, nil })
	if err == nil {
		t.Fatal("expected an error when the trailer keyword is absent")
	}
	if !strings.Contains(err.Error(), "pdfmeta: parse trailer:") {
		t.Errorf("expected the parse-trailer wrap prefix, got %q", err)
	}
}

// TestInjectPAdESSignature_PropagatesLocateCatalogError covers the
// "pdfmeta: locate Catalog: %w" wrap — same fixture pattern as
// TestInjectXMPStream_PropagatesLocateCatalogError.
func TestInjectPAdESSignature_PropagatesLocateCatalogError(t *testing.T) {
	b := []byte("%PDF-1.4\ntrailer\n<< /Size 4 /Root 1 0 R >>\nstartxref\n5\n%%EOF\n")
	_, err := InjectPAdESSignature(b, func(b []byte) ([]byte, error) { return b, nil })
	if err == nil {
		t.Fatal("expected an error when the Catalog object is absent")
	}
	if !strings.Contains(err.Error(), "pdfmeta: locate Catalog:") {
		t.Errorf("expected the locate-Catalog wrap prefix, got %q", err)
	}
}

// TestInjectPAdESSignature_AddsTrailingNewlineWhenMissing covers the
// "pdfBytes[base-1] != '\n'" guard. minimalPDF() already ends in '\n', so
// the fixture trims it to force the guard to fire. Same discriminator
// approach as TestInjectXMPStream_AddsTrailingNewlineWhenMissing: with the
// guard deleted, the appended signature object's leading digit glues
// directly onto the trimmed fixture's last byte with no separator.
func TestInjectPAdESSignature_AddsTrailingNewlineWhenMissing(t *testing.T) {
	pdfBytes := bytes.TrimRight(minimalPDF(), "\n")
	out, err := InjectPAdESSignature(pdfBytes, func(b []byte) ([]byte, error) { return []byte{0xde, 0xad, 0xbe, 0xef}, nil })
	if err != nil {
		t.Fatalf("InjectPAdESSignature: %v", err)
	}
	if out[len(pdfBytes)] != '\n' {
		t.Errorf("expected a newline inserted immediately after the original bytes, got %q", out[len(pdfBytes):min(len(out), len(pdfBytes)+20)])
	}
}

// TestInjectPAdESSignature_PropagatesSignRangesError covers the
// "pdfmeta: range signing failed: %w" wrap around the caller-supplied
// signRanges callback.
func TestInjectPAdESSignature_PropagatesSignRangesError(t *testing.T) {
	wantErr := fmt.Errorf("boom")
	_, err := InjectPAdESSignature(minimalPDF(), func(b []byte) ([]byte, error) { return nil, wantErr })
	if err == nil {
		t.Fatal("expected an error when signRanges fails")
	}
	if !strings.Contains(err.Error(), "pdfmeta: range signing failed:") {
		t.Errorf("expected the range-signing wrap prefix, got %q", err)
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

// --- readRefAt ---
//
// readRefAt's happy path is already exercised indirectly via
// TestInjectPAdESSignature_AppendsExistingIndirectAcroFormFields; these
// cover its own guards directly, the same rationale as readDictRef's
// unit tests above.

func TestReadRefAt_NoIdDigit(t *testing.T) {
	_, _, err := readRefAt([]byte("R"), 0, "/AcroForm")
	if err == nil {
		t.Fatal("expected an error when no id digits are present")
	}
	if !strings.Contains(err.Error(), "no id digit") {
		t.Errorf("expected a \"no id digit\" error, got %q", err)
	}
}

func TestReadRefAt_NoGenDigit(t *testing.T) {
	_, _, err := readRefAt([]byte("4 R"), 0, "/AcroForm")
	if err == nil {
		t.Fatal("expected an error when no gen digits follow the id")
	}
	if !strings.Contains(err.Error(), "no gen digit") {
		t.Errorf("expected a \"no gen digit\" error, got %q", err)
	}
}

func TestReadRefAt_ExpectedRAfterGen(t *testing.T) {
	_, _, err := readRefAt([]byte("4 0 X"), 0, "/AcroForm")
	if err == nil {
		t.Fatal("expected an error when the R marker is missing")
	}
	if !strings.Contains(err.Error(), "expected R after gen") {
		t.Errorf("expected an \"expected R after gen\" error, got %q", err)
	}
}

// TestReadRefAt_IdOverflowReturnsError and
// TestReadRefAt_GenOverflowReturnsError cover the wrapped parseDigits
// errors. Confirmed by mutation.
func TestReadRefAt_IdOverflowReturnsError(t *testing.T) {
	_, _, err := readRefAt([]byte("99999999999999999999 0 R"), 0, "/AcroForm")
	if err == nil {
		t.Fatal("expected an error when the id overflows int")
	}
	if !strings.Contains(err.Error(), "id") || !strings.Contains(err.Error(), "overflows") {
		t.Errorf("expected an id-overflow error, got %q", err)
	}
}

func TestReadRefAt_GenOverflowReturnsError(t *testing.T) {
	_, _, err := readRefAt([]byte("4 99999999999999999999 R"), 0, "/AcroForm")
	if err == nil {
		t.Fatal("expected an error when the gen overflows int")
	}
	if !strings.Contains(err.Error(), "gen") || !strings.Contains(err.Error(), "overflows") {
		t.Errorf("expected a gen-overflow error, got %q", err)
	}
}

// --- findDictionaryEnd / findArrayEnd ---

func TestFindDictionaryEnd_DoesNotStartWithOpenDict(t *testing.T) {
	_, err := findDictionaryEnd([]byte("not a dict"), 0)
	if err == nil {
		t.Fatal("expected an error when the input doesn't start with <<")
	}
	if !strings.Contains(err.Error(), "does not start with <<") {
		t.Errorf("expected the guard's own error text, got %q", err)
	}
}

// TestFindDictionaryEnd_UnterminatedReturnsError covers the terminal
// "unterminated dictionary" fallthrough. Coverage-only, not
// break-verifiable via deletion: it's the function's last statement,
// so removing it leaves a code path with no return — a "missing
// return" compile error, not something a mutate/run/restore cycle can
// verify. Same category as TestShiftClassicXrefOffsets_TrailerNotFoundReturnsError.
func TestFindDictionaryEnd_UnterminatedReturnsError(t *testing.T) {
	_, err := findDictionaryEnd([]byte("<< /Foo 1"), 0)
	if err == nil {
		t.Fatal("expected an error for an unterminated dictionary")
	}
	if !strings.Contains(err.Error(), "unterminated dictionary") {
		t.Errorf("expected the guard's own error text, got %q", err)
	}
}

func TestFindArrayEnd_DoesNotStartWithOpenBracket(t *testing.T) {
	_, err := findArrayEnd([]byte("not an array"), 0)
	if err == nil {
		t.Fatal("expected an error when the input doesn't start with [")
	}
	if !strings.Contains(err.Error(), "does not start with [") {
		t.Errorf("expected the guard's own error text, got %q", err)
	}
}

// TestFindArrayEnd_UnterminatedReturnsError is coverage-only, same
// rationale as TestFindDictionaryEnd_UnterminatedReturnsError: the
// terminal "unterminated array" return is the function's last
// statement.
func TestFindArrayEnd_UnterminatedReturnsError(t *testing.T) {
	_, err := findArrayEnd([]byte("[ 1 2 3"), 0)
	if err == nil {
		t.Fatal("expected an error for an unterminated array")
	}
	if !strings.Contains(err.Error(), "unterminated array") {
		t.Errorf("expected the guard's own error text, got %q", err)
	}
}

// --- signatureFieldReferenceRewrites ---
//
// Tested directly rather than only through InjectPAdESSignature: it's
// a general-purpose helper, and most of these branches (a malformed or
// unsupported /AcroForm entry, a dangling indirect reference) aren't
// reachable through any fixture this app's own PDF generation would
// produce — the same rationale used for replaceStartxrefValue and
// parseTrailerSizeAndRoot's guards in earlier increments.

// TestSignatureFieldReferenceRewrites_NonDictCatalogWrapsFresh covers
// the fallback when catalogBody doesn't start with "<<" at all: the
// entire body is wrapped in a fresh dict carrying a new /AcroForm.
// Not reachable via InjectPAdESSignature today (its Catalog always
// comes from findObjectBody on a real "<< ... >>" object body) but
// this is a general-purpose helper, so the branch gets direct
// coverage regardless.
func TestSignatureFieldReferenceRewrites_NonDictCatalogWrapsFresh(t *testing.T) {
	got, extra, err := signatureFieldReferenceRewrites(nil, []byte("not a dict"), 5)
	if err != nil {
		t.Fatalf("signatureFieldReferenceRewrites: %v", err)
	}
	if extra != nil {
		t.Errorf("expected no extra object rewrites, got %v", extra)
	}
	want := "<<\n/AcroForm << /Fields [ 5 0 R ] /SigFlags 3 >>\nnot a dict\n>>"
	if string(got) != want {
		t.Fatalf("got %q, want %q", string(got), want)
	}
}

// TestSignatureFieldReferenceRewrites_MalformedAcroFormEntryReturnsError
// covers "malformed /AcroForm entry": the key is present but nothing
// (not even whitespace) follows it before the catalog body ends.
func TestSignatureFieldReferenceRewrites_MalformedAcroFormEntryReturnsError(t *testing.T) {
	_, _, err := signatureFieldReferenceRewrites(nil, []byte("<< /AcroForm"), 5)
	if err == nil {
		t.Fatal("expected an error when /AcroForm has no value")
	}
	if !strings.Contains(err.Error(), "malformed /AcroForm entry") {
		t.Errorf("expected the guard's own error text, got %q", err)
	}
}

// TestSignatureFieldReferenceRewrites_DirectAcroFormUnterminatedReturnsError
// covers the "parse direct /AcroForm: %w" wrap around findDictionaryEnd's
// error: the /AcroForm value is a direct dict ("<<...") but never closes.
func TestSignatureFieldReferenceRewrites_DirectAcroFormUnterminatedReturnsError(t *testing.T) {
	_, _, err := signatureFieldReferenceRewrites(nil, []byte("<< /AcroForm << /Fields [ 1 0 R ]"), 5)
	if err == nil {
		t.Fatal("expected an error for an unterminated direct /AcroForm dict")
	}
	if !strings.Contains(err.Error(), "parse direct /AcroForm:") {
		t.Errorf("expected the wrap prefix, got %q", err)
	}
}

// TestSignatureFieldReferenceRewrites_DirectAcroFormMergeErrorPropagates
// covers the "merge direct /AcroForm: %w" wrap: the direct /AcroForm
// dict parses fine (findDictionaryEnd succeeds) but its /Fields value
// is an indirect reference rather than a direct array, which
// appendSignatureFieldToFields (via mergeSignatureFieldIntoAcroForm)
// rejects.
func TestSignatureFieldReferenceRewrites_DirectAcroFormMergeErrorPropagates(t *testing.T) {
	_, _, err := signatureFieldReferenceRewrites(nil, []byte("<< /AcroForm << /Fields 7 0 R /SigFlags 1 >> >>"), 5)
	if err == nil {
		t.Fatal("expected an error when the direct /AcroForm's /Fields is indirect")
	}
	if !strings.Contains(err.Error(), "merge direct /AcroForm:") {
		t.Errorf("expected the wrap prefix, got %q", err)
	}
}

// TestSignatureFieldReferenceRewrites_UnsupportedAcroFormEntryReturnsError
// covers the "unsupported /AcroForm entry: %w" wrap: the value is
// neither a direct dict nor a valid "<id> <gen> R" reference.
func TestSignatureFieldReferenceRewrites_UnsupportedAcroFormEntryReturnsError(t *testing.T) {
	_, _, err := signatureFieldReferenceRewrites(nil, []byte("<< /AcroForm true >>"), 5)
	if err == nil {
		t.Fatal("expected an error for an /AcroForm value that is neither a dict nor a reference")
	}
	if !strings.Contains(err.Error(), "unsupported /AcroForm entry:") {
		t.Errorf("expected the wrap prefix, got %q", err)
	}
}

// TestSignatureFieldReferenceRewrites_LocateAcroFormObjectErrorPropagates
// covers the "locate AcroForm object: %w" wrap: the /AcroForm value is
// a well-formed indirect reference, but the referenced object doesn't
// exist in pdfBytes.
func TestSignatureFieldReferenceRewrites_LocateAcroFormObjectErrorPropagates(t *testing.T) {
	pdf := minimalPDF() // defines objects 1-3 only
	_, _, err := signatureFieldReferenceRewrites(pdf, []byte("<< /AcroForm 99 0 R >>"), 5)
	if err == nil {
		t.Fatal("expected an error when the referenced AcroForm object doesn't exist")
	}
	if !strings.Contains(err.Error(), "locate AcroForm object:") {
		t.Errorf("expected the wrap prefix, got %q", err)
	}
}

// --- appendSignatureFieldToFields ---

func TestAppendSignatureFieldToFields_NonDictReturnsError(t *testing.T) {
	_, err := appendSignatureFieldToFields([]byte("not a dict"), 5)
	if err == nil {
		t.Fatal("expected an error when the input isn't a direct dictionary")
	}
	if !strings.Contains(err.Error(), "is not a direct dictionary") {
		t.Errorf("expected the guard's own error text, got %q", err)
	}
}

// TestAppendSignatureFieldToFields_InsertsFreshFieldsArrayWhenAbsent
// covers the "/Fields key not present" branch. Asserts the exact
// output rather than a substring: a bare bytes.Contains(got,
// []byte("5 0 R")) check would pass under a mutation that instead took
// the (wrong) existing-array append path if one happened to exist, so
// the discriminator needs to be the full rebuilt structure, not just
// the presence of the field reference.
func TestAppendSignatureFieldToFields_InsertsFreshFieldsArrayWhenAbsent(t *testing.T) {
	got, err := appendSignatureFieldToFields([]byte("<< /SigFlags 1 >>"), 5)
	if err != nil {
		t.Fatalf("appendSignatureFieldToFields: %v", err)
	}
	want := "<<\n/Fields [ 5 0 R ] /SigFlags 1 >>"
	if string(got) != want {
		t.Fatalf("got %q, want %q", string(got), want)
	}
}

// TestAppendSignatureFieldToFields_FindArrayEndErrorPropagates covers
// the "parse AcroForm /Fields: %w" wrap: /Fields is present and is a
// direct array, but it never closes.
func TestAppendSignatureFieldToFields_FindArrayEndErrorPropagates(t *testing.T) {
	_, err := appendSignatureFieldToFields([]byte("<< /Fields [ 1 0 R"), 5)
	if err == nil {
		t.Fatal("expected an error for an unterminated /Fields array")
	}
	if !strings.Contains(err.Error(), "parse AcroForm /Fields:") {
		t.Errorf("expected the wrap prefix, got %q", err)
	}
}

// --- ensureSignatureFieldFlags ---

func TestEnsureSignatureFieldFlags_NonDictReturnsError(t *testing.T) {
	_, err := ensureSignatureFieldFlags([]byte("not a dict"))
	if err == nil {
		t.Fatal("expected an error when the input isn't a direct dictionary")
	}
	if !strings.Contains(err.Error(), "is not a direct dictionary") {
		t.Errorf("expected the guard's own error text, got %q", err)
	}
}

// TestEnsureSignatureFieldFlags_InsertsFreshSigFlagsWhenAbsent covers
// the "/SigFlags key not present" branch. Exact-output assertion, same
// masking rationale as TestAppendSignatureFieldToFields_InsertsFreshFieldsArrayWhenAbsent.
func TestEnsureSignatureFieldFlags_InsertsFreshSigFlagsWhenAbsent(t *testing.T) {
	got, err := ensureSignatureFieldFlags([]byte("<< /Fields [ 5 0 R ] >>"))
	if err != nil {
		t.Fatalf("ensureSignatureFieldFlags: %v", err)
	}
	want := "<<\n/SigFlags 3 /Fields [ 5 0 R ] >>"
	if string(got) != want {
		t.Fatalf("got %q, want %q", string(got), want)
	}
}

func TestEnsureSignatureFieldFlags_NonIntegerReturnsError(t *testing.T) {
	_, err := ensureSignatureFieldFlags([]byte("<< /SigFlags true >>"))
	if err == nil {
		t.Fatal("expected an error when /SigFlags isn't an integer")
	}
	if !strings.Contains(err.Error(), "is not an integer") {
		t.Errorf("expected the guard's own error text, got %q", err)
	}
}

// TestEnsureSignatureFieldFlags_OverflowReturnsError covers the new
// overflow guard on /SigFlags. This is the bug /advisor's
// pre-implementation review flagged for this increment: with the raw
// `flags = flags*10 + int(c-'0')` loop (no bounds check), this exact
// fixture returned (nil error, "<< /SigFlags 7766279631452241919 >>")
// — a silently wrong value written straight into the signed PDF's
// AcroForm dict — confirmed directly before writing the fix. Reachable
// severity is low today (every call site feeds this app's own
// generated PDFs, whose /SigFlags is always 1 or 3), but the guard
// costs three lines and the bug class is identical to the
// shiftClassicXrefOffsets truncation fixed in the prior increment:
// unbounded external value into an accumulator, silent wrong output
// instead of an error.
func TestEnsureSignatureFieldFlags_OverflowReturnsError(t *testing.T) {
	_, err := ensureSignatureFieldFlags([]byte("<< /SigFlags 99999999999999999999 >>"))
	if err == nil {
		t.Fatal("expected an error when /SigFlags overflows int")
	}
	if !strings.Contains(err.Error(), "AcroForm /SigFlags:") {
		t.Errorf("expected the wrap prefix, got %q", err)
	}
}

// --- mergeSignatureFieldIntoAcroForm interaction ---

// TestMergeSignatureFieldIntoAcroForm_InsertsBothWhenBothAbsent covers
// the case where an AcroForm dict has neither /Fields nor /SigFlags —
// the realistic "empty AcroForm" shape, not just the two fresh-insert
// branches exercised in isolation above. This is the only test that
// exercises their sequencing: appendSignatureFieldToFields runs first
// and inserts "/Fields [ N 0 R ]" right after "<<"; ensureSignatureFieldFlags
// then runs on that already-rebuilt body and inserts "/SigFlags 3"
// right after the same "<<" — so /SigFlags ends up before /Fields in
// the final output, not after. Asserting the exact byte sequence (not
// just that both keys are present, e.g. via bytes.Count) is what
// catches a mutation to that ordering.
func TestMergeSignatureFieldIntoAcroForm_InsertsBothWhenBothAbsent(t *testing.T) {
	got, err := mergeSignatureFieldIntoAcroForm([]byte("<< >>"), 9)
	if err != nil {
		t.Fatalf("mergeSignatureFieldIntoAcroForm: %v", err)
	}
	want := "<<\n/SigFlags 3\n/Fields [ 9 0 R ] >>"
	if string(got) != want {
		t.Fatalf("got %q, want %q", string(got), want)
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
// caller-supplied field lands in the PDF's /Info dictionary, including a
// caller-supplied CreatorTool (see TestApplyPDFAMetadata_HonorsCreatorTool
// for the break-verified guard behind Creator specifically).
func TestApplyPDFAMetadata_SetsAllFieldsWhenProvided(t *testing.T) {
	b := renderMinimalPDF(t, func(pdf *fpdf.Fpdf) {
		ApplyPDFAMetadata(pdf, XMPSpec{
			Title:       "My Title",
			Subject:     "My Subject",
			Author:      "Alice",
			CreatorTool: "Custom Tool",
			Keywords:    []string{"one", "two", "three"},
		})
	})
	for key, want := range map[string]string{
		"/Title":    "My Title",
		"/Subject":  "My Subject",
		"/Author":   "Alice",
		"/Creator":  "Custom Tool",
		"/Keywords": "one, two, three",
	} {
		needle := append([]byte(key+" ("), utf16beString(want)...)
		if !bytes.Contains(b, needle) {
			t.Errorf("%s: want value %q adjacent to the key, not found", key, want)
		}
	}
}

// TestApplyPDFAMetadata_HonorsCreatorTool covers spec.CreatorTool flowing
// through to /Info's Creator field. Before this test existed,
// ApplyPDFAMetadata hardcoded Creator to "GoPMgr" and ignored CreatorTool
// entirely, unlike its sibling BuildXMPPacket (same file), which does
// honor CreatorTool for the XMP packet's <xmp:CreatorTool> element — a
// caller setting CreatorTool got inconsistent metadata between the two
// surfaces. Fixed by mirroring BuildXMPPacket's default-then-use pattern.
// This is a consistency alignment, not a behavior change for any current
// caller: internal/documents/fonts.go's only direct call site already
// passes CreatorTool: "GoPMgr", identical to the prior hardcoded value,
// and internal/export's ApplyPDFAMetadata wrapper (which used to override
// Creator with a version-suffixed string after delegating here) was dead
// code — unreachable from anywhere in the module — and has been removed.
func TestApplyPDFAMetadata_HonorsCreatorTool(t *testing.T) {
	b := renderMinimalPDF(t, func(pdf *fpdf.Fpdf) {
		ApplyPDFAMetadata(pdf, XMPSpec{CreatorTool: "Custom Tool"})
	})
	needle := append([]byte("/Creator ("), utf16beString("Custom Tool")...)
	if !bytes.Contains(b, needle) {
		t.Error("expected /Creator to reflect the caller-supplied CreatorTool")
	}
}

// TestApplyPDFAMetadata_DefaultsCreatorToolWhenEmpty covers the
// "spec.CreatorTool == ”" default branch, mirroring
// TestApplyPDFAMetadata_DefaultsAuthorWhenEmpty's masking risk: a bare
// bytes.Contains(b, utf16beString("GoPMgr")) would pass even if this
// guard were deleted, since spec.Author also defaults to "GoPMgr" a few
// lines earlier and would still contribute that substring to the output.
// The key-adjacent needle ("/Creator (" + the encoded bytes) isolates
// Creator's own value.
func TestApplyPDFAMetadata_DefaultsCreatorToolWhenEmpty(t *testing.T) {
	b := renderMinimalPDF(t, func(pdf *fpdf.Fpdf) {
		ApplyPDFAMetadata(pdf, XMPSpec{})
	})
	needle := append([]byte("/Creator ("), utf16beString("GoPMgr")...)
	if !bytes.Contains(b, needle) {
		t.Error("expected /Creator to default to \"GoPMgr\" when CreatorTool is unset")
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
