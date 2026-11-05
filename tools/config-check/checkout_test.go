// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"strings"
	"testing"
)

// These fixtures exercise the real YAML consumer, including merge resolution.
func TestWorkflowCheckoutCredentials(t *testing.T) {
	tests := []struct{ name, jobs, want string }{
		{"safe", "guard: {steps: [{uses: actions/checkout@v6, with: {persist-credentials: false}}]}", ""},
		{"true", "guard: {steps: [{uses: actions/checkout@v6, with: {persist-credentials: true}}]}", "persist-credentials must be YAML boolean false"},
		{"missing with", "guard: {steps: [{uses: actions/checkout@v6}]}", "persist-credentials must be YAML boolean false"},
		{"missing input", "guard: {steps: [{uses: actions/checkout@v6, with: {fetch-depth: 0}}]}", "persist-credentials must be YAML boolean false"},
		{"string false", `guard: {steps: [{uses: actions/checkout@v6, with: {persist-credentials: "false"}}]}`, "persist-credentials must be YAML boolean false"},
		{"null", "guard: {steps: [{uses: actions/checkout@v6, with: {persist-credentials: null}}]}", "persist-credentials must be YAML boolean false"},
		{"zero", "guard: {steps: [{uses: actions/checkout@v6, with: {persist-credentials: 0}}]}", "persist-credentials must be YAML boolean false"},
		{"expression", `guard: {steps: [{uses: actions/checkout@v6, with: {persist-credentials: '${{ false }}'}}]}`, "persist-credentials must be YAML boolean false"},
		{"conditional", "guard: {steps: [{if: false, uses: actions/checkout@v6}]}", "persist-credentials must be YAML boolean false"},
		{"casing and arbitrary ref", "guard: {steps: [{uses: AcTiOnS/ChEcKoUt@custom-ref}]}", "persist-credentials must be YAML boolean false"},
		{"checkout root path", "guard: {steps: [{uses: actions/checkout/.@v6, with: {persist-credentials: true}}]}", "persist-credentials must be YAML boolean false"},
		{"checkout repeated separator", "guard: {steps: [{uses: actions//checkout@v6}]}", "persist-credentials must be YAML boolean false"},
		{"checkout optional path safe", "guard: {steps: [{uses: AcTiOnS/ChEcKoUt/subdir@v6, with: {persist-credentials: false}}]}", ""},
		{"checkout optional path unsafe", "guard: {steps: [{uses: AcTiOnS/ChEcKoUt/subdir@v6}]}", "persist-credentials must be YAML boolean false"},
		{"checkout backslash separator", `guard: {steps: [{uses: 'actions\checkout@v6'}]}`, "persist-credentials must be YAML boolean false"},
		{"multiple refs malformed", "guard: {steps: [{uses: actions/checkout@v6@other}]}", "checkout reference must include a nonempty ref"},
		{"container exclusion", "guard: {steps: [{uses: 'docker://actions/checkout@v6'}]}", ""},
		{"uppercase input safe", "guard: {steps: [{uses: actions/checkout@v6, with: {PERSIST-CREDENTIALS: false}}]}", ""},
		{"case input override unsafe", "guard: {steps: [{uses: actions/checkout@v6, with: {persist-credentials: false, PERSIST-CREDENTIALS: true}}]}", "persist-credentials must be YAML boolean false"},
		{"lookalike", "guard: {steps: [{uses: actions/checkout-extra@v6}]}", ""},
		{"local action", "guard: {steps: [{uses: ./actions/checkout}]}", ""},
		{"unrelated action and run", "guard: {steps: [{uses: actions/setup-go@v7}, {run: echo ok}]}", ""},
		{"reusable job", "guard: {uses: owner/repo/.github/workflows/test.yml@main}", ""},
		{"reusable cannot hide steps", "guard: {uses: owner/repo/.github/workflows/test.yml@main, steps: [{uses: actions/checkout@v6}]}", "persist-credentials must be YAML boolean false"},
		{"job scalar", "guard: false", "jobs.guard must be a mapping"},
		{"steps scalar", "guard: {steps: false}", "jobs.guard.steps must be a sequence"},
		{"steps null", "guard: {steps: null}", "jobs.guard.steps must be a sequence"},
		{"step scalar", "guard: {steps: [false]}", "jobs.guard.steps[0] must be a mapping"},
		{"uses scalar", "guard: {steps: [{uses: false}]}", "uses must be a nonempty string"},
		{"uses empty", "guard: {steps: [{uses: ''}]}", "uses must be a nonempty string"},
		{"missing ref", "guard: {steps: [{uses: actions/checkout}]}", "checkout reference must include a nonempty ref"},
		{"empty ref", "guard: {steps: [{uses: actions/checkout@}]}", "checkout reference must include a nonempty ref"},
		{"with scalar", "guard: {steps: [{uses: actions/checkout@v6, with: false}]}", "with must be a mapping"},
		{"with null", "guard: {steps: [{uses: actions/checkout@v6, with: null}]}", "with must be a mapping"},
		{"duplicate input", "guard: {steps: [{uses: actions/checkout@v6, with: {persist-credentials: false, persist-credentials: true}}]}", `duplicate key "persist-credentials"`},
		{"input alias false", "guard: {steps: [{uses: actions/checkout@v6, with: {persist-credentials: &safe false}}, {uses: actions/checkout@v6, with: {persist-credentials: *safe}}]}", ""},
		{"input alias true", "guard: {steps: [{uses: actions/checkout@v6, with: {persist-credentials: *unsafe}}]}", "jobs.guard.steps[0] checkout persist-credentials must be YAML boolean false"},
		{"aliased unsafe input reached", "guard: {steps: [{uses: unrelated/action@v1, with: {value: &unsafe true}}, {uses: actions/checkout@v6, with: {persist-credentials: *unsafe}}]}", "persist-credentials must be YAML boolean false"},
		{"unsafe with alias reached", "guard: {steps: [{uses: unrelated/action@v1, with: &unsafe {persist-credentials: true}}, {uses: actions/checkout@v6, with: *unsafe}]}", "persist-credentials must be YAML boolean false"},
		{"safe job alias", "first: &job {steps: [{uses: actions/checkout@v6, with: {persist-credentials: false}}]}\nguard: *job", ""},
		{"safe step merge", "guard: {steps: [{<<: {uses: actions/checkout@v6, with: {persist-credentials: true}}, with: {persist-credentials: false}}]}", ""},
		{"uses whitespace", "guard: {steps: [{uses: ' actions/checkout@v6 '}]}", "uses must not contain surrounding whitespace"},
		{"with alias", "guard: {steps: [{uses: actions/checkout@v6, with: &safe {persist-credentials: false}}, {uses: actions/checkout@v6, with: *safe}]}", ""},
		{"step alias", "guard: {steps: [&step {uses: actions/checkout@v6, with: {persist-credentials: false}}, *step]}", ""},
		{"job alias", "guard: *unsafe_job", "jobs.guard.steps[0] checkout persist-credentials must be YAML boolean false"},
		{"unsafe merge override", "guard: {steps: [{uses: actions/checkout@v6, with: {<<: &base {persist-credentials: false}, persist-credentials: true}}]}", "persist-credentials must be YAML boolean false"},
		{"safe merge override", "guard: {steps: [{uses: actions/checkout@v6, with: {<<: &base {persist-credentials: true}, persist-credentials: false}}]}", ""},
		{"merge sequence first safe", "guard: {steps: [{uses: actions/checkout@v6, with: {<<: [{persist-credentials: false}, {persist-credentials: true}]}}]}", ""},
		{"merge sequence first unsafe", "guard: {steps: [{uses: actions/checkout@v6, with: {<<: [{persist-credentials: true}, {persist-credentials: false}]}}]}", "persist-credentials must be YAML boolean false"},
		{"step merge", "guard: {steps: [{<<: {uses: actions/checkout@v6}, with: {persist-credentials: true}}]}", "persist-credentials must be YAML boolean false"},
		{"job merge", "guard: {<<: {steps: [{uses: actions/checkout@v6}]}}", "persist-credentials must be YAML boolean false"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := configSpec{path: "checkout.yml", format: formatYAML}
			data := "name: guard\non: push\nx-value: &unsafe true\nx-job: &unsafe_job {steps: [{uses: actions/checkout@v6}]}\njobs:\n  " + strings.ReplaceAll(tt.jobs, "\n", "\n  ") + "\n"
			config, err := parseConfig(spec, []byte(data))
			if err == nil {
				err = validateWorkflow(config)
			}
			if tt.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want %q, got %v", tt.want, err)
			}
			if strings.Contains(tt.want, "persist-credentials") && !strings.Contains(tt.want, "duplicate") && !strings.Contains(err.Error(), ".steps[") {
				t.Fatalf("missing job/step context: %v", err)
			}
		})
	}
}

func TestCheckoutGuardRegisteredWorkflows(t *testing.T) {
	// Literal inventory: deleting a registry entry cannot silently delete a test.
	for _, path := range []string{".github/workflows/ci.yml", ".github/workflows/release.yml", ".github/workflows/adr010-g0-spike.yml"} {
		t.Run(path, func(t *testing.T) {
			for _, value := range []string{"false", "true"} {
				files, tracked := validConfigFixture()
				files[path] = append(files[path], []byte("  checkout_probe:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: actions/checkout@v6\n        with:\n          persist-credentials: "+value+"\n")...)
				errs := validateRepositoryConfigs(files, tracked)
				if value == "false" {
					if len(errs) != 0 {
						t.Fatalf("safe checkout: %v", errs)
					}
					continue
				}
				found := false
				for _, err := range errs {
					if strings.Contains(err.Error(), path+": jobs.checkout_probe.steps[0]") && strings.Contains(err.Error(), "persist-credentials must be YAML boolean false") {
						found = true
					}
				}
				if !found {
					t.Fatalf("unsafe checkout must fail at registered consumer, got %v", errs)
				}
			}
		})
	}
}

func TestCheckoutGuardDiagnosticOrder(t *testing.T) {
	config, err := parseConfig(configSpec{path: "checkout.yml", format: formatYAML}, []byte("name: guard\non: push\njobs:\n  z: {steps: [{uses: actions/checkout@v6}]}\n  a: {steps: [{uses: actions/checkout@v6}]}\n"))
	if err != nil {
		t.Fatal(err)
	}
	for range 30 {
		if err := validateWorkflow(config); err == nil || !strings.Contains(err.Error(), "jobs.a.steps[0]") {
			t.Fatalf("unstable first failure: %v", err)
		}
	}
}
