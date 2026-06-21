// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package agile

import (
	"sort"
	"time"
)

// DORA metric classifications, per the Google/DORA "State of DevOps"
// report. Ranges are evaluated as <= the upper bound of each band.
//
// (The classification is intentionally generous: PMForge's audience
// is not just SaaS teams; on-prem and regulated industries should
// not be shamed by "low".)
type DORAClass string

const (
	DORAClassElite   DORAClass = "elite"
	DORAClassHigh    DORAClass = "high"
	DORAClassMedium  DORAClass = "medium"
	DORAClassLow     DORAClass = "low"
	DORAClassUnknown DORAClass = "unknown" // not enough data
)

// DORAResult holds every metric the GUI renders. Each metric carries
// its numeric value, its classification band, and a one-line caption
// the GUI shows under the KPI tile.
type DORAResult struct {
	WindowDays int       `json:"window_days"`
	From       time.Time `json:"from"`
	To         time.Time `json:"to"`
	TotalDeploys int     `json:"total_deploys"`
	Successful   int     `json:"successful_deploys"`
	Failed       int     `json:"failed_deploys"`

	DeployFrequency DORAMetric `json:"deploy_frequency"` // per day
	LeadTime        DORAMetric `json:"lead_time"`        // hours (median)
	ChangeFailureRate DORAMetric `json:"change_failure_rate"` // 0..1
	MTTR            DORAMetric `json:"mttr"`             // hours (median)

	// Trend series for line-chart rendering: deploys per day across
	// the window. The GUI overlays a target line at the elite
	// boundary (1 deploy/day) for context.
	DailyDeployTrend []DailyPoint `json:"daily_deploy_trend"`
}

// DORAMetric is the per-metric structure used by every KPI.
type DORAMetric struct {
	Value   float64   `json:"value"`
	Class   DORAClass `json:"class"`
	Label   string    `json:"label"`   // human-readable: "5.2/day"
	Caption string    `json:"caption"` // explainer line
}

// DailyPoint is one bucket in the trend timeline.
type DailyPoint struct {
	Date  string `json:"date"`  // YYYY-MM-DD
	Count int    `json:"count"`
}

// ComputeDORA derives all four DORA metrics from a list of
// deployments restricted to a rolling `windowDays` window ending
// at `now`. Passing windowDays <= 0 falls back to 30 days.
//
// Returns a DORAResult that is always safe to render — empty slices
// produce DORAClassUnknown bands rather than panicking.
func ComputeDORA(deployments []Deployment, windowDays int, now time.Time) DORAResult {
	if windowDays <= 0 {
		windowDays = 30
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	from := now.AddDate(0, 0, -windowDays)

	// Filter to the window.
	var window []Deployment
	for _, d := range deployments {
		if d.TS.Before(from) || d.TS.After(now) {
			continue
		}
		window = append(window, d)
	}

	out := DORAResult{
		WindowDays: windowDays,
		From:       from,
		To:         now,
	}

	// Counts.
	var leadTimes, restoreTimes []float64
	for _, d := range window {
		out.TotalDeploys++
		if d.Successful {
			out.Successful++
			if d.LeadTimeHours > 0 {
				leadTimes = append(leadTimes, d.LeadTimeHours)
			}
		} else {
			out.Failed++
			if d.RestoreTimeHours > 0 {
				restoreTimes = append(restoreTimes, d.RestoreTimeHours)
			}
		}
	}

	// 1) Deployment Frequency (deploys per day across the window)
	freqPerDay := 0.0
	if windowDays > 0 {
		freqPerDay = float64(out.TotalDeploys) / float64(windowDays)
	}
	out.DeployFrequency = DORAMetric{
		Value:   freqPerDay,
		Class:   classifyDeployFrequency(freqPerDay, out.TotalDeploys),
		Label:   formatPerDay(freqPerDay),
		Caption: "Deployments per day across the last " + itoaWindow(windowDays),
	}

	// 2) Lead Time for Changes — median commit-to-prod hours.
	leadMedian := median(leadTimes)
	out.LeadTime = DORAMetric{
		Value:   leadMedian,
		Class:   classifyLeadTime(leadMedian, len(leadTimes)),
		Label:   formatHours(leadMedian),
		Caption: "Median time from commit to production",
	}

	// 3) Change Failure Rate — failed / total.
	rate := 0.0
	if out.TotalDeploys > 0 {
		rate = float64(out.Failed) / float64(out.TotalDeploys)
	}
	out.ChangeFailureRate = DORAMetric{
		Value:   rate,
		Class:   classifyCFR(rate, out.TotalDeploys),
		Label:   formatPct(rate),
		Caption: "Share of deployments that required a rollback or hotfix",
	}

	// 4) Mean Time to Restore — median restore_time_hours on failures.
	mttr := median(restoreTimes)
	out.MTTR = DORAMetric{
		Value:   mttr,
		Class:   classifyMTTR(mttr, len(restoreTimes)),
		Label:   formatHours(mttr),
		Caption: "Median time from failure to restoration",
	}

	// Daily trend.
	out.DailyDeployTrend = buildDailyTrend(window, from, now)
	return out
}

// ----- Classification thresholds -----
//
// Aligned with DORA's published bands but rounded for legibility.

func classifyDeployFrequency(perDay float64, n int) DORAClass {
	if n == 0 {
		return DORAClassUnknown
	}
	switch {
	case perDay >= 1.0: // multiple a day
		return DORAClassElite
	case perDay >= 1.0/7.0: // weekly+
		return DORAClassHigh
	case perDay >= 1.0/30.0: // monthly+
		return DORAClassMedium
	default:
		return DORAClassLow
	}
}

func classifyLeadTime(hours float64, n int) DORAClass {
	if n == 0 {
		return DORAClassUnknown
	}
	switch {
	case hours <= 24:
		return DORAClassElite
	case hours <= 24*7:
		return DORAClassHigh
	case hours <= 24*30:
		return DORAClassMedium
	default:
		return DORAClassLow
	}
}

func classifyCFR(rate float64, n int) DORAClass {
	if n == 0 {
		return DORAClassUnknown
	}
	switch {
	case rate <= 0.15:
		return DORAClassElite // and high — same band by DORA's own data
	case rate <= 0.30:
		return DORAClassMedium
	default:
		return DORAClassLow
	}
}

func classifyMTTR(hours float64, n int) DORAClass {
	if n == 0 {
		return DORAClassUnknown
	}
	switch {
	case hours <= 1:
		return DORAClassElite
	case hours <= 24:
		return DORAClassHigh
	case hours <= 24*7:
		return DORAClassMedium
	default:
		return DORAClassLow
	}
}

// ----- Helpers -----

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]float64{}, xs...)
	sort.Float64s(s)
	mid := len(s) / 2
	if len(s)%2 == 1 {
		return s[mid]
	}
	return (s[mid-1] + s[mid]) / 2
}

func buildDailyTrend(window []Deployment, from, to time.Time) []DailyPoint {
	// Bucket per UTC date.
	bucket := make(map[string]int)
	for _, d := range window {
		k := d.TS.UTC().Format("2006-01-02")
		bucket[k]++
	}
	out := []DailyPoint{}
	day := from.UTC()
	for !day.After(to) {
		k := day.Format("2006-01-02")
		out = append(out, DailyPoint{Date: k, Count: bucket[k]})
		day = day.AddDate(0, 0, 1)
	}
	return out
}

func formatPerDay(v float64) string {
	if v >= 1 {
		return formatFloat1(v) + "/day"
	}
	if v >= 1.0/7.0 {
		return formatFloat1(v*7) + "/wk"
	}
	if v >= 1.0/30.0 {
		return formatFloat1(v*30) + "/mo"
	}
	return formatFloat1(v*365) + "/yr"
}

func formatHours(v float64) string {
	if v <= 0 {
		return "—"
	}
	if v < 1 {
		mins := v * 60
		return formatFloat1(mins) + " min"
	}
	if v < 48 {
		return formatFloat1(v) + " h"
	}
	days := v / 24
	if days < 30 {
		return formatFloat1(days) + " d"
	}
	weeks := days / 7
	return formatFloat1(weeks) + " wk"
}

func formatPct(v float64) string {
	return formatFloat1(v*100) + "%"
}

func formatFloat1(v float64) string {
	// One decimal place, trimmed. Tiny shim, avoids strconv import.
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	scaled := int64(v*10 + 0.5)
	whole := scaled / 10
	frac := scaled % 10
	sign := ""
	if neg {
		sign = "-"
	}
	if frac == 0 {
		return sign + itoaW(whole)
	}
	return sign + itoaW(whole) + "." + itoaW(frac)
}

func itoaW(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func itoaWindow(days int) string {
	return itoaW(int64(days)) + " days"
}
