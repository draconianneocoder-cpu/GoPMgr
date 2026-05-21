// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package crypto

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"errors"
	"fmt"
	"os"

	"github.com/digitorus/pkcs7"
	"golang.org/x/crypto/pkcs12"
)

// Signer holds a decoded X.509 certificate and its RSA private key,
// loaded once from a .p12 / .pfx file and reused for every signing
// operation in a session. ExtraCerts holds the certificate chain
// (intermediates + root) extracted from the P12, embedded in the
// CMS SignedData so a verifier can build a trust path without
// reaching out to the network.
type Signer struct {
	Cert       *x509.Certificate
	PrivateKey *rsa.PrivateKey
	ExtraCerts []*x509.Certificate
}

// LoadCertificate reads a PKCS#12 (.p12 / .pfx) bundle and returns a
// Signer ready to sign. Only RSA keys are accepted; if you need EC
// support, branch on the type assertion below.
func LoadCertificate(path, password string) (*Signer, error) {
	p12Data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	privateKey, certificate, caCerts, err := pkcs12.DecodeChain(p12Data, password)
	if err != nil {
		return nil, fmt.Errorf("failed to decode P12: %w", err)
	}

	rsaKey, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("crypto: private key is not RSA")
	}

	return &Signer{
		Cert:       certificate,
		PrivateKey: rsaKey,
		ExtraCerts: caCerts,
	}, nil
}

// SignPDFHash returns an RSA-PKCS#1-v1.5 raw signature over the
// SHA-256 of pdfContent. Kept for callers (or future code paths)
// that want the raw signature; the canonical archival path is
// SignPDFCMS below.
func (s *Signer) SignPDFHash(pdfContent []byte) ([]byte, error) {
	if s.PrivateKey == nil {
		return nil, errors.New("crypto: signer has no private key")
	}
	hash := sha256.Sum256(pdfContent)
	return rsa.SignPKCS1v15(rand.Reader, s.PrivateKey, crypto.SHA256, hash[:])
}

// SignPDFCMS produces a CMS SignedData (PKCS#7) blob wrapping a
// SHA-256 signature over pdfContent, with the signer's certificate
// and any intermediates embedded.
//
// This is the form Adobe Acrobat / PAdES validators look for. The
// returned bytes go directly into the PDF's /Contents entry under
// the /Sig dictionary; embedding (byte-range + zero-padded slot)
// is handled by internal/export/pdf.go.
//
// PKCS#7 "detached" mode is what PAdES requires: the signed data
// is not embedded in the CMS blob — only the hash is — so verifiers
// hash the PDF bytes referenced by /ByteRange and compare.
func (s *Signer) SignPDFCMS(pdfContent []byte) ([]byte, error) {
	if s.PrivateKey == nil || s.Cert == nil {
		return nil, errors.New("crypto: signer missing key or cert")
	}

	sd, err := pkcs7.NewSignedData(pdfContent)
	if err != nil {
		return nil, fmt.Errorf("crypto: new signed data: %w", err)
	}
	// SHA-256 is the modern default; everything we ship signs at
	// this digest level.
	sd.SetDigestAlgorithm(pkcs7.OIDDigestAlgorithmSHA256)

	if err := sd.AddSigner(s.Cert, s.PrivateKey, pkcs7.SignerInfoConfig{}); err != nil {
		return nil, fmt.Errorf("crypto: add signer: %w", err)
	}

	// Embed the intermediate chain so verifiers can build the
	// trust path without OCSP/AIA fetches.
	for _, ca := range s.ExtraCerts {
		sd.AddCertificate(ca)
	}

	// Detached: the PDF content is hashed but not duplicated inside
	// the CMS blob.
	sd.Detach()

	return sd.Finish()
}
