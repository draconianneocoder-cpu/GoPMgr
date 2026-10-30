// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"testing"

	"gopmgr/internal/analytics"
	"gopmgr/spikes/adr012-datafusion/internal/oracle"
)

// TestScoreOverflow pins the Codex review case on #153: an oracle overflow
// is matched only by a typed overflow result, never by a crash, timeout or
// malformed response.
func TestScoreOverflow(t *testing.T) {
	overflow := oracle.Outcome{Err: "overflow"}
	tests := []struct {
		name      string
		err       error
		want      oracle.Outcome
		parity    string
		errClass  string
		hasIssues bool
	}{
		{"typed overflow", overflowResult{errors.New("overflow: total outside int64")}, overflow, "pass", "overflow", false},
		{"CLI crash on an overflow case", errors.New("signal: killed"), overflow, "fail", "execution", true},
		{"malformed JSON on an overflow case", errors.New("invalid character 'x'"), overflow, "fail", "execution", true},
		{"overflow where the oracle has a total", overflowResult{errors.New("x")}, oracle.Outcome{}, "fail", "overflow", true},
		{"no error where the oracle overflows", nil, overflow, "fail", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rec portfolioRecord
			score(&rec, analytics.PortfolioSummary{}, tt.err, tt.want)
			if tt.err != nil && rec.Error != tt.err.Error() {
				t.Errorf("error = %q, want the provider's message %q", rec.Error, tt.err.Error())
			}
			if rec.Parity != tt.parity || rec.ErrorClass != tt.errClass || (len(rec.Issues) > 0) != tt.hasIssues {
				t.Errorf("parity %q class %q issues %v; want %q %q issues=%v", rec.Parity, rec.ErrorClass, rec.Issues, tt.parity, tt.errClass, tt.hasIssues)
			}
		})
	}
}
