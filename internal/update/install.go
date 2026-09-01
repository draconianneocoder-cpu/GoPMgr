// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// maxArtifactBytes bounds the downloaded installer/disk-image size.
// GoPMgr's Wails binary (including the embedded WebView2 bootstrapper on
// Windows and the bundled frontend) is tens of MB; 500 MB leaves generous
// headroom while still bounding memory/disk exposure from a compromised or
// misbehaving download server. A var, not a const, so tests can shrink it
// rather than literally downloading hundreds of MB to exercise the
// oversized-artifact path — mirrors ManifestURL/UpdateChannelPublicKey/
// httpTransport's existing test-seam pattern in check.go.
var maxArtifactBytes int64 = 500 * 1024 * 1024

// currentGOOS is runtime.GOOS by default. A var, not a direct
// runtime.GOOS reference at the call site, so tests can force a
// supported platform regardless of which OS actually runs `go test` —
// mirrors launcher/maxArtifactBytes/httpTransport's existing test-seam
// pattern. Without this, every test exercising the download/verify/
// launch path could only ever pass on darwin or windows, silently
// never running in CI on a standard Linux runner.
var currentGOOS = runtime.GOOS

// installerExtension maps the platforms the release pipeline actually
// signs manifests for (.github/workflows/release.yml: darwin-arm64,
// windows-amd64) to their artifact's file extension. Any other GOOS
// (including linux, deliberately excluded from the update channel — see
// this package's doc comment) returns an error: DownloadAndInstall should
// never be reachable there, since CheckLatest's own platform-match check
// in validatePayload already keeps Status.UpdateAvailable false there —
// but that is asserted explicitly rather than silently doing something
// platform-wrong if it ever is reached.
func installerExtension(goos string) (string, error) {
	switch goos {
	case "darwin":
		return ".dmg", nil
	case "windows":
		return ".exe", nil
	default:
		return "", fmt.Errorf("update: automatic install is not supported on %s", goos)
	}
}

// launcher hands a verified, downloaded installer artifact to the
// platform's own trusted install UX (see install_darwin.go /
// install_windows.go / install_other.go). It deliberately does NOT
// silently replace GoPMgr's own running binary or bundle: on darwin it
// opens the .dmg in Finder (the platform's standard drag-to-Applications
// flow); on windows it launches the NSIS installer as a detached process
// (which itself requests UAC elevation). Overridable in tests, mirroring
// httpTransport in check.go.
var launcher = defaultLauncher

func newDownloadClient() *http.Client {
	return &http.Client{Timeout: 5 * time.Minute, Transport: httpTransport}
}

// DownloadAndInstall re-runs CheckLatest to obtain a freshly-verified
// download URL and SHA-256 — it never accepts a URL or hash from the
// caller. This closes off an entire attack class: a compromised or buggy
// frontend cannot pass DownloadAndInstall an arbitrary URL, because there
// is no parameter for one. The Ed25519 signature verification inside
// CheckLatest is what makes it safe to download, write to disk, and hand
// the result to the OS installer.
//
// destDir is the directory the verified artifact is published into
// (typically <user data dir>/updates); it is created if missing.
//
// Returns the path to the downloaded, verified artifact on success. The
// caller (the App layer) is responsible for surfacing the launcher's
// outcome to the user and, on Windows, for quitting GoPMgr through the
// existing guarded-quit path so the installer can replace the running
// executable.
func DownloadAndInstall(ctx context.Context, destDir string) (string, error) {
	st, err := CheckLatest(ctx)
	if err != nil {
		return "", fmt.Errorf("update: re-verify before install: %w", err)
	}
	if st.Error != "" {
		return "", fmt.Errorf("update: re-verify before install: %s", st.Error)
	}
	if !st.Configured {
		return "", errors.New("update: no update channel configured")
	}
	if !st.UpdateAvailable {
		return "", errors.New("update: no update available")
	}

	ext, err := installerExtension(currentGOOS)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return "", fmt.Errorf("update: create update directory: %w", err)
	}

	// st.Latest already passed validatePayload's semver.IsValid("v"+...)
	// check inside CheckLatest, so it is constrained to SemVer's character
	// set — safe to interpolate into a filename without a separate
	// sanitizer, whose presence would wrongly imply this string is
	// untrusted when the Ed25519 signature already establishes it came
	// from the release pipeline's own key.
	artifactPath := filepath.Join(destDir, fmt.Sprintf("GoPMgr-%s%s", st.Latest, ext))

	if err := downloadVerified(ctx, st.DownloadURL, st.SHA256, artifactPath); err != nil {
		return "", err
	}

	if err := launcher(artifactPath); err != nil {
		return "", fmt.Errorf("update: launch installer: %w", err)
	}
	return artifactPath, nil
}

// downloadVerified fetches downloadURL, hashing the body as it streams to
// a private 0600 temporary file in destPath's directory. The temporary
// file is published (renamed) to destPath only if the computed SHA-256
// matches want — on any mismatch, including an oversized body, the
// temporary file is removed and neither downloadVerified nor any caller
// ever sees a path to unverified content.
func downloadVerified(ctx context.Context, downloadURL, want, destPath string) (err error) {
	u, perr := url.Parse(downloadURL)
	if perr != nil || u.Scheme != "https" || u.Host == "" {
		return errors.New("update: download URL must be HTTPS")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("update: build download request: %w", err)
	}
	resp, err := newDownloadClient().Do(req)
	if err != nil {
		return fmt.Errorf("update: download: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("update: download: HTTP %d", resp.StatusCode)
	}

	dir := filepath.Dir(destPath)
	temp, err := os.CreateTemp(dir, ".gopmgr-update-*")
	if err != nil {
		return fmt.Errorf("update: create temporary file: %w", err)
	}
	tempPath := temp.Name()
	defer func() {
		_ = temp.Close()
		if err != nil {
			_ = os.Remove(tempPath)
		}
	}()
	if err := temp.Chmod(0o600); err != nil {
		return fmt.Errorf("update: set temporary file permissions: %w", err)
	}

	hasher := sha256.New()
	limited := io.LimitReader(resp.Body, maxArtifactBytes+1)
	written, err := io.Copy(io.MultiWriter(temp, hasher), limited)
	if err != nil {
		return fmt.Errorf("update: download body: %w", err)
	}
	if written > maxArtifactBytes {
		return fmt.Errorf("update: artifact exceeds %d bytes", maxArtifactBytes)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("update: sync temporary file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("update: close temporary file: %w", err)
	}

	got := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("update: downloaded artifact checksum mismatch: got %s, want %s", got, want)
	}

	if err := os.Rename(tempPath, destPath); err != nil {
		return fmt.Errorf("update: publish downloaded artifact: %w", err)
	}
	return nil
}
