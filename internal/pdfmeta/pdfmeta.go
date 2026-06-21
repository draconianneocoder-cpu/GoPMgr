// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package pdfmeta provides byte-level PDF metadata operations:
// building the canonical XMP RDF/XML packet PMForge claims for its
// generated PDFs, and injecting that packet into an existing PDF via
// a spec-conformant incremental update.
//
// This package is deliberately gofpdf-free so it can be imported from
// both internal/documents (where the renderers live) and
// internal/export (where the file-format writers live) without
// creating a cycle. The Catalog rewrite is performed via low-level
// byte handling.
//
// What this package provides
//
//   - BuildXMPPacket constructs the XMP RDF/XML packet identifying a
//     PDF as PMForge-generated PDF/A-3 level B.
//   - InjectXMPStream appends the packet as an incremental update,
//     adding a /Metadata reference to the Catalog dictionary.
//
// What it does NOT provide
//
//   - Font embedding. Strict PDF/A-3 still requires shipping a TTF
//     and switching every renderer's SetFont call to it.
//   - OutputIntent / ICC profile. Code is complete (InjectOutputIntent,
//     MakePDFA3). Only the actual profile bytes (fetched via `make icc`)
//     are required at build time.
//   - veraPDF validation. Tracked as a V3 milestone in AGENT.md §8.
package pdfmeta

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/jung-kurt/gofpdf"
)

// XMPSpec describes the metadata we'll claim on a generated PDF.
// Fields default to PMForge / pmforge.local if left blank.
type XMPSpec struct {
	Title       string
	Subject     string
	Description string
	Author      string
	Keywords    []string
	CreateDate  time.Time
	// CreatorTool overrides the default "PMForge" creator label if set.
	CreatorTool string
}

// BuildXMPPacket returns the canonical XMP RDF/XML PMForge embeds in
// generated PDFs. The packet declares PDF/A-3 level B conformance and
// tags PMForge as the producer.
//
// The output is wrapped in the standard XMP packet markers so
// downstream tooling (pdfcpu, ExifTool, veraPDF) can parse it.
func BuildXMPPacket(spec XMPSpec) []byte {
	if spec.CreateDate.IsZero() {
		spec.CreateDate = time.Now().UTC()
	}
	if spec.Title == "" {
		spec.Title = "PMForge document"
	}
	if spec.Author == "" {
		spec.Author = "PMForge"
	}
	if spec.CreatorTool == "" {
		spec.CreatorTool = "PMForge"
	}

	var buf bytes.Buffer
	fmt.Fprintln(&buf, `<?xpacket begin="`+"\xef\xbb\xbf"+`" id="W5M0MpCehiHzreSzNTczkc9d"?>`)
	fmt.Fprintln(&buf, `<x:xmpmeta xmlns:x="adobe:ns:meta/" x:xmptk="PMForge">`)
	fmt.Fprintln(&buf, `  <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">`)
	fmt.Fprintln(&buf, `    <rdf:Description rdf:about=""`)
	fmt.Fprintln(&buf, `        xmlns:dc="http://purl.org/dc/elements/1.1/"`)
	fmt.Fprintln(&buf, `        xmlns:xmp="http://ns.adobe.com/xap/1.0/"`)
	fmt.Fprintln(&buf, `        xmlns:pdf="http://ns.adobe.com/pdf/1.3/"`)
	fmt.Fprintln(&buf, `        xmlns:pdfaid="http://www.aiim.org/pdfa/ns/id/">`)
	fmt.Fprintf(&buf, "      <dc:title><rdf:Alt><rdf:li xml:lang=\"x-default\">%s</rdf:li></rdf:Alt></dc:title>\n", xmlEscape(spec.Title))
	fmt.Fprintf(&buf, "      <dc:creator><rdf:Seq><rdf:li>%s</rdf:li></rdf:Seq></dc:creator>\n", xmlEscape(spec.Author))
	if spec.Description != "" {
		fmt.Fprintf(&buf, "      <dc:description><rdf:Alt><rdf:li xml:lang=\"x-default\">%s</rdf:li></rdf:Alt></dc:description>\n", xmlEscape(spec.Description))
	}
	if spec.Subject != "" {
		fmt.Fprintf(&buf, "      <dc:subject><rdf:Bag><rdf:li>%s</rdf:li></rdf:Bag></dc:subject>\n", xmlEscape(spec.Subject))
	}
	fmt.Fprintf(&buf, "      <xmp:CreateDate>%s</xmp:CreateDate>\n", spec.CreateDate.Format(time.RFC3339))
	fmt.Fprintf(&buf, "      <xmp:CreatorTool>%s</xmp:CreatorTool>\n", xmlEscape(spec.CreatorTool))
	fmt.Fprintln(&buf, `      <pdf:Producer>PMForge</pdf:Producer>`)
	fmt.Fprintln(&buf, `      <pdfaid:part>3</pdfaid:part>`)
	fmt.Fprintln(&buf, `      <pdfaid:conformance>B</pdfaid:conformance>`)
	fmt.Fprintln(&buf, `    </rdf:Description>`)
	fmt.Fprintln(&buf, `  </rdf:RDF>`)
	fmt.Fprintln(&buf, `</x:xmpmeta>`)
	fmt.Fprintln(&buf, `<?xpacket end="w"?>`)
	return buf.Bytes()
}

// InjectXMPStream appends an XMP metadata stream to an existing PDF
// as a PDF-spec incremental update, and updates the Catalog dictionary
// to reference the new stream via /Metadata. The original byte stream
// is preserved verbatim; modifications are appended after %%EOF.
//
// Returns the modified PDF bytes, or an error if the input is not a
// recognisable PDF structure.
func InjectXMPStream(pdfBytes []byte, xmpPacket []byte) ([]byte, error) {
	if len(pdfBytes) == 0 {
		return nil, fmt.Errorf("pdfmeta: empty PDF input")
	}
	if len(xmpPacket) == 0 {
		return nil, fmt.Errorf("pdfmeta: empty XMP packet")
	}

	xrefOffset, err := findLastStartxref(pdfBytes)
	if err != nil {
		return nil, fmt.Errorf("pdfmeta: locate startxref: %w", err)
	}

	trailerSize, catalogID, catalogGen, err := parseTrailerSizeAndRoot(pdfBytes, xrefOffset)
	if err != nil {
		return nil, fmt.Errorf("pdfmeta: parse trailer: %w", err)
	}

	catalogOriginal, err := findObjectBody(pdfBytes, catalogID, catalogGen)
	if err != nil {
		return nil, fmt.Errorf("pdfmeta: locate Catalog object: %w", err)
	}

	metaID := trailerSize
	revisedCatalog := insertMetadataReference(catalogOriginal, metaID)

	var appended bytes.Buffer
	if pdfBytes[len(pdfBytes)-1] != '\n' {
		appended.WriteByte('\n')
	}

	metaObjOffset := len(pdfBytes) + appended.Len()
	xmpStream := streamPayload(xmpPacket)

	fmt.Fprintf(&appended, "%d 0 obj\n", metaID)
	appended.WriteString("<<\n")
	fmt.Fprintf(&appended, "/Type /Metadata\n/Subtype /XML\n/Length %d\n", len(xmpStream))
	appended.WriteString(">>\nstream\n")
	appended.Write(xmpStream)
	appended.WriteByte('\n')
	appended.WriteString("endstream\nendobj\n")

	catalogObjOffset := len(pdfBytes) + appended.Len()
	fmt.Fprintf(&appended, "%d %d obj\n", catalogID, catalogGen)
	appended.Write(revisedCatalog)
	if revisedCatalog[len(revisedCatalog)-1] != '\n' {
		appended.WriteByte('\n')
	}
	appended.WriteString("endobj\n")

	newXrefOffset := len(pdfBytes) + appended.Len()

	appended.WriteString("xref\n")
	appended.WriteString("0 1\n")
	appended.WriteString("0000000000 65535 f \n")

	first, second := catalogID, metaID
	firstOff, secondOff := catalogObjOffset, metaObjOffset
	firstGen, secondGen := catalogGen, 0
	if first > second {
		first, second = second, first
		firstOff, secondOff = secondOff, firstOff
		firstGen, secondGen = secondGen, firstGen
	}
	fmt.Fprintf(&appended, "%d 1\n", first)
	fmt.Fprintf(&appended, "%010d %05d n \n", firstOff, firstGen)
	fmt.Fprintf(&appended, "%d 1\n", second)
	fmt.Fprintf(&appended, "%010d %05d n \n", secondOff, secondGen)

	appended.WriteString("trailer\n<<\n")
	fmt.Fprintf(&appended, "/Size %d\n", trailerSize+1)
	fmt.Fprintf(&appended, "/Root %d %d R\n", catalogID, catalogGen)
	fmt.Fprintf(&appended, "/Prev %d\n", xrefOffset)
	appended.WriteString(trailerIDEntry(pdfBytes, xrefOffset))
	appended.WriteString(">>\n")
	fmt.Fprintf(&appended, "startxref\n%d\n%%%%EOF\n", newXrefOffset)

	out := make([]byte, 0, len(pdfBytes)+appended.Len())
	out = append(out, pdfBytes...)
	out = append(out, appended.Bytes()...)
	return out, nil
}

// findLastStartxref scans backwards from the end for the literal
// `startxref` keyword and parses the integer offset that follows.
func findLastStartxref(b []byte) (int, error) {
	const marker = "startxref"
	idx := bytes.LastIndex(b, []byte(marker))
	if idx < 0 {
		return 0, fmt.Errorf("startxref keyword not found")
	}
	i := idx + len(marker)
	for i < len(b) && (b[i] == '\n' || b[i] == '\r' || b[i] == ' ' || b[i] == '\t') {
		i++
	}
	start := i
	for i < len(b) && b[i] >= '0' && b[i] <= '9' {
		i++
	}
	if i == start {
		return 0, fmt.Errorf("no digits after startxref")
	}
	offset := 0
	for _, c := range b[start:i] {
		offset = offset*10 + int(c-'0')
	}
	if offset <= 0 || offset >= len(b) {
		return 0, fmt.Errorf("startxref offset %d out of range", offset)
	}
	return offset, nil
}

// parseTrailerSizeAndRoot reads the trailer dictionary near the xref
// table and extracts /Size and /Root <id> <gen> R.
func parseTrailerSizeAndRoot(b []byte, xrefOffset int) (size, rootID, rootGen int, err error) {
	if xrefOffset < 0 || xrefOffset >= len(b) {
		return 0, 0, 0, fmt.Errorf("xref offset %d out of range", xrefOffset)
	}
	trailerIdx := bytes.Index(b[xrefOffset:], []byte("trailer"))
	if trailerIdx < 0 {
		return 0, 0, 0, fmt.Errorf("trailer keyword not found after xref")
	}
	absTrailer := xrefOffset + trailerIdx
	endIdx := bytes.Index(b[absTrailer:], []byte("startxref"))
	if endIdx < 0 {
		return 0, 0, 0, fmt.Errorf("startxref keyword not found after trailer")
	}
	block := b[absTrailer : absTrailer+endIdx]

	size, err = readDictInt(block, "/Size")
	if err != nil {
		return 0, 0, 0, fmt.Errorf("/Size: %w", err)
	}
	rootID, rootGen, err = readDictRef(block, "/Root")
	if err != nil {
		return 0, 0, 0, fmt.Errorf("/Root: %w", err)
	}
	return size, rootID, rootGen, nil
}

// readDictInt finds `/Key  N` inside a dict-like byte block.
func readDictInt(block []byte, key string) (int, error) {
	idx := bytes.Index(block, []byte(key))
	if idx < 0 {
		return 0, fmt.Errorf("%s not present", key)
	}
	i := idx + len(key)
	for i < len(block) && (block[i] == ' ' || block[i] == '\t' || block[i] == '\n' || block[i] == '\r') {
		i++
	}
	start := i
	for i < len(block) && block[i] >= '0' && block[i] <= '9' {
		i++
	}
	if i == start {
		return 0, fmt.Errorf("%s: no integer value", key)
	}
	n := 0
	for _, c := range block[start:i] {
		n = n*10 + int(c-'0')
	}
	return n, nil
}

// readDictRef finds `/Key  <id> <gen> R` inside a dict-like byte block.
func readDictRef(block []byte, key string) (id, gen int, err error) {
	idx := bytes.Index(block, []byte(key))
	if idx < 0 {
		return 0, 0, fmt.Errorf("%s not present", key)
	}
	i := idx + len(key)
	for i < len(block) && (block[i] == ' ' || block[i] == '\t' || block[i] == '\n' || block[i] == '\r') {
		i++
	}
	start := i
	for i < len(block) && block[i] >= '0' && block[i] <= '9' {
		i++
	}
	if i == start {
		return 0, 0, fmt.Errorf("%s: no id digit", key)
	}
	for _, c := range block[start:i] {
		id = id*10 + int(c-'0')
	}
	for i < len(block) && (block[i] == ' ' || block[i] == '\t') {
		i++
	}
	start = i
	for i < len(block) && block[i] >= '0' && block[i] <= '9' {
		i++
	}
	if i == start {
		return 0, 0, fmt.Errorf("%s: no gen digit", key)
	}
	for _, c := range block[start:i] {
		gen = gen*10 + int(c-'0')
	}
	for i < len(block) && (block[i] == ' ' || block[i] == '\t') {
		i++
	}
	if i >= len(block) || block[i] != 'R' {
		return 0, 0, fmt.Errorf("%s: expected R after gen", key)
	}
	return id, gen, nil
}

// findObjectBody locates the object body between `<id> <gen> obj` and
// `endobj`. The marker must be preceded by start-of-file or a newline
// so a substring inside a content stream doesn't match.
func findObjectBody(b []byte, id, gen int) ([]byte, error) {
	marker := fmt.Sprintf("%d %d obj", id, gen)
	idx := -1
	start := 0
	for {
		rel := bytes.Index(b[start:], []byte(marker))
		if rel < 0 {
			break
		}
		abs := start + rel
		if abs == 0 || b[abs-1] == '\n' || b[abs-1] == '\r' {
			idx = abs
		}
		start = abs + len(marker)
	}
	if idx < 0 {
		return nil, fmt.Errorf("object %d %d obj not found", id, gen)
	}
	bodyStart := idx + len(marker)
	for bodyStart < len(b) && (b[bodyStart] == '\n' || b[bodyStart] == '\r') {
		bodyStart++
	}
	endIdx := bytes.Index(b[bodyStart:], []byte("endobj"))
	if endIdx < 0 {
		return nil, fmt.Errorf("endobj not found for object %d %d", id, gen)
	}
	body := b[bodyStart : bodyStart+endIdx]
	for len(body) > 0 && (body[len(body)-1] == '\n' || body[len(body)-1] == '\r' || body[len(body)-1] == ' ' || body[len(body)-1] == '\t') {
		body = body[:len(body)-1]
	}
	return body, nil
}

// insertMetadataReference returns a copy of the Catalog object body
// with `/Metadata <id> 0 R` inserted into its dictionary.
func insertMetadataReference(catalogBody []byte, metaID int) []byte {
	trimmed := catalogBody
	for len(trimmed) > 0 && (trimmed[0] == ' ' || trimmed[0] == '\t' || trimmed[0] == '\n' || trimmed[0] == '\r') {
		trimmed = trimmed[1:]
	}
	leadingLen := len(catalogBody) - len(trimmed)

	if len(trimmed) < 2 || trimmed[0] != '<' || trimmed[1] != '<' {
		return []byte(fmt.Sprintf("<<\n/Metadata %d 0 R\n%s\n>>", metaID, catalogBody))
	}

	if idx := bytes.Index(trimmed, []byte("/Metadata")); idx >= 0 {
		i := idx + len("/Metadata")
		for i < len(trimmed) && (trimmed[i] == ' ' || trimmed[i] == '\t') {
			i++
		}
		for i < len(trimmed) && trimmed[i] >= '0' && trimmed[i] <= '9' {
			i++
		}
		for i < len(trimmed) && (trimmed[i] == ' ' || trimmed[i] == '\t') {
			i++
		}
		for i < len(trimmed) && trimmed[i] >= '0' && trimmed[i] <= '9' {
			i++
		}
		for i < len(trimmed) && (trimmed[i] == ' ' || trimmed[i] == '\t') {
			i++
		}
		if i < len(trimmed) && trimmed[i] == 'R' {
			i++
			rebuilt := make([]byte, 0, len(catalogBody)+8)
			rebuilt = append(rebuilt, catalogBody[:leadingLen+idx]...)
			rebuilt = append(rebuilt, []byte(fmt.Sprintf("/Metadata %d 0 R", metaID))...)
			rebuilt = append(rebuilt, trimmed[i:]...)
			return rebuilt
		}
	}

	rebuilt := make([]byte, 0, len(catalogBody)+32)
	rebuilt = append(rebuilt, catalogBody[:leadingLen+2]...)
	rebuilt = append(rebuilt, []byte(fmt.Sprintf("\n/Metadata %d 0 R", metaID))...)
	rebuilt = append(rebuilt, catalogBody[leadingLen+2:]...)
	return rebuilt
}

// xmlEscape replaces the five XML-special characters with their entity
// equivalents. Local copy so this package has zero dependencies on
// internal/export.
func xmlEscape(s string) string {
	if len(s) == 0 {
		return s
	}
	out := make([]byte, 0, len(s))
	for _, c := range []byte(s) {
		switch c {
		case '&':
			out = append(out, []byte("&amp;")...)
		case '<':
			out = append(out, []byte("&lt;")...)
		case '>':
			out = append(out, []byte("&gt;")...)
		case '"':
			out = append(out, []byte("&quot;")...)
		case '\'':
			out = append(out, []byte("&apos;")...)
		default:
			out = append(out, c)
		}
	}
	return string(out)
}

// =============================================================================
// PDF/A-3 OutputIntent + ICC profile support (V3 milestone)
// =============================================================================

// InjectOutputIntent appends an OutputIntent dictionary and an ICC profile
// stream to a PDF using a single PDF-spec compliant incremental update.
//
// The resulting PDF will contain a /OutputIntents array in the Catalog
// pointing to a /OutputIntent object whose /DestOutputProfile references
// an ICCBased color space stream. This is the missing piece for full
// PDF/A-3 conformance (in addition to the XMP metadata already provided
// by InjectXMPStream).
//
// iccProfile should be the raw bytes of a valid ICC profile (commonly
// sRGB IEC61966-2.1). Callers are responsible for supplying a profile
// that matches the color space used in the document (usually /DeviceRGB).
//
// The function reuses the same low-level xref/trailer machinery as
// InjectXMPStream so the two operations compose cleanly.
func InjectOutputIntent(pdfBytes []byte, iccProfile []byte) ([]byte, error) {
	if len(pdfBytes) == 0 {
		return nil, fmt.Errorf("pdfmeta: empty PDF input")
	}
	if len(iccProfile) == 0 {
		return nil, fmt.Errorf("pdfmeta: empty ICC profile")
	}

	// Locate the last xref and trailer (identical logic to XMP injection)
	xrefOffset, err := findLastStartxref(pdfBytes)
	if err != nil {
		return nil, fmt.Errorf("pdfmeta: locate startxref: %w", err)
	}

	trailerSize, catalogID, catalogGen, err := parseTrailerSizeAndRoot(pdfBytes, xrefOffset)
	if err != nil {
		return nil, fmt.Errorf("pdfmeta: parse trailer: %w", err)
	}

	catalogOriginal, err := findObjectBody(pdfBytes, catalogID, catalogGen)
	if err != nil {
		return nil, fmt.Errorf("pdfmeta: locate Catalog object: %w", err)
	}

	// We will add two new objects in this incremental update:
	//   1. ICC profile stream object
	//   2. OutputIntent dictionary object
	//
	// Then we rewrite the Catalog to include /OutputIntents [ oiID 0 R ]

	baseLen := len(pdfBytes)
	var appended bytes.Buffer
	if pdfBytes[baseLen-1] != '\n' {
		appended.WriteByte('\n')
	}

	// --- Object 1: ICCBased color space stream ---
	iccID := trailerSize
	iccObjOffset := baseLen + appended.Len()
	iccStream := streamPayload(iccProfile)

	fmt.Fprintf(&appended, "%d 0 obj\n", iccID)
	appended.WriteString("<<\n")
	fmt.Fprintf(&appended, "  /N 3\n") // 3 components (RGB)
	appended.WriteString("  /Alternate /DeviceRGB\n")
	fmt.Fprintf(&appended, "  /Length %d\n", len(iccStream))
	appended.WriteString(">>\nstream\n")
	appended.Write(iccStream)
	appended.WriteByte('\n')
	appended.WriteString("endstream\nendobj\n")

	// --- Object 2: OutputIntent dictionary ---
	oiID := trailerSize + 1
	oiObjOffset := baseLen + appended.Len()

	fmt.Fprintf(&appended, "%d 0 obj\n", oiID)
	appended.WriteString("<<\n")
	appended.WriteString("  /Type /OutputIntent\n")
	appended.WriteString("  /S /GTS_PDFA1\n") // PDF/A-1/3 OutputIntent subtype
	appended.WriteString("  /OutputConditionIdentifier (sRGB)\n")
	appended.WriteString("  /RegistryName (http://www.color.org)\n")
	fmt.Fprintf(&appended, "  /DestOutputProfile %d 0 R\n", iccID)
	appended.WriteString(">>\nendobj\n")

	// --- Rewrite the Catalog to reference the OutputIntent ---
	// We append /OutputIntents [ oiID 0 R ] (creating the array if necessary)
	revisedCatalog := insertOutputIntentsReference(catalogOriginal, oiID)

	catalogObjOffset := baseLen + appended.Len()
	fmt.Fprintf(&appended, "%d %d obj\n", catalogID, catalogGen)
	appended.Write(revisedCatalog)
	if len(revisedCatalog) > 0 && revisedCatalog[len(revisedCatalog)-1] != '\n' {
		appended.WriteByte('\n')
	}
	appended.WriteString("endobj\n")

	// --- New xref + trailer (incremental update) ---
	newXrefOffset := baseLen + appended.Len()

	appended.WriteString("xref\n")
	appended.WriteString("0 1\n")
	appended.WriteString("0000000000 65535 f \n")

	// Order the new objects for the xref table (smaller ID first)
	first, second := iccID, oiID
	firstOff, secondOff := iccObjOffset, oiObjOffset
	firstGen, secondGen := 0, 0
	if first > second {
		first, second = second, first
		firstOff, secondOff = secondOff, firstOff
	}
	fmt.Fprintf(&appended, "%d 1\n", first)
	fmt.Fprintf(&appended, "%010d %05d n \n", firstOff, firstGen)
	fmt.Fprintf(&appended, "%d 1\n", second)
	fmt.Fprintf(&appended, "%010d %05d n \n", secondOff, secondGen)

	// Also rewrite the Catalog (it already existed, so we need an entry for it too)
	// The Catalog is being overwritten in place in the incremental sense.
	// We must include it in the xref as well.
	fmt.Fprintf(&appended, "%d 1\n", catalogID)
	fmt.Fprintf(&appended, "%010d %05d n \n", catalogObjOffset, catalogGen)

	appended.WriteString("trailer\n<<\n")
	fmt.Fprintf(&appended, "/Size %d\n", trailerSize+2) // we added two objects
	fmt.Fprintf(&appended, "/Root %d %d R\n", catalogID, catalogGen)
	fmt.Fprintf(&appended, "/Prev %d\n", xrefOffset)
	appended.WriteString(trailerIDEntry(pdfBytes, xrefOffset))
	appended.WriteString(">>\n")
	fmt.Fprintf(&appended, "startxref\n%d\n%%%%EOF\n", newXrefOffset)

	out := make([]byte, 0, baseLen+appended.Len())
	out = append(out, pdfBytes...)
	out = append(out, appended.Bytes()...)
	return out, nil
}

// insertOutputIntentsReference is the OutputIntent analogue of
// insertMetadataReference. It injects or replaces the /OutputIntents
// entry in the Catalog dictionary so that it points at the new
// OutputIntent object.
func insertOutputIntentsReference(catalogBody []byte, oiID int) []byte {
	trimmed := catalogBody
	for len(trimmed) > 0 && (trimmed[0] == ' ' || trimmed[0] == '\t' || trimmed[0] == '\n' || trimmed[0] == '\r') {
		trimmed = trimmed[1:]
	}
	leadingLen := len(catalogBody) - len(trimmed)

	if len(trimmed) < 2 || trimmed[0] != '<' || trimmed[1] != '<' {
		return []byte(fmt.Sprintf("<<\n/OutputIntents [ %d 0 R ]\n%s\n>>", oiID, catalogBody))
	}

	// If /OutputIntents already exists, replace its value (simple case)
	if idx := bytes.Index(trimmed, []byte("/OutputIntents")); idx >= 0 {
		// For simplicity in the first implementation we just overwrite the
		// value after the key. A more robust version would parse the array.
		i := idx + len("/OutputIntents")
		for i < len(trimmed) && (trimmed[i] == ' ' || trimmed[i] == '\t') {
			i++
		}
		// Skip over existing array content until we find the next key or >>
		end := i
		depth := 0
		for end < len(trimmed) {
			if trimmed[end] == '[' {
				depth++
			} else if trimmed[end] == ']' {
				depth--
				if depth == 0 {
					end++
					break
				}
			} else if depth == 0 && (trimmed[end] == '/' || (trimmed[end] == '>' && end+1 < len(trimmed) && trimmed[end+1] == '>')) {
				break
			}
			end++
		}
		rebuilt := make([]byte, 0, len(catalogBody)+32)
		rebuilt = append(rebuilt, catalogBody[:leadingLen+idx]...)
		rebuilt = append(rebuilt, []byte(fmt.Sprintf("/OutputIntents [ %d 0 R ]", oiID))...)
		rebuilt = append(rebuilt, trimmed[end:]...)
		return rebuilt
	}

	// Insert a fresh /OutputIntents entry after the opening <<
	rebuilt := make([]byte, 0, len(catalogBody)+64)
	rebuilt = append(rebuilt, catalogBody[:leadingLen+2]...)
	rebuilt = append(rebuilt, []byte(fmt.Sprintf("\n/OutputIntents [ %d 0 R ]", oiID))...)
	rebuilt = append(rebuilt, catalogBody[leadingLen+2:]...)
	return rebuilt
}

// MakePDFA3 performs both the XMP metadata injection (PDF/A identifier)
// and the OutputIntent + ICC profile injection in a single call,
// returning a PDF that claims PDF/A-3b conformance.
//
// This is the recommended high-level entry point for renderers that
// want full archival PDF/A-3 output.
func MakePDFA3(pdfBytes []byte, spec XMPSpec, iccProfile []byte) ([]byte, error) {
	base, err := ensureBinaryHeaderComment(pdfBytes)
	if err != nil {
		return nil, fmt.Errorf("pdfmeta: binary header comment: %w", err)
	}

	// First inject XMP (this does one incremental update)
	withXMP, err := InjectXMPStream(base, BuildXMPPacket(spec))
	if err != nil {
		return nil, fmt.Errorf("pdfmeta: xmp injection: %w", err)
	}

	// Then inject OutputIntent (second incremental update on the result)
	withOI, err := InjectOutputIntent(withXMP, iccProfile)
	if err != nil {
		return nil, fmt.Errorf("pdfmeta: outputintent injection: %w", err)
	}
	return withOI, nil
}

func streamPayload(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	if data[len(data)-1] != '\n' && data[len(data)-1] != '\r' {
		return data
	}
	return append([]byte(nil), data...)
}

func trailerIDEntry(pdfBytes []byte, xrefOffset int) string {
	if id, ok := readTrailerIDValue(pdfBytes, xrefOffset); ok {
		return fmt.Sprintf("/ID %s\n", id)
	}
	sum := sha256.Sum256(pdfBytes)
	id := fmt.Sprintf("%x", sum[:16])
	return fmt.Sprintf("/ID [<%s> <%s>]\n", id, id)
}

func readTrailerIDValue(b []byte, xrefOffset int) (string, bool) {
	if xrefOffset < 0 || xrefOffset >= len(b) {
		return "", false
	}
	trailerIdx := bytes.Index(b[xrefOffset:], []byte("trailer"))
	if trailerIdx < 0 {
		return "", false
	}
	absTrailer := xrefOffset + trailerIdx
	endIdx := bytes.Index(b[absTrailer:], []byte("startxref"))
	if endIdx < 0 {
		return "", false
	}
	block := b[absTrailer : absTrailer+endIdx]
	idx := bytes.Index(block, []byte("/ID"))
	if idx < 0 {
		return "", false
	}
	i := idx + len("/ID")
	for i < len(block) && (block[i] == ' ' || block[i] == '\t' || block[i] == '\n' || block[i] == '\r') {
		i++
	}
	if i >= len(block) || block[i] != '[' {
		return "", false
	}
	start := i
	for i < len(block) && block[i] != ']' {
		i++
	}
	if i >= len(block) {
		return "", false
	}
	return string(block[start : i+1]), true
}

func ensureBinaryHeaderComment(pdfBytes []byte) ([]byte, error) {
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF-")) {
		return nil, fmt.Errorf("missing PDF header")
	}
	headerEnd := bytes.IndexByte(pdfBytes, '\n')
	if headerEnd < 0 {
		return nil, fmt.Errorf("PDF header line is unterminated")
	}
	nextLine := pdfBytes[headerEnd+1:]
	if hasBinaryHeaderComment(nextLine) {
		return pdfBytes, nil
	}

	comment := []byte("%\xe2\xe3\xcf\xd3\n")
	xrefOffset, err := findLastStartxref(pdfBytes)
	if err != nil {
		return nil, err
	}

	out := make([]byte, 0, len(pdfBytes)+len(comment))
	out = append(out, pdfBytes[:headerEnd+1]...)
	out = append(out, comment...)
	out = append(out, pdfBytes[headerEnd+1:]...)

	newXrefOffset := xrefOffset + len(comment)
	if err := shiftClassicXrefOffsets(out, newXrefOffset, len(comment)); err != nil {
		return nil, err
	}
	out, err = replaceStartxrefValue(out, newXrefOffset)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func hasBinaryHeaderComment(line []byte) bool {
	if len(line) < 6 || line[0] != '%' {
		return false
	}
	for i := 1; i <= 4; i++ {
		if line[i] <= 127 {
			return false
		}
	}
	return line[5] == '\n' || line[5] == '\r'
}

func shiftClassicXrefOffsets(pdfBytes []byte, xrefOffset, delta int) error {
	if xrefOffset < 0 || xrefOffset >= len(pdfBytes) || !bytes.HasPrefix(pdfBytes[xrefOffset:], []byte("xref")) {
		return fmt.Errorf("xref offset %d does not point to classic xref", xrefOffset)
	}
	pos := xrefOffset + len("xref")
	pos = skipLineEOL(pdfBytes, pos)
	for pos < len(pdfBytes) {
		lineStart := pos
		lineEnd := bytes.IndexByte(pdfBytes[lineStart:], '\n')
		if lineEnd < 0 {
			return fmt.Errorf("unterminated xref line")
		}
		lineEnd += lineStart
		line := bytes.TrimSpace(pdfBytes[lineStart:lineEnd])
		pos = lineEnd + 1
		if bytes.Equal(line, []byte("trailer")) {
			return nil
		}
		fields := bytes.Fields(line)
		if len(fields) != 2 {
			return fmt.Errorf("malformed xref subsection header %q", string(line))
		}
		count, err := parsePositiveDecimal(fields[1])
		if err != nil {
			return fmt.Errorf("xref subsection count: %w", err)
		}
		for i := 0; i < count; i++ {
			entryStart := pos
			entryEnd := bytes.IndexByte(pdfBytes[entryStart:], '\n')
			if entryEnd < 0 {
				return fmt.Errorf("unterminated xref entry")
			}
			entryEnd += entryStart
			entry := pdfBytes[entryStart:entryEnd]
			if len(entry) >= 18 && entry[17] == 'n' {
				oldOff, err := parsePositiveDecimal(entry[:10])
				if err != nil {
					return fmt.Errorf("xref entry offset: %w", err)
				}
				copy(pdfBytes[entryStart:entryStart+10], []byte(fmt.Sprintf("%010d", oldOff+delta)))
			}
			pos = entryEnd + 1
		}
	}
	return fmt.Errorf("xref trailer not found")
}

func skipLineEOL(b []byte, pos int) int {
	for pos < len(b) && (b[pos] == '\r' || b[pos] == '\n') {
		pos++
	}
	return pos
}

func parsePositiveDecimal(b []byte) (int, error) {
	b = bytes.TrimSpace(b)
	if len(b) == 0 {
		return 0, fmt.Errorf("empty decimal")
	}
	n := 0
	for _, c := range b {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid decimal %q", string(b))
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

func replaceStartxrefValue(pdfBytes []byte, xrefOffset int) ([]byte, error) {
	marker := []byte("startxref")
	idx := bytes.LastIndex(pdfBytes, marker)
	if idx < 0 {
		return nil, fmt.Errorf("startxref keyword not found")
	}
	i := idx + len(marker)
	for i < len(pdfBytes) && (pdfBytes[i] == ' ' || pdfBytes[i] == '\t' || pdfBytes[i] == '\n' || pdfBytes[i] == '\r') {
		i++
	}
	start := i
	for i < len(pdfBytes) && pdfBytes[i] >= '0' && pdfBytes[i] <= '9' {
		i++
	}
	if i == start {
		return nil, fmt.Errorf("startxref value missing")
	}
	replacement := []byte(fmt.Sprintf("%d", xrefOffset))
	out := make([]byte, 0, len(pdfBytes)-i+start+len(replacement))
	out = append(out, pdfBytes[:start]...)
	out = append(out, replacement...)
	out = append(out, pdfBytes[i:]...)
	return out, nil
}

// =============================================================================
// Real PAdES B-B signature embedding
// =============================================================================

// InjectPAdESSignature builds a proper PAdES B-B signature structure
// (incremental update with /Sig dictionary, /ByteRange, and /Contents)
// and signs the *exact* byte ranges.
//
// The caller provides a signing function that will be called with the
// concatenation of the two ByteRange segments. The returned bytes must
// be a CMS/PKCS#7 detached signature over that data.
//
// This produces a fully correct /ByteRange and signature for the final PDF.
func InjectPAdESSignature(pdfBytes []byte, signRanges func([]byte) ([]byte, error)) ([]byte, error) {
	if len(pdfBytes) == 0 {
		return nil, fmt.Errorf("pdfmeta: empty PDF for signing")
	}
	if signRanges == nil {
		return nil, fmt.Errorf("pdfmeta: signRanges callback required")
	}

	const placeholderHexLen = 16384 // 8KB capacity
	const byteRangePlaceholderLen = 96

	// Locate trailer and Catalog of the *original* PDF
	xrefOffset, err := findLastStartxref(pdfBytes)
	if err != nil {
		return nil, fmt.Errorf("pdfmeta: locate startxref: %w", err)
	}
	trailerSize, catalogID, catalogGen, err := parseTrailerSizeAndRoot(pdfBytes, xrefOffset)
	if err != nil {
		return nil, fmt.Errorf("pdfmeta: parse trailer: %w", err)
	}
	catalogOriginal, err := findObjectBody(pdfBytes, catalogID, catalogGen)
	if err != nil {
		return nil, fmt.Errorf("pdfmeta: locate Catalog: %w", err)
	}

	base := len(pdfBytes)
	var appended bytes.Buffer
	if pdfBytes[base-1] != '\n' {
		appended.WriteByte('\n')
	}

	// === Write Signature Dictionary with placeholder Contents and ByteRange ===
	sigID := trailerSize
	sigObjOffset := base + appended.Len()

	fmt.Fprintf(&appended, "%d 0 obj\n", sigID)
	appended.WriteString("<<\n")
	appended.WriteString("  /Type /Sig\n")
	appended.WriteString("  /Filter /Adobe.PPKLite\n")
	appended.WriteString("  /SubFilter /ETSI.CAdES.detached\n")
	fmt.Fprintf(&appended, "  /M (%s)\n", pdfDateUTC(time.Now()))
	// Large zero placeholder for Contents
	fmt.Fprintf(&appended, "  /Contents <%s>\n", bytes.Repeat([]byte("00"), placeholderHexLen/2))
	fmt.Fprintf(&appended, "  /ByteRange [%s]\n", bytes.Repeat([]byte(" "), byteRangePlaceholderLen))
	appended.WriteString("  /Name (PMForge Digital Signature)\n")
	appended.WriteString(">>\nendobj\n")

	// === Signature Field ===
	fieldID := trailerSize + 1
	fieldObjOffset := base + appended.Len()

	fmt.Fprintf(&appended, "%d 0 obj\n", fieldID)
	appended.WriteString("<<\n")
	appended.WriteString("  /Type /Annot\n")
	appended.WriteString("  /Subtype /Widget\n")
	appended.WriteString("  /Rect [0 0 0 0]\n")
	appended.WriteString("  /FT /Sig\n")
	fmt.Fprintf(&appended, "  /T (Signature%d)\n", sigID)
	fmt.Fprintf(&appended, "  /V %d 0 R\n", sigID)
	appended.WriteString("  /F 4\n")
	appended.WriteString(">>\nendobj\n")

	// === Updated Catalog / AcroForm references ===
	// The signature widget must be reachable through the document AcroForm.
	// If the PDF already has one, merge the field instead of creating an
	// orphan widget that validators cannot discover from the form tree.
	revisedCatalog, extraRewrites, err := signatureFieldReferenceRewrites(pdfBytes, catalogOriginal, fieldID)
	if err != nil {
		return nil, err
	}
	catalogObjOffset := base + appended.Len()
	fmt.Fprintf(&appended, "%d %d obj\n", catalogID, catalogGen)
	appended.Write(revisedCatalog)
	if len(revisedCatalog) > 0 && revisedCatalog[len(revisedCatalog)-1] != '\n' {
		appended.WriteByte('\n')
	}
	appended.WriteString("endobj\n")

	rewriteOffsets := make([]int, len(extraRewrites))
	for i, rewrite := range extraRewrites {
		rewriteOffsets[i] = base + appended.Len()
		fmt.Fprintf(&appended, "%d %d obj\n", rewrite.id, rewrite.gen)
		appended.Write(rewrite.body)
		if len(rewrite.body) > 0 && rewrite.body[len(rewrite.body)-1] != '\n' {
			appended.WriteByte('\n')
		}
		appended.WriteString("endobj\n")
	}

	// === xref + trailer ===
	newXrefOffset := base + appended.Len()
	appended.WriteString("xref\n0 1\n0000000000 65535 f \n")

	ids := []int{catalogID, sigID, fieldID}
	offsets := []int{catalogObjOffset, sigObjOffset, fieldObjOffset}
	gens := []int{catalogGen, 0, 0}
	for i, rewrite := range extraRewrites {
		ids = append(ids, rewrite.id)
		offsets = append(offsets, rewriteOffsets[i])
		gens = append(gens, rewrite.gen)
	}
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			if ids[j] < ids[i] {
				ids[i], ids[j] = ids[j], ids[i]
				offsets[i], offsets[j] = offsets[j], offsets[i]
				gens[i], gens[j] = gens[j], gens[i]
			}
		}
	}
	for i := 0; i < len(ids); i++ {
		fmt.Fprintf(&appended, "%d 1\n%010d %05d n \n", ids[i], offsets[i], gens[i])
	}
	appended.WriteString("trailer\n<<\n")
	fmt.Fprintf(&appended, "/Size %d\n", trailerSize+2)
	fmt.Fprintf(&appended, "/Root %d %d R\n", catalogID, catalogGen)
	fmt.Fprintf(&appended, "/Prev %d\n", xrefOffset)
	appended.WriteString(">>\n")
	fmt.Fprintf(&appended, "startxref\n%d\n%%%%EOF\n", newXrefOffset)

	// Final file (with zero placeholder)
	out := make([]byte, 0, base+appended.Len())
	out = append(out, pdfBytes...)
	out = append(out, appended.Bytes()...)

	// === Find exact position of the Contents hex value in the final file ===
	contentsTag := []byte("/Contents <")
	tagIdx := bytes.Index(out[sigObjOffset:], contentsTag)
	if tagIdx < 0 {
		return nil, fmt.Errorf("pdfmeta: could not locate /Contents in signature dictionary")
	}
	contentsHexStart := sigObjOffset + tagIdx + len(contentsTag)
	contentsHexEnd := contentsHexStart + placeholderHexLen
	if contentsHexEnd >= len(out) || out[contentsHexEnd] != '>' {
		closeRel := bytes.IndexByte(out[contentsHexStart:], '>')
		if closeRel < 0 {
			return nil, fmt.Errorf("pdfmeta: could not locate closing /Contents delimiter")
		}
		contentsHexEnd = contentsHexStart + closeRel
	}
	contentsStart := contentsHexStart - 1 // the opening '<'
	contentsEnd := contentsHexEnd + 1     // just after the closing '>'

	// === Build correct ByteRange ===
	// Range 1: 0 .. contentsStart-1 (up to but not including the first '<')
	// Range 2: contentsEnd .. end of file (after the closing '>')
	byteRange := fmt.Sprintf("0 %d %d %d", contentsStart, contentsEnd, len(out)-contentsEnd)
	if len(byteRange) > byteRangePlaceholderLen {
		return nil, fmt.Errorf("pdfmeta: ByteRange too large for placeholder")
	}

	// Patch ByteRange into the signature dictionary
	brTag := []byte("/ByteRange [")
	brIdx := bytes.Index(out[sigObjOffset:], brTag)
	if brIdx < 0 {
		return nil, fmt.Errorf("pdfmeta: could not locate /ByteRange in signature dictionary")
	}
	brValueStart := sigObjOffset + brIdx + len(brTag)
	brValueEnd := brValueStart + byteRangePlaceholderLen
	if brValueEnd >= len(out) || out[brValueEnd] != ']' {
		return nil, fmt.Errorf("pdfmeta: ByteRange placeholder malformed")
	}
	copy(out[brValueStart:brValueEnd], bytes.Repeat([]byte(" "), byteRangePlaceholderLen))
	copy(out[brValueStart:], []byte(byteRange))

	// === Compute the data to be signed (the two ranges) ===
	range1 := out[0:contentsStart]
	range2 := out[contentsEnd:]
	toSign := make([]byte, 0, len(range1)+len(range2))
	toSign = append(toSign, range1...)
	toSign = append(toSign, range2...)

	// Sign the exact ranges
	cmsSignature, err := signRanges(toSign)
	if err != nil {
		return nil, fmt.Errorf("pdfmeta: range signing failed: %w", err)
	}

	if len(cmsSignature)*2 > placeholderHexLen {
		return nil, fmt.Errorf("pdfmeta: produced CMS signature too large for placeholder")
	}

	// Build the real hex for Contents (right-padded with 0s)
	realContents := make([]byte, placeholderHexLen)
	hexEncodeTo(cmsSignature, realContents)
	for i := len(cmsSignature) * 2; i < placeholderHexLen; i++ {
		realContents[i] = '0'
	}

	// Overwrite the placeholder Contents with the real signature
	copy(out[contentsHexStart:], realContents)

	return out, nil
}

func pdfDateUTC(t time.Time) string {
	return t.UTC().Format("D:20060102150405Z")
}

// hexEncodeTo writes the hex representation of src into dst (must be large enough).
func hexEncodeTo(src, dst []byte) {
	const hex = "0123456789abcdef"
	for i, b := range src {
		dst[i*2] = hex[b>>4]
		dst[i*2+1] = hex[b&0x0f]
	}
}

type objectRewrite struct {
	id   int
	gen  int
	body []byte
}

// signatureFieldReferenceRewrites adds the signature widget to the document's
// AcroForm. Existing direct AcroForm dictionaries are merged in the rewritten
// Catalog. Existing indirect AcroForm dictionaries are rewritten as their own
// incremental-update object so existing form fields remain intact.
func signatureFieldReferenceRewrites(pdfBytes, catalogBody []byte, fieldID int) ([]byte, []objectRewrite, error) {
	trimmed := bytes.TrimLeft(catalogBody, " \t\n\r")
	if len(trimmed) < 2 || trimmed[0] != '<' || trimmed[1] != '<' {
		return []byte(fmt.Sprintf("<<\n/AcroForm << /Fields [ %d 0 R ] /SigFlags 3 >>\n%s\n>>", fieldID, catalogBody)), nil, nil
	}
	leadingLen := len(catalogBody) - len(trimmed)

	acroIdx := bytes.Index(trimmed, []byte("/AcroForm"))
	if acroIdx < 0 {
		rebuilt := make([]byte, 0, len(catalogBody)+128)
		rebuilt = append(rebuilt, catalogBody[:leadingLen+2]...)
		rebuilt = append(rebuilt, []byte(fmt.Sprintf("\n/AcroForm << /Fields [ %d 0 R ] /SigFlags 3 >>", fieldID))...)
		rebuilt = append(rebuilt, catalogBody[leadingLen+2:]...)
		return rebuilt, nil, nil
	}

	valueStart := acroIdx + len("/AcroForm")
	valueStart = skipPDFWhitespace(trimmed, valueStart)
	if valueStart >= len(trimmed) {
		return nil, nil, fmt.Errorf("pdfmeta: malformed /AcroForm entry")
	}

	if valueStart+1 < len(trimmed) && trimmed[valueStart] == '<' && trimmed[valueStart+1] == '<' {
		valueEnd, err := findDictionaryEnd(trimmed, valueStart)
		if err != nil {
			return nil, nil, fmt.Errorf("pdfmeta: parse direct /AcroForm: %w", err)
		}
		merged, err := mergeSignatureFieldIntoAcroForm(trimmed[valueStart:valueEnd], fieldID)
		if err != nil {
			return nil, nil, fmt.Errorf("pdfmeta: merge direct /AcroForm: %w", err)
		}
		rebuilt := make([]byte, 0, len(catalogBody)+len(merged)+32)
		rebuilt = append(rebuilt, catalogBody[:leadingLen+valueStart]...)
		rebuilt = append(rebuilt, merged...)
		rebuilt = append(rebuilt, trimmed[valueEnd:]...)
		return rebuilt, nil, nil
	}

	acroID, acroGen, err := readRefAt(trimmed, valueStart, "/AcroForm")
	if err != nil {
		return nil, nil, fmt.Errorf("pdfmeta: unsupported /AcroForm entry: %w", err)
	}
	acroBody, err := findObjectBody(pdfBytes, acroID, acroGen)
	if err != nil {
		return nil, nil, fmt.Errorf("pdfmeta: locate AcroForm object: %w", err)
	}
	merged, err := mergeSignatureFieldIntoAcroForm(acroBody, fieldID)
	if err != nil {
		return nil, nil, fmt.Errorf("pdfmeta: merge indirect /AcroForm: %w", err)
	}
	return catalogBody, []objectRewrite{{id: acroID, gen: acroGen, body: merged}}, nil
}

func mergeSignatureFieldIntoAcroForm(acroFormBody []byte, fieldID int) ([]byte, error) {
	withField, err := appendSignatureFieldToFields(acroFormBody, fieldID)
	if err != nil {
		return nil, err
	}
	return ensureSignatureFieldFlags(withField)
}

func appendSignatureFieldToFields(acroFormBody []byte, fieldID int) ([]byte, error) {
	trimmed := bytes.TrimLeft(acroFormBody, " \t\n\r")
	if len(trimmed) < 2 || trimmed[0] != '<' || trimmed[1] != '<' {
		return nil, fmt.Errorf("AcroForm is not a direct dictionary")
	}
	leadingLen := len(acroFormBody) - len(trimmed)

	fieldsIdx := bytes.Index(trimmed, []byte("/Fields"))
	if fieldsIdx < 0 {
		rebuilt := make([]byte, 0, len(acroFormBody)+32)
		rebuilt = append(rebuilt, acroFormBody[:leadingLen+2]...)
		rebuilt = append(rebuilt, []byte(fmt.Sprintf("\n/Fields [ %d 0 R ]", fieldID))...)
		rebuilt = append(rebuilt, acroFormBody[leadingLen+2:]...)
		return rebuilt, nil
	}

	valueStart := fieldsIdx + len("/Fields")
	valueStart = skipPDFWhitespace(trimmed, valueStart)
	if valueStart >= len(trimmed) || trimmed[valueStart] != '[' {
		return nil, fmt.Errorf("AcroForm /Fields is not a direct array")
	}
	valueEnd, err := findArrayEnd(trimmed, valueStart)
	if err != nil {
		return nil, fmt.Errorf("parse AcroForm /Fields: %w", err)
	}
	closingBracket := valueEnd - 1
	insertAt := closingBracket
	for insertAt > valueStart+1 && isPDFWhitespace(trimmed[insertAt-1]) {
		insertAt--
	}
	rebuilt := make([]byte, 0, len(acroFormBody)+16)
	rebuilt = append(rebuilt, acroFormBody[:leadingLen+insertAt]...)
	rebuilt = append(rebuilt, []byte(fmt.Sprintf(" %d 0 R ", fieldID))...)
	rebuilt = append(rebuilt, trimmed[closingBracket:]...)
	return rebuilt, nil
}

func ensureSignatureFieldFlags(acroFormBody []byte) ([]byte, error) {
	trimmed := bytes.TrimLeft(acroFormBody, " \t\n\r")
	if len(trimmed) < 2 || trimmed[0] != '<' || trimmed[1] != '<' {
		return nil, fmt.Errorf("AcroForm is not a direct dictionary")
	}
	leadingLen := len(acroFormBody) - len(trimmed)

	flagsIdx := bytes.Index(trimmed, []byte("/SigFlags"))
	if flagsIdx < 0 {
		rebuilt := make([]byte, 0, len(acroFormBody)+16)
		rebuilt = append(rebuilt, acroFormBody[:leadingLen+2]...)
		rebuilt = append(rebuilt, []byte("\n/SigFlags 3")...)
		rebuilt = append(rebuilt, acroFormBody[leadingLen+2:]...)
		return rebuilt, nil
	}

	valueStart := flagsIdx + len("/SigFlags")
	valueStart = skipPDFWhitespace(trimmed, valueStart)
	valueEnd := valueStart
	for valueEnd < len(trimmed) && trimmed[valueEnd] >= '0' && trimmed[valueEnd] <= '9' {
		valueEnd++
	}
	if valueEnd == valueStart {
		return nil, fmt.Errorf("AcroForm /SigFlags is not an integer")
	}
	flags := 0
	for _, c := range trimmed[valueStart:valueEnd] {
		flags = flags*10 + int(c-'0')
	}
	flags |= 3

	rebuilt := make([]byte, 0, len(acroFormBody)+4)
	rebuilt = append(rebuilt, acroFormBody[:leadingLen+flagsIdx]...)
	rebuilt = append(rebuilt, []byte(fmt.Sprintf("/SigFlags %d", flags))...)
	rebuilt = append(rebuilt, trimmed[valueEnd:]...)
	return rebuilt, nil
}

func skipPDFWhitespace(b []byte, i int) int {
	for i < len(b) && isPDFWhitespace(b[i]) {
		i++
	}
	return i
}

func isPDFWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func readRefAt(block []byte, i int, key string) (id, gen int, err error) {
	start := i
	for i < len(block) && block[i] >= '0' && block[i] <= '9' {
		i++
	}
	if i == start {
		return 0, 0, fmt.Errorf("%s: no id digit", key)
	}
	for _, c := range block[start:i] {
		id = id*10 + int(c-'0')
	}
	i = skipPDFWhitespace(block, i)
	start = i
	for i < len(block) && block[i] >= '0' && block[i] <= '9' {
		i++
	}
	if i == start {
		return 0, 0, fmt.Errorf("%s: no gen digit", key)
	}
	for _, c := range block[start:i] {
		gen = gen*10 + int(c-'0')
	}
	i = skipPDFWhitespace(block, i)
	if i >= len(block) || block[i] != 'R' {
		return 0, 0, fmt.Errorf("%s: expected R after gen", key)
	}
	return id, gen, nil
}

func findDictionaryEnd(b []byte, start int) (int, error) {
	if start+1 >= len(b) || b[start] != '<' || b[start+1] != '<' {
		return 0, fmt.Errorf("dictionary does not start with <<")
	}
	depth := 0
	for i := start; i+1 < len(b); i++ {
		if b[i] == '<' && b[i+1] == '<' {
			depth++
			i++
			continue
		}
		if b[i] == '>' && b[i+1] == '>' {
			depth--
			i++
			if depth == 0 {
				return i + 1, nil
			}
		}
	}
	return 0, fmt.Errorf("unterminated dictionary")
}

func findArrayEnd(b []byte, start int) (int, error) {
	if start >= len(b) || b[start] != '[' {
		return 0, fmt.Errorf("array does not start with [")
	}
	depth := 0
	for i := start; i < len(b); i++ {
		switch b[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i + 1, nil
			}
		}
	}
	return 0, fmt.Errorf("unterminated array")
}

// ApplyPDFAMetadata sets the standard PDF metadata fields on a gofpdf
// instance in a way that is friendly to PDF/A-3 (Title, Author, Subject,
// Creator, Keywords). It is safe to call with a nil pdf.
//
// This is the canonical home for the helper so both document renderers
// and export renderers can use it without duplication.
func ApplyPDFAMetadata(pdf *gofpdf.Fpdf, spec XMPSpec) {
	if pdf == nil {
		return
	}
	if spec.Title != "" {
		pdf.SetTitle(spec.Title, true)
	}
	if spec.Author == "" {
		spec.Author = "PMForge"
	}
	pdf.SetAuthor(spec.Author, true)
	if spec.Subject != "" {
		pdf.SetSubject(spec.Subject, true)
	}
	// Creator is set without the live version here; the export package
	// overrides with the real version string when it re-exports.
	pdf.SetCreator("PMForge", true)
	if len(spec.Keywords) > 0 {
		// simple join without pulling in strings
		kw := spec.Keywords[0]
		for _, k := range spec.Keywords[1:] {
			kw += ", " + k
		}
		pdf.SetKeywords(kw, true)
	}
}
