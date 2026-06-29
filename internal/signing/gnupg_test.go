// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package signing

import (
	"context"
	"reflect"
	"testing"
)

func TestSignDetachedASCIIArmoredBuildsGnuPGCommandWithKey(t *testing.T) {
	var gotName string
	var gotArgs []string
	runner := func(_ context.Context, name string, args ...string) ([]byte, error) {
		gotName = name
		gotArgs = append([]string(nil), args...)
		return []byte("ok"), nil
	}

	if err := SignDetachedASCIIArmored(context.Background(), runner, "/tmp/document.pdf", "/tmp/document.pdf.asc", "pmforge@example.test"); err != nil {
		t.Fatalf("SignDetachedASCIIArmored: %v", err)
	}

	if gotName != "gpg" {
		t.Fatalf("command = %q, want gpg", gotName)
	}
	wantArgs := []string{
		"--batch",
		"--yes",
		"--armor",
		"--detach-sign",
		"--local-user",
		"pmforge@example.test",
		"--output",
		"/tmp/document.pdf.asc",
		"/tmp/document.pdf",
	}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("args = %#v, want %#v", gotArgs, wantArgs)
	}
}

func TestSignDetachedASCIIArmoredAllowsDefaultGnuPGKey(t *testing.T) {
	var gotArgs []string
	runner := func(_ context.Context, _ string, args ...string) ([]byte, error) {
		gotArgs = append([]string(nil), args...)
		return []byte("ok"), nil
	}

	if err := SignDetachedASCIIArmored(context.Background(), runner, "/tmp/document.pdf", "/tmp/document.pdf.asc", ""); err != nil {
		t.Fatalf("SignDetachedASCIIArmored: %v", err)
	}

	for i, arg := range gotArgs {
		if arg == "--local-user" {
			t.Fatalf("args include --local-user at index %d for default-key signing: %#v", i, gotArgs)
		}
	}
}
