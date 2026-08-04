// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package signing

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/digitorus/timestamp"

	pmcrypto "gopmgr/internal/crypto"
	"gopmgr/internal/rfc3161"
)

var (
	testTSPolicyOID       = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 55555, 7}
	testTSExtendedKeyOID  = asn1.ObjectIdentifier{2, 5, 29, 37}
	testTimestampUsageOID = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 8}
)

type timestampRequesterFunc func(context.Context, string, []byte) (rfc3161.Token, error)

func (f timestampRequesterFunc) Timestamp(
	ctx context.Context,
	endpoint string,
	imprint []byte,
) (rfc3161.Token, error) {
	return f(ctx, endpoint, imprint)
}

func TestPrepareTimestampRejectsUnsafeOrIncompleteConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		config    TimestampConfig
		wantError string
	}{
		{
			name:      "enabled without endpoint",
			config:    TimestampConfig{Enabled: true},
			wantError: "endpoint is required",
		},
		{
			name: "plain HTTP",
			config: TimestampConfig{
				Enabled:  true,
				Endpoint: "http://tsa.example.test",
			},
			wantError: "HTTPS",
		},
		{
			name: "embedded credentials",
			config: TimestampConfig{
				Enabled:  true,
				Endpoint: "https://alice:secret@tsa.example.test",
			},
			wantError: "credentials",
		},
		{
			name: "query credentials",
			config: TimestampConfig{
				Enabled:  true,
				Endpoint: "https://tsa.example.test?api_key=secret",
			},
			wantError: "query",
		},
		{
			name: "invalid policy OID",
			config: TimestampConfig{
				Enabled:   true,
				Endpoint:  "https://tsa.example.test",
				PolicyOID: "not-an-oid",
			},
			wantError: "policy OID",
		},
		{
			name: "missing trust root",
			config: TimestampConfig{
				Enabled:           true,
				Endpoint:          "https://tsa.example.test",
				TrustRootCertPath: "/missing/tsa-root.pem",
			},
			wantError: "trust root",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := PrepareTimestamp(tt.config)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("PrepareTimestamp() error = %v, want substring %q", err, tt.wantError)
			}
		})
	}
}

func TestPrepareTimestampLoadsPolicyAndTrustRoot(t *testing.T) {
	t.Parallel()

	_, certificate := newPAdESTestSigner(t)
	rootPath := filepath.Join(t.TempDir(), "tsa-root.pem")
	rootPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Raw})
	if err := os.WriteFile(rootPath, rootPEM, 0o600); err != nil {
		t.Fatalf("write trust root: %v", err)
	}

	prepared, err := PrepareTimestamp(TimestampConfig{
		Enabled:           true,
		Endpoint:          " https://tsa.example.test/timestamp ",
		PolicyOID:         " 1.3.6.1.4.1.55555.7 ",
		TrustRootCertPath: rootPath,
	})
	if err != nil {
		t.Fatalf("PrepareTimestamp() error = %v", err)
	}
	if prepared.Endpoint != "https://tsa.example.test/timestamp" {
		t.Fatalf("Endpoint = %q, want trimmed endpoint", prepared.Endpoint)
	}
	client, ok := prepared.Requester.(rfc3161.Client)
	if !ok {
		t.Fatalf("Requester type = %T, want rfc3161.Client", prepared.Requester)
	}
	if !client.PolicyOID.Equal(testTSPolicyOID) {
		t.Fatalf("PolicyOID = %s, want %s", client.PolicyOID, testTSPolicyOID)
	}
	if client.TrustRoots == nil {
		t.Fatal("TrustRoots = nil, want configured pool")
	}
}

func TestPrepareTimestampDisabledDoesNotRequireAvailableTrustRoot(t *testing.T) {
	t.Parallel()

	prepared, err := PrepareTimestamp(TimestampConfig{
		Enabled:           false,
		Endpoint:          "https://tsa.example.test/timestamp",
		PolicyOID:         "1.3.6.1.4.1.55555.7",
		TrustRootCertPath: "/disconnected-volume/tsa-root.pem",
	})
	if err != nil {
		t.Fatalf("PrepareTimestamp() error = %v", err)
	}
	if prepared != nil {
		t.Fatalf("PrepareTimestamp() = %+v, want nil while disabled", prepared)
	}
}

func TestApplyPAdESAddsTimestampAndReportsTrust(t *testing.T) {
	t.Parallel()

	signer, _ := newPAdESTestSigner(t)
	generatedAt := time.Now().UTC().Truncate(time.Second)
	requester := timestampRequesterFunc(func(_ context.Context, endpoint string, imprint []byte) (rfc3161.Token, error) {
		if endpoint != "https://tsa.example.test/timestamp" {
			t.Fatalf("endpoint = %q", endpoint)
		}
		tokenDER := newPAdESTestTimestampToken(t, imprint, generatedAt)
		return rfc3161.Token{
			DER:         tokenDER,
			GeneratedAt: generatedAt,
			PolicyOID:   testTSPolicyOID,
			TrustStatus: rfc3161.TrustVerified,
		}, nil
	})

	out, result, err := ApplyPAdES(
		context.Background(),
		[]byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\nxref\n0 2\n0000000000 65535 f \n0000000009 00000 n \ntrailer\n<< /Size 2 /Root 1 0 R >>\nstartxref\n45\n%%EOF\n"),
		signer,
		&PreparedTimestamp{
			Endpoint:  "https://tsa.example.test/timestamp",
			Requester: requester,
		},
	)
	if err != nil {
		t.Fatalf("ApplyPAdES() error = %v", err)
	}
	if len(out) == 0 {
		t.Fatal("ApplyPAdES() returned an empty PDF")
	}
	if result.Format != PAdESBaselineT || result.TrustStatus != rfc3161.TrustVerified {
		t.Fatalf("result = %+v, want Baseline T with verified trust", result)
	}
	if !result.TimestampGeneratedAt.Equal(generatedAt) {
		t.Fatalf("TimestampGeneratedAt = %v, want %v", result.TimestampGeneratedAt, generatedAt)
	}
}

func TestApplyPAdESUsesBaselineBWhenTimestampingIsDisabled(t *testing.T) {
	t.Parallel()

	signer, _ := newPAdESTestSigner(t)
	out, result, err := ApplyPAdES(
		context.Background(),
		[]byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\nxref\n0 2\n0000000000 65535 f \n0000000009 00000 n \ntrailer\n<< /Size 2 /Root 1 0 R >>\nstartxref\n45\n%%EOF\n"),
		signer,
		nil,
	)
	if err != nil {
		t.Fatalf("ApplyPAdES() error = %v", err)
	}
	if len(out) == 0 {
		t.Fatal("ApplyPAdES() returned an empty PDF")
	}
	if result.Format != PAdESBaselineB {
		t.Fatalf("Format = %q, want %q", result.Format, PAdESBaselineB)
	}
	if result.TrustStatus != "" || !result.TimestampGeneratedAt.IsZero() {
		t.Fatalf("Baseline B result contains timestamp metadata: %+v", result)
	}
}

func TestApplyPAdESFailsClosedWhenTimestampRequestFails(t *testing.T) {
	t.Parallel()

	signer, _ := newPAdESTestSigner(t)
	requester := timestampRequesterFunc(func(context.Context, string, []byte) (rfc3161.Token, error) {
		return rfc3161.Token{}, errors.New("TSA unavailable")
	})

	out, result, err := ApplyPAdES(
		context.Background(),
		[]byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\nxref\n0 2\n0000000000 65535 f \n0000000009 00000 n \ntrailer\n<< /Size 2 /Root 1 0 R >>\nstartxref\n45\n%%EOF\n"),
		signer,
		&PreparedTimestamp{
			Endpoint:  "https://tsa.example.test/timestamp",
			Requester: requester,
		},
	)
	if err == nil || !strings.Contains(err.Error(), "TSA unavailable") {
		t.Fatalf("ApplyPAdES() error = %v, want timestamp failure", err)
	}
	if out != nil {
		t.Fatalf("ApplyPAdES() returned %d bytes after timestamp failure", len(out))
	}
	if result.Format != "" {
		t.Fatalf("result = %+v, want zero result after failure", result)
	}
}

func TestApplyPAdESRejectsIncompletePreparedTimestamp(t *testing.T) {
	t.Parallel()

	signer, _ := newPAdESTestSigner(t)
	_, _, err := ApplyPAdES(
		context.Background(),
		[]byte("%PDF-1.4\n%%EOF\n"),
		signer,
		&PreparedTimestamp{Endpoint: "https://tsa.example.test/timestamp"},
	)
	if err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("ApplyPAdES() error = %v, want incomplete configuration rejection", err)
	}
}

func newPAdESTestSigner(t *testing.T) (*pmcrypto.Signer, *x509.Certificate) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate signing key: %v", err)
	}
	now := time.Now().UTC()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(now.UnixNano()),
		Subject:      pkix.Name{CommonName: "GoPMgr PAdES Pipeline Test"},
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
		t.Fatalf("create signing certificate: %v", err)
	}
	certificate, err := x509.ParseCertificate(certificateDER)
	if err != nil {
		t.Fatalf("parse signing certificate: %v", err)
	}
	return &pmcrypto.Signer{Cert: certificate, PrivateKey: privateKey}, certificate
}

func newPAdESTestTimestampToken(t *testing.T, imprint []byte, generatedAt time.Time) []byte {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate TSA key: %v", err)
	}
	ekuValue, err := asn1.Marshal([]asn1.ObjectIdentifier{testTimestampUsageOID})
	if err != nil {
		t.Fatalf("marshal TSA EKU: %v", err)
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
		SerialNumber: big.NewInt(generatedAt.UnixNano()),
		Subject:      pkix.Name{CommonName: "GoPMgr Pipeline Test TSA"},
		// digitorus includes a CMS signing-time attribute using the wall clock,
		// while TSTInfo carries generatedAt. Deriving the test-only certificate
		// window from both prevents deterministic protocol timestamps from
		// expiring as the real calendar advances.
		NotBefore: notBefore.Add(-24 * time.Hour),
		NotAfter:  notAfter.Add(24 * time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature,
		ExtraExtensions: []pkix.Extension{{
			Id:       testTSExtendedKeyOID,
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
		Policy:            testTSPolicyOID,
		Nonce:             big.NewInt(42),
		AddTSACertificate: true,
	}).CreateResponseWithOpts(certificate, privateKey, crypto.SHA256)
	if err != nil {
		t.Fatalf("create timestamp response: %v", err)
	}
	parsed, err := timestamp.ParseResponse(responseDER)
	if err != nil {
		t.Fatalf("parse timestamp response: %v", err)
	}
	return append([]byte(nil), parsed.RawToken...)
}
