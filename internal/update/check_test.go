// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package update

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
)

// captureStdout redirects os.Stdout for the duration of fn, mirroring
// internal/cli/parser_test.go's TestPrintVersion_ContainsBanner pattern.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}
	return buf.String()
}

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

type errReader struct{ err error }

func (r errReader) Read([]byte) (int, error) { return 0, r.err }

func TestReadManifestBodyPropagatesReadError(t *testing.T) {
	wantErr := errors.New("boom")
	_, err := readManifestBody(errReader{err: wantErr})
	if !errors.Is(err, wantErr) {
		t.Fatalf("readManifestBody error = %v, want %v", err, wantErr)
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

func validPayload(version string) Payload {
	return Payload{
		LatestVersion: version,
		Channel:       "stable",
		Platform:      runtime.GOOS,
		Architecture:  runtime.GOARCH,
		ReleaseNotes:  "Bug fixes",
		DownloadURL:   "https://updates.example.test/gopmgr.pkg",
		SHA256:        strings.Repeat("a", 64),
		PublishedAt:   "2026-06-01T00:00:00Z",
	}
}

func TestVerifyManifest_HappyPath(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	want := validPayload("1.3.0")
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

// ----- CheckLatest (end-to-end) -----

// withTestTransport points CheckLatest's internal HTTP client at srv by
// overriding the package-level httpTransport seam (see check.go) rather
// than mutating http.DefaultTransport, so this test file never touches
// process-wide HTTP state shared with other packages' tests.
func withTestTransport(t *testing.T, srv *httptest.Server) {
	t.Helper()
	old := httpTransport
	httpTransport = srv.Client().Transport
	t.Cleanup(func() { httpTransport = old })
}

func manifestServer(t *testing.T, body []byte, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestCheckLatestNotConfiguredWhenURLOrKeyMissing(t *testing.T) {
	withUpdateConfig(t, "", "")
	st, err := CheckLatest(context.Background())
	if err != nil {
		t.Fatalf("CheckLatest returned startup error: %v", err)
	}
	if st.Configured {
		t.Fatal("expected Configured=false with no URL or key set")
	}
	if st.Error != "" {
		t.Fatalf("expected no error for the not-configured case, got %q", st.Error)
	}
}

func TestCheckLatestRejectsBadPublicKeyBase64(t *testing.T) {
	withUpdateConfig(t, "https://updates.example.test/manifest.json", "!!!not-base64!!!")
	_, err := CheckLatest(context.Background())
	if err == nil || !strings.Contains(err.Error(), "decode public key") {
		t.Fatalf("CheckLatest error = %v, want decode public key failure", err)
	}
}

func TestCheckLatestRejectsWrongPublicKeyLength(t *testing.T) {
	withUpdateConfig(t, "https://updates.example.test/manifest.json", base64.StdEncoding.EncodeToString([]byte("short")))
	_, err := CheckLatest(context.Background())
	if err == nil || !strings.Contains(err.Error(), "wrong length") {
		t.Fatalf("CheckLatest error = %v, want wrong length failure", err)
	}
}

func TestCheckLatestSurfacesFetchError(t *testing.T) {
	withUpdateConfig(t, "https://updates.example.test/manifest.json", testPublicKey(t))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already-canceled context makes client.Do fail deterministically, no network flakiness

	st, err := CheckLatest(ctx)
	if err != nil {
		t.Fatalf("CheckLatest returned startup error: %v", err)
	}
	if !strings.HasPrefix(st.Error, "fetch: ") {
		t.Fatalf("status error = %q, want fetch: prefix", st.Error)
	}
}

func TestCheckLatestSurfacesNon200Status(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	srv := manifestServer(t, []byte("not found"), http.StatusNotFound)
	withTestTransport(t, srv)
	withUpdateConfig(t, srv.URL, base64.StdEncoding.EncodeToString(pub))

	st, err := CheckLatest(context.Background())
	if err != nil {
		t.Fatalf("CheckLatest returned startup error: %v", err)
	}
	if !strings.Contains(st.Error, "HTTP 404") {
		t.Fatalf("status error = %q, want HTTP 404", st.Error)
	}
}

func TestCheckLatestSurfacesOversizedBodyThroughReadManifestBody(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	srv := manifestServer(t, []byte(strings.Repeat("x", int(maxManifestBytes)+1)), http.StatusOK)
	withTestTransport(t, srv)
	withUpdateConfig(t, srv.URL, base64.StdEncoding.EncodeToString(pub))

	st, err := CheckLatest(context.Background())
	if err != nil {
		t.Fatalf("CheckLatest returned startup error: %v", err)
	}
	// Asserts the "read: " prefix specifically (not just any error) to prove
	// CheckLatest actually routes the HTTP body through readManifestBody and
	// surfaces its error, rather than merely duplicating the unit-level
	// TestReadManifestBodyRejectsOversizedResponses coverage.
	if !strings.HasPrefix(st.Error, "read: ") || !strings.Contains(st.Error, "too large") {
		t.Fatalf("status error = %q, want read: ...too large", st.Error)
	}
}

func TestCheckLatestSurfacesInvalidSignature(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	_, wrongPriv, _ := ed25519.GenerateKey(nil)
	raw := signedManifest(t, wrongPriv, validPayload("1.2.0"))
	srv := manifestServer(t, raw, http.StatusOK)
	withTestTransport(t, srv)
	withUpdateConfig(t, srv.URL, base64.StdEncoding.EncodeToString(pub))

	st, err := CheckLatest(context.Background())
	if err != nil {
		t.Fatalf("CheckLatest returned startup error: %v", err)
	}
	if st.Error != ErrInvalidSignature.Error() {
		t.Fatalf("status error = %q, want %q", st.Error, ErrInvalidSignature.Error())
	}
}

func TestCheckLatestSurfacesValidatePayloadRejection(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	p := validPayload("1.2.0")
	p.Platform = "not-" + runtime.GOOS
	raw := signedManifest(t, priv, p)
	srv := manifestServer(t, raw, http.StatusOK)
	withTestTransport(t, srv)
	withUpdateConfig(t, srv.URL, base64.StdEncoding.EncodeToString(pub))

	st, err := CheckLatest(context.Background())
	if err != nil {
		t.Fatalf("CheckLatest returned startup error: %v", err)
	}
	if !strings.Contains(st.Error, "artifact target mismatch") {
		t.Fatalf("status error = %q, want artifact target mismatch", st.Error)
	}
}

func TestCheckLatestRejectsDowngrade(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	old := CurrentVersion
	CurrentVersion = "5.0.0"
	t.Cleanup(func() { CurrentVersion = old })

	raw := signedManifest(t, priv, validPayload("1.2.0")) // "latest" older than CurrentVersion
	srv := manifestServer(t, raw, http.StatusOK)
	withTestTransport(t, srv)
	withUpdateConfig(t, srv.URL, base64.StdEncoding.EncodeToString(pub))

	st, err := CheckLatest(context.Background())
	if err != nil {
		t.Fatalf("CheckLatest returned startup error: %v", err)
	}
	if !strings.Contains(st.Error, "refusing downgrade") {
		t.Fatalf("status error = %q, want refusing downgrade", st.Error)
	}
	if st.UpdateAvailable {
		t.Fatal("UpdateAvailable must be false when the manifest offers a downgrade")
	}
}

func TestCheckLatestNoUpdateAvailableWhenVersionsMatch(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	old := CurrentVersion
	CurrentVersion = "1.2.0"
	t.Cleanup(func() { CurrentVersion = old })

	raw := signedManifest(t, priv, validPayload("1.2.0"))
	srv := manifestServer(t, raw, http.StatusOK)
	withTestTransport(t, srv)
	withUpdateConfig(t, srv.URL, base64.StdEncoding.EncodeToString(pub))

	st, err := CheckLatest(context.Background())
	if err != nil {
		t.Fatalf("CheckLatest returned startup error: %v", err)
	}
	if st.Error != "" {
		t.Fatalf("unexpected status error: %q", st.Error)
	}
	if st.UpdateAvailable {
		t.Fatal("UpdateAvailable must be false when latest equals current")
	}
	if st.Latest != "1.2.0" {
		t.Fatalf("Latest = %q, want 1.2.0", st.Latest)
	}
}

func TestCheckLatestUpdateAvailableHappyPath(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	old := CurrentVersion
	CurrentVersion = "1.1.0"
	t.Cleanup(func() { CurrentVersion = old })

	want := validPayload("1.2.0")
	raw := signedManifest(t, priv, want)
	srv := manifestServer(t, raw, http.StatusOK)
	withTestTransport(t, srv)
	withUpdateConfig(t, srv.URL, base64.StdEncoding.EncodeToString(pub))

	st, err := CheckLatest(context.Background())
	if err != nil {
		t.Fatalf("CheckLatest returned startup error: %v", err)
	}
	if st.Error != "" {
		t.Fatalf("unexpected status error: %q", st.Error)
	}
	if !st.UpdateAvailable {
		t.Fatal("expected UpdateAvailable=true when latest > current")
	}
	if st.Latest != want.LatestVersion {
		t.Errorf("Latest = %q, want %q", st.Latest, want.LatestVersion)
	}
	if st.ReleaseNotes != want.ReleaseNotes {
		t.Errorf("ReleaseNotes = %q, want %q", st.ReleaseNotes, want.ReleaseNotes)
	}
	if st.DownloadURL != want.DownloadURL {
		t.Errorf("DownloadURL = %q, want %q", st.DownloadURL, want.DownloadURL)
	}
	if st.SHA256 != want.SHA256 {
		t.Errorf("SHA256 = %q, want %q", st.SHA256, want.SHA256)
	}
	if !st.Configured {
		t.Error("expected Configured=true")
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
	if !isNewer("1.2.0-beta.2", "1.2.0-beta.1") {
		t.Error("beta.2 should be newer than beta.1")
	}
}

func TestIsNewer_StableBeatsPrerelease(t *testing.T) {
	if !isNewer("1.2.0", "1.2.0-rc.1") {
		t.Error("stable release should be newer than its release candidate")
	}
	if isNewer("1.2.0-beta.1", "1.2.0") {
		t.Error("prerelease must not be newer than the stable release")
	}
}

func TestIsNewerRejectsInvalidLegacyVersion(t *testing.T) {
	if isNewer("1.2.0-V2-Expansion", "1.2.0") {
		t.Error("non-SemVer release identity must not participate in update ordering")
	}
}

func TestValidatePayloadRejectsChannelMismatch(t *testing.T) {
	p := validPayload("1.2.0")
	p.Channel = "beta"
	if err := validatePayload(p); err == nil || !strings.Contains(err.Error(), "channel mismatch") {
		t.Fatalf("validatePayload error = %v, want channel mismatch", err)
	}
}

func TestValidatePayloadAcceptsWellFormedPayload(t *testing.T) {
	if err := validatePayload(validPayload("1.2.0")); err != nil {
		t.Fatalf("validatePayload rejected a well-formed payload: %v", err)
	}
}

func TestValidatePayloadRejectsPlatformMismatch(t *testing.T) {
	p := validPayload("1.2.0")
	p.Platform = "not-" + runtime.GOOS
	if err := validatePayload(p); err == nil || !strings.Contains(err.Error(), "artifact target mismatch") {
		t.Fatalf("validatePayload error = %v, want artifact target mismatch", err)
	}
}

func TestValidatePayloadRejectsArchitectureMismatch(t *testing.T) {
	p := validPayload("1.2.0")
	p.Architecture = "not-" + runtime.GOARCH
	if err := validatePayload(p); err == nil || !strings.Contains(err.Error(), "artifact target mismatch") {
		t.Fatalf("validatePayload error = %v, want artifact target mismatch", err)
	}
}

func TestValidatePayloadRejectsInvalidSemver(t *testing.T) {
	p := validPayload("not-a-version")
	if err := validatePayload(p); err == nil || !strings.Contains(err.Error(), "invalid semantic version") {
		t.Fatalf("validatePayload error = %v, want invalid semantic version", err)
	}
}

func TestValidatePayloadRejectsNonHTTPSDownloadURL(t *testing.T) {
	p := validPayload("1.2.0")
	p.DownloadURL = "http://updates.example.test/gopmgr.pkg"
	if err := validatePayload(p); err == nil || !strings.Contains(err.Error(), "download URL must be HTTPS") {
		t.Fatalf("validatePayload error = %v, want download URL must be HTTPS", err)
	}
}

func TestValidatePayloadRejectsEmptyDownloadURL(t *testing.T) {
	p := validPayload("1.2.0")
	p.DownloadURL = ""
	if err := validatePayload(p); err == nil || !strings.Contains(err.Error(), "download URL must be HTTPS") {
		t.Fatalf("validatePayload error = %v, want download URL must be HTTPS", err)
	}
}

func TestValidatePayloadRejectsShortSHA256(t *testing.T) {
	p := validPayload("1.2.0")
	p.SHA256 = strings.Repeat("a", 63)
	if err := validatePayload(p); err == nil || !strings.Contains(err.Error(), "64 hexadecimal characters") {
		t.Fatalf("validatePayload error = %v, want 64 hexadecimal characters", err)
	}
}

func TestValidatePayloadRejectsNonHexSHA256(t *testing.T) {
	p := validPayload("1.2.0")
	p.SHA256 = strings.Repeat("g", 64)
	if err := validatePayload(p); err == nil || !strings.Contains(err.Error(), "64 hexadecimal characters") {
		t.Fatalf("validatePayload error = %v, want 64 hexadecimal characters", err)
	}
}

func TestValidatePayloadRejectsUnparseablePublishedAt(t *testing.T) {
	p := validPayload("1.2.0")
	p.PublishedAt = "not-a-timestamp"
	if err := validatePayload(p); err == nil || !strings.Contains(err.Error(), "invalid publication time") {
		t.Fatalf("validatePayload error = %v, want invalid publication time", err)
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

// ----- Check (CLI entry point) -----

func TestCheck_ExitsNonZeroOnStartupError(t *testing.T) {
	withUpdateConfig(t, "https://updates.example.test/manifest.json", "!!!not-base64!!!")
	var gotCode int
	var exited bool
	osExitOld := osExit
	osExit = func(c int) { exited = true; gotCode = c }
	t.Cleanup(func() { osExit = osExitOld })

	out := captureStdout(t, Check)

	if !exited {
		t.Fatal("Check did not call osExit on a CheckLatest startup error")
	}
	if gotCode != 1 {
		t.Fatalf("exit code = %d, want 1", gotCode)
	}
	// The switch-statement print branches must not also run after the
	// faked (non-terminating) osExit — proves the `return` guard added
	// alongside the osExit seam actually short-circuits.
	if out != "" {
		t.Fatalf("stdout = %q, want nothing printed once the startup-error branch handled it", out)
	}
}

func TestCheck_NotConfigured(t *testing.T) {
	withUpdateConfig(t, "", "")
	out := captureStdout(t, Check)
	if !strings.Contains(out, "not configured") {
		t.Fatalf("output = %q, want a not-configured message", out)
	}
}

func TestCheck_ReportsError(t *testing.T) {
	withUpdateConfig(t, "http://updates.example.test/manifest.json", testPublicKey(t))
	out := captureStdout(t, Check)
	if !strings.Contains(out, "update check failed") {
		t.Fatalf("output = %q, want an update-check-failed message", out)
	}
}

func TestCheck_ReportsUpdateAvailableWithDownloadURL(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	old := CurrentVersion
	CurrentVersion = "1.0.0"
	t.Cleanup(func() { CurrentVersion = old })

	want := validPayload("2.0.0")
	raw := signedManifest(t, priv, want)
	srv := manifestServer(t, raw, http.StatusOK)
	withTestTransport(t, srv)
	withUpdateConfig(t, srv.URL, base64.StdEncoding.EncodeToString(pub))

	out := captureStdout(t, Check)
	if !strings.Contains(out, "update available: 2.0.0") {
		t.Fatalf("output = %q, want an update-available message", out)
	}
	if !strings.Contains(out, want.DownloadURL) {
		t.Fatalf("output = %q, want it to include the download URL", out)
	}
}

func TestCheck_ReportsUpToDate(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	old := CurrentVersion
	CurrentVersion = "1.2.0"
	t.Cleanup(func() { CurrentVersion = old })

	raw := signedManifest(t, priv, validPayload("1.2.0"))
	srv := manifestServer(t, raw, http.StatusOK)
	withTestTransport(t, srv)
	withUpdateConfig(t, srv.URL, base64.StdEncoding.EncodeToString(pub))

	out := captureStdout(t, Check)
	if !strings.Contains(out, "up to date") {
		t.Fatalf("output = %q, want an up-to-date message", out)
	}
}
