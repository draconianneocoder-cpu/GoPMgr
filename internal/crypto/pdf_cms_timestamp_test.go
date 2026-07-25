// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package crypto

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/digitorus/pkcs7"
	"github.com/digitorus/timestamp"
)

var (
	testSignatureTimestampOID = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 2, 14}
	testTimestampPolicyOID    = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 55555, 2}
	testExtendedKeyUsageOID   = asn1.ObjectIdentifier{2, 5, 29, 37}
	testTimestampingUsageOID  = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 8}
)

func TestAddSignatureTimestampEmbedsUnsignedTokenWithoutChangingSignature(t *testing.T) {
	t.Parallel()

	content := []byte("%PDF-1.7\n% timestamped detached signature\n")
	signer := newTestCMSSigner(t, "PMForge PAdES-T Signer")
	baselineCMS, err := signer.SignPDFCMS(content)
	if err != nil {
		t.Fatalf("SignPDFCMS() error = %v", err)
	}
	baselineBefore := append([]byte(nil), baselineCMS...)

	imprint, err := SignatureTimestampImprint(baselineCMS)
	if err != nil {
		t.Fatalf("SignatureTimestampImprint() error = %v", err)
	}
	baselineSignedData := decodeTimestampedCMSForTest(t, baselineCMS)
	wantImprint := sha256.Sum256(baselineSignedData.SignerInfos[0].EncryptedDigest)
	if !bytes.Equal(imprint, wantImprint[:]) {
		t.Fatalf("signature imprint = %x, want %x", imprint, wantImprint)
	}

	tokenDER := newTestSignatureTimestampToken(t, imprint, time.Now().UTC())
	tokenBefore := append([]byte(nil), tokenDER...)
	timestampedCMS, err := AddSignatureTimestamp(baselineCMS, tokenDER)
	if err != nil {
		t.Fatalf("AddSignatureTimestamp() error = %v", err)
	}
	if !bytes.Equal(baselineCMS, baselineBefore) {
		t.Fatal("AddSignatureTimestamp() mutated the input CMS")
	}
	if !bytes.Equal(tokenDER, tokenBefore) {
		t.Fatal("AddSignatureTimestamp() mutated the input timestamp token")
	}

	timestampedSignedData := decodeTimestampedCMSForTest(t, timestampedCMS)
	baselineSigner := baselineSignedData.SignerInfos[0]
	timestampedSigner := timestampedSignedData.SignerInfos[0]
	if !bytes.Equal(timestampedSigner.EncryptedDigest, baselineSigner.EncryptedDigest) {
		t.Fatal("timestamp embedding changed the original CMS signature value")
	}
	if !cmsAttributesEqualForTest(timestampedSigner.AuthenticatedAttributes, baselineSigner.AuthenticatedAttributes) {
		t.Fatal("timestamp embedding changed the signed CMS attributes")
	}
	if len(timestampedSigner.UnauthenticatedAttributes) != 1 {
		t.Fatalf(
			"unsigned attribute count = %d, want 1",
			len(timestampedSigner.UnauthenticatedAttributes),
		)
	}
	attribute := timestampedSigner.UnauthenticatedAttributes[0]
	if !attribute.Type.Equal(testSignatureTimestampOID) {
		t.Fatalf("unsigned attribute OID = %s, want %s", attribute.Type, testSignatureTimestampOID)
	}
	if attribute.Value.Tag != asn1.TagSet || !bytes.Equal(attribute.Value.Bytes, tokenDER) {
		t.Fatal("signature-time-stamp attribute does not contain the original token DER")
	}

	parsed, err := pkcs7.Parse(timestampedCMS)
	if err != nil {
		t.Fatalf("parse timestamped CMS: %v", err)
	}
	parsed.Content = content
	if err := parsed.Verify(); err != nil {
		t.Fatalf("timestamped CMS no longer verifies the original detached content: %v", err)
	}
}

func TestAddSignatureTimestampAcceptsMultipleIndependentTokens(t *testing.T) {
	t.Parallel()

	content := []byte("%PDF-1.7\n% multiple timestamp authorities\n")
	baselineCMS, err := newTestCMSSigner(t, "PMForge Multi-TSA Signer").SignPDFCMS(content)
	if err != nil {
		t.Fatalf("SignPDFCMS() error = %v", err)
	}
	imprint, err := SignatureTimestampImprint(baselineCMS)
	if err != nil {
		t.Fatalf("SignatureTimestampImprint() error = %v", err)
	}

	firstToken := newTestSignatureTimestampToken(t, imprint, time.Now().UTC())
	withFirst, err := AddSignatureTimestamp(baselineCMS, firstToken)
	if err != nil {
		t.Fatalf("add first timestamp: %v", err)
	}
	secondToken := newTestSignatureTimestampToken(t, imprint, time.Now().UTC().Add(time.Second))
	withBoth, err := AddSignatureTimestamp(withFirst, secondToken)
	if err != nil {
		t.Fatalf("add second timestamp: %v", err)
	}

	signedData := decodeTimestampedCMSForTest(t, withBoth)
	if got := len(signedData.SignerInfos[0].UnauthenticatedAttributes); got != 2 {
		t.Fatalf("unsigned timestamp count = %d, want 2", got)
	}
	parsed, err := pkcs7.Parse(withBoth)
	if err != nil {
		t.Fatalf("parse twice-timestamped CMS: %v", err)
	}
	parsed.Content = content
	if err := parsed.Verify(); err != nil {
		t.Fatalf("twice-timestamped CMS no longer verifies: %v", err)
	}
}

func TestAddSignatureTimestampRejectsInvalidCMSOrToken(t *testing.T) {
	t.Parallel()

	content := []byte("%PDF-1.7\n% timestamp validation failures\n")
	baselineCMS, err := newTestCMSSigner(t, "PMForge Timestamp Validation Signer").SignPDFCMS(content)
	if err != nil {
		t.Fatalf("SignPDFCMS() error = %v", err)
	}
	imprint, err := SignatureTimestampImprint(baselineCMS)
	if err != nil {
		t.Fatalf("SignatureTimestampImprint() error = %v", err)
	}
	validToken := newTestSignatureTimestampToken(t, imprint, time.Now().UTC())
	otherImprint := sha256.Sum256([]byte("another CMS signature value"))
	mismatchedToken := newTestSignatureTimestampToken(t, otherImprint[:], time.Now().UTC())

	multipleSignerCMS := appendSecondSignerForTest(t, baselineCMS)
	tests := []struct {
		name      string
		cms       []byte
		token     []byte
		wantError string
	}{
		{
			name:      "malformed CMS",
			cms:       []byte("not DER"),
			token:     validToken,
			wantError: "CMS",
		},
		{
			name:      "multiple signers",
			cms:       multipleSignerCMS,
			token:     validToken,
			wantError: "exactly one signer",
		},
		{
			name:      "malformed timestamp token",
			cms:       baselineCMS,
			token:     []byte("not DER"),
			wantError: "timestamp token",
		},
		{
			name:      "mismatched timestamp imprint",
			cms:       baselineCMS,
			token:     mismatchedToken,
			wantError: "message imprint",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := AddSignatureTimestamp(tt.cms, tt.token)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("AddSignatureTimestamp() error = %v, want substring %q", err, tt.wantError)
			}
		})
	}
}

func TestSignatureTimestampImprintRejectsMultipleSigners(t *testing.T) {
	t.Parallel()

	baselineCMS, err := newTestCMSSigner(t, "PMForge Imprint Signer").SignPDFCMS([]byte("content"))
	if err != nil {
		t.Fatalf("SignPDFCMS() error = %v", err)
	}

	_, err = SignatureTimestampImprint(appendSecondSignerForTest(t, baselineCMS))
	if err == nil || !strings.Contains(err.Error(), "exactly one signer") {
		t.Fatalf("SignatureTimestampImprint() error = %v, want signer-count rejection", err)
	}
}

func newTestSignatureTimestampToken(t *testing.T, imprint []byte, generatedAt time.Time) []byte {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate TSA key: %v", err)
	}
	ekuValue, err := asn1.Marshal([]asn1.ObjectIdentifier{testTimestampingUsageOID})
	if err != nil {
		t.Fatalf("marshal TSA extended key usage: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(generatedAt.UnixNano()),
		Subject:      pkix.Name{CommonName: "PMForge Test Timestamp Authority"},
		NotBefore:    generatedAt.Add(-time.Hour),
		NotAfter:     generatedAt.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtraExtensions: []pkix.Extension{{
			Id:       testExtendedKeyUsageOID,
			Critical: true,
			Value:    ekuValue,
		}},
	}
	certificateDER, err := x509.CreateCertificate(
		rand.Reader,
		template,
		template,
		&privateKey.PublicKey,
		privateKey,
	)
	if err != nil {
		t.Fatalf("create TSA certificate: %v", err)
	}
	certificate, err := x509.ParseCertificate(certificateDER)
	if err != nil {
		t.Fatalf("parse TSA certificate: %v", err)
	}

	responseDER, err := (&timestamp.Timestamp{
		HashAlgorithm:     crypto.SHA256,
		HashedMessage:     append([]byte(nil), imprint...),
		Time:              generatedAt,
		Policy:            testTimestampPolicyOID,
		Nonce:             big.NewInt(17),
		AddTSACertificate: true,
	}).CreateResponseWithOpts(certificate, privateKey, crypto.SHA256)
	if err != nil {
		t.Fatalf("create timestamp response: %v", err)
	}
	parsed, err := timestamp.ParseResponse(responseDER)
	if err != nil {
		t.Fatalf("parse generated timestamp response: %v", err)
	}
	return append([]byte(nil), parsed.RawToken...)
}

func decodeTimestampedCMSForTest(t *testing.T, cmsDER []byte) cmsSignedData {
	t.Helper()

	var outer cmsContentInfo
	rest, err := asn1.Unmarshal(cmsDER, &outer)
	if err != nil {
		t.Fatalf("parse CMS ContentInfo: %v", err)
	}
	if len(rest) != 0 {
		t.Fatalf("CMS ContentInfo has %d trailing bytes", len(rest))
	}

	var signedData cmsSignedData
	rest, err = asn1.Unmarshal(outer.Content.Bytes, &signedData)
	if err != nil {
		t.Fatalf("parse CMS SignedData: %v", err)
	}
	if len(rest) != 0 {
		t.Fatalf("CMS SignedData has %d trailing bytes", len(rest))
	}
	return signedData
}

func appendSecondSignerForTest(t *testing.T, cmsDER []byte) []byte {
	t.Helper()

	signedData := decodeTimestampedCMSForTest(t, cmsDER)
	signedData.SignerInfos = append(signedData.SignerInfos, signedData.SignerInfos[0])
	innerDER, err := asn1.Marshal(signedData)
	if err != nil {
		t.Fatalf("marshal CMS with second signer: %v", err)
	}
	out, err := asn1.Marshal(cmsContentInfo{
		ContentType: pkcs7.OIDSignedData,
		Content: asn1.RawValue{
			Class:      asn1.ClassContextSpecific,
			Tag:        0,
			IsCompound: true,
			Bytes:      innerDER,
		},
	})
	if err != nil {
		t.Fatalf("marshal CMS ContentInfo with second signer: %v", err)
	}
	return out
}

func cmsAttributesEqualForTest(left, right []cmsAttribute) bool {
	leftDER, leftErr := asn1.Marshal(left)
	rightDER, rightErr := asn1.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftDER, rightDER)
}
