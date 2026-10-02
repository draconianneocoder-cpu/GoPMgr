// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/hpke"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// Administrator escrow for ADR-004 (docs/design/ADR-004-administrator-access-
// to-user-data.md). Key pairs are HPKE (RFC 9180) X25519 keys; seals use
// HKDF-SHA256 and AES-256-GCM. Every protected value is bound to its purpose
// and owner, so a value copied onto another account's row does not open or
// verify there:
//
//   - a user's DEK, sealed to the escrow public key;
//   - the escrow private key, sealed to an administrator's personal public
//     key (a grant);
//   - a personal private key, wrapped under a subkey of its owner's DEK;
//   - an escrow pin, a MAC of the escrow key under its owner's DEK;
//   - an attestation, a MAC of a personal public key under a key derived
//     from the escrow private key.
//
// Private keys and DEKs are passed as byte slices the caller owns and should
// clear when done. Copies made inside crypto/hpke, crypto/ecdh, and
// crypto/aes (key schedules) for one operation cannot be cleared from here.

// Purpose labels. Changing one makes every value made with the old label
// fail to open or verify, so a new format gets a new version suffix.
const (
	escrowDEKLabel         = "gopmgr/escrow/dek/v1"
	escrowGrantLabel       = "gopmgr/escrow/grant/v1"
	escrowPersonalKeyLabel = "gopmgr/escrow/personal-key/v1"
	escrowPinLabel         = "gopmgr/escrow/pin/v1"
	escrowAttestKeyLabel   = "gopmgr/escrow/attest-key/v1"
	escrowAttestLabel      = "gopmgr/escrow/attest/v1"
)

// EscrowKEMID is the HPKE KEM identifier of DHKEM(X25519, HKDF-SHA256).
// Each escrow key stores it, so a later KEM is a rotation, not a migration.
var EscrowKEMID = escrowKEM.ID()

var (
	escrowKEM  = hpke.DHKEM(ecdh.X25519())
	escrowKDF  = hpke.HKDFSHA256()
	escrowAEAD = hpke.AES256GCM()
)

// ErrEscrowOpen is returned when a sealed or wrapped value does not open:
// the wrong key, a value made for another owner or purpose, or tampering.
var ErrEscrowOpen = errors.New("crypto: escrow value does not open")

// bindFields encodes a purpose label and fields unambiguously: each part is
// preceded by its length, so different inputs never encode the same.
func bindFields(label string, fields ...[]byte) []byte {
	out := make([]byte, 0, 64)
	out = appendField(out, []byte(label))
	for _, f := range fields {
		out = appendField(out, f)
	}
	return out
}

func appendField(out, field []byte) []byte {
	out = binary.BigEndian.AppendUint32(out, uint32(len(field)))
	return append(out, field...)
}

// GenerateEscrowKeyPair returns a new HPKE key pair as serialized bytes. It
// is used for the escrow key and for each account's personal key.
func GenerateEscrowKeyPair() (privateKey, publicKey []byte, err error) {
	k, err := escrowKEM.GenerateKey()
	if err != nil {
		return nil, nil, fmt.Errorf("crypto: generate escrow key: %w", err)
	}
	priv, err := k.Bytes()
	if err != nil {
		return nil, nil, fmt.Errorf("crypto: serialize escrow key: %w", err)
	}
	return priv, k.PublicKey().Bytes(), nil
}

// EscrowPublicKey derives the public key of a serialized private key, so a
// session that holds the private key never trusts a public key read from
// disk.
func EscrowPublicKey(privateKey []byte) ([]byte, error) {
	k, err := escrowKEM.NewPrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("crypto: escrow private key: %w", err)
	}
	return k.PublicKey().Bytes(), nil
}

// NewEscrowKeyID returns a random identifier for an escrow key.
func NewEscrowKeyID() (string, error) {
	b := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", fmt.Errorf("crypto: escrow key id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func sealTo(publicKey, info, plaintext []byte) ([]byte, error) {
	pk, err := escrowKEM.NewPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("crypto: escrow public key: %w", err)
	}
	return hpke.Seal(pk, escrowKDF, escrowAEAD, info, plaintext)
}

func openWith(privateKey, info, sealed []byte) ([]byte, error) {
	k, err := escrowKEM.NewPrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("crypto: escrow private key: %w", err)
	}
	plaintext, err := hpke.Open(k, escrowKDF, escrowAEAD, info, sealed)
	if err != nil {
		return nil, ErrEscrowOpen
	}
	return plaintext, nil
}

// SealDEK seals username's DEK to the escrow public key escrowKeyID.
func SealDEK(escrowPublicKey []byte, escrowKeyID, username string, dek []byte) ([]byte, error) {
	if len(dek) != DEKSize {
		return nil, ErrBadDEK
	}
	return sealTo(escrowPublicKey, bindFields(escrowDEKLabel, []byte(escrowKeyID), []byte(username)), dek)
}

// OpenDEK reverses SealDEK. It fails for a DEK sealed for another account
// or escrow key.
func OpenDEK(escrowPrivateKey []byte, escrowKeyID, username string, sealed []byte) ([]byte, error) {
	dek, err := openWith(escrowPrivateKey, bindFields(escrowDEKLabel, []byte(escrowKeyID), []byte(username)), sealed)
	if err != nil {
		return nil, err
	}
	if len(dek) != DEKSize {
		clear(dek)
		return nil, ErrBadDEK
	}
	return dek, nil
}

// SealGrant seals the escrow private key escrowKeyID to administrator
// admin's personal public key.
func SealGrant(adminPublicKey []byte, escrowKeyID, admin string, escrowPrivateKey []byte) ([]byte, error) {
	return sealTo(adminPublicKey, bindFields(escrowGrantLabel, []byte(escrowKeyID), []byte(admin)), escrowPrivateKey)
}

// OpenGrant reverses SealGrant with the administrator's personal private
// key. The caller must check the result against its escrow pin before use.
func OpenGrant(adminPrivateKey []byte, escrowKeyID, admin string, sealed []byte) ([]byte, error) {
	return openWith(adminPrivateKey, bindFields(escrowGrantLabel, []byte(escrowKeyID), []byte(admin)), sealed)
}

// WrapPersonalKey encrypts username's personal private key under a subkey
// of their DEK (AES-256-GCM, a random nonce, the owner bound as additional
// data). The result is the nonce followed by the ciphertext.
func WrapPersonalKey(dek []byte, username string, privateKey []byte) ([]byte, error) {
	aead, err := personalKeyAEAD(dek)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("crypto: personal key nonce: %w", err)
	}
	return aead.Seal(nonce, nonce, privateKey, bindFields(escrowPersonalKeyLabel, []byte(username))), nil
}

// UnwrapPersonalKey reverses WrapPersonalKey.
func UnwrapPersonalKey(dek []byte, username string, wrapped []byte) ([]byte, error) {
	aead, err := personalKeyAEAD(dek)
	if err != nil {
		return nil, err
	}
	if len(wrapped) < aead.NonceSize() {
		return nil, ErrEscrowOpen
	}
	nonce, ciphertext := wrapped[:aead.NonceSize()], wrapped[aead.NonceSize():]
	privateKey, err := aead.Open(nil, nonce, ciphertext, bindFields(escrowPersonalKeyLabel, []byte(username)))
	if err != nil {
		return nil, ErrEscrowOpen
	}
	return privateKey, nil
}

func personalKeyAEAD(dek []byte) (cipher.AEAD, error) {
	key, err := DeriveSubkey(dek, escrowPersonalKeyLabel)
	if err != nil {
		return nil, err
	}
	defer clear(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// EscrowPin returns username's pin of escrow key escrowKeyID, a MAC under a
// subkey of their DEK. A sign-in seals only to an escrow key that matches.
func EscrowPin(dek []byte, username, escrowKeyID string, escrowPublicKey []byte) ([]byte, error) {
	key, err := DeriveSubkey(dek, escrowPinLabel)
	if err != nil {
		return nil, err
	}
	defer clear(key)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(bindFields(escrowPinLabel, []byte(username), []byte(escrowKeyID), escrowPublicKey))
	return mac.Sum(nil), nil
}

// VerifyEscrowPin reports, in constant time, whether pin is username's pin
// of the given escrow key.
func VerifyEscrowPin(dek []byte, username, escrowKeyID string, escrowPublicKey, pin []byte) (bool, error) {
	want, err := EscrowPin(dek, username, escrowKeyID, escrowPublicKey)
	if err != nil {
		return false, err
	}
	return hmac.Equal(want, pin), nil
}

// Attestation returns the escrow key's attestation of username's personal
// public key: a MAC under a key derived from the escrow private key, so
// only an administrator session can make one.
func Attestation(escrowPrivateKey []byte, username string, personalPublicKey []byte) ([]byte, error) {
	if len(escrowPrivateKey) == 0 {
		return nil, errors.New("crypto: empty escrow private key")
	}
	keyMAC := hmac.New(sha256.New, escrowPrivateKey)
	_, _ = keyMAC.Write([]byte(escrowAttestKeyLabel))
	key := keyMAC.Sum(nil)
	defer clear(key)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(bindFields(escrowAttestLabel, []byte(username), personalPublicKey))
	return mac.Sum(nil), nil
}

// VerifyAttestation reports, in constant time, whether attestation is the
// escrow key's attestation of username's personal public key.
func VerifyAttestation(escrowPrivateKey []byte, username string, personalPublicKey, attestation []byte) (bool, error) {
	want, err := Attestation(escrowPrivateKey, username, personalPublicKey)
	if err != nil {
		return false, err
	}
	return hmac.Equal(want, attestation), nil
}
