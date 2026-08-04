// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/digitorus/timestamp"

	pmcrypto "gopmgr/internal/crypto"
	"gopmgr/internal/db"
	"gopmgr/internal/documents"
	"gopmgr/internal/rfc3161"
	"gopmgr/internal/signing"
)

var (
	appTestTSPolicyOID       = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 55555, 26}
	appTestTSExtendedKeyOID  = asn1.ObjectIdentifier{2, 5, 29, 37}
	appTestTimestampUsageOID = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 8}
)

type appTimestampRequesterFunc func(context.Context, string, []byte) (rfc3161.Token, error)

func (f appTimestampRequesterFunc) Timestamp(
	ctx context.Context,
	endpoint string,
	imprint []byte,
) (rfc3161.Token, error) {
	return f(ctx, endpoint, imprint)
}

func TestExportDocumentPDFSignedWithRuntimeWritesVerifiedPAdEST(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "pass-horse-battery-staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Verified Timestamp Plan")
	project, err := app.GetProjectMeta()
	if err != nil {
		t.Fatalf("GetProjectMeta: %v", err)
	}
	doc, err := app.NewDocument("charter_word", "Verified Timestamp Charter")
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}

	generatedAt := time.Now().UTC().Truncate(time.Second)
	outputPath, err := app.exportDocumentPDFSignedWithRuntime(
		doc.ID,
		"test-signer.p12",
		"test-password",
		newAppPAdESTestRuntime(t, rfc3161.TrustVerified, generatedAt),
	)
	if err != nil {
		t.Fatalf("exportDocumentPDFSignedWithRuntime: %v", err)
	}
	assertAppPAdESTPDF(t, outputPath)
	if got := auditEventSignatureStatus(t, app, project.ID, doc.ID); got != "pades_t_verified" {
		t.Fatalf("signature_status = %q, want pades_t_verified", got)
	}
}

func TestExportCombinedReportSignedWithRuntimeWritesUnevaluatedPAdEST(t *testing.T) {
	app := newEncryptionProjectTestApp(t)
	if _, err := app.CreateAccount("alice", "Alice", "pass-horse-battery-staple", false); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	mustOpenProject(t, app, "Unevaluated Timestamp Plan")
	project, err := app.GetProjectMeta()
	if err != nil {
		t.Fatalf("GetProjectMeta: %v", err)
	}
	doc, err := app.NewDocument("charter_word", "Unevaluated Timestamp Charter")
	if err != nil {
		t.Fatalf("NewDocument: %v", err)
	}
	sections := []documents.ReportSection{{
		DocumentID:  doc.ID,
		Title:       doc.Title,
		Description: "Executive approval section",
	}}
	reportTitle := "Timestamped Governance Pack"
	subtitle := "Prerelease Fixture"
	reportID := combinedReportCheckpointID(project.ID, reportTitle, subtitle, sections)

	generatedAt := time.Now().UTC().Truncate(time.Second)
	outputPath, err := app.exportCombinedReportSignedWithRuntime(
		reportTitle,
		subtitle,
		sections,
		"test-signer.p12",
		"test-password",
		newAppPAdESTestRuntime(t, rfc3161.TrustNotEvaluated, generatedAt),
	)
	if err != nil {
		t.Fatalf("exportCombinedReportSignedWithRuntime: %v", err)
	}
	assertAppPAdESTPDF(t, outputPath)
	status, payload := combinedReportSignatureEvent(t, app, project.ID, reportID)
	if status != "pades_t_not_evaluated" {
		t.Fatalf("signature_status = %q, want pades_t_not_evaluated", status)
	}
	if !strings.Contains(payload, `"output_path":"`+outputPath+`"`) {
		t.Fatalf("combined report signature payload = %s, want output path %q", payload, outputPath)
	}
}

func newAppPAdESTestRuntime(
	t *testing.T,
	trustStatus rfc3161.TrustStatus,
	generatedAt time.Time,
) padesExportRuntime {
	t.Helper()

	signer := newAppPAdESTestSigner(t)
	requester := appTimestampRequesterFunc(func(
		_ context.Context,
		endpoint string,
		imprint []byte,
	) (rfc3161.Token, error) {
		if endpoint != "https://tsa.example.test/timestamp" {
			t.Fatalf("timestamp endpoint = %q", endpoint)
		}
		return rfc3161.Token{
			DER:         newAppPAdESTestTimestampToken(t, imprint, generatedAt),
			GeneratedAt: generatedAt,
			PolicyOID:   appTestTSPolicyOID,
			TrustStatus: trustStatus,
		}, nil
	})
	return padesExportRuntime{
		loadCertificate: func(path, password string) (*pmcrypto.Signer, error) {
			if path != "test-signer.p12" || password != "test-password" {
				t.Fatalf("certificate loader received path %q and password %q", path, password)
			}
			return signer, nil
		},
		prepareTimestamp: func(database *db.Database) (*signing.PreparedTimestamp, error) {
			if database == nil {
				t.Fatal("timestamp preparer received a nil project database")
			}
			return &signing.PreparedTimestamp{
				Endpoint:  "https://tsa.example.test/timestamp",
				Requester: requester,
			}, nil
		},
	}
}

func assertAppPAdESTPDF(t *testing.T, outputPath string) {
	t.Helper()

	pdfBytes, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read signed PDF: %v", err)
	}
	for _, marker := range [][]byte{
		[]byte("/Type /Sig"),
		[]byte("/ByteRange ["),
		[]byte("/SubFilter /ETSI.CAdES.detached"),
		// id-aa-signatureTimeStampToken proves the PDF contains Baseline T CMS,
		// rather than trusting the audit status alone.
		[]byte("060b2a864886f70d010910020e"),
	} {
		if !bytes.Contains(pdfBytes, marker) {
			t.Fatalf("signed PDF does not contain %q", marker)
		}
	}
	if bytes.Contains(pdfBytes, []byte("%%GoPMgrCMSSignature:")) {
		t.Fatal("signed PDF contains the retired comment-marker signature")
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("stat signed PDF: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("signed PDF permissions = %v, want 0600", got)
	}
}

func newAppPAdESTestSigner(t *testing.T) *pmcrypto.Signer {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate signing key: %v", err)
	}
	now := time.Now().UTC()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(now.UnixNano()),
		Subject:      pkix.Name{CommonName: "GoPMgr Application Export Test"},
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
	return &pmcrypto.Signer{Cert: certificate, PrivateKey: privateKey}
}

func newAppPAdESTestTimestampToken(t *testing.T, imprint []byte, generatedAt time.Time) []byte {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate TSA key: %v", err)
	}
	ekuValue, err := asn1.Marshal([]asn1.ObjectIdentifier{appTestTimestampUsageOID})
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
		Subject:      pkix.Name{CommonName: "GoPMgr Application Export Test TSA"},
		// The timestamp library records the wall clock in a CMS attribute while
		// TSTInfo carries generatedAt. Deriving the test-only certificate window
		// from both prevents deterministic protocol timestamps from expiring as
		// the real calendar advances.
		NotBefore: notBefore.Add(-24 * time.Hour),
		NotAfter:  notAfter.Add(24 * time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature,
		ExtraExtensions: []pkix.Extension{{
			Id:       appTestTSExtendedKeyOID,
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
		Policy:            appTestTSPolicyOID,
		Nonce:             big.NewInt(26),
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
