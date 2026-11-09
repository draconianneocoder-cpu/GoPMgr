// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package crypto

import (
	"bytes"
	"errors"
	"testing"
)

// Escrow keys, sealed DEKs and grants are read back from system.db. These
// tests cover what a corrupted or substituted record can contain: key bytes
// of the wrong length, a low-order X25519 point and a well-formed ciphertext
// of the wrong size.

func TestEscrowSealRejectsUnusablePublicKeys(t *testing.T) {
	dek := mustDEK(t)
	escrowPrivate, _ := mustKeyPair(t)
	keys := map[string][]byte{
		"empty":                    nil,
		"short":                    bytes.Repeat([]byte{9}, 31),
		"long":                     bytes.Repeat([]byte{9}, 33),
		"all-zero low-order point": make([]byte, 32),
	}
	for name, key := range keys {
		t.Run(name, func(t *testing.T) {
			if sealed, err := SealDEK(key, "key-1", "alice", dek); err == nil {
				t.Fatalf("SealDEK sealed %d bytes to an unusable public key", len(sealed))
			}
			if sealed, err := SealGrant(key, "key-1", "admin", escrowPrivate); err == nil {
				t.Fatalf("SealGrant sealed %d bytes to an unusable public key", len(sealed))
			}
		})
	}
}

func TestEscrowOpenRejectsMalformedPrivateKeys(t *testing.T) {
	escrowPrivate, escrowPublic := mustKeyPair(t)
	sealedDEK, err := SealDEK(escrowPublic, "key-1", "alice", mustDEK(t))
	if err != nil {
		t.Fatalf("SealDEK: %v", err)
	}
	adminPrivate, adminPublic := mustKeyPair(t)
	grant, err := SealGrant(adminPublic, "key-1", "admin", escrowPrivate)
	if err != nil {
		t.Fatalf("SealGrant: %v", err)
	}
	for name, key := range map[string][]byte{
		"empty": nil,
		"short": escrowPrivate[:len(escrowPrivate)-1],
		"long":  append(bytes.Clone(adminPrivate), 0),
	} {
		t.Run(name, func(t *testing.T) {
			if dek, err := OpenDEK(key, "key-1", "alice", sealedDEK); err == nil || dek != nil {
				t.Fatalf("OpenDEK = %x, %v; want an error", dek, err)
			}
			if private, err := OpenGrant(key, "key-1", "admin", grant); err == nil || private != nil {
				t.Fatalf("OpenGrant = %x, %v; want an error", private, err)
			}
		})
	}
}

// Anyone who can read the escrow public key can seal a value of any length
// under the DEK binding, so OpenDEK's length check is the only thing that
// stops a short or long value from being used as an account's DEK.
func TestOpenDEKRejectsAnAuthenticValueOfTheWrongLength(t *testing.T) {
	escrowPrivate, escrowPublic := mustKeyPair(t)
	info, err := bindFields(escrowDEKLabel, "key-1", "alice")
	if err != nil {
		t.Fatalf("bindFields: %v", err)
	}
	for _, size := range []int{0, DEKSize - 1, DEKSize + 1} {
		sealed, err := sealTo(escrowPublic, info, bytes.Repeat([]byte{7}, size))
		if err != nil {
			t.Fatalf("sealTo(%d bytes): %v", size, err)
		}
		if dek, err := OpenDEK(escrowPrivate, "key-1", "alice", sealed); !errors.Is(err, ErrBadDEK) || dek != nil {
			t.Fatalf("OpenDEK(%d-byte value) = %x, %v; want ErrBadDEK", size, dek, err)
		}
	}
}

func TestDEKSubkeyOperationsRejectAWrongSizeDEK(t *testing.T) {
	dek := mustDEK(t)
	_, escrowPublic := mustKeyPair(t)
	wrapped, err := WrapPersonalKey(dek, "alice", []byte("personal key"))
	if err != nil {
		t.Fatalf("WrapPersonalKey: %v", err)
	}
	pin, err := EscrowPin(dek, "alice", "key-1", escrowPublic)
	if err != nil {
		t.Fatalf("EscrowPin: %v", err)
	}
	for _, bad := range [][]byte{nil, dek[:DEKSize-1], append(bytes.Clone(dek), 0)} {
		if _, err := WrapPersonalKey(bad, "alice", []byte("personal key")); !errors.Is(err, ErrBadDEK) {
			t.Fatalf("WrapPersonalKey with a %d-byte DEK: %v; want ErrBadDEK", len(bad), err)
		}
		if _, err := UnwrapPersonalKey(bad, "alice", wrapped); !errors.Is(err, ErrBadDEK) {
			t.Fatalf("UnwrapPersonalKey with a %d-byte DEK: %v; want ErrBadDEK", len(bad), err)
		}
		if _, err := EscrowPin(bad, "alice", "key-1", escrowPublic); !errors.Is(err, ErrBadDEK) {
			t.Fatalf("EscrowPin with a %d-byte DEK: %v; want ErrBadDEK", len(bad), err)
		}
		if ok, err := VerifyEscrowPin(bad, "alice", "key-1", escrowPublic, pin); ok || !errors.Is(err, ErrBadDEK) {
			t.Fatalf("VerifyEscrowPin with a %d-byte DEK = %v, %v; want false, ErrBadDEK", len(bad), ok, err)
		}
	}
}

func TestDeriveSubkeyRequiresAContext(t *testing.T) {
	if key, err := DeriveSubkey(mustDEK(t), ""); err == nil || key != nil {
		t.Fatalf("DeriveSubkey with an empty context = %x, %v; want an error", key, err)
	}
}

func TestWrapEscrowRecoveryRejectsAWrongLengthCode(t *testing.T) {
	escrowPrivate, _ := mustKeyPair(t)
	for _, size := range []int{0, EscrowRecoveryCodeSize - 1, EscrowRecoveryCodeSize + 1} {
		if wrapped, err := WrapEscrowRecovery(make([]byte, size), "key-1", escrowPrivate); err == nil || wrapped != nil {
			t.Fatalf("WrapEscrowRecovery with a %d-byte code = %x, %v; want an error", size, wrapped, err)
		}
	}
}

func TestVerifyAttestationRejectsAnEmptyEscrowKey(t *testing.T) {
	escrowPrivate, _ := mustKeyPair(t)
	_, personalPublic := mustKeyPair(t)
	attestation, err := Attestation(escrowPrivate, "alice", personalPublic)
	if err != nil {
		t.Fatalf("Attestation: %v", err)
	}
	if ok, err := VerifyAttestation(nil, "alice", personalPublic, attestation); ok || err == nil {
		t.Fatalf("VerifyAttestation with an empty escrow key = %v, %v; want false and an error", ok, err)
	}
}
