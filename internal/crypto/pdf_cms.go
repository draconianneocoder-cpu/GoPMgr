// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package crypto

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"sort"

	"github.com/digitorus/pkcs7"
)

type cmsContentInfo struct {
	ContentType asn1.ObjectIdentifier
	Content     asn1.RawValue `asn1:"explicit,optional,tag:0"`
}

type cmsSignedData struct {
	Version                    int                        `asn1:"default:1"`
	DigestAlgorithmIdentifiers []pkix.AlgorithmIdentifier `asn1:"set"`
	ContentInfo                cmsContentInfo
	Certificates               cmsRawCertificates     `asn1:"optional,tag:0"`
	CRLs                       []pkix.CertificateList `asn1:"optional,tag:1"`
	SignerInfos                []cmsSignerInfo        `asn1:"set"`
}

type cmsSignerInfo struct {
	Version                   int `asn1:"default:1"`
	IssuerAndSerialNumber     cmsIssuerAndSerial
	DigestAlgorithm           pkix.AlgorithmIdentifier
	AuthenticatedAttributes   []cmsAttribute `asn1:"optional,omitempty,tag:0"`
	DigestEncryptionAlgorithm pkix.AlgorithmIdentifier
	EncryptedDigest           []byte
	UnauthenticatedAttributes []cmsAttribute `asn1:"optional,omitempty,tag:1"`
}

type cmsIssuerAndSerial struct {
	IssuerName   asn1.RawValue
	SerialNumber *big.Int
}

type cmsAttribute struct {
	Type  asn1.ObjectIdentifier
	Value asn1.RawValue `asn1:"set"`
}

type cmsRawCertificates struct {
	Raw asn1.RawContent
}

type cmsSortableAttribute struct {
	sortKey   []byte
	attribute cmsAttribute
}

func signDetachedPAdESCMS(content []byte, cert *x509.Certificate, key *rsa.PrivateKey, extraCerts []*x509.Certificate) ([]byte, error) {
	contentDigest := sha256.Sum256(content)
	attrs, err := cmsSignedAttributes([]pkcs7.Attribute{
		{Type: pkcs7.OIDAttributeContentType, Value: pkcs7.OIDData},
		{Type: pkcs7.OIDAttributeMessageDigest, Value: contentDigest[:]},
		signingCertificateV2Attribute(cert),
	})
	if err != nil {
		return nil, err
	}

	signature, err := signCMSSignedAttributes(attrs, key)
	if err != nil {
		return nil, err
	}

	certs := append([]*x509.Certificate{cert}, extraCerts...)
	rawCerts, err := marshalCMSCertificates(certs)
	if err != nil {
		return nil, err
	}

	inner, err := asn1.Marshal(cmsSignedData{
		Version:                    1,
		DigestAlgorithmIdentifiers: []pkix.AlgorithmIdentifier{{Algorithm: pkcs7.OIDDigestAlgorithmSHA256}},
		ContentInfo:                cmsContentInfo{ContentType: pkcs7.OIDData},
		Certificates:               rawCerts,
		SignerInfos: []cmsSignerInfo{{
			Version: 1,
			IssuerAndSerialNumber: cmsIssuerAndSerial{
				IssuerName:   asn1.RawValue{FullBytes: cert.RawIssuer},
				SerialNumber: cert.SerialNumber,
			},
			DigestAlgorithm:           pkix.AlgorithmIdentifier{Algorithm: pkcs7.OIDDigestAlgorithmSHA256},
			AuthenticatedAttributes:   attrs,
			DigestEncryptionAlgorithm: pkix.AlgorithmIdentifier{Algorithm: pkcs7.OIDEncryptionAlgorithmRSASHA256},
			EncryptedDigest:           signature,
		}},
	})
	if err != nil {
		return nil, err
	}

	return asn1.Marshal(cmsContentInfo{
		ContentType: pkcs7.OIDSignedData,
		Content:     asn1.RawValue{Class: 2, Tag: 0, Bytes: inner, IsCompound: true},
	})
}

func cmsSignedAttributes(attrs []pkcs7.Attribute) ([]cmsAttribute, error) {
	sortables := make([]cmsSortableAttribute, len(attrs))
	for i, attr := range attrs {
		asn1Value, err := asn1.Marshal(attr.Value)
		if err != nil {
			return nil, err
		}
		cmsAttr := cmsAttribute{
			Type:  attr.Type,
			Value: asn1.RawValue{Tag: 17, IsCompound: true, Bytes: asn1Value},
		}
		encoded, err := asn1.Marshal(cmsAttr)
		if err != nil {
			return nil, err
		}
		sortables[i] = cmsSortableAttribute{
			sortKey:   encoded,
			attribute: cmsAttr,
		}
	}
	sort.Slice(sortables, func(i, j int) bool {
		return bytes.Compare(sortables[i].sortKey, sortables[j].sortKey) < 0
	})

	marshaled := make([]cmsAttribute, len(sortables))
	for i, attr := range sortables {
		marshaled[i] = attr.attribute
	}
	return marshaled, nil
}

func signCMSSignedAttributes(attrs []cmsAttribute, key *rsa.PrivateKey) ([]byte, error) {
	attrBytes, err := marshalCMSSignedAttributeSet(attrs)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(attrBytes)
	return rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
}

func marshalCMSSignedAttributeSet(attrs []cmsAttribute) ([]byte, error) {
	encodedAttributes, err := asn1.Marshal(struct {
		Attrs []cmsAttribute `asn1:"set"`
	}{Attrs: attrs})
	if err != nil {
		return nil, err
	}

	var raw asn1.RawValue
	if _, err := asn1.Unmarshal(encodedAttributes, &raw); err != nil {
		return nil, err
	}
	return raw.Bytes, nil
}

func marshalCMSCertificates(certs []*x509.Certificate) (cmsRawCertificates, error) {
	var buf bytes.Buffer
	for _, cert := range certs {
		buf.Write(cert.Raw)
	}
	return marshalCMSCertificateBytes(buf.Bytes())
}

func marshalCMSCertificateBytes(certs []byte) (cmsRawCertificates, error) {
	val := asn1.RawValue{Bytes: certs, Class: 2, Tag: 0, IsCompound: true}
	b, err := asn1.Marshal(val)
	if err != nil {
		return cmsRawCertificates{}, err
	}
	return cmsRawCertificates{Raw: b}, nil
}
