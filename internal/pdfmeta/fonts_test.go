// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package pdfmeta

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/go-pdf/fpdf"

	"gopmgr/internal/fonts"
)

// renderPlexPDF renders text in IBM Plex Sans Regular and SemiBold through
// the same registration path the renderers use, returning fpdf's raw output.
func renderPlexPDF(t *testing.T, text string) []byte {
	t.Helper()
	pdf := fpdf.New("P", "mm", "A4", "")
	if err := fonts.NewManager("").RegisterAs(pdf, fonts.DefaultFamily, "Helvetica"); err != nil {
		t.Fatalf("RegisterAs: %v", err)
	}
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 11)
	pdf.Cell(0, 8, text)
	pdf.Ln(10)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.Cell(0, 8, text)
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		t.Fatalf("Output: %v", err)
	}
	return buf.Bytes()
}

type fpdfFontRefs struct {
	type0, cid, toUnicode int
}

// fpdfFonts lists the UTF-8 fonts fpdf wrote into the original revision.
func fpdfFonts(t *testing.T, pdf []byte) []fpdfFontRefs {
	t.Helper()
	var refs []fpdfFontRefs
	for _, m := range fpdfType0Font.FindAllSubmatch(pdf, -1) {
		type0, _ := strconv.Atoi(string(m[1]))
		cid, _ := strconv.Atoi(string(m[3]))
		toUnicode, _ := strconv.Atoi(string(m[4]))
		refs = append(refs, fpdfFontRefs{type0, cid, toUnicode})
	}
	if len(refs) == 0 {
		t.Fatal("rendered PDF has no fpdf UTF-8 fonts")
	}
	return refs
}

func inflate(t *testing.T, data []byte) []byte {
	t.Helper()
	zr, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("zlib: %v", err)
	}
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("inflate: %v", err)
	}
	return out
}

var bfrangeEntry = regexp.MustCompile(`<([0-9A-F]{4})> <([0-9A-F]{4})> <([0-9A-F]{4})>`)

// cmapRanges parses the bfrange entries of a CMap and fails the test if any
// entry breaks ISO 32000-1 9.10.3 or is not an identity mapping.
func cmapRanges(t *testing.T, cmap []byte) [][2]int {
	t.Helper()
	var ranges [][2]int
	for _, m := range bfrangeEntry.FindAllSubmatch(cmap, -1) {
		lo, _ := strconv.ParseUint(string(m[1]), 16, 16)
		hi, _ := strconv.ParseUint(string(m[2]), 16, 16)
		dst, _ := strconv.ParseUint(string(m[3]), 16, 16)
		if lo>>8 != hi>>8 {
			t.Fatalf("bfrange <%04X> <%04X> varies more than the last byte", lo, hi)
		}
		if lo > hi || dst != lo {
			t.Fatalf("bfrange <%04X> <%04X> <%04X> is not an identity range", lo, hi, dst)
		}
		ranges = append(ranges, [2]int{int(lo), int(hi)})
	}
	return ranges
}

func cmapMaps(ranges [][2]int, code int) bool {
	for _, r := range ranges {
		if code >= r[0] && code <= r[1] {
			return true
		}
	}
	return false
}

func TestRepairFPDFFonts_ReplacesEveryInvalidToUnicodeCMap(t *testing.T) {
	raw := renderPlexPDF(t, "“Quoted” — • € Ω Привет Tiếng")
	if !bytes.Contains(raw, []byte(fpdfToUnicodeCMap)) {
		t.Fatal("fpdf no longer writes the ToUnicode CMap this repair targets; review RepairFPDFFonts against the new fpdf output")
	}
	repaired, err := RepairFPDFFonts(raw)
	if err != nil {
		t.Fatalf("RepairFPDFFonts: %v", err)
	}
	for _, font := range fpdfFonts(t, raw) {
		dict, data, err := readStreamObject(repaired, font.toUnicode)
		if err != nil {
			t.Fatalf("ToUnicode %d: %v", font.toUnicode, err)
		}
		if !bytes.Contains(dict, []byte("/FlateDecode")) {
			t.Fatalf("ToUnicode %d was not replaced: %s", font.toUnicode, dict)
		}
		ranges := cmapRanges(t, inflate(t, data))
		for _, code := range []int{'A', 0x00E9, 0x2014, 0x2022, 0x20AC, 0x03A9, 0x041F, 0x1EBF, 0xFFFD} {
			if !cmapMaps(ranges, code) {
				t.Errorf("ToUnicode %d does not map U+%04X", font.toUnicode, code)
			}
		}
		for _, code := range []int{0x0000, 0xD800, 0xDBFF, 0xDC00, 0xDFFF, 0xFEFF, 0xFFFE, 0xFFFF} {
			if cmapMaps(ranges, code) {
				t.Errorf("ToUnicode %d maps U+%04X, which PDF/A-2 forbids", font.toUnicode, code)
			}
		}
	}
}

func TestRepairFPDFFonts_NamesFontsAfterTheirPostScriptNames(t *testing.T) {
	raw := renderPlexPDF(t, "Schedule")
	repaired, err := RepairFPDFFonts(raw)
	if err != nil {
		t.Fatalf("RepairFPDFFonts: %v", err)
	}
	subsetName := regexp.MustCompile(`^([A-Z]{6})\+(IBMPlexSans[-A-Za-z]*)$`)
	tags := map[string]string{}
	seen := map[string]bool{}
	for _, font := range fpdfFonts(t, raw) {
		type0, err := findObjectBody(repaired, font.type0, 0)
		if err != nil {
			t.Fatal(err)
		}
		cid, err := findObjectBody(repaired, font.cid, 0)
		if err != nil {
			t.Fatal(err)
		}
		descID, _, err := readDictRef(cid, "/FontDescriptor")
		if err != nil {
			t.Fatal(err)
		}
		desc, err := findObjectBody(repaired, descID, 0)
		if err != nil {
			t.Fatal(err)
		}
		name := pdfNameAfter(t, type0, "/BaseFont /")
		if got := pdfNameAfter(t, cid, "/BaseFont /"); got != name {
			t.Errorf("CIDFont BaseFont %q differs from Type0 BaseFont %q", got, name)
		}
		if got := pdfNameAfter(t, desc, "/FontName /"); got != name {
			t.Errorf("FontDescriptor FontName %q differs from BaseFont %q", got, name)
		}
		m := subsetName.FindStringSubmatch(name)
		if m == nil {
			t.Fatalf("font %d is named %q, want <TAG>+IBMPlexSans...", font.type0, name)
		}
		if other, ok := tags[m[1]]; ok && other != name {
			t.Errorf("subset tag %s is shared by %q and %q", m[1], other, name)
		}
		tags[m[1]] = name
		seen[m[2]] = true
	}
	for _, want := range []string{"IBMPlexSans", "IBMPlexSans-SmBld"} {
		if !seen[want] {
			t.Errorf("no embedded font is named after %s; saw %v", want, seen)
		}
	}
}

func pdfNameAfter(t *testing.T, body []byte, key string) string {
	t.Helper()
	idx := bytes.Index(body, []byte(key))
	if idx < 0 {
		t.Fatalf("%q missing from %q", key, body)
	}
	rest := body[idx+len(key):]
	end := 0
	for end < len(rest) && !isPDFNameTerminator(rest[end]) {
		end++
	}
	return string(rest[:end])
}

func TestRepairFPDFFonts_IsIdempotent(t *testing.T) {
	once, err := RepairFPDFFonts(renderPlexPDF(t, "Budget • 1,250 €"))
	if err != nil {
		t.Fatalf("first repair: %v", err)
	}
	twice, err := RepairFPDFFonts(once)
	if err != nil {
		t.Fatalf("second repair: %v", err)
	}
	if !bytes.Equal(once, twice) {
		t.Fatalf("a second repair appended another revision (%d -> %d bytes)", len(once), len(twice))
	}
}

func TestRepairFPDFFonts_IsDeterministic(t *testing.T) {
	raw := renderPlexPDF(t, "Milestone")
	a, err := RepairFPDFFonts(raw)
	if err != nil {
		t.Fatal(err)
	}
	b, err := RepairFPDFFonts(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("repairing the same PDF twice produced different bytes")
	}
}

func TestRepairFPDFFonts_LeavesPDFsWithoutFPDFFontsUnchanged(t *testing.T) {
	in := minimalPDF()
	out, err := RepairFPDFFonts(in)
	if err != nil {
		t.Fatalf("RepairFPDFFonts: %v", err)
	}
	if !bytes.Equal(in, out) {
		t.Fatal("a PDF without fpdf fonts was modified")
	}
}

func TestRepairFPDFFonts_RejectsEmptyInput(t *testing.T) {
	if _, err := RepairFPDFFonts(nil); err == nil || !strings.Contains(err.Error(), "empty PDF input") {
		t.Fatalf("err = %v, want empty-input error", err)
	}
}

// TestRepairFPDFFonts_KeepsNameWhenFontProgramIsUnreadable corrupts every
// embedded font program. The CMaps must still be repaired, and the fonts
// must keep fpdf's names rather than gain a half-applied rename.
func TestRepairFPDFFonts_KeepsNameWhenFontProgramIsUnreadable(t *testing.T) {
	raw := renderPlexPDF(t, "Risk")
	for _, font := range fpdfFonts(t, raw) {
		cid, err := findObjectBody(raw, font.cid, 0)
		if err != nil {
			t.Fatal(err)
		}
		descID, _, _ := readDictRef(cid, "/FontDescriptor")
		desc, _ := findObjectBody(raw, descID, 0)
		fileID, _, err := readDictRef(desc, "/FontFile2")
		if err != nil {
			t.Fatal(err)
		}
		_, data, err := readStreamObject(raw, fileID)
		if err != nil {
			t.Fatal(err)
		}
		for i := range data {
			data[i] = 'x' // data aliases raw, so this corrupts the PDF in place
		}
	}

	repaired, err := RepairFPDFFonts(raw)
	if err != nil {
		t.Fatalf("RepairFPDFFonts: %v", err)
	}
	for _, font := range fpdfFonts(t, raw) {
		dict, _, err := readStreamObject(repaired, font.toUnicode)
		if err != nil || !bytes.Contains(dict, []byte("/FlateDecode")) {
			t.Fatalf("ToUnicode %d not repaired (err %v)", font.toUnicode, err)
		}
		type0, _ := findObjectBody(repaired, font.type0, 0)
		if !bytes.Contains(type0, []byte("/BaseFont /utf8")) {
			t.Fatalf("font %d was renamed although its program is unreadable: %q", font.type0, type0)
		}
	}
}

func TestIdentityCMapObjectBody_BlocksHoldAtMost100Ranges(t *testing.T) {
	body := identityCMapObjectBody()
	i := bytes.Index(body, []byte("stream\n"))
	end := bytes.LastIndex(body, []byte("\nendstream"))
	if i < 0 || end < 0 {
		t.Fatalf("malformed CMap object: %.60q", body)
	}
	length, err := readDirectLength(body)
	if err != nil {
		t.Fatal(err)
	}
	data := body[i+len("stream\n") : end]
	if length != len(data) {
		t.Fatalf("/Length %d, stream holds %d bytes", length, len(data))
	}
	cmap := inflate(t, data)
	total := 0
	for _, m := range regexp.MustCompile(`(\d+) beginbfrange`).FindAllSubmatch(cmap, -1) {
		n, _ := strconv.Atoi(string(m[1]))
		if n > 100 {
			t.Errorf("bfrange block has %d entries; a CMap block may hold at most 100", n)
		}
		total += n
	}
	if got := len(cmapRanges(t, cmap)); got != total {
		t.Fatalf("block counts add up to %d, CMap has %d ranges", total, got)
	}
	if total != 248 {
		t.Fatalf("CMap has %d ranges, want 248 (256 high bytes minus 8 surrogate bytes)", total)
	}
}

func TestTrueTypePostScriptName_ReadsBundledFont(t *testing.T) {
	program, err := os.ReadFile("../fonts/assets/IBMPlexMono-SemiBold.ttf")
	if err != nil {
		t.Fatal(err)
	}
	name, ok := trueTypePostScriptName(program)
	if !ok || name != "IBMPlexMono-SmBld" {
		t.Fatalf("trueTypePostScriptName = %q, %v; want IBMPlexMono-SmBld", name, ok)
	}
}

type nameRecord struct {
	platform, encoding, nameID uint16
	value                      []byte
}

// fontWithNameTable builds the smallest TrueType byte layout
// trueTypePostScriptName reads: a table directory holding one name table.
func fontWithNameTable(records ...nameRecord) []byte {
	var storage bytes.Buffer
	var table bytes.Buffer
	be := func(w *bytes.Buffer, v uint16) { _ = binary.Write(w, binary.BigEndian, v) }
	be(&table, 0)
	be(&table, uint16(len(records)))
	be(&table, uint16(6+12*len(records)))
	for _, r := range records {
		be(&table, r.platform)
		be(&table, r.encoding)
		be(&table, 0x409)
		be(&table, r.nameID)
		be(&table, uint16(len(r.value)))
		be(&table, uint16(storage.Len()))
		storage.Write(r.value)
	}
	table.Write(storage.Bytes())

	var font bytes.Buffer
	_ = binary.Write(&font, binary.BigEndian, uint32(0x00010000))
	be(&font, 1)
	be(&font, 0)
	be(&font, 0)
	be(&font, 0)
	font.WriteString("name")
	_ = binary.Write(&font, binary.BigEndian, uint32(0))
	_ = binary.Write(&font, binary.BigEndian, uint32(12+16))
	_ = binary.Write(&font, binary.BigEndian, uint32(table.Len()))
	font.Write(table.Bytes())
	return font.Bytes()
}

func utf16be(s string) []byte {
	var b bytes.Buffer
	for _, u := range utf16.Encode([]rune(s)) {
		_ = binary.Write(&b, binary.BigEndian, u)
	}
	return b.Bytes()
}

func TestTrueTypePostScriptName_Cases(t *testing.T) {
	for _, tc := range []struct {
		name    string
		program []byte
		want    string
		ok      bool
	}{
		{"windows record", fontWithNameTable(nameRecord{3, 1, 6, utf16be("Example-Bold")}), "Example-Bold", true},
		{"windows preferred over mac", fontWithNameTable(nameRecord{1, 0, 6, []byte("MacName")}, nameRecord{3, 1, 6, utf16be("WinName")}), "WinName", true},
		{"mac record only", fontWithNameTable(nameRecord{1, 0, 6, []byte("MacOnly")}), "MacOnly", true},
		{"ignores other name IDs", fontWithNameTable(nameRecord{3, 1, 4, utf16be("Full Name")}), "", false},
		{"rejects a space", fontWithNameTable(nameRecord{3, 1, 6, utf16be("Bad Name")}), "", false},
		{"rejects a delimiter", fontWithNameTable(nameRecord{3, 1, 6, utf16be("Bad/Name")}), "", false},
		{"rejects non-ASCII", fontWithNameTable(nameRecord{3, 1, 6, utf16be("Café")}), "", false},
		{"rejects over 63 characters", fontWithNameTable(nameRecord{3, 1, 6, utf16be(strings.Repeat("A", 64))}), "", false},
		{"too short", []byte{0, 1, 0, 0}, "", false},
		{"no name table", fontWithNameTable()[:12], "", false},
	} {
		got, ok := trueTypePostScriptName(tc.program)
		if got != tc.want || ok != tc.ok {
			t.Errorf("%s: got %q, %v; want %q, %v", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}

func TestTrueTypePostScriptName_RejectsTablesPastTheEnd(t *testing.T) {
	program := fontWithNameTable(nameRecord{3, 1, 6, utf16be("Example")})
	binary.BigEndian.PutUint32(program[12+12:], uint32(len(program))) // table length past EOF
	if name, ok := trueTypePostScriptName(program); ok {
		t.Fatalf("read %q from a name table that runs past the end of the font", name)
	}
}

func TestSubsetTag_IsDeterministicAndUniquePerDocument(t *testing.T) {
	program := []byte("font program bytes")
	first := subsetTag(program, map[string]bool{})
	if !regexp.MustCompile(`^[A-Z]{6}$`).MatchString(first) {
		t.Fatalf("tag %q is not six uppercase letters", first)
	}
	if again := subsetTag(program, map[string]bool{}); again != first {
		t.Fatalf("same program got tags %q and %q", first, again)
	}
	used := map[string]bool{first: true}
	if next := subsetTag(program, used); next == first || !used[next] {
		t.Fatalf("a used tag was reused or not recorded: %q (used %v)", next, used)
	}
}

func TestReadDirectLength_SkipsLength1(t *testing.T) {
	for _, tc := range []struct {
		dict string
		want int
		ok   bool
	}{
		{"<</Length 7\n/Filter /FlateDecode\n/Length1 99\n>>", 7, true},
		{"<</Length1 99 /Length 5>>", 5, true},
		{"<</Length1 99>>", 0, false},
	} {
		got, err := readDirectLength([]byte(tc.dict))
		if (err == nil) != tc.ok || got != tc.want {
			t.Errorf("readDirectLength(%q) = %d, %v; want %d, ok=%v", tc.dict, got, err, tc.want, tc.ok)
		}
	}
}

func TestReadStreamObject_Errors(t *testing.T) {
	for _, tc := range []struct {
		name, obj, want string
	}{
		{"not a stream", "<< /Type /Catalog /Length 3 >>\nendobj", "is not a stream"},
		{"no end-of-line after stream", "<< /Length 3 >>\nstreamabc\nendstream", "end-of-line"},
		{"length past end of file", "<< /Length 999 >>\nstream\nabc\nendstream", "runs past the end"},
		{"no length", "<< /Filter /FlateDecode >>\nstream\nabc\nendstream", "/Length not present"},
		{"not a dictionary", "[1 2 3]", "does not start with <<"},
	} {
		pdf := []byte("%PDF-1.4\n5 0 obj\n" + tc.obj + "\n")
		if _, _, err := readStreamObject(pdf, 5); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want %q", tc.name, err, tc.want)
		}
	}
	if _, _, err := readStreamObject([]byte("%PDF-1.4\n"), 5); err == nil {
		t.Error("missing object: expected an error")
	}
}

func TestReadStreamObject_ReadsBinaryDataByLength(t *testing.T) {
	data := "endobj inside\r\nbinary"
	pdf := []byte("%PDF-1.4\n5 0 obj\n<< /Length " + strconv.Itoa(len(data)) + " >>\r\nstream\r\n" + data + "\nendstream\nendobj\n")
	_, got, err := readStreamObject(pdf, 5)
	if err != nil || string(got) != data {
		t.Fatalf("readStreamObject = %q, %v; want %q", got, err, data)
	}
}

func TestReplacePDFName_RequiresWholeName(t *testing.T) {
	if _, ok := replacePDFName([]byte("/BaseFont /utf8fB\n"), "/BaseFont /utf8f", "/BaseFont /X"); ok {
		t.Fatal("replaced a name that only shares a prefix")
	}
	got, ok := replacePDFName([]byte("<</BaseFont /utf8f/Encoding"), "/BaseFont /utf8f", "/BaseFont /ABCDEF+X")
	if !ok || string(got) != "<</BaseFont /ABCDEF+X/Encoding" {
		t.Fatalf("replacePDFName = %q, %v", got, ok)
	}
}

func TestMakePDFA_RepairsFontsAndDeclaresPDFA2U(t *testing.T) {
	raw := renderPlexPDF(t, "Charter — “Approved”")
	out, err := MakePDFA(raw, XMPSpec{Title: "Charter"}, DefaultICCProfile())
	if err != nil {
		t.Fatalf("MakePDFA: %v", err)
	}
	if !bytes.Contains(out, []byte("<pdfaid:part>2</pdfaid:part>")) || !bytes.Contains(out, []byte("<pdfaid:conformance>U</pdfaid:conformance>")) {
		t.Fatal("output does not declare PDF/A-2u")
	}
	for _, font := range fpdfFonts(t, out) {
		dict, _, err := readStreamObject(out, font.toUnicode)
		if err != nil || !bytes.Contains(dict, []byte("/FlateDecode")) {
			t.Fatalf("ToUnicode %d not repaired in MakePDFA output (err %v)", font.toUnicode, err)
		}
	}
	xrefOff, err := findLastStartxref(out)
	if err != nil {
		t.Fatal(err)
	}
	_, root, gen, err := parseTrailerSizeAndRoot(out, xrefOff)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := findObjectBody(out, root, gen)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"/Metadata ", "/OutputIntents "} {
		if !bytes.Contains(catalog, []byte(key)) {
			t.Errorf("latest Catalog missing %s", key)
		}
	}
}

func TestTagPDFA_ReturnsInputUnchangedOnFailure(t *testing.T) {
	in := []byte("not a pdf")
	out, err := TagPDFA(in, XMPSpec{})
	if err == nil {
		t.Fatal("expected an error for input that is not a PDF")
	}
	if !bytes.Equal(out, in) {
		t.Fatalf("TagPDFA returned %q, want the input unchanged", out)
	}
}

func TestTagPDFA_UsesEmbeddedProfile(t *testing.T) {
	out, err := TagPDFA(minimalPDF(), XMPSpec{Title: "Report"})
	if err != nil {
		t.Fatalf("TagPDFA: %v", err)
	}
	if !bytes.Contains(out, DefaultICCProfile()) {
		t.Fatal("TagPDFA output does not embed the sRGB profile")
	}
}

func TestInjectPAdESSignature_KeepsTrailerID(t *testing.T) {
	in, err := MakePDFA(minimalPDF(), XMPSpec{Title: "Signed"}, []byte("icc"))
	if err != nil {
		t.Fatal(err)
	}
	inXref, err := findLastStartxref(in)
	if err != nil {
		t.Fatal(err)
	}
	wantID, ok := readTrailerIDValue(in, inXref)
	if !ok {
		t.Fatal("fixture has no trailer /ID")
	}

	out, err := InjectPAdESSignature(in, func([]byte) ([]byte, error) { return []byte{0x30, 0x00}, nil })
	if err != nil {
		t.Fatalf("InjectPAdESSignature: %v", err)
	}
	outXref, err := findLastStartxref(out)
	if err != nil {
		t.Fatal(err)
	}
	gotID, ok := readTrailerIDValue(out, outXref)
	if !ok {
		t.Fatal("signed PDF's last trailer has no /ID, which PDF/A requires")
	}
	if gotID != wantID {
		t.Fatalf("signed trailer /ID = %s, want the signed file's %s", gotID, wantID)
	}
}

// synthFont describes one font laid out exactly as fpdf writes it, with
// each link in the lookup chain breakable on its own.
type synthFont struct {
	type0ID         string // object number text; fpdf writes small integers
	descriptorRef   bool   // CIDFont names its /FontDescriptor
	descriptor      bool   // the descriptor object exists
	fontFileRef     bool   // descriptor names its /FontFile2
	fontFile        bool   // the font file object exists
	filter          string // font file /Filter entry, "" for none
	program         []byte // font file stream data
	endsWithNewline bool
	omitStartxref   bool
}

func defaultSynthFont() synthFont {
	return synthFont{
		type0ID: "1", descriptorRef: true, descriptor: true, fontFileRef: true, fontFile: true,
		program:         fontWithNameTable(nameRecord{3, 1, 6, utf16be("Synthetic-Regular")}),
		endsWithNewline: true,
	}
}

func (f synthFont) build() []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	offsets := map[int]int{}
	obj := func(id int, body string) {
		offsets[id] = b.Len()
		b.WriteString(strconv.Itoa(id) + " 0 obj\n" + body + "\nendobj\n")
	}
	b.WriteString(f.type0ID + " 0 obj\n<</Type /Font\n/Subtype /Type0\n/BaseFont /utf8synth\n/Encoding /Identity-H\n/DescendantFonts [2 0 R]\n/ToUnicode 3 0 R>>\nendobj\n")
	cid := "<</Type /Font\n/Subtype /CIDFontType2\n/BaseFont /utf8synth\n"
	if f.descriptorRef {
		cid += "/FontDescriptor 4 0 R"
	}
	obj(2, cid+">>")
	obj(3, "<</Length "+strconv.Itoa(len(fpdfToUnicodeCMap))+">>\nstream\n"+fpdfToUnicodeCMap+"\nendstream")
	if f.descriptor {
		desc := "<</Type /FontDescriptor /FontName /utf8synth\n /Ascent 1000"
		if f.fontFileRef {
			desc += "/FontFile2 5 0 R"
		}
		obj(4, desc+">>")
	}
	if f.fontFile {
		dict := "<</Length " + strconv.Itoa(len(f.program))
		if f.filter != "" {
			dict += "\n/Filter " + f.filter
		}
		obj(5, dict+">>\nstream\n"+string(f.program)+"\nendstream")
	}
	obj(6, "<< /Type /Catalog >>")
	if f.omitStartxref {
		return b.Bytes()
	}
	xref := b.Len()
	b.WriteString("xref\n0 1\n0000000000 65535 f \n")
	for id := 2; id <= 6; id++ {
		if off, ok := offsets[id]; ok {
			fmt.Fprintf(&b, "%d 1\n%010d 00000 n \n", id, off)
		}
	}
	fmt.Fprintf(&b, "trailer\n<< /Size 7 /Root 6 0 R >>\nstartxref\n%d\n%%%%EOF", xref)
	if f.endsWithNewline {
		b.WriteByte('\n')
	}
	return b.Bytes()
}

func baseFontOf(t *testing.T, pdf []byte, id int) string {
	t.Helper()
	body, err := findObjectBody(pdf, id, 0)
	if err != nil {
		t.Fatal(err)
	}
	return pdfNameAfter(t, body, "/BaseFont /")
}

// TestRepairFPDFFonts_RenamesOnlyWhenTheWholeChainResolves breaks each link
// from the CIDFont to the embedded program in turn. The CMap is always
// repaired; the font is renamed only when its PostScript name is readable.
func TestRepairFPDFFonts_RenamesOnlyWhenTheWholeChainResolves(t *testing.T) {
	compressed := func(data []byte) []byte {
		var b bytes.Buffer
		zw := zlib.NewWriter(&b)
		_, _ = zw.Write(data)
		_ = zw.Close()
		return b.Bytes()
	}
	for _, tc := range []struct {
		name    string
		modify  func(*synthFont)
		renamed bool
	}{
		{"uncompressed program", func(*synthFont) {}, true},
		{"FlateDecode program", func(f *synthFont) {
			f.filter, f.program = "/FlateDecode", compressed(f.program)
		}, true},
		{"no /FontDescriptor", func(f *synthFont) { f.descriptorRef = false }, false},
		{"missing descriptor object", func(f *synthFont) { f.descriptor = false }, false},
		{"no /FontFile2", func(f *synthFont) { f.fontFileRef = false }, false},
		{"missing font file object", func(f *synthFont) { f.fontFile = false }, false},
		{"unsupported filter", func(f *synthFont) { f.filter = "/LZWDecode" }, false},
		{"corrupt FlateDecode data", func(f *synthFont) { f.filter = "/FlateDecode" }, false},
		{"program without a PostScript name", func(f *synthFont) { f.program = fontWithNameTable() }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := defaultSynthFont()
			tc.modify(&f)
			out, err := RepairFPDFFonts(f.build())
			if err != nil {
				t.Fatalf("RepairFPDFFonts: %v", err)
			}
			if dict, _, err := readStreamObject(out, 3); err != nil || !bytes.Contains(dict, []byte("/FlateDecode")) {
				t.Fatalf("CMap not repaired (err %v)", err)
			}
			name := baseFontOf(t, out, 1)
			if tc.renamed && !regexp.MustCompile(`^[A-Z]{6}\+Synthetic-Regular$`).MatchString(name) {
				t.Fatalf("BaseFont = %q, want <TAG>+Synthetic-Regular", name)
			}
			if !tc.renamed && name != "utf8synth" {
				t.Fatalf("BaseFont = %q, want fpdf's name kept", name)
			}
		})
	}
}

func TestRepairFPDFFonts_RejectsOversizedFontPrograms(t *testing.T) {
	prev := maxEmbeddedFontBytes
	maxEmbeddedFontBytes = 16
	t.Cleanup(func() { maxEmbeddedFontBytes = prev })
	f := defaultSynthFont()
	var b bytes.Buffer
	zw := zlib.NewWriter(&b)
	_, _ = zw.Write(f.program)
	_ = zw.Close()
	f.filter, f.program = "/FlateDecode", b.Bytes()
	if _, err := readFontProgram(f.build(), 5); err == nil || !strings.Contains(err.Error(), "larger than 16 bytes") {
		t.Fatalf("readFontProgram err = %v, want the size cap", err)
	}
}

func TestRepairFPDFFonts_SkipsObjectNumbersThatOverflow(t *testing.T) {
	f := defaultSynthFont()
	f.type0ID = strings.Repeat("9", 30)
	in := f.build()
	out, err := RepairFPDFFonts(in)
	if err != nil {
		t.Fatalf("RepairFPDFFonts: %v", err)
	}
	if !bytes.Equal(in, out) {
		t.Fatal("a font whose object number overflows int was repaired")
	}
}

func TestRepairFPDFFonts_AddsNewlineBeforeAppendedRevision(t *testing.T) {
	f := defaultSynthFont()
	f.endsWithNewline = false
	out, err := RepairFPDFFonts(f.build())
	if err != nil {
		t.Fatalf("RepairFPDFFonts: %v", err)
	}
	if !bytes.Contains(out, []byte("%%EOF\n3 0 obj")) && !bytes.Contains(out, []byte("%%EOF\n1 0 obj")) {
		t.Fatalf("appended revision does not start on a new line: %q", out[bytes.LastIndex(out, []byte("%%EOF")):][:24])
	}
}

func TestAppendObjectRevisions_PropagatesTrailerErrors(t *testing.T) {
	f := defaultSynthFont()
	f.omitStartxref = true
	if _, err := RepairFPDFFonts(f.build()); err == nil || !strings.Contains(err.Error(), "locate startxref") {
		t.Fatalf("missing startxref: err = %v", err)
	}
	if _, err := appendObjectRevisions([]byte("%PDF-1.4\nxref\n0 1\n0000000000 65535 f \nstartxref\n9\n%%EOF\n"), map[int][]byte{1: []byte("<<>>")}); err == nil || !strings.Contains(err.Error(), "parse trailer") {
		t.Fatalf("unparseable trailer: err = %v", err)
	}
}

func TestMakePDFA_PropagatesFontRepairError(t *testing.T) {
	f := defaultSynthFont()
	f.omitStartxref = true
	if _, err := MakePDFA(f.build(), XMPSpec{}, []byte("icc")); err == nil || !strings.Contains(err.Error(), "pdfmeta: font repair:") {
		t.Fatalf("MakePDFA err = %v, want the font-repair wrap", err)
	}
}

func TestTrueTypePostScriptName_TruncatedTables(t *testing.T) {
	noName := fontWithNameTable(nameRecord{3, 1, 6, utf16be("X")})
	copy(noName[12:16], "cmap") // the only table is no longer "name"
	short := fontWithNameTable()
	binary.BigEndian.PutUint32(short[12+12:], 4) // name table shorter than its header
	truncated := fontWithNameTable(nameRecord{3, 1, 6, utf16be("X")})
	binary.BigEndian.PutUint16(truncated[12+16+2:], 5) // count promises more records than exist
	for name, program := range map[string][]byte{"no name table": noName, "short name table": short, "truncated records": truncated} {
		if got, ok := trueTypePostScriptName(program); name == "truncated records" {
			if !ok || got != "X" {
				t.Errorf("%s: got %q, %v; want the one complete record", name, got, ok)
			}
		} else if ok {
			t.Errorf("%s: read %q", name, got)
		}
	}
}
