// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Command update-manifest creates an Ed25519-signed GoPMgr update manifest.
// The private key is read only from a named environment variable and is never
// accepted on the command line, written to disk, or printed.
package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopmgr/internal/update"
)

func main() {
	var version, channel, platform, architecture, artifact, downloadURL, output, keyEnv string
	flag.StringVar(&version, "version", "", "exact semantic version")
	flag.StringVar(&channel, "channel", "", "release channel")
	flag.StringVar(&platform, "platform", "", "target operating system")
	flag.StringVar(&architecture, "architecture", "", "target architecture")
	flag.StringVar(&artifact, "artifact", "", "installer artifact path")
	flag.StringVar(&downloadURL, "download-url", "", "HTTPS artifact URL")
	flag.StringVar(&output, "output", "", "manifest output path")
	flag.StringVar(&keyEnv, "private-key-env", "GOPMGR_UPDATE_PRIVATE_KEY", "environment variable containing the base64 Ed25519 private key")
	flag.Parse()
	if err := run(version, channel, platform, architecture, artifact, downloadURL, output, keyEnv); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "update-manifest: %v\n", err)
		os.Exit(1)
	}
}

func run(version, channel, platform, architecture, artifact, downloadURL, output, keyEnv string) error {
	if version == "" || channel == "" || platform == "" || architecture == "" || artifact == "" || downloadURL == "" || output == "" {
		return fmt.Errorf("all manifest fields and paths are required")
	}
	keyBytes, err := base64.StdEncoding.DecodeString(os.Getenv(keyEnv))
	if err != nil || len(keyBytes) != ed25519.PrivateKeySize {
		return fmt.Errorf("%s must contain a base64 Ed25519 private key", keyEnv)
	}
	artifactBytes, err := os.ReadFile(artifact) // #nosec G304 -- explicit release artifact selected by the caller.
	if err != nil {
		return fmt.Errorf("read artifact: %w", err)
	}
	digest := sha256.Sum256(artifactBytes)
	payload := update.Payload{
		LatestVersion: version,
		Channel:       channel,
		Platform:      platform,
		Architecture:  architecture,
		ReleaseNotes:  "See the GoPMgr release page for this version.",
		DownloadURL:   downloadURL,
		SHA256:        hex.EncodeToString(digest[:]),
		PublishedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	manifest := update.Manifest{
		PayloadB64:   base64.StdEncoding.EncodeToString(payloadJSON),
		SignatureB64: base64.StdEncoding.EncodeToString(ed25519.Sign(ed25519.PrivateKey(keyBytes), payloadJSON)),
	}
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o700); err != nil {
		return err
	}
	return os.WriteFile(output, append(manifestJSON, '\n'), 0o600)
}
