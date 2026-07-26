// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"strings"
	"testing"
	"time"

	pmcrypto "pmforge/internal/crypto"
)

func TestRenderSignedWithSignerUsesSharedPAdESPipeline(t *testing.T) {
	t.Parallel()

	out, err := renderSignedWithSigner(
		KindProjectCharterWord,
		DefaultContent(KindProjectCharterWord),
		"Pipeline Project",
		newDocumentTestSigner(t),
	)
	if err != nil {
		t.Fatalf("renderSignedWithSigner() error = %v", err)
	}
	for _, marker := range [][]byte{
		[]byte("/Type /Sig"),
		[]byte("/ByteRange ["),
		[]byte("/SubFilter /ETSI.CAdES.detached"),
	} {
		if !bytes.Contains(out, marker) {
			t.Fatalf("signed document does not contain %q", marker)
		}
	}
	if bytes.Contains(out, []byte("%%PMForgeCMSSignature:")) {
		t.Fatal("signed document contains the retired comment-marker signature")
	}
}

func TestRenderSignedWithSignerReturnsNoBytesWhenSigningFails(t *testing.T) {
	t.Parallel()

	signer := newDocumentTestSigner(t)
	signer.PrivateKey = nil
	out, err := renderSignedWithSigner(
		KindProjectCharterWord,
		DefaultContent(KindProjectCharterWord),
		"Fail-Closed Project",
		signer,
	)
	if err == nil || !strings.Contains(err.Error(), "missing key") {
		t.Fatalf("renderSignedWithSigner() error = %v, want missing-key failure", err)
	}
	if out != nil {
		t.Fatalf("renderSignedWithSigner() returned %d bytes after signing failure", len(out))
	}
}

func newDocumentTestSigner(t *testing.T) *pmcrypto.Signer {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	now := time.Now().UTC()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(now.UnixNano()),
		Subject:      pkix.Name{CommonName: "PMForge Document Test Signer"},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	certificateDER, err := x509.CreateCertificate(
		rand.Reader,
		template,
		template,
		&privateKey.PublicKey,
		privateKey,
	)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	certificate, err := x509.ParseCertificate(certificateDER)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	return &pmcrypto.Signer{Cert: certificate, PrivateKey: privateKey}
}
