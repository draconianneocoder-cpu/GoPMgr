// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package rfc3161

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"io"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/digitorus/timestamp"
)

var (
	testPolicyOID   = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 55555, 1}
	extKeyUsageOID  = asn1.ObjectIdentifier{2, 5, 29, 37}
	timeStampingOID = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 8}
	serverAuthOID   = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 1}
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestClientTimestampValidatesResponseAndRequest(t *testing.T) {
	t.Parallel()

	digest := sha256.Sum256([]byte("signed CMS value"))
	nonceBytes := []byte{
		0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
		0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10,
	}
	expectedNonce := new(big.Int).SetBytes(nonceBytes)
	generatedAt := time.Date(2026, time.July, 25, 12, 30, 0, 0, time.UTC)
	responseDER, tsaCertificate := createTimestampResponse(t, digest[:], expectedNonce, generatedAt)

	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost {
			t.Errorf("request method = %q, want POST", request.Method)
		}
		if got := request.Header.Get("Content-Type"); got != QueryMediaType {
			t.Errorf("Content-Type = %q, want %q", got, QueryMediaType)
		}
		if got := request.Header.Get("Accept"); got != ReplyMediaType {
			t.Errorf("Accept = %q, want %q", got, ReplyMediaType)
		}

		requestDER, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("read timestamp request: %v", err)
		}
		parsed, err := timestamp.ParseRequest(requestDER)
		if err != nil {
			t.Fatalf("parse timestamp request: %v", err)
		}
		if parsed.HashAlgorithm != crypto.SHA256 {
			t.Errorf("request hash = %v, want SHA-256", parsed.HashAlgorithm)
		}
		if !bytes.Equal(parsed.HashedMessage, digest[:]) {
			t.Errorf("request imprint = %x, want %x", parsed.HashedMessage, digest)
		}
		if !parsed.Certificates {
			t.Error("request did not require the TSA certificate")
		}
		if parsed.Nonce == nil || parsed.Nonce.Cmp(expectedNonce) != 0 {
			t.Errorf("request nonce = %v, want %v", parsed.Nonce, expectedNonce)
		}
		if !parsed.TSAPolicyOID.Equal(testPolicyOID) {
			t.Errorf("request policy = %s, want %s", parsed.TSAPolicyOID, testPolicyOID)
		}

		return timestampHTTPResponse(http.StatusOK, ReplyMediaType, responseDER), nil
	})

	token, err := (Client{
		Transport: transport,
		Random:    bytes.NewReader(nonceBytes),
		PolicyOID: testPolicyOID,
	}).Timestamp(context.Background(), "https://tsa.example.test/v1", digest[:])
	if err != nil {
		t.Fatalf("Timestamp() error = %v", err)
	}

	if token.GeneratedAt != generatedAt {
		t.Errorf("GeneratedAt = %v, want %v", token.GeneratedAt, generatedAt)
	}
	if !token.PolicyOID.Equal(testPolicyOID) {
		t.Errorf("PolicyOID = %s, want %s", token.PolicyOID, testPolicyOID)
	}
	if token.SignerCertificate == nil || token.SignerCertificate.SerialNumber.Cmp(tsaCertificate.SerialNumber) != 0 {
		t.Errorf("SignerCertificate serial = %v, want %v", token.SignerCertificate, tsaCertificate.SerialNumber)
	}
	if token.TrustStatus != TrustNotEvaluated {
		t.Errorf("TrustStatus = %q, want %q", token.TrustStatus, TrustNotEvaluated)
	}
	if len(token.DER) == 0 {
		t.Error("timestamp token DER is empty")
	}
}

func TestClientTimestampVerifiesConfiguredTrustRoots(t *testing.T) {
	t.Parallel()

	digest := sha256.Sum256([]byte("trusted timestamp"))
	nonceBytes := bytes.Repeat([]byte{0x02}, NonceBytes)
	responseDER, tsaCertificate := createTimestampResponse(
		t,
		digest[:],
		new(big.Int).SetBytes(nonceBytes),
		time.Date(2026, time.July, 25, 13, 0, 0, 0, time.UTC),
	)
	roots := x509.NewCertPool()
	roots.AddCert(tsaCertificate)

	client := Client{
		Transport:  staticResponseTransport(responseDER),
		Random:     bytes.NewReader(nonceBytes),
		PolicyOID:  testPolicyOID,
		TrustRoots: roots,
	}
	token, err := client.Timestamp(context.Background(), "https://tsa.example.test", digest[:])
	if err != nil {
		t.Fatalf("Timestamp() error = %v", err)
	}
	if token.TrustStatus != TrustVerified {
		t.Errorf("TrustStatus = %q, want %q", token.TrustStatus, TrustVerified)
	}

	_, err = (Client{
		Transport:  staticResponseTransport(responseDER),
		Random:     bytes.NewReader(nonceBytes),
		PolicyOID:  testPolicyOID,
		TrustRoots: x509.NewCertPool(),
	}).Timestamp(context.Background(), "https://tsa.example.test", digest[:])
	if err == nil || !strings.Contains(err.Error(), "trust chain") {
		t.Fatalf("Timestamp() error = %v, want trust-chain rejection", err)
	}
}

func TestValidateTokenVerifiesSignatureImprintWithoutRequestState(t *testing.T) {
	t.Parallel()

	digest := sha256.Sum256([]byte("detached signature value"))
	responseDER, _ := createTimestampResponse(
		t,
		digest[:],
		big.NewInt(7),
		time.Date(2026, time.July, 25, 15, 0, 0, 0, time.UTC),
	)
	_, tokenDER, err := parseResponseEnvelope(responseDER)
	if err != nil {
		t.Fatalf("parse timestamp response envelope: %v", err)
	}

	token, err := ValidateToken(tokenDER, digest[:])
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}
	if token.TrustStatus != TrustNotEvaluated {
		t.Errorf("TrustStatus = %q, want %q", token.TrustStatus, TrustNotEvaluated)
	}

	otherDigest := sha256.Sum256([]byte("other signature value"))
	if _, err := ValidateToken(tokenDER, otherDigest[:]); err == nil ||
		!strings.Contains(err.Error(), "message imprint") {
		t.Fatalf("ValidateToken() mismatch error = %v, want message-imprint rejection", err)
	}
}

func TestClientTimestampAcceptsGrantedWithModifications(t *testing.T) {
	t.Parallel()

	digest := sha256.Sum256([]byte("modified grant"))
	nonceBytes := bytes.Repeat([]byte{0x04}, NonceBytes)
	responseDER, _ := createTimestampResponse(
		t,
		digest[:],
		new(big.Int).SetBytes(nonceBytes),
		time.Date(2026, time.July, 25, 13, 30, 0, 0, time.UTC),
	)
	var envelope responseEnvelope
	if _, err := asn1.Unmarshal(responseDER, &envelope); err != nil {
		t.Fatalf("parse timestamp response envelope: %v", err)
	}
	envelope.Status.Status = 1
	responseDER, err := asn1.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal modified timestamp response: %v", err)
	}

	_, err = (Client{
		Transport: staticResponseTransport(responseDER),
		Random:    bytes.NewReader(nonceBytes),
		PolicyOID: testPolicyOID,
	}).Timestamp(context.Background(), "https://tsa.example.test", digest[:])
	if err != nil {
		t.Fatalf("Timestamp() error = %v", err)
	}
}

func TestClientTimestampRejectsInvalidInputsAndResponses(t *testing.T) {
	t.Parallel()

	digest := sha256.Sum256([]byte("timestamp validation"))
	nonceBytes := bytes.Repeat([]byte{0x03}, NonceBytes)
	validResponse, _ := createTimestampResponse(
		t,
		digest[:],
		new(big.Int).SetBytes(nonceBytes),
		time.Date(2026, time.July, 25, 14, 0, 0, 0, time.UTC),
	)
	wrongDigest := sha256.Sum256([]byte("different imprint"))
	wrongImprintResponse, _ := createTimestampResponse(
		t,
		wrongDigest[:],
		new(big.Int).SetBytes(nonceBytes),
		time.Date(2026, time.July, 25, 14, 1, 0, 0, time.UTC),
	)
	wrongNonceResponse, _ := createTimestampResponse(
		t,
		digest[:],
		big.NewInt(999),
		time.Date(2026, time.July, 25, 14, 2, 0, 0, time.UTC),
	)
	rejectedResponse, err := timestamp.CreateErrorResponse(timestamp.Rejection, timestamp.BadAlgorithm)
	if err != nil {
		t.Fatalf("create rejected response: %v", err)
	}
	tamperedResponse := append([]byte(nil), validResponse...)
	tamperedResponse[len(tamperedResponse)-1] ^= 0x01
	wrongContentTypeResponse := replaceFirstOID(t, validResponse, oidTSTInfo, asn1.ObjectIdentifier{
		1, 2, 840, 113549, 1, 9, 16, 1, 5,
	})

	tests := []struct {
		name      string
		endpoint  string
		digest    []byte
		transport http.RoundTripper
		maxBytes  int64
		wantError string
	}{
		{
			name:      "non HTTPS endpoint",
			endpoint:  "http://tsa.example.test",
			digest:    digest[:],
			transport: panicTransport(t),
			wantError: "HTTPS",
		},
		{
			name:      "wrong digest length",
			endpoint:  "https://tsa.example.test",
			digest:    digest[:sha256.Size-1],
			transport: panicTransport(t),
			wantError: "32 bytes",
		},
		{
			name:      "HTTP error",
			endpoint:  "https://tsa.example.test",
			digest:    digest[:],
			transport: responseTransport(http.StatusBadGateway, ReplyMediaType, nil),
			wantError: "HTTP 502",
		},
		{
			name:      "wrong media type",
			endpoint:  "https://tsa.example.test",
			digest:    digest[:],
			transport: responseTransport(http.StatusOK, "application/octet-stream", validResponse),
			wantError: ReplyMediaType,
		},
		{
			name:      "oversized response",
			endpoint:  "https://tsa.example.test",
			digest:    digest[:],
			transport: staticResponseTransport(validResponse),
			maxBytes:  8,
			wantError: "exceeds",
		},
		{
			name:      "TSA rejection",
			endpoint:  "https://tsa.example.test",
			digest:    digest[:],
			transport: staticResponseTransport(rejectedResponse),
			wantError: "rejected",
		},
		{
			name:      "message imprint mismatch",
			endpoint:  "https://tsa.example.test",
			digest:    digest[:],
			transport: staticResponseTransport(wrongImprintResponse),
			wantError: "message imprint",
		},
		{
			name:      "nonce mismatch",
			endpoint:  "https://tsa.example.test",
			digest:    digest[:],
			transport: staticResponseTransport(wrongNonceResponse),
			wantError: "nonce",
		},
		{
			name:      "tampered CMS signature",
			endpoint:  "https://tsa.example.test",
			digest:    digest[:],
			transport: staticResponseTransport(tamperedResponse),
			wantError: "validate RFC 3161 token",
		},
		{
			name:      "wrong CMS content type",
			endpoint:  "https://tsa.example.test",
			digest:    digest[:],
			transport: staticResponseTransport(wrongContentTypeResponse),
			wantError: "content type",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := (Client{
				Transport:        tt.transport,
				Random:           bytes.NewReader(nonceBytes),
				PolicyOID:        testPolicyOID,
				MaxResponseBytes: tt.maxBytes,
			}).Timestamp(context.Background(), tt.endpoint, tt.digest)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("Timestamp() error = %v, want substring %q", err, tt.wantError)
			}
		})
	}
}

func TestClientTimestampRejectsInvalidTSACertificateUsage(t *testing.T) {
	t.Parallel()

	digest := sha256.Sum256([]byte("TSA certificate usage"))
	nonceBytes := bytes.Repeat([]byte{0x05}, NonceBytes)
	tests := []struct {
		name      string
		critical  bool
		usages    []asn1.ObjectIdentifier
		wantError string
	}{
		{
			name:      "non critical timestamping usage",
			critical:  false,
			usages:    []asn1.ObjectIdentifier{timeStampingOID},
			wantError: "must be critical",
		},
		{
			name:      "additional server authentication usage",
			critical:  true,
			usages:    []asn1.ObjectIdentifier{timeStampingOID, serverAuthOID},
			wantError: "restricted to timestamping",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			responseDER, _ := createTimestampResponseWithEKU(
				t,
				digest[:],
				new(big.Int).SetBytes(nonceBytes),
				time.Date(2026, time.July, 25, 14, 30, 0, 0, time.UTC),
				tt.critical,
				tt.usages,
			)
			_, err := (Client{
				Transport: staticResponseTransport(responseDER),
				Random:    bytes.NewReader(nonceBytes),
				PolicyOID: testPolicyOID,
			}).Timestamp(context.Background(), "https://tsa.example.test", digest[:])
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("Timestamp() error = %v, want substring %q", err, tt.wantError)
			}
		})
	}
}

func TestClientTimestampDoesNotFollowRedirects(t *testing.T) {
	t.Parallel()

	digest := sha256.Sum256([]byte("redirect refusal"))
	nonceBytes := bytes.Repeat([]byte{0x06}, NonceBytes)
	requestCount := 0
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestCount++
		if requestCount > 1 {
			t.Fatal("timestamp client followed a redirect")
		}
		return &http.Response{
			StatusCode: http.StatusFound,
			Header:     http.Header{"Location": []string{"https://other.example.test"}},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    request,
		}, nil
	})

	_, err := (Client{
		Transport: transport,
		Random:    bytes.NewReader(nonceBytes),
	}).Timestamp(context.Background(), "https://tsa.example.test", digest[:])
	if err == nil || !strings.Contains(err.Error(), "HTTP 302") {
		t.Fatalf("Timestamp() error = %v, want HTTP 302", err)
	}
	if requestCount != 1 {
		t.Fatalf("request count = %d, want 1", requestCount)
	}
}

func TestClientTimestampRejectsInvalidNonceEntropy(t *testing.T) {
	t.Parallel()

	digest := sha256.Sum256([]byte("nonce entropy"))
	tests := []struct {
		name      string
		random    io.Reader
		wantError string
	}{
		{name: "short read", random: bytes.NewReader([]byte{0x01}), wantError: "nonce"},
		{name: "all zeros", random: bytes.NewReader(make([]byte, NonceBytes)), wantError: "all zeros"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := (Client{
				Transport: panicTransport(t),
				Random:    tt.random,
			}).Timestamp(context.Background(), "https://tsa.example.test", digest[:])
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("Timestamp() error = %v, want substring %q", err, tt.wantError)
			}
		})
	}
}

func createTimestampResponse(
	t *testing.T,
	digest []byte,
	nonce *big.Int,
	generatedAt time.Time,
) ([]byte, *x509.Certificate) {
	t.Helper()
	return createTimestampResponseWithEKU(
		t,
		digest,
		nonce,
		generatedAt,
		true,
		[]asn1.ObjectIdentifier{timeStampingOID},
	)
}

func createTimestampResponseWithEKU(
	t *testing.T,
	digest []byte,
	nonce *big.Int,
	generatedAt time.Time,
	critical bool,
	usages []asn1.ObjectIdentifier,
) ([]byte, *x509.Certificate) {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate TSA private key: %v", err)
	}
	ekuValue, err := asn1.Marshal(usages)
	if err != nil {
		t.Fatalf("marshal TSA extended key usage: %v", err)
	}
	wallClock := time.Now().UTC()
	notBefore := generatedAt
	notAfter := generatedAt
	if wallClock.Before(notBefore) {
		notBefore = wallClock
	}
	if wallClock.After(notAfter) {
		notAfter = wallClock
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(42),
		Subject:      pkix.Name{CommonName: "GoPMgr Test TSA"},
		// The timestamp package signs its CMS attributes at wall-clock time,
		// while TSTInfo uses generatedAt. Deriving the validity window from both
		// prevents a fixed protocol timestamp from becoming an expiring test
		// fixture as the real calendar advances.
		NotBefore: notBefore.Add(-24 * time.Hour),
		NotAfter:  notAfter.Add(24 * time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature,
		ExtraExtensions: []pkix.Extension{{
			Id:       extKeyUsageOID,
			Critical: critical,
			Value:    ekuValue,
		}},
	}
	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("create TSA certificate: %v", err)
	}
	certificate, err := x509.ParseCertificate(certificateDER)
	if err != nil {
		t.Fatalf("parse TSA certificate: %v", err)
	}

	responseDER, err := (&timestamp.Timestamp{
		HashAlgorithm:     crypto.SHA256,
		HashedMessage:     append([]byte(nil), digest...),
		Time:              generatedAt,
		Policy:            testPolicyOID,
		Nonce:             new(big.Int).Set(nonce),
		AddTSACertificate: true,
	}).CreateResponseWithOpts(certificate, privateKey, crypto.SHA256)
	if err != nil {
		t.Fatalf("create timestamp response: %v", err)
	}
	return responseDER, certificate
}

func replaceFirstOID(
	t *testing.T,
	der []byte,
	oldOID asn1.ObjectIdentifier,
	newOID asn1.ObjectIdentifier,
) []byte {
	t.Helper()

	oldDER, err := asn1.Marshal(oldOID)
	if err != nil {
		t.Fatalf("marshal old OID: %v", err)
	}
	newDER, err := asn1.Marshal(newOID)
	if err != nil {
		t.Fatalf("marshal new OID: %v", err)
	}
	if len(oldDER) != len(newDER) {
		t.Fatal("test OID replacements must have equal DER lengths")
	}
	index := bytes.Index(der, oldDER)
	if index < 0 {
		t.Fatalf("OID %s not found in timestamp response", oldOID)
	}
	mutated := append([]byte(nil), der...)
	copy(mutated[index:index+len(newDER)], newDER)
	return mutated
}

func staticResponseTransport(body []byte) http.RoundTripper {
	return responseTransport(http.StatusOK, ReplyMediaType, body)
}

func responseTransport(status int, mediaType string, body []byte) http.RoundTripper {
	return roundTripFunc(func(*http.Request) (*http.Response, error) {
		return timestampHTTPResponse(status, mediaType, body), nil
	})
}

func timestampHTTPResponse(status int, mediaType string, body []byte) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{mediaType}},
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
}

func panicTransport(t *testing.T) http.RoundTripper {
	t.Helper()
	return roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("transport called for locally rejected input")
		return nil, nil
	})
}
