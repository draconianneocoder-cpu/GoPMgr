// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package update

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// artifactManifestServer starts a TLS test server that serves a signed
// manifest at "/manifest.json" and artifactBytes at "/gopmgr-artifact" —
// both from the same host, satisfying validatePayload's same-origin
// requirement (check.go). The manifest's SHA256 is computed from
// artifactBytes unless wrongSHA256 is non-empty, in which case that value
// is used instead (to test checksum-mismatch handling). Returns the
// server and the unsigned Payload used to build the manifest, so callers
// can assert against its fields (e.g. LatestVersion).
func artifactManifestServer(t *testing.T, priv ed25519.PrivateKey, version string, artifactBytes []byte, wrongSHA256 string) (*httptest.Server, Payload) {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)

	sum := sha256.Sum256(artifactBytes)
	sha := hex.EncodeToString(sum[:])
	if wrongSHA256 != "" {
		sha = wrongSHA256
	}

	p := validPayload(version)
	p.DownloadURL = srv.URL + "/gopmgr-artifact"
	p.SHA256 = sha
	manifestBytes := signedManifest(t, priv, p)

	mux.HandleFunc("/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(manifestBytes)
	})
	mux.HandleFunc("/gopmgr-artifact", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(artifactBytes)
	})
	return srv, p
}

// swapLauncher replaces the package-level launcher seam for the duration
// of the calling test, restoring it in t.Cleanup — mirrors
// withTestTransport's shape in check_test.go.
func swapLauncher(t *testing.T, fn func(path string) error) *[]string {
	t.Helper()
	calls := &[]string{}
	old := launcher
	launcher = func(path string) error {
		*calls = append(*calls, path)
		return fn(path)
	}
	t.Cleanup(func() { launcher = old })
	return calls
}

func withCurrentVersion(t *testing.T, v string) {
	t.Helper()
	old := CurrentVersion
	CurrentVersion = v
	t.Cleanup(func() { CurrentVersion = old })
}

// withCurrentGOOS forces the platform DownloadAndInstall's installer-
// extension check sees, restoring it in t.Cleanup. Without this, every
// test exercising the download/verify/launch path could only ever pass
// on darwin or windows — the two platforms installerExtension supports —
// which silently meant they never ran on a standard Linux CI runner.
func withCurrentGOOS(t *testing.T, goos string) {
	t.Helper()
	old := currentGOOS
	currentGOOS = goos
	t.Cleanup(func() { currentGOOS = old })
}

// wantArtifactPath's Skipf branch is unreachable today — every current
// caller sets currentGOOS via withCurrentGOOS(t, "darwin") first — but it
// stays as a guard for a future caller that forgets to: a clear skip beats
// a confusing extension mismatch further down the test.
func wantArtifactPath(t *testing.T, destDir, version string) string {
	t.Helper()
	ext, err := installerExtension(currentGOOS)
	if err != nil {
		t.Skipf("installerExtension(%s): %v — no install support on this test platform", currentGOOS, err)
	}
	return filepath.Join(destDir, "GoPMgr-"+version+ext)
}

func tempDownloadFilesIn(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("ReadDir(%s): %v", dir, err)
	}
	var out []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".gopmgr-update-") {
			out = append(out, e.Name())
		}
	}
	return out
}

func TestDownloadAndInstall_HappyPath(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	withCurrentVersion(t, "1.0.0")
	withCurrentGOOS(t, "darwin")

	artifact := []byte("fake installer bytes")
	srv, want := artifactManifestServer(t, priv, "2.0.0", artifact, "")
	withTestTransport(t, srv)
	withUpdateConfig(t, srv.URL+"/manifest.json", base64.StdEncoding.EncodeToString(pub))
	calls := swapLauncher(t, func(string) error { return nil })

	destDir := t.TempDir()
	got, err := DownloadAndInstall(context.Background(), destDir)
	if err != nil {
		t.Fatalf("DownloadAndInstall: %v", err)
	}

	wantPath := wantArtifactPath(t, destDir, want.LatestVersion)
	if got != wantPath {
		t.Errorf("returned path = %q, want %q", got, wantPath)
	}
	if len(*calls) != 1 || (*calls)[0] != wantPath {
		t.Errorf("launcher calls = %v, want exactly one call with %q", *calls, wantPath)
	}

	data, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", wantPath, err)
	}
	if string(data) != string(artifact) {
		t.Errorf("published artifact contents = %q, want %q", data, artifact)
	}
	if len(tempDownloadFilesIn(t, destDir)) != 0 {
		t.Error("no .gopmgr-update-* temporary file should remain after a successful publish")
	}
}

// TestDownloadAndInstall_ChecksumMismatch proves two distinct things a
// test that only checks the error return would miss: the unverified
// temporary file is deleted, AND the launcher is never invoked on
// unverified content. Either one failing alone is a real defect (a
// leftover temp file is a disk-hygiene issue; a launched unverified file
// is a security issue) — collapsing them into one assertion would hide
// which one broke.
func TestDownloadAndInstall_ChecksumMismatch(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	withCurrentVersion(t, "1.0.0")
	withCurrentGOOS(t, "darwin")

	artifact := []byte("fake installer bytes")
	srv, want := artifactManifestServer(t, priv, "2.0.0", artifact, strings.Repeat("f", 64))
	withTestTransport(t, srv)
	withUpdateConfig(t, srv.URL+"/manifest.json", base64.StdEncoding.EncodeToString(pub))
	calls := swapLauncher(t, func(string) error { return nil })

	destDir := t.TempDir()
	_, err := DownloadAndInstall(context.Background(), destDir)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("DownloadAndInstall error = %v, want checksum mismatch", err)
	}

	if len(*calls) != 0 {
		t.Errorf("launcher must not be called when checksum verification fails, got calls %v", *calls)
	}

	finalPath := wantArtifactPath(t, destDir, want.LatestVersion)
	if _, statErr := os.Stat(finalPath); !os.IsNotExist(statErr) {
		t.Errorf("final artifact path must not exist after a checksum mismatch, stat err = %v", statErr)
	}
	if remaining := tempDownloadFilesIn(t, destDir); len(remaining) != 0 {
		t.Errorf("temporary download file(s) %v must be cleaned up after a checksum mismatch", remaining)
	}
}

func TestDownloadAndInstall_OversizedArtifactRejectedBeforeChecksum(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	withCurrentVersion(t, "1.0.0")
	withCurrentGOOS(t, "darwin")

	oldMax := maxArtifactBytes
	maxArtifactBytes = 8
	t.Cleanup(func() { maxArtifactBytes = oldMax })

	artifact := []byte("this artifact is longer than 8 bytes")
	srv, _ := artifactManifestServer(t, priv, "2.0.0", artifact, "")
	withTestTransport(t, srv)
	withUpdateConfig(t, srv.URL+"/manifest.json", base64.StdEncoding.EncodeToString(pub))
	calls := swapLauncher(t, func(string) error { return nil })

	destDir := t.TempDir()
	_, err := DownloadAndInstall(context.Background(), destDir)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("DownloadAndInstall error = %v, want an exceeds-cap error", err)
	}
	if len(*calls) != 0 {
		t.Errorf("launcher must not be called when the artifact exceeds the size cap, got calls %v", *calls)
	}
	if remaining := tempDownloadFilesIn(t, destDir); len(remaining) != 0 {
		t.Errorf("temporary download file(s) %v must be cleaned up after an oversized artifact", remaining)
	}
}

// TestDownloadAndInstall_LauncherErrorKeepsVerifiedArtifact documents
// deliberate behavior: when only the OS hand-off fails (not verification),
// the already-verified artifact is left on disk rather than deleted, since
// the user can still act on it manually.
func TestDownloadAndInstall_LauncherErrorKeepsVerifiedArtifact(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	withCurrentVersion(t, "1.0.0")
	withCurrentGOOS(t, "darwin")

	artifact := []byte("fake installer bytes")
	srv, want := artifactManifestServer(t, priv, "2.0.0", artifact, "")
	withTestTransport(t, srv)
	withUpdateConfig(t, srv.URL+"/manifest.json", base64.StdEncoding.EncodeToString(pub))
	swapLauncher(t, func(string) error { return errors.New("boom") })

	destDir := t.TempDir()
	_, err := DownloadAndInstall(context.Background(), destDir)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("DownloadAndInstall error = %v, want it to surface the launcher error", err)
	}

	finalPath := wantArtifactPath(t, destDir, want.LatestVersion)
	if _, statErr := os.Stat(finalPath); statErr != nil {
		t.Errorf("verified artifact should remain on disk after a launcher failure: %v", statErr)
	}
}

func TestDownloadAndInstall_NoUpdateAvailable(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	withCurrentVersion(t, "2.0.0") // already current; manifest also offers 2.0.0

	srv, _ := selfHostedManifestServer(t, priv, "2.0.0", nil)
	withTestTransport(t, srv)
	withUpdateConfig(t, srv.URL, base64.StdEncoding.EncodeToString(pub))
	calls := swapLauncher(t, func(string) error { return nil })

	_, err := DownloadAndInstall(context.Background(), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "no update available") {
		t.Fatalf("DownloadAndInstall error = %v, want no update available", err)
	}
	if len(*calls) != 0 {
		t.Errorf("launcher must not be called when there is no update available, got calls %v", *calls)
	}
}

func TestDownloadAndInstall_NotConfigured(t *testing.T) {
	withUpdateConfig(t, "", "")
	_, err := DownloadAndInstall(context.Background(), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "no update channel configured") {
		t.Fatalf("DownloadAndInstall error = %v, want no update channel configured", err)
	}
}

func TestDownloadAndInstall_SurfacesCheckLatestFailure(t *testing.T) {
	// An invalid signature is a CheckLatest-level failure (st.Error set),
	// distinct from "not configured" or "no update available" — proves
	// DownloadAndInstall propagates CheckLatest's own verification
	// failures rather than masking them.
	pub, _, _ := ed25519.GenerateKey(nil)
	_, wrongPriv, _ := ed25519.GenerateKey(nil)
	srv, _ := selfHostedManifestServer(t, wrongPriv, "2.0.0", nil)
	withTestTransport(t, srv)
	withUpdateConfig(t, srv.URL, base64.StdEncoding.EncodeToString(pub))

	_, err := DownloadAndInstall(context.Background(), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), ErrInvalidSignature.Error()) {
		t.Fatalf("DownloadAndInstall error = %v, want it to surface the invalid-signature failure", err)
	}
}

func TestDownloadVerifiedRejectsNonHTTPSURL(t *testing.T) {
	destPath := filepath.Join(t.TempDir(), "artifact")
	err := downloadVerified(context.Background(), "http://insecure.example.test/x", strings.Repeat("a", 64), destPath)
	if err == nil || !strings.Contains(err.Error(), "must be HTTPS") {
		t.Fatalf("downloadVerified error = %v, want an HTTPS requirement error", err)
	}
}

func TestInstallerExtension(t *testing.T) {
	tests := []struct {
		goos    string
		want    string
		wantErr bool
	}{
		{"darwin", ".dmg", false},
		{"windows", ".exe", false},
		{"linux", "", true},
		{"freebsd", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			got, err := installerExtension(tt.goos)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("installerExtension(%q) error = nil, want an error", tt.goos)
				}
				return
			}
			if err != nil {
				t.Fatalf("installerExtension(%q) unexpected error: %v", tt.goos, err)
			}
			if got != tt.want {
				t.Errorf("installerExtension(%q) = %q, want %q", tt.goos, got, tt.want)
			}
		})
	}
}
