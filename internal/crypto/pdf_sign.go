// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package crypto

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"

	"github.com/digitorus/pkcs7"
	"golang.org/x/crypto/pkcs12"
)

var (
	oidAttributeSigningCertificateV2 = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 2, 47}
	oidDigestAlgorithmSHA256         = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}
)

type signingCertificateV2 struct {
	Certs []essCertIDV2
}

type essCertIDV2 struct {
	HashAlgorithm pkix.AlgorithmIdentifier
	CertHash      []byte
	IssuerSerial  essIssuerSerial
}

type essIssuerSerial struct {
	Issuer       []asn1.RawValue
	SerialNumber *big.Int
}

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

// maxP12ImportSize bounds the user-selected PKCS#12 (.p12/.pfx) certificate
// bundle GoPMgr will read into memory before parsing. It's a var, not a
// const, so tests can shrink it and prove the LimitReader bound actually
// holds rather than only exercising the early os.Stat-based refusal. The
// value is generous relative to real PKCS#12 bundles (typically a few KB
// to a few hundred KB, even with a full issuing chain bundled in) rather
// than tightly measured -- the goal is bounding worst-case memory use on a
// malformed or oversized import, not fitting realistic bundle sizes
// exactly. Mirrors the size-capping pattern app_charts.go applies to MSPDI
// schedule imports (maxMSPDIImportSize).
var maxP12ImportSize int64 = 16 << 20 // 16 MiB

// p12ImportTooLargeErr is shared by both size guards below (the fast
// os.Stat check and the io.LimitReader-backed post-read check) so a caller
// sees the same actionable message regardless of which one catches it.
func p12ImportTooLargeErr(path string) error {
	return fmt.Errorf(
		"certificate bundle %q exceeds GoPMgr's %d MiB PKCS#12 import limit; this is "+
			"far larger than any real signing certificate bundle and was refused "+
			"rather than read into memory", path, maxP12ImportSize>>20)
}

// LoadCertificate reads a PKCS#12 (.p12 / .pfx) bundle and returns a
// Signer ready to sign. Only RSA keys are accepted; if you need EC
// support, branch on the type assertion in parseP12PrivateKey below.
//
// Uses pkcs12.ToPEM, not pkcs12.Decode. Decode's own doc comment says it
// "assumes there is only one certificate and only one private key ...
// if there are more use ToPEM instead" -- and a real commercially-issued
// signing certificate is routinely exported with its issuing chain
// bundled into the same file, which Decode rejects outright with
// "expected exactly two safe bags in the PFX PDU". That made every such
// certificate fail to load here at all, not just lose its chain. ToPEM has no
// such 2-bag limit; parseP12PrivateKey/splitLeafCertificate below do the
// classification pkcs12.Decode used to do internally.
func LoadCertificate(path, password string) (*Signer, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("certificate bundle %q is not a regular file", path)
	}
	if info.Size() > maxP12ImportSize {
		return nil, p12ImportTooLargeErr(path)
	}
	f, err := os.Open(path) // #nosec G304 -- user-selected PKCS#12 certificate bundle path; size-checked above.
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	// Bound the actual read regardless of what os.Stat reported above
	// (TOCTOU, or a path whose Stat().Size() doesn't reflect what a
	// read actually returns).
	p12Data, err := io.ReadAll(io.LimitReader(f, maxP12ImportSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(p12Data)) > maxP12ImportSize {
		return nil, p12ImportTooLargeErr(path)
	}

	blocks, err := pkcs12.ToPEM(p12Data, password)
	if err != nil {
		return nil, fmt.Errorf("failed to decode P12: %w", err)
	}

	rsaKey, certs, err := parseP12Blocks(blocks)
	if err != nil {
		return nil, err
	}

	leaf, extraCerts, err := splitLeafCertificate(rsaKey, certs)
	if err != nil {
		return nil, err
	}

	return &Signer{
		Cert:       leaf,
		PrivateKey: rsaKey,
		ExtraCerts: extraCerts,
	}, nil
}

// parseP12Blocks scans the PEM blocks pkcs12.ToPEM produces for exactly
// one private key and every certificate present. Per ToPEM's own doc
// comment, "PRIVATE KEY" block bytes are PKCS#1-encoded for RSA keys and
// SEC 1-encoded for EC keys -- the block Type alone can't tell them
// apart, so both parsers are tried, RSA first: RSA is this package's
// only supported case, so it's checked first, and the EC attempt exists
// solely so a non-RSA key gets the specific "not RSA" error below
// instead of a generic "could not parse" one that would leave a caller
// guessing why a key that opened fine in every other tool is rejected
// here. Only RSA keys are accepted, matching this package's existing
// contract.
func parseP12Blocks(blocks []*pem.Block) (*rsa.PrivateKey, []*x509.Certificate, error) {
	var (
		rsaKey *rsa.PrivateKey
		certs  []*x509.Certificate
	)

	for _, block := range blocks {
		switch block.Type {
		case "PRIVATE KEY":
			if rsaKey != nil {
				return nil, nil, errors.New("crypto: P12 contains more than one private key")
			}
			if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
				rsaKey = key
				continue
			}
			if _, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
				return nil, nil, errors.New("crypto: private key is not RSA")
			}
			return nil, nil, errors.New("crypto: could not parse private key")
		case "CERTIFICATE":
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, nil, fmt.Errorf("crypto: parse certificate: %w", err)
			}
			certs = append(certs, cert)
		}
	}

	if rsaKey == nil {
		return nil, nil, errors.New("crypto: P12 contains no private key")
	}
	if len(certs) == 0 {
		return nil, nil, errors.New("crypto: P12 contains no certificate")
	}
	return rsaKey, certs, nil
}

// splitLeafCertificate picks the signer's own certificate out of certs --
// the one whose public key matches key -- and returns the rest as the
// issuing chain (ExtraCerts). Matching is by public-key identity, not by
// the P12's optional localKeyId attribute: some exporters omit it, and a
// wrong guess here would silently put an intermediate in Cert and the
// real leaf in ExtraCerts, producing a CMS signature validators reject.
// Public-key identity can't be fooled by missing metadata; it can only
// fail closed if genuinely no certificate in the bundle belongs to key.
func splitLeafCertificate(key *rsa.PrivateKey, certs []*x509.Certificate) (*x509.Certificate, []*x509.Certificate, error) {
	for i, cert := range certs {
		certKey, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			continue
		}
		if certKey.E == key.E && certKey.N.Cmp(key.N) == 0 {
			extraCerts := make([]*x509.Certificate, 0, len(certs)-1)
			extraCerts = append(extraCerts, certs[:i]...)
			extraCerts = append(extraCerts, certs[i+1:]...)
			return cert, extraCerts, nil
		}
	}
	return nil, nil, errors.New("crypto: no certificate in the P12 matches the private key")
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
// is handled by internal/pdfmeta.InjectPAdESSignature, reached via
// internal/signing.ApplyPAdES.
//
// PKCS#7 "detached" mode is what PAdES requires: the signed data
// is not embedded in the CMS blob — only the hash is — so verifiers
// hash the PDF bytes referenced by /ByteRange and compare.
func (s *Signer) SignPDFCMS(pdfContent []byte) ([]byte, error) {
	if s.PrivateKey == nil || s.Cert == nil {
		return nil, errors.New("crypto: signer missing key or cert")
	}

	return signDetachedPAdESCMS(pdfContent, s.Cert, s.PrivateKey, s.ExtraCerts)
}

func signingCertificateV2Attribute(cert *x509.Certificate) pkcs7.Attribute {
	certHash := sha256.Sum256(cert.Raw)
	return pkcs7.Attribute{
		Type: oidAttributeSigningCertificateV2,
		Value: signingCertificateV2{
			Certs: []essCertIDV2{{
				HashAlgorithm: pkix.AlgorithmIdentifier{Algorithm: oidDigestAlgorithmSHA256},
				CertHash:      certHash[:],
				IssuerSerial: essIssuerSerial{
					Issuer: []asn1.RawValue{{
						Class:      asn1.ClassContextSpecific,
						Tag:        4,
						IsCompound: true,
						Bytes:      cert.RawIssuer,
					}},
					SerialNumber: cert.SerialNumber,
				},
			}},
		},
	}
}
