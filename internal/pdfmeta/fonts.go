// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package pdfmeta

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf16"
)

// fpdfToUnicodeCMap is the ToUnicode CMap go-pdf/fpdf writes, unchanged, for
// every UTF-8 TrueType font (its toUnicode constant). Its single bfrange,
// <0000> <FFFF> <0000>, breaks ISO 32000-1 9.10.3, which lets a bfrange vary
// only the last byte of its codes. Strict readers, veraPDF among them,
// therefore map no code above 0x00FF to Unicode, so curly quotes, dashes,
// bullets, the euro sign, and Greek or Cyrillic text fail PDF/A-2u and lose
// their meaning for search, copy, and screen readers.
const fpdfToUnicodeCMap = "/CIDInit /ProcSet findresource begin\n12 dict begin\nbegincmap\n/CIDSystemInfo\n<</Registry (Adobe)\n/Ordering (UCS)\n/Supplement 0\n>> def\n/CMapName /Adobe-Identity-UCS def\n/CMapType 2 def\n1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\n1 beginbfrange\n<0000> <FFFF> <0000>\nendbfrange\nendcmap\nCMapName currentdict /CMap defineresource pop\nend\nend"

// fpdfType0Font matches the Type0 font dictionary go-pdf/fpdf writes for each
// UTF-8 font, capturing its object ID, its fpdf name ("utf8" plus the family
// key and style), its descendant CIDFont, and its ToUnicode stream.
var fpdfType0Font = regexp.MustCompile(`(?m)^(\d+) 0 obj\n<</Type /Font\n/Subtype /Type0\n/BaseFont /(utf8[^\s/<>\[\]()%]+)\n/Encoding /Identity-H\n/DescendantFonts \[(\d+) 0 R\]\n/ToUnicode (\d+) 0 R>>`)

// maxEmbeddedFontBytes bounds how much of an embedded font program
// RepairFPDFFonts decompresses to read its name. GoPMgr's own fonts are a few
// hundred kilobytes; the cap only guards against a malformed stream. It is a
// variable so tests can shrink it.
var maxEmbeddedFontBytes int64 = 64 << 20

// RepairFPDFFonts fixes two defects in the UTF-8 TrueType fonts go-pdf/fpdf
// embeds, through one incremental update:
//
//   - It replaces each font's ToUnicode CMap with one that follows the PDF
//     specification, so every character maps back to its Unicode value.
//   - It renames each font from fpdf's internal key (for example
//     "utf8helvetica") to the PostScript name inside the embedded font
//     program, prefixed with the six-letter subset tag ISO 32000-1 9.6.4
//     requires (for example "QXRMTA+IBMPlexSans").
//
// A PDF without fpdf UTF-8 fonts, or one already repaired, is returned
// unchanged. A font whose PostScript name cannot be read keeps its fpdf
// name; its CMap is still replaced.
func RepairFPDFFonts(pdfBytes []byte) ([]byte, error) {
	if len(pdfBytes) == 0 {
		return nil, fmt.Errorf("pdfmeta: empty PDF input")
	}
	matches := fpdfType0Font.FindAllSubmatch(pdfBytes, -1)
	if len(matches) == 0 {
		return pdfBytes, nil
	}

	cmapBody := identityCMapObjectBody()

	replacements := make(map[int][]byte)
	usedTags := make(map[string]bool)
	for _, m := range matches {
		type0ID, err1 := strconv.Atoi(string(m[1]))
		cidID, err2 := strconv.Atoi(string(m[3]))
		toUnicodeID, err3 := strconv.Atoi(string(m[4]))
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		if dict, data, err := readStreamObject(pdfBytes, toUnicodeID); err == nil &&
			!bytes.Contains(dict, []byte("/Filter")) && string(data) == fpdfToUnicodeCMap {
			replacements[toUnicodeID] = cmapBody
		}
		for id, body := range renamedFontObjects(pdfBytes, type0ID, cidID, string(m[2]), usedTags) {
			replacements[id] = body
		}
	}
	if len(replacements) == 0 {
		return pdfBytes, nil
	}
	return appendObjectRevisions(pdfBytes, replacements)
}

// renamedFontObjects returns new revisions of one fpdf UTF-8 font's Type0
// dictionary, CIDFont dictionary, and font descriptor, each naming the font
// "<TAG>+<PostScript name>". It returns nil when the font is already renamed
// or its PostScript name cannot be read, leaving the fpdf name in place.
func renamedFontObjects(pdfBytes []byte, type0ID, cidID int, oldName string, usedTags map[string]bool) map[int][]byte {
	type0, err := findObjectBody(pdfBytes, type0ID, 0)
	if err != nil {
		return nil
	}
	cid, err := findObjectBody(pdfBytes, cidID, 0)
	if err != nil {
		return nil
	}
	descID, _, err := readDictRef(cid, "/FontDescriptor")
	if err != nil {
		return nil
	}
	desc, err := findObjectBody(pdfBytes, descID, 0)
	if err != nil {
		return nil
	}
	fileID, _, err := readDictRef(desc, "/FontFile2")
	if err != nil {
		return nil
	}
	program, err := readFontProgram(pdfBytes, fileID)
	if err != nil {
		return nil
	}
	psName, ok := trueTypePostScriptName(program)
	if !ok {
		return nil
	}
	newName := subsetTag(program, usedTags) + "+" + psName

	revised := make(map[int][]byte, 3)
	for _, obj := range []struct {
		id   int
		body []byte
		key  string
	}{
		{type0ID, type0, "/BaseFont /"},
		{cidID, cid, "/BaseFont /"},
		{descID, desc, "/FontName /"},
	} {
		body, ok := replacePDFName(obj.body, obj.key+oldName, obj.key+newName)
		if !ok {
			return nil
		}
		revised[obj.id] = body
	}
	return revised
}

// replacePDFName replaces the one occurrence of the name token old in body,
// requiring a PDF delimiter or whitespace after it so "/BaseFont /utf8f" does
// not match inside "/BaseFont /utf8fB".
func replacePDFName(body []byte, old, replacement string) ([]byte, bool) {
	idx := bytes.Index(body, []byte(old))
	if idx < 0 {
		return nil, false
	}
	end := idx + len(old)
	if end < len(body) && !isPDFNameTerminator(body[end]) {
		return nil, false
	}
	out := make([]byte, 0, len(body)-len(old)+len(replacement))
	out = append(out, body[:idx]...)
	out = append(out, replacement...)
	out = append(out, body[end:]...)
	return out, true
}

func isPDFNameTerminator(c byte) bool {
	return isPDFWhitespace(c) || strings.IndexByte("/<>[]()%", c) >= 0
}

// identityCMapObjectBody returns the body of a compressed ToUnicode stream
// object that maps each 2-byte code to the Unicode value of the same number,
// the encoding fpdf uses for its UTF-8 fonts. The map is split into one
// bfrange per high byte, as ISO 32000-1 9.10.3 requires, and leaves out the
// codes that are never valid PDF/A-2 mappings: U+0000, the UTF-16 surrogates,
// U+FEFF, and the noncharacters U+FFFE and U+FFFF. It is built once.
var identityCMapObjectBody = sync.OnceValue(func() []byte {
	var ranges []string
	for hi := 0x00; hi <= 0xFF; hi++ {
		if hi >= 0xD8 && hi <= 0xDF {
			continue
		}
		lo, last := 0x00, 0xFF
		switch hi {
		case 0x00:
			lo = 0x01
		case 0xFE:
			last = 0xFE
		case 0xFF:
			last = 0xFD
		}
		ranges = append(ranges, fmt.Sprintf("<%02X%02X> <%02X%02X> <%02X%02X>", hi, lo, hi, last, hi, lo))
	}

	var cmap bytes.Buffer
	cmap.WriteString("/CIDInit /ProcSet findresource begin\n12 dict begin\nbegincmap\n/CIDSystemInfo\n<</Registry (Adobe)\n/Ordering (UCS)\n/Supplement 0\n>> def\n/CMapName /Adobe-Identity-UCS def\n/CMapType 2 def\n1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\n")
	// A CMap block may hold at most 100 entries.
	for start := 0; start < len(ranges); start += 100 {
		end := min(start+100, len(ranges))
		fmt.Fprintf(&cmap, "%d beginbfrange\n%s\nendbfrange\n", end-start, strings.Join(ranges[start:end], "\n"))
	}
	cmap.WriteString("endcmap\nCMapName currentdict /CMap defineresource pop\nend\nend")

	// Compressing into a bytes.Buffer cannot fail: the buffer never
	// returns a write error, and zlib reports only its writer's errors.
	var compressed bytes.Buffer
	zw := zlib.NewWriter(&compressed)
	_, _ = zw.Write(cmap.Bytes())
	_ = zw.Close()

	var body bytes.Buffer
	fmt.Fprintf(&body, "<</Length %d /Filter /FlateDecode>>\nstream\n", compressed.Len())
	body.Write(compressed.Bytes())
	body.WriteString("\nendstream")
	return body.Bytes()
})

// readFontProgram returns the decompressed bytes of an embedded FontFile2
// stream. fpdf compresses font programs with FlateDecode.
func readFontProgram(pdfBytes []byte, id int) ([]byte, error) {
	dict, data, err := readStreamObject(pdfBytes, id)
	if err != nil {
		return nil, err
	}
	if !bytes.Contains(dict, []byte("/FlateDecode")) {
		if bytes.Contains(dict, []byte("/Filter")) {
			return nil, fmt.Errorf("font file %d uses an unsupported filter", id)
		}
		return data, nil
	}
	zr, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("font file %d: %w", id, err)
	}
	defer zr.Close()
	program, err := io.ReadAll(io.LimitReader(zr, maxEmbeddedFontBytes+1))
	if err != nil {
		return nil, fmt.Errorf("font file %d: %w", id, err)
	}
	if int64(len(program)) > maxEmbeddedFontBytes {
		return nil, fmt.Errorf("font file %d is larger than %d bytes", id, maxEmbeddedFontBytes)
	}
	return program, nil
}

// readStreamObject returns the dictionary and raw data of the latest revision
// of stream object id. Unlike findObjectBody it reads the data by its
// /Length, so binary stream content that happens to contain "endobj" cannot
// cut the stream short.
func readStreamObject(pdfBytes []byte, id int) (dict, data []byte, err error) {
	start, err := findObjectStart(pdfBytes, id, 0)
	if err != nil {
		return nil, nil, err
	}
	i := skipPDFWhitespace(pdfBytes, start)
	end, err := findDictionaryEnd(pdfBytes, i)
	if err != nil {
		return nil, nil, fmt.Errorf("object %d: %w", id, err)
	}
	dict = pdfBytes[i:end]
	length, err := readDirectLength(dict)
	if err != nil {
		return nil, nil, fmt.Errorf("object %d: %w", id, err)
	}
	i = skipPDFWhitespace(pdfBytes, end)
	if !bytes.HasPrefix(pdfBytes[i:], []byte("stream")) {
		return nil, nil, fmt.Errorf("object %d is not a stream", id)
	}
	i += len("stream")
	switch {
	case bytes.HasPrefix(pdfBytes[i:], []byte("\r\n")):
		i += 2
	case bytes.HasPrefix(pdfBytes[i:], []byte("\n")):
		i++
	default:
		return nil, nil, fmt.Errorf("object %d: stream keyword not followed by an end-of-line marker", id)
	}
	if length > len(pdfBytes)-i {
		return nil, nil, fmt.Errorf("object %d: stream /Length %d runs past the end of the file", id, length)
	}
	return dict, pdfBytes[i : i+length], nil
}

// readDirectLength reads a stream dictionary's direct /Length value, skipping
// keys such as /Length1 that share its prefix.
func readDirectLength(dict []byte) (int, error) {
	for from := 0; ; {
		rel := bytes.Index(dict[from:], []byte("/Length"))
		if rel < 0 {
			return 0, fmt.Errorf("/Length not present")
		}
		at := from + rel
		next := at + len("/Length")
		if next < len(dict) && isPDFWhitespace(dict[next]) {
			return readDictInt(dict[at:], "/Length")
		}
		from = next
	}
}

// trueTypePostScriptName returns the PostScript name (name ID 6) of a
// TrueType font program, preferring the Windows Unicode record. It reports
// false when the program has no usable name, so the caller keeps fpdf's.
func trueTypePostScriptName(program []byte) (string, bool) {
	if len(program) < 12 {
		return "", false
	}
	numTables := int(binary.BigEndian.Uint16(program[4:6]))
	for i := 0; i < numTables; i++ {
		rec := 12 + 16*i
		if rec+16 > len(program) {
			return "", false
		}
		if string(program[rec:rec+4]) != "name" {
			continue
		}
		off := uint64(binary.BigEndian.Uint32(program[rec+8:]))
		length := uint64(binary.BigEndian.Uint32(program[rec+12:]))
		if off+length > uint64(len(program)) {
			return "", false
		}
		return nameTablePostScriptName(program[off : off+length])
	}
	return "", false
}

func nameTablePostScriptName(table []byte) (string, bool) {
	if len(table) < 6 {
		return "", false
	}
	count := int(binary.BigEndian.Uint16(table[2:4]))
	storage := int(binary.BigEndian.Uint16(table[4:6]))
	mac := ""
	for i := 0; i < count; i++ {
		rec := 6 + 12*i
		if rec+12 > len(table) {
			break
		}
		platform := binary.BigEndian.Uint16(table[rec:])
		encoding := binary.BigEndian.Uint16(table[rec+2:])
		nameID := binary.BigEndian.Uint16(table[rec+6:])
		length := int(binary.BigEndian.Uint16(table[rec+8:]))
		offset := int(binary.BigEndian.Uint16(table[rec+10:]))
		if nameID != 6 || storage+offset+length > len(table) {
			continue
		}
		raw := table[storage+offset : storage+offset+length]
		switch {
		case platform == 3 && (encoding == 0 || encoding == 1) && length%2 == 0:
			units := make([]uint16, length/2)
			for j := range units {
				units[j] = binary.BigEndian.Uint16(raw[2*j:])
			}
			if name := string(utf16.Decode(units)); isPostScriptName(name) {
				return name, true
			}
		case platform == 1 && encoding == 0 && mac == "":
			mac = string(raw)
		}
	}
	if isPostScriptName(mac) {
		return mac, true
	}
	return "", false
}

// isPostScriptName reports whether s can serve as a PDF font name: 1 to 63
// printable ASCII characters with none of the PDF delimiters.
func isPostScriptName(s string) bool {
	if s == "" || len(s) > 63 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '!' || c > '~' || strings.IndexByte("[](){}<>/%#", c) >= 0 {
			return false
		}
	}
	return true
}

// subsetTag derives the six uppercase letters that mark a font subset from
// the subset's own bytes, so the same document always gets the same tags,
// and different subsets in one file get different ones.
func subsetTag(program []byte, used map[string]bool) string {
	sum := sha256.Sum256(program)
	for {
		var tag [6]byte
		for i := range tag {
			tag[i] = 'A' + sum[i]%26
		}
		if t := string(tag[:]); !used[t] {
			used[t] = true
			return t
		}
		sum = sha256.Sum256(sum[:])
	}
}

// appendObjectRevisions writes new revisions of existing objects as one
// incremental update. It allocates no object IDs, so /Size is unchanged.
func appendObjectRevisions(pdfBytes []byte, bodies map[int][]byte) ([]byte, error) {
	xrefOffset, err := findLastStartxref(pdfBytes)
	if err != nil {
		return nil, fmt.Errorf("pdfmeta: locate startxref: %w", err)
	}
	trailerSize, rootID, rootGen, err := parseTrailerSizeAndRoot(pdfBytes, xrefOffset)
	if err != nil {
		return nil, fmt.Errorf("pdfmeta: parse trailer: %w", err)
	}

	ids := make([]int, 0, len(bodies))
	for id := range bodies {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	var appended bytes.Buffer
	if pdfBytes[len(pdfBytes)-1] != '\n' {
		appended.WriteByte('\n')
	}
	offsets := make(map[int]int, len(ids))
	for _, id := range ids {
		offsets[id] = len(pdfBytes) + appended.Len()
		fmt.Fprintf(&appended, "%d 0 obj\n", id)
		appended.Write(bodies[id])
		appended.WriteString("\nendobj\n")
	}

	newXrefOffset := len(pdfBytes) + appended.Len()
	appended.WriteString("xref\n0 1\n0000000000 65535 f \n")
	for _, id := range ids {
		if err := writeClassicXrefEntry(&appended, id, offsets[id], 0); err != nil {
			return nil, fmt.Errorf("pdfmeta: font repair xref: %w", err)
		}
	}
	appended.WriteString("trailer\n<<\n")
	fmt.Fprintf(&appended, "/Size %d\n", trailerSize)
	fmt.Fprintf(&appended, "/Root %d %d R\n", rootID, rootGen)
	fmt.Fprintf(&appended, "/Prev %d\n", xrefOffset)
	appended.WriteString(trailerIDEntry(pdfBytes, xrefOffset))
	appended.WriteString(">>\n")
	fmt.Fprintf(&appended, "startxref\n%d\n%%%%EOF\n", newXrefOffset)

	out := make([]byte, 0, len(pdfBytes)+appended.Len())
	out = append(out, pdfBytes...)
	return append(out, appended.Bytes()...), nil
}
