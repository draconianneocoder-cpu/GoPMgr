// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package documents

import "testing"

func TestPreflightCertifiedBlocksMissingRequiredDocuments(t *testing.T) {
	profile := ReportProfileFor("software", "")
	result := Preflight(profile, ReportModeCertified, []ReportInput{{ID: "req", Kind: KindRequirements, Status: "approved"}})
	if result.Ready {
		t.Fatal("certified preflight unexpectedly passed without charter, schedule, and status report")
	}
	found := false
	for _, issue := range result.Issues {
		if issue.Code == "required_document_missing" && issue.Severity == "error" {
			found = true
		}
	}
	if !found {
		t.Fatalf("issues = %#v, want required-document error", result.Issues)
	}
}

func TestPreflightCustomProfileAllowsIntentionalSelection(t *testing.T) {
	profile := ReportProfileFor("custom", "")
	result := Preflight(profile, ReportModeCertified, []ReportInput{{ID: "note", Kind: KindProjectBrief, Status: "approved"}})
	if !result.Ready {
		t.Fatalf("custom approved report unexpectedly blocked: %#v", result.Issues)
	}
}
