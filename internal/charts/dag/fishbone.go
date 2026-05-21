// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package dag

import (
	"encoding/json"
	"math"
)

// Fishbone (Ishikawa) Diagram
// ===========================
//
// Standard root-cause analysis layout:
//
//   - One central "Effect" (the problem being analysed)
//   - A horizontal spine running left toward the effect
//   - N "Category" bones radiating diagonally above and below the
//     spine. The classic set is the "6 Ms" — People, Process,
//     Equipment, Materials, Environment, Measurement — but the user
//     can name them anything.
//   - Each category contains a list of "Causes" that attach
//     perpendicularly to its bone.
//
// PMForge's representation is intentionally narrative: causes are
// short text strings rather than nodes of their own, because Ishikawa
// diagrams are read top-to-bottom, not traversed like a graph.

// FishboneDocument is the JSON shape stored in db.charts.data for a
// Fishbone chart.
type FishboneDocument struct {
	Effect     string              `json:"effect"`
	Categories []FishboneCategory  `json:"categories"`
}

// FishboneCategory is one diagonal bone.
type FishboneCategory struct {
	Name   string   `json:"name"`
	Causes []string `json:"causes"`
}

// FishboneNode is the rendering-ready primitive emitted by
// LayoutFishbone. Type discriminator drives the Svelte renderer.
type FishboneNode struct {
	ID     string  `json:"id"`
	Type   string  `json:"type"` // "effect" | "category" | "cause" | "spine_start"
	Label  string  `json:"label"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Side   string  `json:"side,omitempty"` // "above" | "below" for category/cause
}

// FishboneEdge is one bone segment (spine or category bone). Cause
// strokes are emitted as additional edges from the cause position
// perpendicular onto the bone.
type FishboneEdge struct {
	X1 float64 `json:"x1"`
	Y1 float64 `json:"y1"`
	X2 float64 `json:"x2"`
	Y2 float64 `json:"y2"`
	Kind string `json:"kind"` // "spine" | "bone" | "cause"
}

// FishboneLayout is the full render-ready output.
type FishboneLayout struct {
	Nodes  []FishboneNode `json:"nodes"`
	Edges  []FishboneEdge `json:"edges"`
	Width  float64        `json:"width"`
	Height float64        `json:"height"`
}

// ParseFishbone decodes a JSON blob into a FishboneDocument.
func ParseFishbone(raw string) (FishboneDocument, error) {
	if raw == "" || raw == "{}" {
		return FishboneDocument{}, nil
	}
	var doc FishboneDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return FishboneDocument{}, err
	}
	return doc, nil
}

// EncodeFishbone serialises a FishboneDocument back to JSON.
func EncodeFishbone(doc FishboneDocument) (string, error) {
	b, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// FishboneLayoutOptions controls visual spacing.
type FishboneLayoutOptions struct {
	EffectWidth   float64 // width of the effect node
	EffectHeight  float64
	CategoryWidth float64 // width of each category label
	CategoryHeight float64
	CauseHeight   float64 // height of each cause line
	BoneLengthMin float64 // shortest diagonal bone
	BoneStep      float64 // extra length per cause on the bone
	CauseGap      float64 // horizontal gap between causes on a bone
	BoneAngleDeg  float64 // angle of bones from horizontal
}

// DefaultFishboneOptions returns the spacing the GUI uses by default.
func DefaultFishboneOptions() FishboneLayoutOptions {
	return FishboneLayoutOptions{
		EffectWidth:    180,
		EffectHeight:   60,
		CategoryWidth:  140,
		CategoryHeight: 28,
		CauseHeight:    20,
		BoneLengthMin:  200,
		BoneStep:       40,
		CauseGap:       110,
		BoneAngleDeg:   55,
	}
}

// LayoutFishbone produces a render-ready FishboneLayout.
//
// Geometry, briefly:
//
//   - The spine is horizontal at y = canvasHeight/2.
//   - The effect node sits at the right end of the spine.
//   - Categories alternate above (even indices) and below (odd) the
//     spine. Each is placed along the spine at evenly-spaced
//     "attachment points" to the left of the effect.
//   - Each category's bone goes diagonally up-right (above) or
//     down-right (below) at BoneAngleDeg.
//   - Causes are placed perpendicularly along the bone, one per
//     cause, with a small horizontal stroke leading to the bone.
func LayoutFishbone(doc FishboneDocument, opt FishboneLayoutOptions) FishboneLayout {
	if len(doc.Categories) == 0 {
		// Even with no categories, render the effect so the user can
		// see something while they start building.
		return FishboneLayout{
			Nodes: []FishboneNode{{
				ID: "effect", Type: "effect", Label: doc.Effect,
				X: 0, Y: 0, Width: opt.EffectWidth, Height: opt.EffectHeight,
			}},
			Width:  opt.EffectWidth,
			Height: opt.EffectHeight,
		}
	}

	angleRad := opt.BoneAngleDeg * math.Pi / 180

	// Each bone's length grows with the number of causes it carries.
	boneLengths := make([]float64, len(doc.Categories))
	for i, c := range doc.Categories {
		boneLengths[i] = opt.BoneLengthMin + float64(len(c.Causes))*opt.BoneStep
	}
	// The horizontal projection of each bone determines how far apart
	// the attachment points need to be along the spine.
	maxHorizProj := 0.0
	for _, bl := range boneLengths {
		hp := bl * math.Cos(angleRad)
		if hp > maxHorizProj {
			maxHorizProj = hp
		}
	}
	attachmentStride := 110.0 // x-distance between successive attachments

	// Compute spine length:
	//   left margin (for cause labels) + (N-1) * stride + max bone horiz projection +
	//   gap before effect + effect width
	leftMargin := 80.0
	gapBeforeEffect := 40.0
	spineLeftX := leftMargin + maxHorizProj
	spineRightX := spineLeftX + float64(len(doc.Categories)-1)*attachmentStride/2 + maxHorizProj + gapBeforeEffect
	// We want at least N/2 attachments above + N/2 below; each side
	// has ceil(N/2) attachments separated by attachmentStride.
	halfCount := (len(doc.Categories) + 1) / 2
	spineWidth := float64(halfCount-1)*attachmentStride + maxHorizProj + gapBeforeEffect
	if spineWidth < attachmentStride*2 {
		spineWidth = attachmentStride * 2
	}
	spineRightX = spineLeftX + spineWidth

	// Vertical extent: max vertical projection of any bone + cause overhang.
	maxVertProj := 0.0
	for _, bl := range boneLengths {
		vp := bl * math.Sin(angleRad)
		if vp > maxVertProj {
			maxVertProj = vp
		}
	}
	spineY := maxVertProj + opt.CauseHeight*2 + 40
	canvasHeight := spineY*2 + 40

	out := FishboneLayout{
		Width:  spineRightX + opt.EffectWidth + 20,
		Height: canvasHeight,
	}

	// Spine
	effectX := spineRightX
	out.Edges = append(out.Edges, FishboneEdge{
		X1: spineLeftX, Y1: spineY, X2: effectX, Y2: spineY, Kind: "spine",
	})

	// Effect node
	out.Nodes = append(out.Nodes, FishboneNode{
		ID:     "effect",
		Type:   "effect",
		Label:  doc.Effect,
		X:      effectX,
		Y:      spineY - opt.EffectHeight/2,
		Width:  opt.EffectWidth,
		Height: opt.EffectHeight,
	})

	// Categories: alternate above (even index) / below (odd index).
	// Within each side, lay attachments out left-to-right along the spine.
	aboveCount := 0
	belowCount := 0
	for i, cat := range doc.Categories {
		side := "above"
		var sideIdx int
		if i%2 == 0 {
			sideIdx = aboveCount
			aboveCount++
		} else {
			side = "below"
			sideIdx = belowCount
			belowCount++
		}
		attachX := spineLeftX + maxHorizProj + float64(sideIdx)*attachmentStride
		if attachX > effectX-gapBeforeEffect {
			attachX = effectX - gapBeforeEffect
		}

		bl := boneLengths[i]
		dx := bl * math.Cos(angleRad)
		dy := bl * math.Sin(angleRad)
		var endX, endY float64
		if side == "above" {
			endX = attachX - dx
			endY = spineY - dy
		} else {
			endX = attachX - dx
			endY = spineY + dy
		}

		// Bone edge
		out.Edges = append(out.Edges, FishboneEdge{
			X1: attachX, Y1: spineY, X2: endX, Y2: endY, Kind: "bone",
		})

		// Category label at the far end of the bone
		labelY := endY - opt.CategoryHeight/2
		if side == "below" {
			labelY = endY - opt.CategoryHeight/2
		}
		out.Nodes = append(out.Nodes, FishboneNode{
			ID:     "cat_" + itoa(i),
			Type:   "category",
			Label:  cat.Name,
			X:      endX - opt.CategoryWidth/2,
			Y:      labelY,
			Width:  opt.CategoryWidth,
			Height: opt.CategoryHeight,
			Side:   side,
		})

		// Causes along the bone. Parametrise the bone from t=0 (at
		// the spine) to t=1 (at the category label). Causes occupy
		// t ∈ [0.2, 0.85].
		n := len(cat.Causes)
		for j, cause := range cat.Causes {
			t := 0.2 + (0.85-0.2)*(float64(j)+0.5)/math.Max(float64(n), 1)
			cx := attachX - t*dx
			var cy float64
			if side == "above" {
				cy = spineY - t*dy
			} else {
				cy = spineY + t*dy
			}
			// Short perpendicular stroke from the bone outward
			// horizontally for the cause text.
			strokeLen := 90.0
			textX := cx - strokeLen
			out.Edges = append(out.Edges, FishboneEdge{
				X1: cx, Y1: cy, X2: textX, Y2: cy, Kind: "cause",
			})
			out.Nodes = append(out.Nodes, FishboneNode{
				ID:     "cause_" + itoa(i) + "_" + itoa(j),
				Type:   "cause",
				Label:  cause,
				X:      textX - 120,
				Y:      cy - opt.CauseHeight/2,
				Width:  120,
				Height: opt.CauseHeight,
				Side:   side,
			})
		}
	}

	return out
}
