// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	pmcrypto "gopmgr/internal/crypto"
	"gopmgr/internal/kernel"
)

func TestRenderPDFWithSignerLoaderEmbedsRealPAdESStructure(t *testing.T) {
	t.Parallel()

	signer := newExportTestSigner(t)
	out, err := renderPDFWithSignerLoader(
		ReportPayload{Tasks: map[string]*kernel.Task{}},
		ExportOptions{
			Format:           FormatPDF,
			Title:            "Signed Schedule",
			DigitalSignature: true,
		},
		func(string, string) (*pmcrypto.Signer, error) {
			return signer, nil
		},
	)
	if err != nil {
		t.Fatalf("renderPDFWithSignerLoader() error = %v", err)
	}
	for _, marker := range [][]byte{
		[]byte("/Type /Sig"),
		[]byte("/ByteRange ["),
		[]byte("/SubFilter /ETSI.CAdES.detached"),
	} {
		if !bytes.Contains(out, marker) {
			t.Fatalf("signed PDF does not contain %q", marker)
		}
	}
	if bytes.Contains(out, []byte("%%GoPMgrCMSSignature:")) {
		t.Fatal("signed PDF contains the retired comment-marker signature")
	}
}

func TestRenderPDFWithSignerLoaderReturnsNoBytesWhenPAdESEmbeddingFails(t *testing.T) {
	t.Parallel()

	signer := newExportTestSigner(t)
	signer.PrivateKey = nil
	out, err := renderPDFWithSignerLoader(
		ReportPayload{Tasks: map[string]*kernel.Task{}},
		ExportOptions{
			Format:           FormatPDF,
			Title:            "Fail-Closed Schedule",
			DigitalSignature: true,
		},
		func(string, string) (*pmcrypto.Signer, error) {
			return signer, nil
		},
	)
	if err == nil || !strings.Contains(err.Error(), "missing key") {
		t.Fatalf("renderPDFWithSignerLoader() error = %v, want missing-key failure", err)
	}
	if out != nil {
		t.Fatalf("renderPDFWithSignerLoader() returned %d bytes after signing failure", len(out))
	}
}

func TestRenderPDFWithSignerLoaderPropagatesCertificateLoadFailure(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("certificate unavailable")
	out, err := renderPDFWithSignerLoader(
		ReportPayload{Tasks: map[string]*kernel.Task{}},
		ExportOptions{
			Format:           FormatPDF,
			Title:            "Missing Certificate",
			DigitalSignature: true,
		},
		func(string, string) (*pmcrypto.Signer, error) {
			return nil, wantErr
		},
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("renderPDFWithSignerLoader() error = %v, want %v", err, wantErr)
	}
	if out != nil {
		t.Fatalf("renderPDFWithSignerLoader() returned %d bytes after certificate failure", len(out))
	}
}

func newExportTestSigner(t *testing.T) *pmcrypto.Signer {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	now := time.Now().UTC()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(now.UnixNano()),
		Subject:      pkix.Name{CommonName: "GoPMgr Export Test Signer"},
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
