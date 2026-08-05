// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package crypto

import (
	"encoding/asn1"
	"testing"

	"github.com/digitorus/pkcs7"
)

// TestCmsSignedAttributes_PropagatesMarshalError exercises the
// asn1.Marshal error branch directly against the unexported helper: no
// production call site in this package ever constructs a
// pkcs7.Attribute.Value that fails to marshal (always an
// ObjectIdentifier, a []byte digest, or the signingCertificateV2
// struct), so this can only be forced with a value type asn1.Marshal
// genuinely cannot encode -- confirmed by direct experimentation before
// writing this test, not assumed.
func TestCmsSignedAttributes_PropagatesMarshalError(t *testing.T) {
	_, err := cmsSignedAttributes([]pkcs7.Attribute{
		{Type: asn1.ObjectIdentifier{1, 2, 3}, Value: make(chan int)},
	})
	if err == nil {
		t.Fatal("expected an error for an attribute value asn1.Marshal cannot encode")
	}
}

// TestSortCMSAttributes_PropagatesMarshalError exercises
// sortCMSAttributes's own asn1.Marshal error branch the same way: an
// empty asn1.ObjectIdentifier fails DER encoding ("invalid object
// identifier"), confirmed by direct experimentation. No real caller in
// this package ever builds a cmsAttribute with an empty Type.
func TestSortCMSAttributes_PropagatesMarshalError(t *testing.T) {
	attrs := []cmsAttribute{
		{Type: asn1.ObjectIdentifier{}, Value: asn1.RawValue{Tag: 17, IsCompound: true, Bytes: []byte{1}}},
	}
	if _, err := sortCMSAttributes(attrs); err == nil {
		t.Fatal("expected an error for an attribute with an invalid object identifier")
	}
}

// TestSortCMSAttributes_OrdersByDEREncoding pins the actual reason this
// function exists (canonical DER SET ordering), not just its error path:
// attributes must come out sorted by their encoded bytes, not insertion
// order, and the input slice must be left unmodified.
func TestSortCMSAttributes_OrdersByDEREncoding(t *testing.T) {
	high := cmsAttribute{Type: asn1.ObjectIdentifier{2, 5, 29, 99}, Value: asn1.RawValue{Tag: 17, IsCompound: true, Bytes: []byte("z")}}
	low := cmsAttribute{Type: asn1.ObjectIdentifier{1, 2, 3}, Value: asn1.RawValue{Tag: 17, IsCompound: true, Bytes: []byte("a")}}
	input := []cmsAttribute{high, low}

	sorted, err := sortCMSAttributes(input)
	if err != nil {
		t.Fatalf("sortCMSAttributes: %v", err)
	}
	if len(sorted) != 2 || !sorted[0].Type.Equal(low.Type) || !sorted[1].Type.Equal(high.Type) {
		t.Fatalf("sortCMSAttributes did not order by DER encoding: %+v", sorted)
	}
	if !input[0].Type.Equal(high.Type) || !input[1].Type.Equal(low.Type) {
		t.Fatal("sortCMSAttributes mutated the caller's input slice")
	}
}
