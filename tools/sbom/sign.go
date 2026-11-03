// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
)

// The SBOM signature is a detached Ed25519 signature (FIPS 186-5) over the
// exact bytes of an SBOM file, stored base64-encoded in <file>.sig. Keys use
// the same base64 encoding as the update-manifest key, but the SBOM key is
// separate and serves only this purpose.

const signatureSuffix = ".sig"

func decodeKey(text string, size int, what string) ([]byte, error) {
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(text))
	if err != nil || len(b) != size {
		return nil, fmt.Errorf("%s must be a base64 Ed25519 key of %d bytes", what, size)
	}
	return b, nil
}

// keyID names a public key by the SHA-256 of its raw bytes, so an SBOM can
// say which key signs it before the signature exists.
func keyID(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return "sha-256:" + hex.EncodeToString(sum[:])
}

func readPublicKey(name string) (ed25519.PublicKey, error) {
	b, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	raw, err := decodeKey(string(b), ed25519.PublicKeySize, name)
	if err != nil {
		return nil, err
	}
	return ed25519.PublicKey(raw), nil
}

// signFiles signs each file with the private key and refuses to sign unless
// that key's public half is the committed public key.
func signFiles(priv ed25519.PrivateKey, pub ed25519.PublicKey, files []string) error {
	if !pub.Equal(priv.Public()) {
		return errors.New("the private key does not match the committed public key; refusing to sign")
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		sig := base64.StdEncoding.EncodeToString(ed25519.Sign(priv, b)) + "\n"
		if err := os.WriteFile(f+signatureSuffix, []byte(sig), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// verifyFiles checks each file's detached signature with pub.
func verifyFiles(pub ed25519.PublicKey, files []string) error {
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		s, err := os.ReadFile(f + signatureSuffix)
		if err != nil {
			return fmt.Errorf("%s: no signature: %w", f, err)
		}
		sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(s)))
		if err != nil || len(sig) != ed25519.SignatureSize {
			return fmt.Errorf("%s: malformed signature", f)
		}
		if !ed25519.Verify(pub, b, sig) {
			return fmt.Errorf("%s: signature does not verify with key %s", f, keyID(pub))
		}
		if !strings.Contains(string(b), keyID(pub)) {
			return fmt.Errorf("%s: the SBOM does not name key %s as its signing key", f, keyID(pub))
		}
	}
	return nil
}

// generateKey writes a new private key to privateOut (which must not exist)
// and returns the public key text to commit.
func generateKey(privateOut string) (string, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", err
	}
	f, err := os.OpenFile(privateOut, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	if _, err := f.WriteString(base64.StdEncoding.EncodeToString(priv) + "\n"); err != nil {
		_ = f.Close()
		_ = os.Remove(privateOut) // a partial key would block a retry (O_EXCL)
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(privateOut)
		return "", err
	}
	return base64.StdEncoding.EncodeToString(pub) + "\n", nil
}
