// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package signing

import (
	"context"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	pmcrypto "pmforge/internal/crypto"
	"pmforge/internal/pdfmeta"
	"pmforge/internal/rfc3161"
)

const maxTSATrustRootBytes = 1 << 20

const (
	PAdESBaselineB = "PAdES-BASELINE-B"
	PAdESBaselineT = "PAdES-BASELINE-T"
)

// TimestampConfig is the persisted, non-secret configuration needed to
// upgrade a PAdES Baseline B signature to Baseline T. Authentication material
// is deliberately excluded: PMForge never persists TSA credentials or accepts
// credentials embedded in the endpoint URL.
type TimestampConfig struct {
	Enabled           bool
	Endpoint          string
	PolicyOID         string
	TrustRootCertPath string
}

// TimestampRequester is the narrow RFC 3161 seam used by ApplyPAdES. Keeping
// the interface at the protocol call allows deterministic tests to exercise
// fail-closed behavior without contacting an external timestamp authority.
type TimestampRequester interface {
	Timestamp(context.Context, string, []byte) (rfc3161.Token, error)
}

// PreparedTimestamp contains a validated endpoint and a client configured
// with the requested policy and optional TSA trust roots.
type PreparedTimestamp struct {
	Endpoint  string
	Requester TimestampRequester
}

// PAdESResult describes the actual signature level written to the PDF.
// TrustStatus is meaningful only for Baseline T and remains not_evaluated when
// the user did not configure an explicit TSA root certificate.
type PAdESResult struct {
	Format               string
	TrustStatus          rfc3161.TrustStatus
	TimestampGeneratedAt time.Time
}

// PrepareTimestamp validates persisted configuration and constructs the
// production RFC 3161 client. Disabled timestamping returns nil so callers can
// use one code path for Baseline B and opt-in Baseline T exports.
func PrepareTimestamp(config TimestampConfig) (*PreparedTimestamp, error) {
	endpoint := strings.TrimSpace(config.Endpoint)
	policyText := strings.TrimSpace(config.PolicyOID)
	rootPath := strings.TrimSpace(config.TrustRootCertPath)

	if endpoint != "" {
		if err := validateTSAEndpoint(endpoint); err != nil {
			return nil, err
		}
	}
	policyOID, err := parsePolicyOID(policyText)
	if err != nil {
		return nil, err
	}
	if !config.Enabled {
		return nil, nil
	}
	if endpoint == "" {
		return nil, errors.New("timestamp authority endpoint is required when PAdES-T is enabled")
	}

	var roots *x509.CertPool
	if rootPath != "" {
		roots, err = loadTSATrustRoots(rootPath)
		if err != nil {
			return nil, err
		}
	}
	return &PreparedTimestamp{
		Endpoint: endpoint,
		Requester: rfc3161.Client{
			PolicyOID:  policyOID,
			TrustRoots: roots,
		},
	}, nil
}

// ApplyPAdES signs the PDF as Baseline B or, when timestamping is prepared,
// obtains and embeds an RFC 3161 signature timestamp before the final PDF
// mutation. Any TSA or embedding failure returns no PDF bytes, preventing an
// enabled Baseline T request from silently degrading to Baseline B.
func ApplyPAdES(
	ctx context.Context,
	pdfBytes []byte,
	signer *pmcrypto.Signer,
	timestampConfig *PreparedTimestamp,
) ([]byte, PAdESResult, error) {
	if signer == nil {
		return nil, PAdESResult{}, errors.New("PAdES signer is required")
	}
	if timestampConfig != nil &&
		(strings.TrimSpace(timestampConfig.Endpoint) == "" || timestampConfig.Requester == nil) {
		return nil, PAdESResult{}, errors.New("prepared timestamp configuration is incomplete")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	result := PAdESResult{Format: PAdESBaselineB}
	signedPDF, err := pdfmeta.InjectPAdESSignature(pdfBytes, func(byteRange []byte) ([]byte, error) {
		cmsDER, err := signer.SignPDFCMS(byteRange)
		if err != nil {
			return nil, fmt.Errorf("create detached PAdES CMS: %w", err)
		}
		if timestampConfig == nil {
			return cmsDER, nil
		}

		imprint, err := pmcrypto.SignatureTimestampImprint(cmsDER)
		if err != nil {
			return nil, fmt.Errorf("calculate PAdES timestamp imprint: %w", err)
		}
		token, err := timestampConfig.Requester.Timestamp(ctx, timestampConfig.Endpoint, imprint)
		if err != nil {
			return nil, fmt.Errorf("request RFC 3161 signature timestamp: %w", err)
		}
		timestampedCMS, err := pmcrypto.AddSignatureTimestamp(cmsDER, token.DER)
		if err != nil {
			return nil, fmt.Errorf("embed RFC 3161 signature timestamp: %w", err)
		}
		result = PAdESResult{
			Format:               PAdESBaselineT,
			TrustStatus:          token.TrustStatus,
			TimestampGeneratedAt: token.GeneratedAt,
		}
		return timestampedCMS, nil
	})
	if err != nil {
		return nil, PAdESResult{}, fmt.Errorf("embed PAdES signature: %w", err)
	}
	return signedPDF, result, nil
}

func validateTSAEndpoint(endpoint string) error {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("invalid timestamp authority endpoint: %w", err)
	}
	if !strings.EqualFold(parsed.Scheme, "https") || parsed.Host == "" {
		return errors.New("timestamp authority endpoint must be an absolute HTTPS URL")
	}
	if parsed.User != nil {
		return errors.New("timestamp authority endpoint must not contain credentials")
	}
	// Query strings frequently carry API keys. Refusing them makes the
	// non-secret settings boundary enforceable instead of relying on users to
	// recognize every provider-specific credential parameter.
	if parsed.RawQuery != "" {
		return errors.New("timestamp authority endpoint must not contain a query string")
	}
	if parsed.Fragment != "" {
		return errors.New("timestamp authority endpoint must not contain a fragment")
	}
	return nil
}

func parsePolicyOID(value string) (asn1.ObjectIdentifier, error) {
	if value == "" {
		return nil, nil
	}
	parts := strings.Split(value, ".")
	if len(parts) < 2 {
		return nil, errors.New("timestamp policy OID must contain at least two arcs")
	}
	oid := make(asn1.ObjectIdentifier, len(parts))
	for index, part := range parts {
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return nil, fmt.Errorf("timestamp policy OID has invalid arc %q", part)
		}
		arc, err := strconv.ParseUint(part, 10, 31)
		if err != nil {
			return nil, fmt.Errorf("timestamp policy OID has invalid arc %q", part)
		}
		oid[index] = int(arc)
	}
	if oid[0] > 2 || (oid[0] < 2 && oid[1] > 39) {
		return nil, errors.New("timestamp policy OID has an invalid first or second arc")
	}
	if _, err := asn1.Marshal(oid); err != nil {
		return nil, fmt.Errorf("timestamp policy OID is invalid: %w", err)
	}
	return oid, nil
}

func loadTSATrustRoots(path string) (*x509.CertPool, error) {
	file, err := os.Open(path) // #nosec G304 -- the project owner explicitly selects this local trust-root file.
	if err != nil {
		return nil, fmt.Errorf("open TSA trust root %q: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(io.LimitReader(file, maxTSATrustRootBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read TSA trust root %q: %w", path, err)
	}
	if len(data) > maxTSATrustRootBytes {
		return nil, fmt.Errorf("TSA trust root %q exceeds the %d-byte limit", path, maxTSATrustRootBytes)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(data) {
		return nil, fmt.Errorf("TSA trust root %q contains no PEM certificates", path)
	}
	return roots, nil
}
