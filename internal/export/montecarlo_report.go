// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package export

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/go-pdf/fpdf"

	"pmforge/internal/fonts"
	"pmforge/internal/kernel"
)

// MonteCarloRiskReportSpec describes the static Monte Carlo schedule-risk
// report rendered for export from the CPM editor.
type MonteCarloRiskReportSpec struct {
	ProjectName string
	ChartTitle  string
	Result      kernel.SimResult
	GeneratedAt time.Time
}

// GenerateMonteCarloRiskReport renders a PDF/A-3-tagged Monte Carlo schedule
// risk report with finish percentiles, an S-curve, and tornado risk drivers.
func GenerateMonteCarloRiskReport(spec MonteCarloRiskReportSpec) ([]byte, error) {
	if !spec.Result.Valid {
		if spec.Result.Error != "" {
			return nil, fmt.Errorf("monte carlo risk report: %s", spec.Result.Error)
		}
		return nil, errors.New("monte carlo risk report: simulation result is invalid")
	}
	if spec.Result.Iterations <= 0 {
		return nil, errors.New("monte carlo risk report: iterations must be positive")
	}
	if spec.ProjectName == "" {
		spec.ProjectName = "PMForge Project"
	}
	if spec.ChartTitle == "" {
		spec.ChartTitle = "CPM Schedule"
	}
	if spec.GeneratedAt.IsZero() {
		spec.GeneratedAt = time.Now().UTC()
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetCompression(false)
	_ = fonts.NewManager("").RegisterAs(pdf, "Source Sans 3", "Helvetica")
	reportTitle := "Monte Carlo Risk Report - " + spec.ProjectName + " - " + spec.ChartTitle
	pdf.SetTitle(reportTitle, true)
	pdf.SetAuthor("PMForge", true)
	pdf.SetCreator("PMForge "+exportVersion(), true)
	pdf.SetMargins(18, 16, 18)
	pdf.SetAutoPageBreak(true, 16)
	pdf.AddPage()

	drawMonteCarloReportHeader(pdf, spec)
	drawMonteCarloSummaryCards(pdf, spec.Result)
	drawMonteCarloSCurve(pdf, spec.Result)
	drawMonteCarloTornado(pdf, spec.Result)
	drawMonteCarloNarrative(pdf, spec.Result)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	out := buf.Bytes()
	xmp := XMPSpec{
		Title:       reportTitle,
		Author:      "PMForge",
		Subject:     fmt.Sprintf("Monte Carlo schedule risk report with P50 %s, P80 %s, P90 %s, Finish Probability S-curve, and Tornado Risk Drivers", daysLabel(spec.Result.P50), daysLabel(spec.Result.P80), daysLabel(spec.Result.P90)),
		CreatorTool: "PMForge " + exportVersion(),
	}
	if icc := defaultICCProfile(); len(icc) > 0 {
		if tagged, err := MakePDFA3(out, xmp, icc); err == nil {
			return tagged, nil
		}
	}
	if packet := BuildXMPPacket(xmp); len(packet) > 0 {
		if tagged, err := InjectXMPStream(out, packet); err == nil {
			return tagged, nil
		}
	}
	return out, nil
}

func drawMonteCarloReportHeader(pdf *fpdf.Fpdf, spec MonteCarloRiskReportSpec) {
	pdf.SetFillColor(15, 23, 42)
	pdf.Rect(0, 0, 210, 38, "F")
	pdf.SetTextColor(241, 245, 249)
	pdf.SetFont("Helvetica", "B", 20)
	pdf.SetXY(18, 12)
	pdf.Cell(0, 8, "Monte Carlo Risk Report")
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetXY(18, 23)
	pdf.Cell(0, 5, spec.ProjectName+" - "+spec.ChartTitle)
	pdf.SetTextColor(148, 163, 184)
	pdf.SetXY(18, 30)
	pdf.Cell(0, 5, "Generated "+spec.GeneratedAt.UTC().Format(time.RFC3339))
	pdf.SetTextColor(0, 0, 0)
	pdf.SetY(46)
}

func drawMonteCarloSummaryCards(pdf *fpdf.Fpdf, result kernel.SimResult) {
	cards := []struct {
		label string
		value string
		r     int
		g     int
		b     int
	}{
		{"P50 finish", daysLabel(result.P50), 8, 145, 178},
		{"P80 finish", daysLabel(result.P80), 180, 83, 9},
		{"P90 finish", daysLabel(result.P90), 185, 28, 28},
		{"Iterations", fmt.Sprintf("%d", result.Iterations), 71, 85, 105},
	}
	startX, y, w, h, gap := 18.0, pdf.GetY(), 39.0, 24.0, 5.0
	for i, card := range cards {
		x := startX + float64(i)*(w+gap)
		pdf.SetFillColor(248, 250, 252)
		pdf.SetDrawColor(203, 213, 225)
		pdf.Rect(x, y, w, h, "DF")
		pdf.SetXY(x+4, y+4)
		pdf.SetFont("Helvetica", "B", 8)
		pdf.SetTextColor(71, 85, 105)
		pdf.Cell(w-8, 5, card.label)
		pdf.SetXY(x+4, y+11)
		pdf.SetFont("Helvetica", "B", 15)
		pdf.SetTextColor(card.r, card.g, card.b)
		pdf.Cell(w-8, 7, card.value)
	}
	pdf.SetTextColor(0, 0, 0)
	pdf.SetY(y + h + 10)
}

func drawMonteCarloSCurve(pdf *fpdf.Fpdf, result kernel.SimResult) {
	pdf.SetFont("Helvetica", "B", 12)
	pdf.SetTextColor(15, 23, 42)
	pdf.Cell(0, 7, "Finish Probability S-curve")
	pdf.Ln(9)

	x, y, w, h := 24.0, pdf.GetY()+2, 160.0, 54.0
	pdf.SetDrawColor(203, 213, 225)
	pdf.SetFillColor(248, 250, 252)
	pdf.Rect(x, y, w, h, "DF")
	plotX, plotY, plotW, plotH := x+12, y+8, w-22, h-18
	pdf.SetDrawColor(100, 116, 139)
	pdf.Line(plotX, plotY+plotH, plotX+plotW, plotY+plotH)
	pdf.Line(plotX, plotY, plotX, plotY+plotH)
	pdf.SetDrawColor(226, 232, 240)
	pdf.Line(plotX, plotY+plotH/2, plotX+plotW, plotY+plotH/2)

	minDay, maxDay := monteCarloDayDomain(result)
	points := result.FinishCDF
	if len(points) > 0 {
		pdf.SetDrawColor(8, 145, 178)
		var lastX, lastY float64
		for i, point := range points {
			px := plotX + ((point.Day - minDay) / (maxDay - minDay) * plotW)
			py := plotY + plotH - clamp01(point.Probability)*plotH
			if i > 0 {
				pdf.Line(lastX, lastY, px, py)
			}
			lastX, lastY = px, py
		}
	}
	drawMonteCarloMarker(pdf, "P50", result.P50, 0.50, minDay, maxDay, plotX, plotY, plotW, plotH)
	drawMonteCarloMarker(pdf, "P80", result.P80, 0.80, minDay, maxDay, plotX, plotY, plotW, plotH)
	drawMonteCarloMarker(pdf, "P90", result.P90, 0.90, minDay, maxDay, plotX, plotY, plotW, plotH)

	pdf.SetFont("Helvetica", "", 7)
	pdf.SetTextColor(100, 116, 139)
	pdf.SetXY(plotX, plotY+plotH+2)
	pdf.Cell(30, 4, daysLabel(minDay))
	pdf.SetXY(plotX+plotW-30, plotY+plotH+2)
	pdf.CellFormat(30, 4, daysLabel(maxDay), "", 0, "R", false, 0, "")
	pdf.SetY(y + h + 10)
	pdf.SetTextColor(0, 0, 0)
}

func drawMonteCarloMarker(pdf *fpdf.Fpdf, label string, day, probability, minDay, maxDay, x, y, w, h float64) {
	px := x + ((day - minDay) / (maxDay - minDay) * w)
	py := y + h - clamp01(probability)*h
	pdf.SetDrawColor(185, 28, 28)
	pdf.Line(px, py, px, y+h)
	pdf.SetFillColor(185, 28, 28)
	pdf.Circle(px, py, 1.6, "F")
	pdf.SetFont("Helvetica", "B", 7)
	pdf.SetTextColor(185, 28, 28)
	pdf.SetXY(px-8, math.Max(y-1, py-5))
	pdf.CellFormat(16, 4, label, "", 0, "C", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
}

func drawMonteCarloTornado(pdf *fpdf.Fpdf, result kernel.SimResult) {
	pdf.SetFont("Helvetica", "B", 12)
	pdf.SetTextColor(15, 23, 42)
	pdf.Cell(0, 7, "Tornado Risk Drivers")
	pdf.Ln(8)
	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(71, 85, 105)
	pdf.MultiCell(0, 4.5, "Score = critical-path frequency multiplied by P90-P50 sampled duration spread.", "", "L", false)
	pdf.Ln(2)

	if len(result.TornadoDrivers) == 0 {
		pdf.SetTextColor(100, 116, 139)
		pdf.Cell(0, 6, "No variable risk drivers were detected.")
		pdf.SetTextColor(0, 0, 0)
		return
	}

	maxScore := 0.0
	for _, driver := range result.TornadoDrivers {
		if driver.Score > maxScore {
			maxScore = driver.Score
		}
	}
	if maxScore <= 0 {
		maxScore = 1
	}

	x, barW := 40.0, 102.0
	for _, driver := range result.TornadoDrivers {
		y := pdf.GetY()
		pdf.SetFont("Helvetica", "B", 8)
		pdf.SetTextColor(15, 23, 42)
		pdf.SetXY(18, y)
		pdf.CellFormat(20, 5, truncate(driver.TaskID, 16), "", 0, "L", false, 0, "")
		pdf.SetFillColor(226, 232, 240)
		pdf.Rect(x, y+1, barW, 3.8, "F")
		pdf.SetFillColor(8, 145, 178)
		pdf.Rect(x, y+1, math.Max(1.2, driver.Score/maxScore*barW), 3.8, "F")
		pdf.SetFont("Helvetica", "", 7)
		pdf.SetTextColor(71, 85, 105)
		pdf.SetXY(x+barW+4, y)
		pdf.CellFormat(44, 5, fmt.Sprintf("score %.2f, crit %.0f%%, spread %s", driver.Score, driver.CriticalFrequency*100, daysLabel(driver.DurationSpread)), "", 0, "L", false, 0, "")
		pdf.Ln(6)
	}
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(4)
}

func drawMonteCarloNarrative(pdf *fpdf.Fpdf, result kernel.SimResult) {
	pdf.SetFont("Helvetica", "B", 12)
	pdf.SetTextColor(15, 23, 42)
	pdf.Cell(0, 7, "Narrative Summary")
	pdf.Ln(8)
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(51, 65, 85)
	topDriver := "no variable task"
	if len(result.TornadoDrivers) > 0 {
		topDriver = result.TornadoDrivers[0].TaskID
	}
	pdf.MultiCell(0, 5, fmt.Sprintf(
		"The simulated schedule has a median finish at %s, with an 80 percent confidence finish at %s and a 90 percent confidence finish at %s. The leading tornado driver is %s. Review high-score drivers first when reducing schedule uncertainty.",
		daysLabel(result.P50),
		daysLabel(result.P80),
		daysLabel(result.P90),
		topDriver,
	), "", "L", false)
	pdf.SetTextColor(0, 0, 0)
}

func monteCarloDayDomain(result kernel.SimResult) (float64, float64) {
	minDay, maxDay := result.P50, result.P90
	for _, point := range result.FinishCDF {
		if point.Day < minDay {
			minDay = point.Day
		}
		if point.Day > maxDay {
			maxDay = point.Day
		}
	}
	if math.Abs(maxDay-minDay) < 1e-9 {
		return minDay - 0.5, maxDay + 0.5
	}
	return minDay, maxDay
}

func daysLabel(value float64) string {
	return fmt.Sprintf("%.1fd", value)
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
