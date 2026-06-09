// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package update

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
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

// ----- VerifyManifest -----

func signedManifest(t *testing.T, priv ed25519.PrivateKey, p Payload) []byte {
	t.Helper()
	payloadJSON, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal payload: %v", err)
	}
	sig := ed25519.Sign(priv, payloadJSON)
	m := Manifest{
		PayloadB64:   base64.StdEncoding.EncodeToString(payloadJSON),
		SignatureB64: base64.StdEncoding.EncodeToString(sig),
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal manifest: %v", err)
	}
	return raw
}

func TestVerifyManifest_HappyPath(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	want := Payload{LatestVersion: "1.3.0", ReleaseNotes: "Bug fixes", PublishedAt: "2026-06-01T00:00:00Z"}
	got, err := VerifyManifest(signedManifest(t, priv, want), pub)
	if err != nil {
		t.Fatalf("VerifyManifest: %v", err)
	}
	if got.LatestVersion != want.LatestVersion {
		t.Errorf("LatestVersion: got %q, want %q", got.LatestVersion, want.LatestVersion)
	}
}

func TestVerifyManifest_WrongKey_ErrInvalidSignature(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)
	pub2, _, _ := ed25519.GenerateKey(nil)
	raw := signedManifest(t, priv, Payload{LatestVersion: "1.0.0"})
	_, err := VerifyManifest(raw, pub2)
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("err = %v, want ErrInvalidSignature", err)
	}
}

func TestVerifyManifest_BadPublicKeyLength(t *testing.T) {
	_, err := VerifyManifest([]byte(`{"payload":"","signature":""}`), ed25519.PublicKey{})
	if err == nil {
		t.Fatal("expected error for empty public key")
	}
}

func TestVerifyManifest_InvalidManifestJSON(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	_, err := VerifyManifest([]byte("{bad}"), pub)
	if err == nil {
		t.Fatal("expected error for invalid manifest JSON")
	}
}

func TestVerifyManifest_BadPayloadBase64(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	m := Manifest{PayloadB64: "!!!notbase64!!!", SignatureB64: ""}
	raw, _ := json.Marshal(m)
	_, err := VerifyManifest(raw, pub)
	if err == nil {
		t.Fatal("expected error for invalid payload base64")
	}
}

func TestVerifyManifest_InvalidPayloadJSON(t *testing.T) {
	// Valid Ed25519 signature over non-JSON bytes triggers the post-verify parse error.
	pub, priv, _ := ed25519.GenerateKey(nil)
	garbage := []byte("not-json")
	sig := ed25519.Sign(priv, garbage)
	m := Manifest{
		PayloadB64:   base64.StdEncoding.EncodeToString(garbage),
		SignatureB64: base64.StdEncoding.EncodeToString(sig),
	}
	raw, _ := json.Marshal(m)
	_, err := VerifyManifest(raw, pub)
	if err == nil {
		t.Fatal("expected error for non-JSON payload after signature verification")
	}
}

func TestVerifyManifest_BadSignatureBase64(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	payloadJSON, _ := json.Marshal(Payload{LatestVersion: "1.0.0"})
	m := Manifest{
		PayloadB64:   base64.StdEncoding.EncodeToString(payloadJSON),
		SignatureB64: "!!!",
	}
	raw, _ := json.Marshal(m)
	_, err := VerifyManifest(raw, pub)
	if err == nil {
		t.Fatal("expected error for invalid signature base64")
	}
}

// ----- isNewer -----

func TestIsNewer_PatchUpgrade(t *testing.T) {
	if !isNewer("1.2.3", "1.2.2") {
		t.Error("1.2.3 should be newer than 1.2.2")
	}
}

func TestIsNewer_PatchDowngrade(t *testing.T) {
	if isNewer("1.2.2", "1.2.3") {
		t.Error("1.2.2 should not be newer than 1.2.3")
	}
}

func TestIsNewer_Equal(t *testing.T) {
	if isNewer("1.2.3", "1.2.3") {
		t.Error("equal versions should not be newer")
	}
}

func TestIsNewer_MinorUpgrade(t *testing.T) {
	if !isNewer("1.3.0", "1.2.9") {
		t.Error("1.3.0 should be newer than 1.2.9")
	}
}

func TestIsNewer_MajorUpgrade(t *testing.T) {
	if !isNewer("2.0.0", "1.9.9") {
		t.Error("2.0.0 should be newer than 1.9.9")
	}
}

func TestIsNewer_NumericBeatsLexical(t *testing.T) {
	// "10" > "9" numerically but "9" > "10" lexically.
	if !isNewer("1.2.10", "1.2.9") {
		t.Error("1.2.10 should be newer than 1.2.9 (numeric comparison)")
	}
}

func TestIsNewer_SuffixUpgrade(t *testing.T) {
	if !isNewer("1.2.0-V2-Expansion", "1.2.0-V1-Expansion") {
		t.Error("V2 suffix should be newer than V1 suffix")
	}
}

// ----- splitVer -----

func TestSplitVer_DotSeparated(t *testing.T) {
	got := splitVer("1.2.3")
	want := []string{"1", "2", "3"}
	if len(got) != len(want) {
		t.Fatalf("len: got %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSplitVer_DotAndDash(t *testing.T) {
	got := splitVer("1.2.0-V1-Expansion")
	want := []string{"1", "2", "0", "V1", "Expansion"}
	if len(got) != len(want) {
		t.Fatalf("len: got %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSplitVer_Empty(t *testing.T) {
	if got := splitVer(""); len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestSplitVer_NoSeparators(t *testing.T) {
	got := splitVer("42")
	if len(got) != 1 || got[0] != "42" {
		t.Errorf("got %v, want [42]", got)
	}
}

// ----- atoi -----

func TestAtoi_ValidInt(t *testing.T) {
	n, ok := atoi("42")
	if !ok || n != 42 {
		t.Errorf("got (%d, %v), want (42, true)", n, ok)
	}
}

func TestAtoi_Zero(t *testing.T) {
	n, ok := atoi("0")
	if !ok || n != 0 {
		t.Errorf("got (%d, %v), want (0, true)", n, ok)
	}
}

func TestAtoi_Empty(t *testing.T) {
	if _, ok := atoi(""); ok {
		t.Error("expected false for empty string")
	}
}

func TestAtoi_Alpha(t *testing.T) {
	if _, ok := atoi("V1"); ok {
		t.Error("expected false for alpha string")
	}
}

func TestAtoi_Mixed(t *testing.T) {
	if _, ok := atoi("1V"); ok {
		t.Error("expected false for mixed digit/alpha string")
	}
}
