// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package crypto

import (
	"crypto/sha256"
	"encoding/asn1"
	"errors"
	"fmt"

	"github.com/digitorus/pkcs7"

	"pmforge/internal/rfc3161"
)

var oidAttributeSignatureTimestampToken = asn1.ObjectIdentifier{
	1, 2, 840, 113549, 1, 9, 16, 2, 14,
}

// SignatureTimestampImprint returns the SHA-256 digest that an RFC 3161
// signature-time-stamp token must bind for a detached, single-signer PMForge
// CMS value. RFC 5126 hashes the SignerInfo signature OCTET STRING bytes, not
// the PDF content or the complete CMS encoding.
func SignatureTimestampImprint(cmsDER []byte) ([]byte, error) {
	_, signedData, err := parseDetachedPAdESCMS(cmsDER)
	if err != nil {
		return nil, err
	}
	signature := signedData.SignerInfos[0].EncryptedDigest
	imprint := sha256.Sum256(signature)
	return imprint[:], nil
}

// AddSignatureTimestamp returns a new CMS value containing tokenDER as an
// id-aa-signatureTimeStampToken unsigned attribute.
//
// The function revalidates the token signature and its binding to the original
// signer signature value before mutation. It preserves the signed attributes,
// signature bytes, certificates, and detached-content contract, so callers do
// not need the document bytes and the original signature remains verifiable.
func AddSignatureTimestamp(cmsDER, tokenDER []byte) ([]byte, error) {
	outer, signedData, err := parseDetachedPAdESCMS(cmsDER)
	if err != nil {
		return nil, err
	}

	signature := signedData.SignerInfos[0].EncryptedDigest
	imprint := sha256.Sum256(signature)
	if _, err := rfc3161.ValidateToken(tokenDER, imprint[:]); err != nil {
		return nil, fmt.Errorf("crypto: validate signature timestamp token: %w", err)
	}

	timestampAttribute := cmsAttribute{
		Type: oidAttributeSignatureTimestampToken,
		// Attribute values are a SET OF AttributeValue. TimeStampToken is
		// already a complete DER ContentInfo value, so it becomes the SET body
		// verbatim rather than being wrapped in an OCTET STRING.
		Value: asn1.RawValue{
			Tag:        asn1.TagSet,
			IsCompound: true,
			Bytes:      append([]byte(nil), tokenDER...),
		},
	}
	unsigned := append(
		append([]cmsAttribute(nil), signedData.SignerInfos[0].UnauthenticatedAttributes...),
		timestampAttribute,
	)
	unsigned, err = sortCMSAttributes(unsigned)
	if err != nil {
		return nil, fmt.Errorf("crypto: sort unsigned CMS attributes: %w", err)
	}
	signedData.SignerInfos[0].UnauthenticatedAttributes = unsigned

	innerDER, err := asn1.Marshal(signedData)
	if err != nil {
		return nil, fmt.Errorf("crypto: marshal timestamped CMS SignedData: %w", err)
	}
	outer.Content = asn1.RawValue{
		Class:      asn1.ClassContextSpecific,
		Tag:        0,
		IsCompound: true,
		Bytes:      innerDER,
	}
	out, err := asn1.Marshal(outer)
	if err != nil {
		return nil, fmt.Errorf("crypto: marshal timestamped CMS ContentInfo: %w", err)
	}
	return out, nil
}

func parseDetachedPAdESCMS(cmsDER []byte) (cmsContentInfo, cmsSignedData, error) {
	var outer cmsContentInfo
	rest, err := asn1.Unmarshal(cmsDER, &outer)
	if err != nil {
		return cmsContentInfo{}, cmsSignedData{}, fmt.Errorf("crypto: parse CMS ContentInfo: %w", err)
	}
	if len(rest) != 0 {
		return cmsContentInfo{}, cmsSignedData{}, errors.New("crypto: CMS ContentInfo contains trailing data")
	}
	if !outer.ContentType.Equal(pkcs7.OIDSignedData) ||
		outer.Content.Class != asn1.ClassContextSpecific ||
		outer.Content.Tag != 0 {
		return cmsContentInfo{}, cmsSignedData{}, errors.New("crypto: CMS value is not SignedData")
	}

	var signedData cmsSignedData
	rest, err = asn1.Unmarshal(outer.Content.Bytes, &signedData)
	if err != nil {
		return cmsContentInfo{}, cmsSignedData{}, fmt.Errorf("crypto: parse CMS SignedData: %w", err)
	}
	if len(rest) != 0 {
		return cmsContentInfo{}, cmsSignedData{}, errors.New("crypto: CMS SignedData contains trailing data")
	}
	if !signedData.ContentInfo.ContentType.Equal(pkcs7.OIDData) ||
		len(signedData.ContentInfo.Content.FullBytes) != 0 {
		return cmsContentInfo{}, cmsSignedData{}, errors.New("crypto: CMS must contain detached id-data content")
	}
	if len(signedData.SignerInfos) != 1 {
		return cmsContentInfo{}, cmsSignedData{}, fmt.Errorf(
			"crypto: CMS must contain exactly one signer, got %d",
			len(signedData.SignerInfos),
		)
	}
	if len(signedData.SignerInfos[0].EncryptedDigest) == 0 {
		return cmsContentInfo{}, cmsSignedData{}, errors.New("crypto: CMS signer contains no signature value")
	}
	return outer, signedData, nil
}
