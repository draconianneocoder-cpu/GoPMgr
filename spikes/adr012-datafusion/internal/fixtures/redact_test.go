// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package fixtures

import "testing"

func TestRedactDir(t *testing.T) {
	const dir = "/tmp/corpus-123456/tabular"
	tests := []struct {
		name, msg, want string
	}{
		{"whole path", "open '/tmp/corpus-123456/tabular/a.csv'", "open '<corpus>/a.csv'"},
		{"no leading slash", "object store: tmp/corpus-123456/tabular/a.csv", "object store: <corpus>/a.csv"},
		{"truncated", "LINE 1: read_csv('/tmp/corpus-12...\n", "LINE 1: read_csv('<corpus>...\n"},
		{"truncated without slash", "at tmp/corpus-1234...", "at <corpus>..."},
		{"short prefix kept", "see /tmp/c... and /tmp", "see /tmp/c... and /tmp"},
		{"unrelated path kept", "open '/var/data/a.csv'", "open '/var/data/a.csv'"},
		{"several", "/tmp/corpus-123456/tabular/x and /tmp/corpus-123456/tab...", "<corpus>/x and <corpus>..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RedactDir(tt.msg, dir); got != tt.want {
				t.Fatalf("RedactDir(%q) = %q, want %q", tt.msg, got, tt.want)
			}
		})
	}
}
