// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package update fetches a signed release manifest over HTTPS and
// reports whether a newer PMForge version is available.
//
// Threat model: a malicious upstream or compromised TLS endpoint
// must NOT be able to convince PMForge that a downgrade or fake
// release exists. We pin a single Ed25519 public key
// (UpdateChannelPublicKey, set by the release pipeline at build
// time) and reject any manifest whose signature doesn't verify.
//
// The current binary's version is held in cli.Version. The CLI
// `--update` flag prints a one-line status; the GUI Settings panel
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
	"time"

	"pmforge/internal/cli"
)

// ManifestURL is the URL the binary fetches. Override at build time:
//
//	go build -ldflags "-X pmforge/internal/update.ManifestURL=https://pmforge.example/updates.json"
//
// Empty string disables the update check (useful for offline /
// distribution-managed builds).
var ManifestURL = ""

// UpdateChannelPublicKey is the base64-encoded Ed25519 public key
// the release pipeline signs with. Override at build time via
// -ldflags like ManifestURL above. Empty key disables verification
// AND the update check, fail-closed.
var UpdateChannelPublicKey = ""

const maxManifestBytes int64 = 64 * 1024

// Status is the result returned to the GUI / CLI.
type Status struct {
	Configured      bool   `json:"configured"`       // ManifestURL + key set?
	Current         string `json:"current"`          // running binary version
	Latest          string `json:"latest,omitempty"` // empty when no update
	UpdateAvailable bool   `json:"update_available"`
	ReleaseNotes    string `json:"release_notes,omitempty"`
	DownloadURL     string `json:"download_url,omitempty"`
	Error           string `json:"error,omitempty"`
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
	st := Status{Current: cli.Version}
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

	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", ManifestURL, nil)
	if err != nil {
		st.Error = err.Error()
		return st, nil
	}
	req.Header.Set("User-Agent", "PMForge/"+cli.Version)
	resp, err := client.Do(req)
	if err != nil {
		st.Error = "fetch: " + err.Error()
		return st, nil
	}
	defer resp.Body.Close()
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

	st.Latest = payload.LatestVersion
	st.ReleaseNotes = payload.ReleaseNotes
	st.DownloadURL = payload.DownloadURL
	st.UpdateAvailable = isNewer(payload.LatestVersion, cli.Version)
	return st, nil
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

// Check is the CLI `--update` entry point. Prints a one-line
// summary to stdout.
func Check() {
	st, err := CheckLatest(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "PMForge update check failed: %v\n", err)
		os.Exit(1)
	}
	switch {
	case !st.Configured:
		fmt.Printf("PMForge %s — automatic update channel not configured.\n", st.Current)
	case st.Error != "":
		fmt.Printf("PMForge %s — update check failed: %s\n", st.Current, st.Error)
	case st.UpdateAvailable:
		fmt.Printf("PMForge %s — update available: %s\n", st.Current, st.Latest)
		if st.DownloadURL != "" {
			fmt.Printf("  download: %s\n", st.DownloadURL)
		}
	default:
		fmt.Printf("PMForge %s — up to date.\n", st.Current)
	}
}

// isNewer compares two semver-ish strings "X.Y.Z[-suffix]" and
// reports whether `latest` is strictly newer than `current`.
// PMForge versions are clean semver (e.g. "1.1.0", "1.2.0-rc.1"), but the
// parser still tolerates a legacy "1.2.0-V1-Expansion" style suffix:
// non-numeric tails compare lexically. Wrong answers here only delay an
// update notification, never cause incorrect behaviour, so the simplicity
// trade-off is fine.
func isNewer(latest, current string) bool {
	la := splitVer(latest)
	cu := splitVer(current)
	for i := 0; i < len(la) || i < len(cu); i++ {
		var a, b string
		if i < len(la) {
			a = la[i]
		}
		if i < len(cu) {
			b = cu[i]
		}
		if a == b {
			continue
		}
		ai, aOK := atoi(a)
		bi, bOK := atoi(b)
		if aOK && bOK {
			return ai > bi
		}
		return a > b
	}
	return false
}

func splitVer(s string) []string {
	out := []string{}
	cur := ""
	for _, r := range s {
		if r == '.' || r == '-' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
		} else {
			cur += string(r)
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
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
