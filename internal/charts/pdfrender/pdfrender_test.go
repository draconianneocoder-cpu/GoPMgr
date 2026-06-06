// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package pdfrender

import (
	"encoding/json"
	"math"
	"testing"
)

// --- fit ---

func withinF(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s: got %v, want %v", label, got, want)
	}
}

func TestFit_ZeroLayoutWidth_ReturnsDefaults(t *testing.T) {
	scale, ox, oy := fit(0, 100, 200, 200)
	withinF(t, "scale", scale, 1)
	withinF(t, "ox", ox, 0)
	withinF(t, "oy", oy, 0)
}

func TestFit_ZeroLayoutHeight_ReturnsDefaults(t *testing.T) {
	scale, ox, oy := fit(100, 0, 200, 200)
	withinF(t, "scale", scale, 1)
	withinF(t, "ox", ox, 0)
	withinF(t, "oy", oy, 0)
}

func TestFit_LayoutSmallerThanFrame_ScaleCappedAt1(t *testing.T) {
	// layout 100×100, frame 200×200 → natural scale=2, capped at 1
	// centred offsets: ox = (200-100)/2 = 50
	scale, ox, oy := fit(100, 100, 200, 200)
	withinF(t, "scale", scale, 1)
	withinF(t, "ox", ox, 50)
	withinF(t, "oy", oy, 50)
}

func TestFit_LayoutLargerThanFrame_ScaleDown(t *testing.T) {
	// layout 200×200, frame 100×100 → sx=sy=0.5, offsets=0
	scale, ox, oy := fit(200, 200, 100, 100)
	withinF(t, "scale", scale, 0.5)
	withinF(t, "ox", ox, 0)
	withinF(t, "oy", oy, 0)
}

func TestFit_WideLayout_ConstrainedByWidth(t *testing.T) {
	// layout 200×100 (wider), frame 100×100
	// sx=100/200=0.5, sy=100/100=1 → scale=0.5
	// ox=(100-200*0.5)/2=0, oy=(100-100*0.5)/2=25
	scale, ox, oy := fit(200, 100, 100, 100)
	withinF(t, "scale", scale, 0.5)
	withinF(t, "ox", ox, 0)
	withinF(t, "oy", oy, 25)
}

func TestFit_TallLayout_ConstrainedByHeight(t *testing.T) {
	// layout 100×200 (taller), frame 100×100
	// sx=1, sy=0.5 → scale=0.5
	// ox=(100-100*0.5)/2=25, oy=(100-200*0.5)/2=0
	scale, ox, oy := fit(100, 200, 100, 100)
	withinF(t, "scale", scale, 0.5)
	withinF(t, "ox", ox, 25)
	withinF(t, "oy", oy, 0)
}

func TestFit_ExactMatch_ScaleOne(t *testing.T) {
	// layout == frame → scale=1, no centering offset
	scale, ox, oy := fit(150, 100, 150, 100)
	withinF(t, "scale", scale, 1)
	withinF(t, "ox", ox, 0)
	withinF(t, "oy", oy, 0)
}

// --- parseBody ---

func TestParseBody_NilBody_ReturnsError(t *testing.T) {
	var out map[string]any
	if err := parseBody(nil, &out); err == nil {
		t.Error("parseBody(nil) = nil error, want error")
	}
}

func TestParseBody_EmptyBody_ReturnsError(t *testing.T) {
	var out map[string]any
	if err := parseBody(json.RawMessage{}, &out); err == nil {
		t.Error("parseBody(empty) = nil error, want error")
	}
}

func TestParseBody_InvalidJSON_ReturnsError(t *testing.T) {
	var out map[string]any
	if err := parseBody(json.RawMessage(`{not json`), &out); err == nil {
		t.Error("parseBody(invalid) = nil error, want error")
	}
}

func TestParseBody_ValidJSON_PopulatesOut(t *testing.T) {
	var out struct {
		Name string `json:"name"`
	}
	if err := parseBody(json.RawMessage(`{"name":"wbs"}`), &out); err != nil {
		t.Fatalf("parseBody error: %v", err)
	}
	if out.Name != "wbs" {
		t.Errorf("out.Name = %q, want %q", out.Name, "wbs")
	}
}

func TestParseBody_EmptyObject_NoError(t *testing.T) {
	var out map[string]any
	if err := parseBody(json.RawMessage(`{}`), &out); err != nil {
		t.Errorf("parseBody({}) error: %v", err)
	}
}
