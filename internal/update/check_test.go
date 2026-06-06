// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package update

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"io"
	"strings"
	"testing"
)

func withUpdateConfig(t *testing.T, url, key string) {
	t.Helper()
	oldURL := ManifestURL
	oldKey := UpdateChannelPublicKey
	ManifestURL = url
	UpdateChannelPublicKey = key
	t.Cleanup(func() {
		ManifestURL = oldURL
		UpdateChannelPublicKey = oldKey
	})
}

func testPublicKey(t *testing.T) string {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	return base64.StdEncoding.EncodeToString(pub)
}

func TestCheckLatestRejectsNonHTTPSManifestURL(t *testing.T) {
	withUpdateConfig(t, "http://updates.example.test/manifest.json", testPublicKey(t))

	st, err := CheckLatest(context.Background())
	if err != nil {
		t.Fatalf("CheckLatest returned startup error: %v", err)
	}
	if !st.Configured {
		t.Fatal("expected configured status for URL plus public key")
	}
	if !strings.Contains(strings.ToLower(st.Error), "https") {
		t.Fatalf("status error = %q, want HTTPS failure", st.Error)
	}
}

func TestReadManifestBodyRejectsOversizedResponses(t *testing.T) {
	body := strings.NewReader(strings.Repeat("x", int(maxManifestBytes)+1))

	_, err := readManifestBody(body)
	if err == nil {
		t.Fatal("expected oversized manifest error")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Fatalf("error = %q, want too large", err)
	}

	got, err := readManifestBody(io.LimitReader(strings.NewReader("ok"), maxManifestBytes))
	if err != nil {
		t.Fatalf("readManifestBody small response: %v", err)
	}
	if string(got) != "ok" {
		t.Fatalf("body = %q, want ok", got)
	}
}
