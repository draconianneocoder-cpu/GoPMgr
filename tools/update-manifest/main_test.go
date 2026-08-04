// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"gopmgr/internal/update"
)

func TestRunCreatesVerifiableManifest(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_UPDATE_PRIVATE_KEY", base64.StdEncoding.EncodeToString(priv))
	dir := t.TempDir()
	artifact := filepath.Join(dir, "gopmgr.pkg")
	if err := os.WriteFile(artifact, []byte("installer"), 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "manifest.json")
	if err := run("1.1.0-beta.1", "beta", "darwin", "arm64", artifact,
		"https://example.test/gopmgr.pkg", output, "TEST_UPDATE_PRIVATE_KEY"); err != nil {
		t.Fatalf("run: %v", err)
	}
	raw, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := update.VerifyManifest(raw, pub)
	if err != nil {
		t.Fatalf("VerifyManifest: %v", err)
	}
	if payload.LatestVersion != "1.1.0-beta.1" || len(payload.SHA256) != 64 {
		t.Fatalf("payload = %#v", payload)
	}
}
