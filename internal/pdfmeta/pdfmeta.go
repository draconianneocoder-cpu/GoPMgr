// SPDX-FileCopyrightText: 2026 The PMForge Contributors
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
//   - OutputIntent / ICC profile. Required for full PDF/A compliance.
//   - veraPDF validation. Tracked as a V3 milestone in AGENT.md §8.
package pdfmeta

import (
	"bytes"
	"fmt"
	"time"
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

	fmt.Fprintf(&appended, "%d 0 obj\n", metaID)
	appended.WriteString("<<\n")
	fmt.Fprintf(&appended, "/Type /Metadata\n/Subtype /XML\n/Length %d\n", len(xmpPacket))
	appended.WriteString(">>\nstream\n")
	appended.Write(xmpPacket)
	if len(xmpPacket) > 0 && xmpPacket[len(xmpPacket)-1] != '\n' {
		appended.WriteByte('\n')
	}
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
			break
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
