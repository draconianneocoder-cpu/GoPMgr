// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package crypto

import (
	"bytes"
	"errors"
	"math"
	"strings"
	"testing"
)

func TestEscrowFieldLengthRejectsUnrepresentableLengths(t *testing.T) {
	for _, tc := range []struct {
		length uint64
		want   uint32
		reject bool
	}{
		{0, 0, false},
		{1, 1, false},
		{math.MaxUint32 - 1, math.MaxUint32 - 1, false},
		{math.MaxUint32, math.MaxUint32, false},
		{math.MaxUint32 + 1, 0, true},
		{math.MaxUint64, 0, true},
	} {
		got, err := escrowFieldLength(tc.length)
		if got != tc.want || (tc.reject && !errors.Is(err, errEscrowFieldTooLarge)) || (!tc.reject && err != nil) {
			t.Errorf("length %d = %d, %v; want %d, reject=%v", tc.length, got, err, tc.want, tc.reject)
		}
	}
}

func TestEscrowBindingPreservesLiteralV1Bytes(t *testing.T) {
	got, err := bindFields("v1", "Zoë", string([]byte{0, 255}), "")
	want := []byte{0, 0, 0, 2, 'v', '1', 0, 0, 0, 4, 'Z', 'o', 0xc3, 0xab,
		0, 0, 0, 2, 0, 255, 0, 0, 0, 0}
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("binding = %x, %v; want %x", got, err, want)
	}
	got, err = appendField([]byte{0xfa}, "")
	if err != nil || !bytes.Equal(got, []byte{0xfa, 0, 0, 0, 0}) {
		t.Fatalf("empty field = %x, %v", got, err)
	}
}

func TestEscrowContextLimitDoesNotImposeAccountNamePolicy(t *testing.T) {
	// The crypto binding's wire limit is independent of the account UI's
	// narrower identity policy. Existing representable contexts stay valid.
	identity := strings.Repeat("é", 65)
	key := bytes.Repeat([]byte{1}, DEKSize)
	publicContext := bytes.Repeat([]byte{0xff}, 65)
	pin, err := EscrowPin(key, identity, identity, publicContext)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := VerifyEscrowPin(key, identity, identity, publicContext, pin); err != nil || !ok {
		t.Fatalf("representable PIN context refused: %v, %v", ok, err)
	}
	attestation, err := Attestation(key, identity, publicContext)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := VerifyAttestation(key, identity, publicContext, attestation); err != nil || !ok {
		t.Fatalf("representable attestation context refused: %v, %v", ok, err)
	}
}
