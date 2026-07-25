// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package rfc3161 provides the fail-closed timestamp-authority client used by
// PMForge's signing pipeline. It validates timestamp tokens but does not embed
// them into PAdES signatures; that integration is intentionally a separate
// release slice.
package rfc3161

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"fmt"
	"io"
	"math/big"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/digitorus/pkcs7"
	"github.com/digitorus/timestamp"
)

const (
	// QueryMediaType is the RFC 3161 MIME type for a timestamp request.
	QueryMediaType = "application/timestamp-query"
	// ReplyMediaType is the RFC 3161 MIME type for a timestamp response.
	ReplyMediaType = "application/timestamp-reply"

	// NonceBytes gives every request a 128-bit replay-resistant nonce.
	NonceBytes = 16

	defaultTimeout          = 15 * time.Second
	defaultMaxResponseBytes = 1 << 20
)

var (
	oidExtKeyUsage = asn1.ObjectIdentifier{2, 5, 29, 37}
	oidTSTInfo     = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 16, 1, 4}
)

// TrustStatus separates a cryptographically valid token from one whose signer
// also chains to caller-provided trust roots.
type TrustStatus string

const (
	// TrustNotEvaluated means the CMS signature was valid, but no root store was
	// configured. Callers must not present this state as a trusted timestamp.
	TrustNotEvaluated TrustStatus = "not_evaluated"
	// TrustVerified means the timestamp signer chained to the configured roots.
	TrustVerified TrustStatus = "verified"
)

// Token is an RFC 3161 token that passed the request-binding, CMS-signature,
// and TSA-certificate checks performed by Client.Timestamp.
type Token struct {
	DER               []byte
	GeneratedAt       time.Time
	PolicyOID         asn1.ObjectIdentifier
	SerialNumber      *big.Int
	SignerCertificate *x509.Certificate
	TrustStatus       TrustStatus
}

// Client controls timestamp transport and validation policy.
//
// Transport and Random are injectable so protocol tests never need a live TSA.
// Production callers should leave them nil to use the standard HTTPS transport
// and crypto/rand.Reader. TrustRoots is optional: a nil pool verifies token
// integrity but deliberately reports TrustNotEvaluated.
type Client struct {
	Transport        http.RoundTripper
	Random           io.Reader
	Timeout          time.Duration
	MaxResponseBytes int64
	PolicyOID        asn1.ObjectIdentifier
	TrustRoots       *x509.CertPool
}

// Timestamp requests a SHA-256 timestamp for imprint from endpoint.
//
// imprint is already hashed because PAdES-T timestamps the existing CMS
// signature value, not the original document bytes. The method requires HTTPS,
// refuses redirects, bounds the response, and validates all fields that bind a
// response to its request before returning the raw token for later embedding.
func (c Client) Timestamp(ctx context.Context, endpoint string, imprint []byte) (Token, error) {
	if len(imprint) != sha256.Size {
		return Token{}, fmt.Errorf("RFC 3161 SHA-256 message imprint must be %d bytes", sha256.Size)
	}
	if err := validateEndpoint(endpoint); err != nil {
		return Token{}, err
	}

	nonce, err := c.newNonce()
	if err != nil {
		return Token{}, err
	}
	requestDER, err := (&timestamp.Request{
		HashAlgorithm: crypto.SHA256,
		HashedMessage: append([]byte(nil), imprint...),
		Certificates:  true,
		TSAPolicyOID:  append(asn1.ObjectIdentifier(nil), c.PolicyOID...),
		Nonce:         nonce,
	}).Marshal()
	if err != nil {
		return Token{}, fmt.Errorf("marshal RFC 3161 request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestDER))
	if err != nil {
		return Token{}, fmt.Errorf("create RFC 3161 request: %w", err)
	}
	request.Header.Set("Content-Type", QueryMediaType)
	request.Header.Set("Accept", ReplyMediaType)

	response, err := c.httpClient().Do(request)
	if err != nil {
		return Token{}, fmt.Errorf("send RFC 3161 request: %w", err)
	}
	defer func() {
		// The bounded body read below is the authoritative operation; a close
		// error cannot invalidate bytes that have already been validated.
		_ = response.Body.Close()
	}()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return Token{}, fmt.Errorf("RFC 3161 endpoint returned HTTP %d", response.StatusCode)
	}
	if err := validateReplyMediaType(response.Header.Get("Content-Type")); err != nil {
		return Token{}, err
	}
	responseDER, err := readBounded(response.Body, c.responseLimit())
	if err != nil {
		return Token{}, err
	}

	return c.validateResponse(responseDER, imprint, nonce)
}

func (c Client) newNonce() (*big.Int, error) {
	source := c.Random
	if source == nil {
		source = rand.Reader
	}
	raw := make([]byte, NonceBytes)
	if _, err := io.ReadFull(source, raw); err != nil {
		return nil, fmt.Errorf("read RFC 3161 nonce entropy: %w", err)
	}
	nonce := new(big.Int).SetBytes(raw)
	if nonce.Sign() == 0 {
		return nil, errors.New("RFC 3161 nonce entropy was all zeros")
	}
	return nonce, nil
}

func (c Client) httpClient() *http.Client {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &http.Client{
		Transport: c.Transport,
		Timeout:   timeout,
		// A TSA redirect would send the signed-message imprint and nonce to an
		// endpoint the user did not configure, so every redirect is explicit.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func (c Client) responseLimit() int64 {
	if c.MaxResponseBytes > 0 {
		return c.MaxResponseBytes
	}
	return defaultMaxResponseBytes
}

func (c Client) validateResponse(responseDER, imprint []byte, nonce *big.Int) (Token, error) {
	status, tokenDER, err := parseResponseEnvelope(responseDER)
	if err != nil {
		return Token{}, err
	}
	if status != 0 && status != 1 {
		return Token{}, fmt.Errorf("RFC 3161 request rejected with status %d", status)
	}
	if len(tokenDER) == 0 {
		return Token{}, errors.New("RFC 3161 success response contained no timestamp token")
	}

	signedData, err := pkcs7.Parse(tokenDER)
	if err != nil {
		return Token{}, fmt.Errorf("parse RFC 3161 CMS token: %w", err)
	}
	if err := validateCMSContentType(tokenDER, signedData); err != nil {
		return Token{}, err
	}
	parsed, err := timestamp.Parse(tokenDER)
	if err != nil {
		return Token{}, fmt.Errorf("validate RFC 3161 token: %w", err)
	}
	if parsed.HashAlgorithm != crypto.SHA256 {
		return Token{}, fmt.Errorf("RFC 3161 token used %v instead of SHA-256", parsed.HashAlgorithm)
	}
	if len(parsed.HashedMessage) != len(imprint) ||
		subtle.ConstantTimeCompare(parsed.HashedMessage, imprint) != 1 {
		return Token{}, errors.New("RFC 3161 token message imprint does not match the request")
	}
	if parsed.Nonce == nil || parsed.Nonce.Cmp(nonce) != 0 {
		return Token{}, errors.New("RFC 3161 token nonce does not match the request")
	}
	if len(c.PolicyOID) > 0 && !parsed.Policy.Equal(c.PolicyOID) {
		return Token{}, errors.New("RFC 3161 token policy does not match the requested policy")
	}
	if parsed.Time.IsZero() {
		return Token{}, errors.New("RFC 3161 token contains no generation time")
	}
	if parsed.SerialNumber == nil {
		return Token{}, errors.New("RFC 3161 token contains no serial number")
	}
	signer := signedData.GetOnlySigner()
	if signer == nil {
		return Token{}, errors.New("RFC 3161 token must contain exactly one signer certificate")
	}
	if err := validateTSACertificate(signer); err != nil {
		return Token{}, err
	}
	if parsed.Time.Before(signer.NotBefore) || parsed.Time.After(signer.NotAfter) {
		return Token{}, errors.New("RFC 3161 token generation time is outside the TSA certificate validity period")
	}

	trustStatus := TrustNotEvaluated
	if c.TrustRoots != nil {
		intermediates := x509.NewCertPool()
		for _, certificate := range signedData.Certificates {
			if !certificate.Equal(signer) {
				intermediates.AddCert(certificate)
			}
		}
		if err := signedData.VerifyWithOpts(x509.VerifyOptions{
			Roots:         c.TrustRoots,
			Intermediates: intermediates,
			CurrentTime:   parsed.Time,
			KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageTimeStamping},
		}); err != nil {
			return Token{}, fmt.Errorf("verify RFC 3161 TSA trust chain: %w", err)
		}
		trustStatus = TrustVerified
	}

	return Token{
		DER:               append([]byte(nil), tokenDER...),
		GeneratedAt:       parsed.Time,
		PolicyOID:         append(asn1.ObjectIdentifier(nil), parsed.Policy...),
		SerialNumber:      new(big.Int).Set(parsed.SerialNumber),
		SignerCertificate: signer,
		TrustStatus:       trustStatus,
	}, nil
}

func validateEndpoint(endpoint string) error {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("parse RFC 3161 endpoint: %w", err)
	}
	if !strings.EqualFold(parsed.Scheme, "https") || parsed.Host == "" {
		return errors.New("RFC 3161 endpoint must be an absolute HTTPS URL")
	}
	if parsed.User != nil {
		return errors.New("RFC 3161 endpoint must not contain URL user information")
	}
	if parsed.Fragment != "" {
		return errors.New("RFC 3161 endpoint must not contain a fragment")
	}
	return nil
}

func validateReplyMediaType(contentType string) error {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || !strings.EqualFold(mediaType, ReplyMediaType) {
		return fmt.Errorf("RFC 3161 response Content-Type must be %q", ReplyMediaType)
	}
	return nil
}

func readBounded(reader io.Reader, limit int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read RFC 3161 response: %w", err)
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("RFC 3161 response exceeds %d bytes", limit)
	}
	return body, nil
}

type responseEnvelope struct {
	Status struct {
		Status       int
		StatusString []string       `asn1:"optional"`
		FailInfo     asn1.BitString `asn1:"optional"`
	}
	TimeStampToken asn1.RawValue `asn1:"optional"`
}

func parseResponseEnvelope(der []byte) (int, []byte, error) {
	var envelope responseEnvelope
	rest, err := asn1.Unmarshal(der, &envelope)
	if err != nil {
		return 0, nil, fmt.Errorf("parse RFC 3161 response: %w", err)
	}
	if len(rest) != 0 {
		return 0, nil, errors.New("parse RFC 3161 response: trailing data")
	}
	if envelope.Status.Status < 0 || envelope.Status.Status > 5 {
		return 0, nil, fmt.Errorf("RFC 3161 response used unknown status %d", envelope.Status.Status)
	}
	if envelope.Status.Status > 1 {
		detail := strings.Join(envelope.Status.StatusString, ", ")
		if detail == "" {
			return envelope.Status.Status, nil, nil
		}
		return 0, nil, fmt.Errorf("RFC 3161 request rejected with status %d: %s", envelope.Status.Status, detail)
	}
	return envelope.Status.Status, append([]byte(nil), envelope.TimeStampToken.FullBytes...), nil
}

func validateTSACertificate(certificate *x509.Certificate) error {
	if len(certificate.ExtKeyUsage) != 1 ||
		certificate.ExtKeyUsage[0] != x509.ExtKeyUsageTimeStamping ||
		len(certificate.UnknownExtKeyUsage) != 0 {
		return errors.New("RFC 3161 TSA certificate must be restricted to timestamping extended key usage")
	}
	for _, extension := range certificate.Extensions {
		if extension.Id.Equal(oidExtKeyUsage) {
			if !extension.Critical {
				return errors.New("RFC 3161 TSA certificate timestamping extended key usage must be critical")
			}
			return nil
		}
	}
	return errors.New("RFC 3161 TSA certificate contains no extended key usage extension")
}

type cmsContentInfo struct {
	ContentType asn1.ObjectIdentifier
	Content     asn1.RawValue `asn1:"explicit,tag:0"`
}

type cmsSignedDataPrefix struct {
	Version          int
	DigestAlgorithms asn1.RawValue
	ContentInfo      struct {
		ContentType asn1.ObjectIdentifier
		Content     asn1.RawValue `asn1:"explicit,optional,tag:0"`
	}
}

func validateCMSContentType(tokenDER []byte, signedData *pkcs7.PKCS7) error {
	var outer cmsContentInfo
	rest, err := asn1.Unmarshal(tokenDER, &outer)
	if err != nil || len(rest) != 0 || !outer.ContentType.Equal(pkcs7.OIDSignedData) {
		return errors.New("RFC 3161 token is not a complete CMS SignedData value")
	}

	var inner cmsSignedDataPrefix
	rest, err = asn1.Unmarshal(outer.Content.Bytes, &inner)
	if err != nil || len(rest) != 0 {
		return errors.New("RFC 3161 token contains malformed CMS SignedData")
	}
	if !inner.ContentInfo.ContentType.Equal(oidTSTInfo) {
		return errors.New("RFC 3161 token CMS content type is not id-ct-TSTInfo")
	}

	var signedContentType asn1.ObjectIdentifier
	if err := signedData.UnmarshalSignedAttribute(pkcs7.OIDAttributeContentType, &signedContentType); err != nil {
		return fmt.Errorf("read RFC 3161 signed content type: %w", err)
	}
	if !signedContentType.Equal(oidTSTInfo) {
		return errors.New("RFC 3161 signed content type is not id-ct-TSTInfo")
	}
	return nil
}
