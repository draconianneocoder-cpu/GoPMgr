// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package crypto

import (
	"bytes"
	"errors"
	"testing"
)

func mustKeyPair(t *testing.T) (priv, pub []byte) {
	t.Helper()
	priv, pub, err := GenerateEscrowKeyPair()
	if err != nil {
		t.Fatalf("GenerateEscrowKeyPair: %v", err)
	}
	return priv, pub
}

func mustDEK(t *testing.T) []byte {
	t.Helper()
	dek, err := GenerateDEK()
	if err != nil {
		t.Fatalf("GenerateDEK: %v", err)
	}
	return dek
}

func TestEscrowPublicKeyIsDerivedFromThePrivateKey(t *testing.T) {
	priv, pub := mustKeyPair(t)
	got, err := EscrowPublicKey(priv)
	if err != nil || !bytes.Equal(got, pub) {
		t.Fatalf("EscrowPublicKey = %x, %v; want %x", got, err, pub)
	}
	_, other := mustKeyPair(t)
	if bytes.Equal(other, pub) {
		t.Fatal("two generated key pairs share a public key")
	}
	if _, err := EscrowPublicKey([]byte("short")); err == nil {
		t.Fatal("EscrowPublicKey accepted a malformed private key")
	}
}

func TestSealedDEKOpensOnlyForItsAccountAndEscrowKey(t *testing.T) {
	priv, pub := mustKeyPair(t)
	dek := mustDEK(t)
	sealed, err := SealDEK(pub, "key-1", "alice", dek)
	if err != nil {
		t.Fatalf("SealDEK: %v", err)
	}
	got, err := OpenDEK(priv, "key-1", "alice", sealed)
	if err != nil || !bytes.Equal(got, dek) {
		t.Fatalf("OpenDEK = %x, %v; want the sealed DEK", got, err)
	}

	again, err := SealDEK(pub, "key-1", "alice", dek)
	if err != nil || bytes.Equal(again, sealed) {
		t.Fatalf("sealing twice gave the same bytes (%v); each seal must be fresh", err)
	}

	otherPriv, _ := mustKeyPair(t)
	for name, open := range map[string]func() ([]byte, error){
		"another account":    func() ([]byte, error) { return OpenDEK(priv, "key-1", "bob", sealed) },
		"another escrow key": func() ([]byte, error) { return OpenDEK(priv, "key-2", "alice", sealed) },
		"the wrong key":      func() ([]byte, error) { return OpenDEK(otherPriv, "key-1", "alice", sealed) },
		"read as a grant":    func() ([]byte, error) { return OpenGrant(priv, "key-1", "alice", sealed) },
		"tampered":           func() ([]byte, error) { return OpenDEK(priv, "key-1", "alice", flipLastByte(sealed)) },
		"truncated":          func() ([]byte, error) { return OpenDEK(priv, "key-1", "alice", sealed[:10]) },
	} {
		if got, err := open(); err == nil {
			t.Errorf("%s: opened to %x, want a failure", name, got)
		}
	}
	if _, err := SealDEK(pub, "key-1", "alice", []byte("short")); !errors.Is(err, ErrBadDEK) {
		t.Fatalf("SealDEK of a short key = %v, want ErrBadDEK", err)
	}
}

func TestGrantOpensOnlyForItsAdministratorAndEscrowKey(t *testing.T) {
	escrowPriv, _ := mustKeyPair(t)
	adminPriv, adminPub := mustKeyPair(t)
	sealed, err := SealGrant(adminPub, "key-1", "alice", escrowPriv)
	if err != nil {
		t.Fatalf("SealGrant: %v", err)
	}
	got, err := OpenGrant(adminPriv, "key-1", "alice", sealed)
	if err != nil || !bytes.Equal(got, escrowPriv) {
		t.Fatalf("OpenGrant = %v; want the escrow private key", err)
	}
	for name, open := range map[string]func() ([]byte, error){
		"another administrator": func() ([]byte, error) { return OpenGrant(adminPriv, "key-1", "bob", sealed) },
		"another escrow key":    func() ([]byte, error) { return OpenGrant(adminPriv, "key-2", "alice", sealed) },
		"read as a sealed DEK":  func() ([]byte, error) { return OpenDEK(adminPriv, "key-1", "alice", sealed) },
	} {
		if _, err := open(); err == nil {
			t.Errorf("%s: opened, want a failure", name)
		}
	}
}

func TestPersonalKeyUnwrapsOnlyForItsOwnerAndDEK(t *testing.T) {
	priv, _ := mustKeyPair(t)
	dek := mustDEK(t)
	wrapped, err := WrapPersonalKey(dek, "alice", priv)
	if err != nil {
		t.Fatalf("WrapPersonalKey: %v", err)
	}
	got, err := UnwrapPersonalKey(dek, "alice", wrapped)
	if err != nil || !bytes.Equal(got, priv) {
		t.Fatalf("UnwrapPersonalKey = %v; want the personal private key", err)
	}
	for name, unwrap := range map[string]func() ([]byte, error){
		"another account": func() ([]byte, error) { return UnwrapPersonalKey(dek, "bob", wrapped) },
		"another DEK":     func() ([]byte, error) { return UnwrapPersonalKey(mustDEK(t), "alice", wrapped) },
		"tampered":        func() ([]byte, error) { return UnwrapPersonalKey(dek, "alice", flipLastByte(wrapped)) },
		"too short":       func() ([]byte, error) { return UnwrapPersonalKey(dek, "alice", wrapped[:4]) },
	} {
		if _, err := unwrap(); !errors.Is(err, ErrEscrowOpen) {
			t.Errorf("%s: err = %v, want ErrEscrowOpen", name, err)
		}
	}
	if _, err := WrapPersonalKey([]byte("short"), "alice", priv); !errors.Is(err, ErrBadDEK) {
		t.Fatalf("WrapPersonalKey with a short DEK = %v, want ErrBadDEK", err)
	}
}

func TestEscrowPinVerifiesOnlyForItsOwnerAndEscrowKey(t *testing.T) {
	dek := mustDEK(t)
	_, pub := mustKeyPair(t)
	_, otherPub := mustKeyPair(t)
	pin, err := EscrowPin(dek, "alice", "key-1", pub)
	if err != nil {
		t.Fatalf("EscrowPin: %v", err)
	}
	if ok, err := VerifyEscrowPin(dek, "alice", "key-1", pub, pin); err != nil || !ok {
		t.Fatalf("VerifyEscrowPin of its own pin = %v, %v; want true", ok, err)
	}
	for name, verify := range map[string]func() (bool, error){
		"another account":       func() (bool, error) { return VerifyEscrowPin(dek, "bob", "key-1", pub, pin) },
		"another escrow key id": func() (bool, error) { return VerifyEscrowPin(dek, "alice", "key-2", pub, pin) },
		"a swapped public key":  func() (bool, error) { return VerifyEscrowPin(dek, "alice", "key-1", otherPub, pin) },
		"another account's DEK": func() (bool, error) { return VerifyEscrowPin(mustDEK(t), "alice", "key-1", pub, pin) },
		"a tampered pin":        func() (bool, error) { return VerifyEscrowPin(dek, "alice", "key-1", pub, flipLastByte(pin)) },
	} {
		if ok, err := verify(); err != nil || ok {
			t.Errorf("%s: verified = %v, %v; want false", name, ok, err)
		}
	}
}

func TestAttestationVerifiesOnlyForItsAccountAndKey(t *testing.T) {
	escrowPriv, _ := mustKeyPair(t)
	otherEscrowPriv, _ := mustKeyPair(t)
	_, personalPub := mustKeyPair(t)
	_, swappedPub := mustKeyPair(t)
	att, err := Attestation(escrowPriv, "alice", personalPub)
	if err != nil {
		t.Fatalf("Attestation: %v", err)
	}
	if ok, err := VerifyAttestation(escrowPriv, "alice", personalPub, att); err != nil || !ok {
		t.Fatalf("VerifyAttestation of its own attestation = %v, %v; want true", ok, err)
	}
	for name, verify := range map[string]func() (bool, error){
		"another account":      func() (bool, error) { return VerifyAttestation(escrowPriv, "bob", personalPub, att) },
		"a swapped public key": func() (bool, error) { return VerifyAttestation(escrowPriv, "alice", swappedPub, att) },
		"another escrow key":   func() (bool, error) { return VerifyAttestation(otherEscrowPriv, "alice", personalPub, att) },
	} {
		if ok, err := verify(); err != nil || ok {
			t.Errorf("%s: verified = %v, %v; want false", name, ok, err)
		}
	}
	if _, err := Attestation(nil, "alice", personalPub); err == nil {
		t.Fatal("Attestation with no escrow key succeeded")
	}
}

// The attestation key comes from the serialized escrow private key, so it
// must not change when that key is re-serialized or passed through a grant
// (X25519 serialization clamps the key).
func TestAttestationSurvivesReserializingTheEscrowKey(t *testing.T) {
	escrowPriv, _ := mustKeyPair(t)
	adminPriv, adminPub := mustKeyPair(t)
	_, personalPub := mustKeyPair(t)
	want, err := Attestation(escrowPriv, "alice", personalPub)
	if err != nil {
		t.Fatalf("Attestation: %v", err)
	}

	k, err := escrowKEM.NewPrivateKey(escrowPriv)
	if err != nil {
		t.Fatalf("NewPrivateKey: %v", err)
	}
	reserialized, err := k.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	sealed, err := SealGrant(adminPub, "key-1", "alice", escrowPriv)
	if err != nil {
		t.Fatalf("SealGrant: %v", err)
	}
	fromGrant, err := OpenGrant(adminPriv, "key-1", "alice", sealed)
	if err != nil {
		t.Fatalf("OpenGrant: %v", err)
	}
	for name, key := range map[string][]byte{"reserialized": reserialized, "from a grant": fromGrant} {
		if ok, err := VerifyAttestation(key, "alice", personalPub, want); err != nil || !ok {
			t.Errorf("attestation with the %s escrow key = %v, %v; want it to verify", name, ok, err)
		}
	}
}

// Different fields must never encode the same, or a value bound to one
// owner could verify for another.
func TestBindFieldsIsUnambiguous(t *testing.T) {
	a := bindFields("label", []byte("ab"), []byte("c"))
	b := bindFields("label", []byte("a"), []byte("bc"))
	c := bindFields("label", []byte("abc"))
	if bytes.Equal(a, b) || bytes.Equal(a, c) || bytes.Equal(b, c) {
		t.Fatalf("bindFields collided: %x %x %x", a, b, c)
	}
	if bytes.Equal(bindFields("x/v1", []byte("u")), bindFields("x/v2", []byte("u"))) {
		t.Fatal("bindFields ignores the label")
	}
}

func TestNewEscrowKeyIDIsRandom(t *testing.T) {
	a, err := NewEscrowKeyID()
	if err != nil || len(a) != 32 {
		t.Fatalf("NewEscrowKeyID = %q, %v; want 32 hex characters", a, err)
	}
	b, _ := NewEscrowKeyID()
	if a == b {
		t.Fatal("two escrow key ids are equal")
	}
	if EscrowKEMID != 0x0020 {
		t.Fatalf("EscrowKEMID = %#x, want DHKEM(X25519, HKDF-SHA256) 0x0020", EscrowKEMID)
	}
}

func flipLastByte(b []byte) []byte {
	out := bytes.Clone(b)
	out[len(out)-1] ^= 0x01
	return out
}
