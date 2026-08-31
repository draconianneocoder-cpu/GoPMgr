#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Local PAdES-T validation gate.
#
# This does not replace Acrobat/DSS/veraPDF interoperability testing. It
# generates a signed PDF sample with GoPMgr's real CMS signer, RFC 3161
# timestamp mutator, and PDF incremental-update code, then verifies the
# embedded PKCS#7 signature against the declared /ByteRange. The sample remains
# under .tmp so external validators can be pointed at it manually.

set -eu
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

SAMPLE_DIR="$ROOT/.tmp/gopmgr-pades-test"
PADES_LOCK="$ROOT/.tmp/gopmgr-pades-test.lock"
source "$ROOT/scripts/pades-lock.sh"

echo "=== PAdES Local Validation Gate ==="

acquire_pades_lock() {
	if [ "${GOPMGR_PADES_LOCK_HELD:-0}" = "1" ]; then
		return
	fi
	pades_acquire_directory_lock "$PADES_LOCK" "${GOPMGR_PADES_LOCK_TIMEOUT_SECONDS:-30}"
	trap 'rm -rf "$PADES_LOCK"' EXIT INT TERM
	export GOPMGR_PADES_LOCK_HELD=1
}

acquire_pades_lock

# Build the whole sample directory in an isolated, private scratch dir and
# atomically publish it, rather than clearing and repopulating $SAMPLE_DIR
# in place. An earlier version of this script did the latter and, despite
# holding PADES_LOCK throughout, still hit two distinct CI-only failures
# (a truncated generator source read mid-heredoc-write, and the sample
# directory reported missing entirely) -- both consistent with a second
# reader observing $SAMPLE_DIR during the window it's being torn down and
# rebuilt, however that happens under CI's specific timing. Publishing via
# rename(2) removes that window instead of trying to further narrow it:
# any observer of $SAMPLE_DIR sees either the complete prior directory or
# the complete new one, never a partial one, no matter what raced it.
mkdir -p "$ROOT/.tmp"
WORK_DIR="$(mktemp -d "$ROOT/.tmp/gopmgr-pades-test.build.XXXXXX")"
GENERATOR="$WORK_DIR/validate_pades.go"

cat > "$GENERATOR" <<'EOF'
package main

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
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
	"github.com/digitorus/timestamp"

	pmcrypto "gopmgr/internal/crypto"
	"gopmgr/internal/pdfmeta"
)

func main() {
	signer, err := newSigner("GoPMgr PAdES Gate Signer")
	if err != nil {
		fatal(err)
	}

	out, err := pdfmeta.InjectPAdESSignature(minimalPDF(), func(signedBytes []byte) ([]byte, error) {
		baselineCMS, err := signer.SignPDFCMS(signedBytes)
		if err != nil {
			return nil, fmt.Errorf("create baseline CMS: %w", err)
		}
		imprint, err := pmcrypto.SignatureTimestampImprint(baselineCMS)
		if err != nil {
			return nil, fmt.Errorf("compute signature timestamp imprint: %w", err)
		}
		tokenDER, err := newTimestampToken(imprint)
		if err != nil {
			return nil, fmt.Errorf("create RFC 3161 token: %w", err)
		}
		return pmcrypto.AddSignatureTimestamp(baselineCMS, tokenDER)
	})
	if err != nil {
		fatal(fmt.Errorf("inject PAdES signature: %w", err))
	}

	for _, marker := range [][]byte{
		[]byte("/Type /Sig"),
		[]byte("/Filter /Adobe.PPKLite"),
		[]byte("/SubFilter /ETSI.CAdES.detached"),
		[]byte("/M (D:"),
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
	if bytes.Contains(out, []byte("%%GoPMgrCMSSignature:")) {
		fatal(fmt.Errorf("signed PDF used fallback CMS comment marker instead of embedded PAdES"))
	}

	br, err := parseByteRange(out)
	if err != nil {
		fatal(err)
	}
	if br[0] != 0 || br[1] <= 0 || br[2] <= br[1] || br[3] <= 0 || br[2]+br[3] != len(out) {
		fatal(fmt.Errorf("invalid ByteRange %v over %d-byte PDF", br, len(out)))
	}

	p7, cmsDER, err := parseEmbeddedCMS(out, br)
	if err != nil {
		fatal(err)
	}
	signatureTimestampOIDDER, err := asn1.Marshal(asn1.ObjectIdentifier{
		1, 2, 840, 113549, 1, 9, 16, 2, 14,
	})
	if err != nil {
		fatal(fmt.Errorf("marshal signature timestamp OID: %w", err))
	}
	if !bytes.Contains(cmsDER, signatureTimestampOIDDER) {
		fatal(fmt.Errorf("embedded CMS has no signature-time-stamp unsigned attribute"))
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

	sampleDir := filepath.Join(".tmp", "gopmgr-pades-test")
	if override := os.Getenv("GOPMGR_PADES_SAMPLE_DIR"); override != "" {
		sampleDir = override
	}
	samplePath := filepath.Join(sampleDir, "signed-sample.pdf")
	if err := os.WriteFile(samplePath, out, 0o644); err != nil {
		fatal(fmt.Errorf("write signed sample: %w", err))
	}

	// Report the sample's canonical, post-publish location rather than
	// samplePath: when GOPMGR_PADES_SAMPLE_DIR overrides sampleDir to a
	// private build directory (see validate-pades.sh's atomic-publish
	// wrapper), samplePath points at that scratch location, which the
	// wrapper renames into place immediately after this program exits --
	// printing it here would show a path that no longer exists once the
	// caller's swap completes.
	fmt.Printf("Generated %s\n", filepath.Join(".tmp", "gopmgr-pades-test", "signed-sample.pdf"))
	fmt.Println("PAdES-T local validation gate PASSED.")
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

func newTimestampToken(imprint []byte) ([]byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate TSA key: %w", err)
	}
	ekuValue, err := asn1.Marshal([]asn1.ObjectIdentifier{
		{1, 3, 6, 1, 5, 5, 7, 3, 8},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal TSA extended key usage: %w", err)
	}

	now := time.Now().UTC()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(now.UnixNano()),
		Subject:      pkix.Name{CommonName: "GoPMgr PAdES Gate TSA"},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtraExtensions: []pkix.Extension{{
			Id:       asn1.ObjectIdentifier{2, 5, 29, 37},
			Critical: true,
			Value:    ekuValue,
		}},
	}
	certificateDER, err := x509.CreateCertificate(
		rand.Reader,
		template,
		template,
		&key.PublicKey,
		key,
	)
	if err != nil {
		return nil, fmt.Errorf("create TSA certificate: %w", err)
	}
	certificate, err := x509.ParseCertificate(certificateDER)
	if err != nil {
		return nil, fmt.Errorf("parse TSA certificate: %w", err)
	}

	responseDER, err := (&timestamp.Timestamp{
		HashAlgorithm:     crypto.SHA256,
		HashedMessage:     append([]byte(nil), imprint...),
		Time:              now,
		Policy:            asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 55555, 3},
		Nonce:             big.NewInt(now.UnixNano()),
		AddTSACertificate: true,
	}).CreateResponseWithOpts(certificate, key, crypto.SHA256)
	if err != nil {
		return nil, fmt.Errorf("create timestamp response: %w", err)
	}
	parsed, err := timestamp.ParseResponse(responseDER)
	if err != nil {
		return nil, fmt.Errorf("parse timestamp response: %w", err)
	}
	return append([]byte(nil), parsed.RawToken...), nil
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

func parseEmbeddedCMS(pdf []byte, br [4]int) (*pkcs7.PKCS7, []byte, error) {
	if br[1] >= br[2] || br[1] < 0 || br[2] > len(pdf) {
		return nil, nil, fmt.Errorf("ByteRange does not enclose /Contents: %v over %d-byte PDF", br, len(pdf))
	}
	if pdf[br[1]] != '<' || pdf[br[2]-1] != '>' {
		return nil, nil, fmt.Errorf("ByteRange gap is not a PDF hex string")
	}

	contentsHex := pdf[br[1]+1 : br[2]-1]
	contents := make([]byte, hex.DecodedLen(len(contentsHex)))
	n, err := hex.Decode(contents, contentsHex)
	if err != nil {
		return nil, nil, fmt.Errorf("decode /Contents hex: %w", err)
	}
	contents = contents[:n]

	var raw asn1.RawValue
	rest, err := asn1.Unmarshal(contents, &raw)
	if err != nil {
		return nil, nil, fmt.Errorf("decode CMS DER from padded /Contents: %w", err)
	}
	for _, b := range rest {
		if b != 0 {
			return nil, nil, fmt.Errorf("non-zero data after CMS DER in padded /Contents")
		}
	}

	p7, err := pkcs7.Parse(raw.FullBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse embedded CMS: %w", err)
	}
	if len(p7.Content) != 0 {
		return nil, nil, fmt.Errorf("embedded CMS is not detached; content length = %d", len(p7.Content))
	}
	if len(p7.Signers) != 1 {
		return nil, nil, fmt.Errorf("embedded CMS signer count = %d, want 1", len(p7.Signers))
	}
	if got := p7.Signers[0].DigestAlgorithm.Algorithm; !got.Equal(pkcs7.OIDDigestAlgorithmSHA256) {
		return nil, nil, fmt.Errorf("embedded CMS digest = %v, want %v", got, pkcs7.OIDDigestAlgorithmSHA256)
	}
	return p7, append([]byte(nil), raw.FullBytes...), nil
}

func byteRangeBytes(pdf []byte, br [4]int) []byte {
	out := make([]byte, 0, br[1]+br[3])
	out = append(out, pdf[br[0]:br[0]+br[1]]...)
	out = append(out, pdf[br[2]:br[2]+br[3]]...)
	return out
}
EOF

GOPMGR_PADES_SAMPLE_DIR="$WORK_DIR" go run "$GENERATOR"

# Publish atomically: move the old directory aside (if any) only after the
# new one is already live at $SAMPLE_DIR, so $SAMPLE_DIR is never briefly
# absent to a concurrent reader; then discard the old one.
OLD_SAMPLE_DIR=""
if [ -e "$SAMPLE_DIR" ]; then
	OLD_SAMPLE_DIR="$SAMPLE_DIR.stale.$$"
	mv "$SAMPLE_DIR" "$OLD_SAMPLE_DIR"
fi
mv "$WORK_DIR" "$SAMPLE_DIR"
if [ -n "$OLD_SAMPLE_DIR" ]; then
	rm -rf "$OLD_SAMPLE_DIR"
fi
