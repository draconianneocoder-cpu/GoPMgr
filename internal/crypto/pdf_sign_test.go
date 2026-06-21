// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package crypto

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"testing"
	"time"

	"github.com/digitorus/pkcs7"
)

func TestSignPDFCMSRequiresKeyAndCert(t *testing.T) {
	signer := newTestCMSSigner(t, "PMForge Missing Field Signer")
	content := []byte("%PDF-1.7\n% test content\n")

	if _, err := (&Signer{Cert: signer.Cert}).SignPDFCMS(content); err == nil {
		t.Fatal("expected error when signer has no private key")
	}
	if _, err := (&Signer{PrivateKey: signer.PrivateKey}).SignPDFCMS(content); err == nil {
		t.Fatal("expected error when signer has no certificate")
	}
}

func TestSignPDFCMSProducesDetachedVerifiableSignedData(t *testing.T) {
	signer := newTestCMSSigner(t, "PMForge Test Signer")
	extraCert := newTestCMSCertificate(t, "PMForge Test Intermediate")
	signer.ExtraCerts = []*x509.Certificate{extraCert}

	content := []byte("%PDF-1.7\n1 0 obj\n<< /Type /Catalog >>\nendobj\n%%EOF\n")
	cms, err := signer.SignPDFCMS(content)
	if err != nil {
		t.Fatalf("SignPDFCMS: %v", err)
	}
	if len(cms) == 0 {
		t.Fatal("SignPDFCMS returned an empty CMS blob")
	}

	p7, err := pkcs7.Parse(cms)
	if err != nil {
		t.Fatalf("parse CMS: %v", err)
	}
	if len(p7.Content) != 0 {
		t.Fatalf("CMS must be detached; parsed content length = %d", len(p7.Content))
	}
	if len(p7.Signers) != 1 {
		t.Fatalf("expected exactly one signer, got %d", len(p7.Signers))
	}
	if got := p7.Signers[0].DigestAlgorithm.Algorithm; !got.Equal(pkcs7.OIDDigestAlgorithmSHA256) {
		t.Fatalf("digest algorithm = %v, want %v", got, pkcs7.OIDDigestAlgorithmSHA256)
	}
	assertSigningCertificateV2Attribute(t, p7, signer.Cert)
	if !cmsContainsCertificate(p7.Certificates, signer.Cert) {
		t.Fatal("CMS did not embed the signer certificate")
	}
	if !cmsContainsCertificate(p7.Certificates, extraCert) {
		t.Fatal("CMS did not embed extra certificates")
	}

	p7.Content = content
	if err := p7.Verify(); err != nil {
		t.Fatalf("verify CMS against original content: %v", err)
	}

	tampered := append([]byte(nil), content...)
	tampered[len(tampered)-1] ^= 0x01
	p7.Content = tampered
	if err := p7.Verify(); err == nil {
		t.Fatal("expected CMS verification to fail for tampered content")
	}
}

func TestSignPDFCMSOmitsPAdESBaselineBSigningTime(t *testing.T) {
	signer := newTestCMSSigner(t, "PMForge PAdES Baseline B Signer")
	content := []byte("%PDF-1.7\n% PAdES baseline-B signed sample\n")

	cms, err := signer.SignPDFCMS(content)
	if err != nil {
		t.Fatalf("SignPDFCMS: %v", err)
	}
	p7, err := pkcs7.Parse(cms)
	if err != nil {
		t.Fatalf("parse CMS: %v", err)
	}

	var signingTime time.Time
	if err := p7.UnmarshalSignedAttribute(pkcs7.OIDAttributeSigningTime, &signingTime); err == nil {
		t.Fatalf("CMS includes signing-time %s; PAdES baseline-B requires omitting it", signingTime.Format(time.RFC3339))
	}
}

func assertSigningCertificateV2Attribute(t *testing.T, p7 *pkcs7.PKCS7, signerCert *x509.Certificate) {
	t.Helper()

	var attr signingCertificateV2
	if err := p7.UnmarshalSignedAttribute(oidAttributeSigningCertificateV2, &attr); err != nil {
		t.Fatalf("CMS missing signingCertificateV2 attribute: %v", err)
	}
	if len(attr.Certs) != 1 {
		t.Fatalf("signingCertificateV2 cert count = %d, want 1", len(attr.Certs))
	}

	certID := attr.Certs[0]
	if got := certID.HashAlgorithm.Algorithm; !got.Equal(oidDigestAlgorithmSHA256) {
		t.Fatalf("signingCertificateV2 hash algorithm = %v, want %v", got, oidDigestAlgorithmSHA256)
	}
	wantHash := sha256.Sum256(signerCert.Raw)
	if !bytes.Equal(certID.CertHash, wantHash[:]) {
		t.Fatal("signingCertificateV2 certificate hash does not match signer certificate")
	}
	if certID.IssuerSerial.SerialNumber.Cmp(signerCert.SerialNumber) != 0 {
		t.Fatalf("signingCertificateV2 serial = %v, want %v", certID.IssuerSerial.SerialNumber, signerCert.SerialNumber)
	}
	if len(certID.IssuerSerial.Issuer) != 1 {
		t.Fatalf("signingCertificateV2 issuer count = %d, want 1", len(certID.IssuerSerial.Issuer))
	}
	if got := certID.IssuerSerial.Issuer[0]; got.Class != asn1.ClassContextSpecific || got.Tag != 4 || !bytes.Equal(got.Bytes, signerCert.RawIssuer) {
		t.Fatalf("signingCertificateV2 issuer does not match signer RawIssuer")
	}
}

func newTestCMSSigner(t *testing.T, commonName string) *Signer {
	t.Helper()

	key, cert := newTestCMSKeyAndCertificate(t, commonName)
	return &Signer{
		Cert:       cert,
		PrivateKey: key,
	}
}

func newTestCMSCertificate(t *testing.T, commonName string) *x509.Certificate {
	t.Helper()

	_, cert := newTestCMSKeyAndCertificate(t, commonName)
	return cert
}

func newTestCMSKeyAndCertificate(t *testing.T, commonName string) (*rsa.PrivateKey, *x509.Certificate) {
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
	return key, cert
}

func cmsContainsCertificate(certs []*x509.Certificate, want *x509.Certificate) bool {
	for _, cert := range certs {
		if cert.Equal(want) {
			return true
		}
	}
	return false
}
