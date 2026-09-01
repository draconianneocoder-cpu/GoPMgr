// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package update fetches a signed release manifest over HTTPS and
// reports whether a newer GoPMgr version is available.
//
// Threat model: a malicious upstream or compromised TLS endpoint
// must NOT be able to convince GoPMgr that a downgrade or fake
// release exists. We pin a single Ed25519 public key
// (UpdateChannelPublicKey, set by the release pipeline at build
// time) and reject any manifest whose signature doesn't verify.
//
// The current binary's version for update purposes is held in
// CurrentVersion (initialized from cli.Version, independently
// overridable via -ldflags in release builds, so it can diverge). The
// CLI `--update` flag prints a one-line status; the GUI Settings panel
// calls CheckLatest() and shows the result.
package update

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"

	"golang.org/x/mod/semver"

	"gopmgr/internal/cli"
)

// ManifestURL is the URL the binary fetches. Override at build time:
//
//	go build -ldflags "-X gopmgr/internal/update.ManifestURL=https://gopmgr.example/updates.json"
//
// Empty string disables the update check (useful for offline /
// distribution-managed builds).
var ManifestURL = ""

// UpdateChannelPublicKey is the base64-encoded Ed25519 public key
// the release pipeline signs with. Override at build time via
// -ldflags like ManifestURL above. Empty key disables verification
// AND the update check, fail-closed.
var UpdateChannelPublicKey = ""

// CurrentVersion and UpdateChannel are separate from the package-manager-safe
// cli.Version. Release builds inject the exact tag and channel with -ldflags.
var CurrentVersion = cli.Version
var UpdateChannel = "stable"

const maxManifestBytes int64 = 64 * 1024

// httpTransport overrides the HTTP client's Transport in tests. Nil in
// production: http.Client falls back to http.DefaultTransport when
// Transport is nil, so this is a no-op behavioral change — it exists only
// to give tests a seam to point CheckLatest at an httptest.NewTLSServer
// without mutating package-level global HTTP state.
var httpTransport http.RoundTripper

// Status is the result returned to the GUI / CLI.
type Status struct {
	Configured      bool   `json:"configured"`       // ManifestURL + key set?
	Current         string `json:"current"`          // running binary version
	Latest          string `json:"latest,omitempty"` // empty when no update
	UpdateAvailable bool   `json:"update_available"`
	ReleaseNotes    string `json:"release_notes,omitempty"`
	DownloadURL     string `json:"download_url,omitempty"`
	SHA256          string `json:"sha256,omitempty"`
	Channel         string `json:"channel"`
	Error           string `json:"error,omitempty"`
	// Platform is always runtime.GOOS, regardless of Configured/Error —
	// unlike every other field here, it doesn't come from the manifest.
	// The GUI uses it to decide whether DownloadAndInstallUpdate's result
	// needs the Windows-only quit-to-install confirmation (see
	// install_windows.go's doc comment: the NSIS installer cannot
	// overwrite a running GoPMgr.exe, but macOS's Finder-mounted .dmg
	// flow needs no such step).
	Platform string `json:"platform"`
}

// CheckLatest performs the full update-check flow:
//   - fetch ManifestURL
//   - verify Ed25519 signature
//   - compare versions
//
// Network and verification errors are surfaced in Status.Error
// rather than as a Go error so the GUI can render them inline. A
// returned error means we couldn't even start the check (no URL or
// bad public key).
func CheckLatest(ctx context.Context) (Status, error) {
	st := Status{Current: CurrentVersion, Channel: UpdateChannel, Platform: runtime.GOOS}
	if ManifestURL == "" || UpdateChannelPublicKey == "" {
		// Not a misconfiguration — the build chose not to wire an
		// update channel. The GUI shows "automatic updates not
		// configured" rather than an error.
		return st, nil
	}
	st.Configured = true

	pubBytes, err := base64.StdEncoding.DecodeString(UpdateChannelPublicKey)
	if err != nil {
		return st, fmt.Errorf("update: decode public key: %w", err)
	}
	if len(pubBytes) != ed25519.PublicKeySize {
		return st, fmt.Errorf("update: public key has wrong length %d", len(pubBytes))
	}

	manifestURL, err := url.Parse(ManifestURL)
	if err != nil || manifestURL.Scheme != "https" || manifestURL.Host == "" {
		st.Error = "update: manifest URL must be HTTPS"
		return st, nil
	}

	client := &http.Client{Timeout: 8 * time.Second, Transport: httpTransport}
	// NewRequestWithContext's own error is disclosed-untested: it only
	// fails on a nil ctx or an unparseable URL/method, and ManifestURL was
	// already parsed successfully above, method is a fixed "GET" literal,
	// and every real caller (Check, the GUI Settings panel) passes a live
	// context. Reaching this branch would require calling CheckLatest with
	// an atypical nil ctx, which isn't a realistic call pattern worth
	// asserting against.
	req, err := http.NewRequestWithContext(ctx, "GET", ManifestURL, nil)
	if err != nil {
		st.Error = err.Error()
		return st, nil
	}
	req.Header.Set("User-Agent", "GoPMgr/"+cli.Version)
	resp, err := client.Do(req)
	if err != nil {
		st.Error = "fetch: " + err.Error()
		return st, nil
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		st.Error = fmt.Sprintf("fetch: HTTP %d", resp.StatusCode)
		return st, nil
	}

	raw, err := readManifestBody(resp.Body)
	if err != nil {
		st.Error = "read: " + err.Error()
		return st, nil
	}

	payload, err := VerifyManifest(raw, pubBytes)
	if err != nil {
		st.Error = err.Error()
		return st, nil
	}
	if err := validatePayload(payload, manifestURL.Host); err != nil {
		st.Error = err.Error()
		return st, nil
	}
	if isNewer(CurrentVersion, payload.LatestVersion) {
		st.Error = fmt.Sprintf("update: refusing downgrade from %s to %s", CurrentVersion, payload.LatestVersion)
		return st, nil
	}

	st.Latest = payload.LatestVersion
	st.ReleaseNotes = payload.ReleaseNotes
	st.DownloadURL = payload.DownloadURL
	st.SHA256 = payload.SHA256
	st.UpdateAvailable = isNewer(payload.LatestVersion, CurrentVersion)
	return st, nil
}

// validatePayload checks the signed payload's contents against the running
// binary's channel/platform/architecture and rejects a malformed download
// target. manifestHost is the already-verified ManifestURL's host: the
// download URL must resolve to the same host. A correctly-signed manifest
// could otherwise point download_url at any HTTPS host, and unlike the
// check-only flow, that URL's contents are now downloaded, written to disk,
// and handed to the OS installer -- same-origin pinning is cheap
// defense-in-depth against a compromised or overly-permissive release
// pipeline, and costs nothing against the real one: the release workflow
// derives both ManifestURL and download_url from the same
// $GITHUB_REPOSITORY (.github/workflows/release.yml), so they are
// same-origin by construction today.
func validatePayload(p Payload, manifestHost string) error {
	if p.Channel != UpdateChannel {
		return fmt.Errorf("update: channel mismatch: manifest %q, binary %q", p.Channel, UpdateChannel)
	}
	if p.Platform != runtime.GOOS || p.Architecture != runtime.GOARCH {
		return fmt.Errorf("update: artifact target mismatch: %s/%s", p.Platform, p.Architecture)
	}
	if !semver.IsValid("v" + p.LatestVersion) {
		return fmt.Errorf("update: invalid semantic version %q", p.LatestVersion)
	}
	download, err := url.Parse(p.DownloadURL)
	if err != nil || download.Scheme != "https" || download.Host == "" {
		return fmt.Errorf("update: download URL must be HTTPS")
	}
	if download.Host != manifestHost {
		return fmt.Errorf("update: download URL host %q does not match manifest host %q", download.Host, manifestHost)
	}
	if len(p.SHA256) != 64 {
		return fmt.Errorf("update: artifact SHA-256 must contain 64 hexadecimal characters")
	}
	for _, c := range strings.ToLower(p.SHA256) {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return fmt.Errorf("update: artifact SHA-256 must contain 64 hexadecimal characters")
		}
	}
	if _, err := time.Parse(time.RFC3339, p.PublishedAt); err != nil {
		return fmt.Errorf("update: invalid publication time: %w", err)
	}
	return nil
}

func readManifestBody(r io.Reader) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(r, maxManifestBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > maxManifestBytes {
		return nil, fmt.Errorf("manifest too large: exceeds %d bytes", maxManifestBytes)
	}
	return raw, nil
}

// osExit is indirected so tests can force Check's startup-error exit path
// without actually terminating the test binary, mirroring
// internal/applog's osExit seam.
var osExit = os.Exit

// Check is the CLI `--update` entry point. Prints a one-line
// summary to stdout.
func Check() {
	st, err := CheckLatest(context.Background())
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "GoPMgr update check failed: %v\n", err)
		osExit(1)
		return // unreached with the real os.Exit; guards the switch below when osExit is faked in tests
	}
	switch {
	case !st.Configured:
		fmt.Printf("GoPMgr %s — automatic update channel not configured.\n", st.Current)
	case st.Error != "":
		fmt.Printf("GoPMgr %s — update check failed: %s\n", st.Current, st.Error)
	case st.UpdateAvailable:
		fmt.Printf("GoPMgr %s — update available: %s\n", st.Current, st.Latest)
		if st.DownloadURL != "" {
			fmt.Printf("  download: %s\n", st.DownloadURL)
		}
	default:
		fmt.Printf("GoPMgr %s — up to date.\n", st.Current)
	}
}

// isNewer compares two semver-ish strings "X.Y.Z[-suffix]" and
// reports whether `latest` is strictly newer than `current`.
// GoPMgr versions are clean semver (e.g. "1.1.0", "1.2.0-rc.1"), but the
// parser still tolerates a legacy "1.2.0-V1-Expansion" style suffix:
// non-numeric tails compare lexically. Wrong answers here only delay an
// update notification, never cause incorrect behaviour, so the simplicity
// trade-off is fine.
func isNewer(latest, current string) bool {
	latest = "v" + strings.TrimPrefix(latest, "v")
	current = "v" + strings.TrimPrefix(current, "v")
	return semver.IsValid(latest) && semver.IsValid(current) && semver.Compare(latest, current) > 0
}

// splitVer and atoi remain narrow helpers for legacy-version diagnostics and
// their regression tests. Update decisions use strict SemVer above.
func splitVer(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r == '.' || r == '-' })
}

func atoi(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, true
}
