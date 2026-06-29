// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package signing

import (
	"context"
	"fmt"
	"os/exec"
)

// CommandRunner is the narrow seam used by tests so GnuPG behavior can be
// verified without requiring a local keyring.
type CommandRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

// ExecCommandRunner runs the requested command and returns combined output so
// callers can surface actionable GnuPG errors to the UI.
func ExecCommandRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput() // #nosec G204 -- fixed binary supplied by caller; paths and key IDs are argv.
}

// SignDetachedASCIIArmored writes an ASCII-armored detached GnuPG signature.
// The signed PDF bytes are not modified, preserving PDF/A and allowing users to
// distribute a .asc sidecar or print the document and wet-sign it separately.
func SignDetachedASCIIArmored(ctx context.Context, runner CommandRunner, inputPath, outputPath, keyID string) error {
	if runner == nil {
		runner = ExecCommandRunner
	}
	args := []string{
		"--batch",
		"--yes",
		"--armor",
		"--detach-sign",
	}
	if keyID != "" {
		args = append(args, "--local-user", keyID)
	}
	args = append(args, "--output", outputPath, inputPath)

	output, err := runner(ctx, "gpg", args...)
	if err != nil {
		if len(output) == 0 {
			return fmt.Errorf("gpg detached signature failed: %w", err)
		}
		return fmt.Errorf("gpg detached signature failed: %w: %s", err, string(output))
	}
	return nil
}
