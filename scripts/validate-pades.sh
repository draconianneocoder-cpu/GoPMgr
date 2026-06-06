#!/bin/bash
# SPDX-FileCopyrightText: 2026 The PMForge Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Local PAdES validation gate.
#
# This does not replace Acrobat/DSS/veraPDF interoperability testing. It
# generates a deterministic signed PDF sample with PMForge's real CMS signer and
# PDF incremental-update code, then verifies the embedded PKCS#7 signature
# against the declared /ByteRange. The sample remains under .tmp so external
# validators can be pointed at it manually.

set -eu
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

SAMPLE_DIR="$ROOT/.tmp/pmforge-pades-test"
GENERATOR="$SAMPLE_DIR/validate_pades.go"

echo "=== PAdES Local Validation Gate ==="

rm -rf "$SAMPLE_DIR"
mkdir -p "$SAMPLE_DIR"

cat > "$GENERATOR" <<'EOF'
package main

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
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/digitorus/pkcs7"

	pmcrypto "pmforge/internal/crypto"
	"pmforge/internal/pdfmeta"
)

func main() {
	signer, err := newSigner("PMForge PAdES Gate Signer")
	if err != nil {
		fatal(err)
	}

	out, err := pdfmeta.InjectPAdESSignature(minimalPDF(), signer.SignPDFCMS)
	if err != nil {
		fatal(fmt.Errorf("inject PAdES signature: %w", err))
	}

	for _, marker := range [][]byte{
		[]byte("/Type /Sig"),
		[]byte("/Filter /Adobe.PPKLite"),
		[]byte("/SubFilter /ETSI.CAdES.detached"),
		[]byte("/Subtype /Widget"),
		[]byte("/FT /Sig"),
		[]byte("/AcroForm"),
		[]byte("/ByteRange ["),
		[]byte("/Contents <"),
	} {
		if !bytes.Contains(out, marker) {
			fatal(fmt.Errorf("signed PDF missing marker %q", marker))
		}
	}
	if bytes.Contains(out, []byte("%%PMForgeCMSSignature:")) {
		fatal(fmt.Errorf("signed PDF used fallback CMS comment marker instead of embedded PAdES"))
	}

	br, err := parseByteRange(out)
	if err != nil {
		fatal(err)
	}
	if br[0] != 0 || br[1] <= 0 || br[2] <= br[1] || br[3] <= 0 || br[2]+br[3] != len(out) {
		fatal(fmt.Errorf("invalid ByteRange %v over %d-byte PDF", br, len(out)))
	}

	p7, err := parseEmbeddedCMS(out, br)
	if err != nil {
		fatal(err)
	}
	p7.Content = byteRangeBytes(out, br)
	if err := p7.Verify(); err != nil {
		fatal(fmt.Errorf("CMS verification failed against declared ByteRange: %w", err))
	}

	tampered := append([]byte(nil), out...)
	tampered[0] ^= 0x01
	p7.Content = byteRangeBytes(tampered, br)
	if err := p7.Verify(); err == nil {
		fatal(fmt.Errorf("CMS verification unexpectedly passed after tampering with signed bytes"))
	}

	samplePath := filepath.Join(".tmp", "pmforge-pades-test", "signed-sample.pdf")
	if err := os.WriteFile(samplePath, out, 0o644); err != nil {
		fatal(fmt.Errorf("write signed sample: %w", err))
	}

	fmt.Printf("Generated %s\n", samplePath)
	fmt.Println("PAdES local validation gate PASSED.")
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "PAdES validation failed: %v\n", err)
	os.Exit(1)
}

func minimalPDF() []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")

	obj1Off := b.Len()
	b.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

	obj2Off := b.Len()
	b.WriteString("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")

	obj3Off := b.Len()
	b.WriteString("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << >> /Contents 4 0 R >>\nendobj\n")

	obj4Off := b.Len()
	const content = "q\nQ\n"
	fmt.Fprintf(&b, "4 0 obj\n<< /Length %d >>\nstream\n%sendstream\nendobj\n", len(content), content)

	xrefOff := b.Len()
	b.WriteString("xref\n0 5\n")
	b.WriteString("0000000000 65535 f \n")
	fmt.Fprintf(&b, "%010d 00000 n \n", obj1Off)
	fmt.Fprintf(&b, "%010d 00000 n \n", obj2Off)
	fmt.Fprintf(&b, "%010d 00000 n \n", obj3Off)
	fmt.Fprintf(&b, "%010d 00000 n \n", obj4Off)
	fmt.Fprintf(&b, "trailer\n<<\n/Size 5\n/Root 1 0 R\n>>\nstartxref\n%d\n%%%%EOF\n", xrefOff)
	return b.Bytes()
}

func newSigner(commonName string) (*pmcrypto.Signer, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate RSA key: %w", err)
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
		return nil, fmt.Errorf("create certificate: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("parse certificate: %w", err)
	}

	return &pmcrypto.Signer{
		Cert:       cert,
		PrivateKey: key,
	}, nil
}

func parseByteRange(pdf []byte) ([4]int, error) {
	const marker = "/ByteRange ["
	idx := bytes.LastIndex(pdf, []byte(marker))
	if idx < 0 {
		return [4]int{}, fmt.Errorf("PDF missing /ByteRange")
	}
	start := idx + len(marker)
	endRel := bytes.IndexByte(pdf[start:], ']')
	if endRel < 0 {
		return [4]int{}, fmt.Errorf("PDF missing /ByteRange closing bracket")
	}

	fields := strings.Fields(string(pdf[start : start+endRel]))
	if len(fields) != 4 {
		return [4]int{}, fmt.Errorf("ByteRange field count = %d, want 4", len(fields))
	}

	var out [4]int
	for i, field := range fields {
		n, err := strconv.Atoi(field)
		if err != nil {
			return [4]int{}, fmt.Errorf("parse ByteRange field %q: %w", field, err)
		}
		out[i] = n
	}
	return out, nil
}

func parseEmbeddedCMS(pdf []byte, br [4]int) (*pkcs7.PKCS7, error) {
	if br[1] >= br[2] || br[1] < 0 || br[2] > len(pdf) {
		return nil, fmt.Errorf("ByteRange does not enclose /Contents: %v over %d-byte PDF", br, len(pdf))
	}
	if pdf[br[1]] != '<' || pdf[br[2]-1] != '>' {
		return nil, fmt.Errorf("ByteRange gap is not a PDF hex string")
	}

	contentsHex := pdf[br[1]+1 : br[2]-1]
	contents := make([]byte, hex.DecodedLen(len(contentsHex)))
	n, err := hex.Decode(contents, contentsHex)
	if err != nil {
		return nil, fmt.Errorf("decode /Contents hex: %w", err)
	}
	contents = contents[:n]

	var raw asn1.RawValue
	rest, err := asn1.Unmarshal(contents, &raw)
	if err != nil {
		return nil, fmt.Errorf("decode CMS DER from padded /Contents: %w", err)
	}
	for _, b := range rest {
		if b != 0 {
			return nil, fmt.Errorf("non-zero data after CMS DER in padded /Contents")
		}
	}

	p7, err := pkcs7.Parse(raw.FullBytes)
	if err != nil {
		return nil, fmt.Errorf("parse embedded CMS: %w", err)
	}
	if len(p7.Content) != 0 {
		return nil, fmt.Errorf("embedded CMS is not detached; content length = %d", len(p7.Content))
	}
	if len(p7.Signers) != 1 {
		return nil, fmt.Errorf("embedded CMS signer count = %d, want 1", len(p7.Signers))
	}
	if got := p7.Signers[0].DigestAlgorithm.Algorithm; !got.Equal(pkcs7.OIDDigestAlgorithmSHA256) {
		return nil, fmt.Errorf("embedded CMS digest = %v, want %v", got, pkcs7.OIDDigestAlgorithmSHA256)
	}
	return p7, nil
}

func byteRangeBytes(pdf []byte, br [4]int) []byte {
	out := make([]byte, 0, br[1]+br[3])
	out = append(out, pdf[br[0]:br[0]+br[1]]...)
	out = append(out, pdf[br[2]:br[2]+br[3]]...)
	return out
}
EOF

go run "$GENERATOR"
