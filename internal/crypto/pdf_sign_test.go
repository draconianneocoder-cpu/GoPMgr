// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package crypto

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/digitorus/pkcs7"
)

const testP12Password = "gopmgr-test-only"

// TestLoadCertificate_ChainBundledInP12 is the bug report and its own
// regression test in one: a real commercially-issued signing certificate
// is routinely exported with its issuing chain bundled into the same
// .p12 file. LoadCertificate must accept that shape and return the chain
// via Signer.ExtraCerts, not reject the whole file.
func TestLoadCertificate_ChainBundledInP12(t *testing.T) {
	signer, err := LoadCertificate("testdata/testonly-rsa-3bag.p12", testP12Password)
	if err != nil {
		t.Fatalf("LoadCertificate with a chain-bundled P12: %v", err)
	}
	if signer.Cert == nil || signer.Cert.Subject.CommonName != "GoPMgr Test RSA Signer" {
		t.Fatalf("Signer.Cert = %v, want the leaf cert (CN=GoPMgr Test RSA Signer)", signer.Cert)
	}
	if len(signer.ExtraCerts) != 1 || signer.ExtraCerts[0].Subject.CommonName != "GoPMgr Test Extra Cert" {
		t.Fatalf("Signer.ExtraCerts = %v, want exactly the bundled extra certificate", signer.ExtraCerts)
	}
}

// TestLoadCertificate_TwoBagRegression is the regression fixture for the
// chain-bundling fix above: every certificate that worked with
// LoadCertificate before that change is a 2-bag file (one key, one
// cert), and must still load identically after the ToPEM-based rewrite.
func TestLoadCertificate_TwoBagRegression(t *testing.T) {
	signer, err := LoadCertificate("testdata/testonly-rsa-2bag.p12", testP12Password)
	if err != nil {
		t.Fatalf("LoadCertificate on a plain 2-bag P12: %v", err)
	}
	if signer.Cert == nil || signer.Cert.Subject.CommonName != "GoPMgr Test RSA Signer" {
		t.Fatalf("Signer.Cert = %v, want CN=GoPMgr Test RSA Signer", signer.Cert)
	}
	if signer.PrivateKey == nil {
		t.Fatal("Signer.PrivateKey is nil")
	}
	if len(signer.ExtraCerts) != 0 {
		t.Fatalf("Signer.ExtraCerts = %v, want none for a 2-bag P12", signer.ExtraCerts)
	}
	// Prove the loaded key material is actually usable, not just parsed:
	// a full sign+verify round-trip against the loaded certificate's
	// public key.
	sig, err := signer.SignPDFHash([]byte("round-trip content"))
	if err != nil {
		t.Fatalf("SignPDFHash with loaded key: %v", err)
	}
	hash := sha256.Sum256([]byte("round-trip content"))
	pub, ok := signer.Cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		t.Fatalf("Signer.Cert.PublicKey is %T, want *rsa.PublicKey", signer.Cert.PublicKey)
	}
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, hash[:], sig); err != nil {
		t.Fatalf("signature does not verify against the loaded certificate's public key: %v", err)
	}
}

func TestLoadCertificate_FileNotFound(t *testing.T) {
	if _, err := LoadCertificate("testdata/does-not-exist.p12", testP12Password); err == nil {
		t.Fatal("expected an error for a nonexistent P12 path")
	}
}

func TestLoadCertificate_WrongPassword(t *testing.T) {
	if _, err := LoadCertificate("testdata/testonly-rsa-2bag.p12", "not-the-right-password"); err == nil {
		t.Fatal("expected an error for the wrong P12 password")
	}
}

// TestLoadCertificate_RejectsNonRSAKey locks in the RSA-only contract:
// LoadCertificate must reject an EC-keyed P12 rather than silently
// accepting a key type SignPDFHash/SignPDFCMS can't use.
func TestLoadCertificate_RejectsNonRSAKey(t *testing.T) {
	_, err := LoadCertificate("testdata/testonly-ec-2bag.p12", testP12Password)
	if err == nil {
		t.Fatal("expected an error for an EC-keyed P12")
	}
	if !strings.Contains(err.Error(), "not RSA") {
		t.Fatalf("error = %q, want it to mention the key is not RSA", err.Error())
	}
}

// The remaining parseP12Blocks/splitLeafCertificate branches below are
// pure-function edge cases (malformed or multi-key/multi-cert PEM block
// sets) that don't need a P12 file at all -- constructing *pem.Block
// values directly is simpler and clearer than encoding another fixture.

func rsaPrivateKeyBlock(t *testing.T, key *rsa.PrivateKey) *pem.Block {
	t.Helper()
	return &pem.Block{Type: "PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
}

func certificateBlock(cert *x509.Certificate) *pem.Block {
	return &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}
}

func TestParseP12Blocks_RejectsMultiplePrivateKeys(t *testing.T) {
	key, cert := newTestCMSKeyAndCertificate(t, "GoPMgr Multi-Key Test")
	blocks := []*pem.Block{rsaPrivateKeyBlock(t, key), rsaPrivateKeyBlock(t, key), certificateBlock(cert)}

	if _, _, err := parseP12Blocks(blocks); err == nil {
		t.Fatal("expected an error for a P12 with more than one private key")
	}
}

func TestParseP12Blocks_RejectsUnparseablePrivateKey(t *testing.T) {
	blocks := []*pem.Block{{Type: "PRIVATE KEY", Bytes: []byte("not a key")}}
	if _, _, err := parseP12Blocks(blocks); err == nil {
		t.Fatal("expected an error for unparseable private key bytes")
	}
}

func TestParseP12Blocks_RejectsUnparseableCertificate(t *testing.T) {
	key, _ := newTestCMSKeyAndCertificate(t, "GoPMgr Bad Cert Test")
	blocks := []*pem.Block{rsaPrivateKeyBlock(t, key), {Type: "CERTIFICATE", Bytes: []byte("not a cert")}}

	if _, _, err := parseP12Blocks(blocks); err == nil {
		t.Fatal("expected an error for unparseable certificate bytes")
	}
}

func TestParseP12Blocks_RequiresAPrivateKey(t *testing.T) {
	_, cert := newTestCMSKeyAndCertificate(t, "GoPMgr No Key Test")
	blocks := []*pem.Block{certificateBlock(cert)}

	if _, _, err := parseP12Blocks(blocks); err == nil {
		t.Fatal("expected an error for a P12 with no private key")
	}
}

func TestParseP12Blocks_RequiresACertificate(t *testing.T) {
	key, _ := newTestCMSKeyAndCertificate(t, "GoPMgr No Cert Test")
	blocks := []*pem.Block{rsaPrivateKeyBlock(t, key)}

	if _, _, err := parseP12Blocks(blocks); err == nil {
		t.Fatal("expected an error for a P12 with no certificate")
	}
}

// TestSplitLeafCertificate_SkipsNonRSACertAndErrorsWithNoMatch exercises
// both remaining branches together: a non-RSA-keyed certificate in the
// candidate list must be skipped (not misidentified as the leaf), and if
// nothing left matches the private key, the function must fail closed
// rather than guess.
func TestSplitLeafCertificate_SkipsNonRSACertAndErrorsWithNoMatch(t *testing.T) {
	rsaKey, _ := newTestCMSKeyAndCertificate(t, "GoPMgr Split Test RSA")
	ecCert := newTestECCertificate(t, "GoPMgr Split Test EC")
	_, unrelatedCert := newTestCMSKeyAndCertificate(t, "GoPMgr Split Test Unrelated")

	_, _, err := splitLeafCertificate(rsaKey, []*x509.Certificate{ecCert, unrelatedCert})
	if err == nil {
		t.Fatal("expected an error when no certificate in the list matches the private key")
	}
}

func newTestECCertificate(t *testing.T, commonName string) *x509.Certificate {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate EC key: %v", err)
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
		t.Fatalf("create EC certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse EC certificate: %v", err)
	}
	return cert
}

func TestSignPDFHash_NoPrivateKey(t *testing.T) {
	if _, err := (&Signer{}).SignPDFHash([]byte("content")); err == nil {
		t.Fatal("expected an error when the signer has no private key")
	}
}

// TestSignPDFHash_ProducesVerifiableSignature uses the existing
// newTestCMSKeyAndCertificate helper (an in-memory generated key), not a
// P12 fixture -- SignPDFHash only needs a *rsa.PrivateKey, so there's no
// reason to route this through file-based loading.
func TestSignPDFHash_ProducesVerifiableSignature(t *testing.T) {
	key, cert := newTestCMSKeyAndCertificate(t, "GoPMgr SignPDFHash Test Signer")
	signer := &Signer{Cert: cert, PrivateKey: key}

	content := []byte("%PDF-1.7\n% SignPDFHash content\n")
	sig, err := signer.SignPDFHash(content)
	if err != nil {
		t.Fatalf("SignPDFHash: %v", err)
	}
	hash := sha256.Sum256(content)
	if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, hash[:], sig); err != nil {
		t.Fatalf("signature does not verify: %v", err)
	}

	tampered := sha256.Sum256([]byte("tampered content"))
	if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, tampered[:], sig); err == nil {
		t.Fatal("signature unexpectedly verified against a different hash")
	}
}

func TestSignPDFCMSRequiresKeyAndCert(t *testing.T) {
	signer := newTestCMSSigner(t, "GoPMgr Missing Field Signer")
	content := []byte("%PDF-1.7\n% test content\n")

	if _, err := (&Signer{Cert: signer.Cert}).SignPDFCMS(content); err == nil {
		t.Fatal("expected error when signer has no private key")
	}
	if _, err := (&Signer{PrivateKey: signer.PrivateKey}).SignPDFCMS(content); err == nil {
		t.Fatal("expected error when signer has no certificate")
	}
}

func TestSignPDFCMSProducesDetachedVerifiableSignedData(t *testing.T) {
	signer := newTestCMSSigner(t, "GoPMgr Test Signer")
	extraCert := newTestCMSCertificate(t, "GoPMgr Test Intermediate")
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
	signer := newTestCMSSigner(t, "GoPMgr PAdES Baseline B Signer")
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
