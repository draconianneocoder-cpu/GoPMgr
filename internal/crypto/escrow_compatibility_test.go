// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package crypto

import (
	"bytes"
	"encoding/hex"
	"testing"
)

// These frozen v1 values were captured through public APIs at cda84c3,
// before the field-length repair. All keys are synthetic public test data,
// generated from byte sequences; none are production credentials.
// Ciphertext expectations must not be regenerated with the candidate.
var legacyEscrowV1 = map[string]string{
	"admin_private":    "2122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f40",
	"attestation":      "431c6202cf42115e9a469d0a093ad400b3d4516c6309d5f28bf19cf3f6badc10",
	"baseline":         "cda84c3183e2b389895b2d21a4df9dcee64d366a",
	"dek":              "8182838485868788898a8b8c8d8e8f909192939495969798999a9b9c9d9e9fa0",
	"escrow_private":   "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
	"escrow_public":    "07a37cbc142093c8b755dc1b10e86cb426374ad16aa853ed0bdfc0b2b86d1c7c",
	"personal_private": "4142434445464748494a4b4c4d4e4f505152535455565758595a5b5c5d5e5f60",
	"personal_public":  "64b101b1d0be5a8704bd078f9895001fc03e8e9f9522f188dd128d9846d48466",
	"pin":              "bb07e02ecd09328ad9b024bc531e581a8136c1cd05f0add79ada753f4368bdfe",
	"recovery_code":    "a1a2a3a4a5a6a7a8a9aaabacadaeafb0b1b2b3b4",
	"sealed_dek":       "ab6304ac7e1b492c03c7d20b3b9b04b032d4cb250bd46047e6be2e03426efe71e70dc895f5f1b8c9f7441099b60302549e5d6f6ebf3e355b078c202eba320b45a66eb0ea0ff20f83a1cb42d9bdd75170",
	"sealed_grant":     "b0468b18bf62068a806d3da9ffa6d0f3d0950a307206f0b3fbbcce2df421a9548d8f098905d767b0cb946dc842acceb30062a90a085e637842233a4dd97798bda8f4e439d75d419376dd1e09cd48680a",
	"wrapped_personal": "68cd20e17d8b260a30e51149ae5d58975eee3f81120acc48b408e278f7e89db5868b484437e871dbd7d5a0ecc14a3cfc135faf5806805cefb23dc6c2",
	"wrapped_recovery": "38bad94f646d35a71bb41a69041eb29a35a4b876deca75939aa77925faa6a82bae161abdfdd52d3e6f0f0159e763de077094e82c2e2c4bd12b991d5c",
}

func TestEscrowReadsFrozenV1BindingsAndCiphertexts(t *testing.T) {
	decoded := make(map[string][]byte)
	for name, value := range legacyEscrowV1 {
		if name == "baseline" {
			continue
		}
		b, err := hex.DecodeString(value)
		if err != nil || len(b) == 0 {
			t.Fatalf("fixture %s: %v", name, err)
		}
		decoded[name] = b
	}
	read := func(name string) []byte { return decoded[name] }
	private, public := read("escrow_private"), read("escrow_public")
	admin, personal := read("admin_private"), read("personal_private")
	dek, code := read("dek"), read("recovery_code")
	for _, tc := range []struct {
		name string
		open func() ([]byte, error)
		want []byte
	}{
		{"DEK", func() ([]byte, error) { return OpenDEK(private, "test-key-v1", "fixture-user", read("sealed_dek")) }, dek},
		{"grant", func() ([]byte, error) { return OpenGrant(admin, "test-key-v1", "fixture-admin", read("sealed_grant")) }, private},
		{"personal", func() ([]byte, error) { return UnwrapPersonalKey(dek, "fixture-user", read("wrapped_personal")) }, personal},
		{"recovery", func() ([]byte, error) { return OpenEscrowRecovery(code, "test-key-v1", read("wrapped_recovery")) }, private},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.open()
			if err != nil || !bytes.Equal(got, tc.want) {
				t.Fatalf("legacy %s: %x, %v", tc.name, got, err)
			}
		})
	}
	pin, err := EscrowPin(dek, "fixture-user", "test-key-v1", public)
	if err != nil || !bytes.Equal(pin, read("pin")) {
		t.Fatalf("legacy PIN changed: %x, %v", pin, err)
	}
	attestation, err := Attestation(private, "fixture-user", read("personal_public"))
	if err != nil || !bytes.Equal(attestation, read("attestation")) {
		t.Fatalf("legacy attestation changed: %x, %v", attestation, err)
	}
	for _, tc := range []struct {
		name string
		open func() ([]byte, error)
	}{
		{"DEK owner", func() ([]byte, error) { return OpenDEK(private, "test-key-v1", "another-user", read("sealed_dek")) }},
		{"grant owner", func() ([]byte, error) { return OpenGrant(admin, "test-key-v1", "another-admin", read("sealed_grant")) }},
		{"personal owner", func() ([]byte, error) { return UnwrapPersonalKey(dek, "another-user", read("wrapped_personal")) }},
		{"recovery key", func() ([]byte, error) { return OpenEscrowRecovery(code, "another-key", read("wrapped_recovery")) }},
		{"purpose", func() ([]byte, error) { return OpenGrant(private, "test-key-v1", "fixture-user", read("sealed_dek")) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.open()
			if err == nil || len(got) != 0 {
				t.Fatal("mismatched binding released plaintext")
			}
		})
	}
}
