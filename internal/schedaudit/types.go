// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package schedaudit checks one chart schedule for quality problems: missing
// logic, dependency cycles, leads and lags, hard and violated constraints,
// float, invalid progress and unresourced work. It is the PS-02 slice 1 audit
// (docs/testing/PS-02-schedule-quality-audit-spec.md).
//
// The audit is read-only. It works on a deep copy of the tasks it is given,
// never on the caller's values, and it returns findings; it repairs nothing.
// It uses the kernel's own scheduler and calendar mapping and adds no second
// scheduling or calendar implementation.
package schedaudit

import "gopmgr/internal/kernel"

// EngineVersion names this audit's rules. Each finding carries it, so a saved
// result can be matched to the rules that produced it.
const EngineVersion = "schedaudit/1"

// RuleID is one of the rules SQ-01 to SQ-18 in the PS-02 specification.
type RuleID string

// The rules, in the order the specification lists them.
const (
	MissingPredecessor RuleID = "SQ-01"
	MissingSuccessor   RuleID = "SQ-02"
	DanglingReference  RuleID = "SQ-03"
	DependencyCycle    RuleID = "SQ-04"
	Lead               RuleID = "SQ-05"
	LongLag            RuleID = "SQ-06"
	NonFinishToStart   RuleID = "SQ-07"
	HardConstraint     RuleID = "SQ-08"
	ConstraintViolated RuleID = "SQ-09"
	HighFloat          RuleID = "SQ-10"
	NegativeFloat      RuleID = "SQ-11"
	LongDuration       RuleID = "SQ-12"
	MilestoneWithout   RuleID = "SQ-13"
	InvalidActuals     RuleID = "SQ-14"
	MissingProgress    RuleID = "SQ-15"
	Overallocation     RuleID = "SQ-16"
	UnresourcedTask    RuleID = "SQ-17"
	ReportingContext   RuleID = "SQ-18"
)

// LogicMetric is the metric ID that counts SQ-01 and SQ-02 together, as the
// DCMA-style "logic" test does. Every other metric's ID is its rule ID.
const LogicMetric = "logic"

// cancellationInterval is how many tasks a loop handles between checks of
// the context.
const cancellationInterval = 4096

// AllRules lists every rule in specification order.
func AllRules() []RuleID {
	return []RuleID{
		MissingPredecessor, MissingSuccessor, DanglingReference, DependencyCycle,
		Lead, LongLag, NonFinishToStart, HardConstraint, ConstraintViolated,
		HighFloat, NegativeFloat, LongDuration, MilestoneWithout, InvalidActuals,
		MissingProgress, Overallocation, UnresourcedTask, ReportingContext,
	}
}

// Severity orders findings: errors first, then warnings, then information.
type Severity string

// The three severities.
const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

func (s Severity) rank() int {
	switch s {
	case SeverityError:
		return 0
	case SeverityWarning:
		return 1
	default:
		return 2
	}
}

// Source identifies the chart an audit read. The caller supplies it; the
// audit copies it into every finding so a result cannot be mistaken for
// another chart's.
type Source struct {
	ChartID  string `json:"chart_id"`
	Revision string `json:"revision"`
}

// Input is everything one audit reads.
type Input struct {
	// Tasks are the chart's tasks. They are copied before use. Each ID must
	// be non-empty and unique.
	Tasks []kernel.Task

	// ProjectStart is the project start date in kernel.DateLayout only.
	// Without a readable one, date constraints cannot be applied, so SQ-08's
	// binding report and SQ-09 to SQ-11 and SQ-15 are not evaluated. The
	// stored project date may also be RFC3339 (see the desktop's
	// parseProjectDate); the caller must convert it, or the audit reports a
	// valid date as unreadable.
	ProjectStart string
	// IsWorkday is the project's work calendar. Nil means every day is
	// worked, as in the kernel.
	IsWorkday kernel.WorkdayFunc
	// ReportingDate is the status date (kernel.DateLayout). Without one,
	// SQ-15 and the after-the-reporting-date part of SQ-14 are not
	// evaluated.
	ReportingDate string
	// BaselineAvailable says whether the chart has a baseline. Only SQ-18
	// reads it.
	BaselineAvailable bool

	// Capacity is the resource capacity plan SQ-16 checks demand against.
	// The caller must pass the plan the chart itself uses (app_charts.go
	// builds it); the audit never reads a stored Overallocated flag.
	Capacity kernel.ResourceCapacityPlan

	// StartAnchors and FinishAnchors are the task IDs the chart designates
	// as its start and finish. SQ-01 does not report a start anchor and
	// SQ-02 does not report a finish anchor. They are never derived from
	// computed dates. An ID not in Tasks is reported under SQ-18.
	StartAnchors  []string
	FinishAnchors []string

	Source  Source
	Profile Profile
}

// LinkRef names one precedence relationship.
type LinkRef struct {
	Pred string          `json:"pred"`
	Succ string          `json:"succ"`
	Type kernel.LinkType `json:"type"`
	Lag  float64         `json:"lag"`
}

// Finding is one problem the audit found.
type Finding struct {
	Rule          RuleID   `json:"rule"`
	Profile       string   `json:"profile"`
	Severity      Severity `json:"severity"`
	TaskID        string   `json:"task_id,omitempty"`
	TaskTitle     string   `json:"task_title,omitempty"`
	Link          *LinkRef `json:"link,omitempty"`
	Observed      string   `json:"observed"`
	Threshold     string   `json:"threshold,omitempty"`
	Explanation   string   `json:"explanation"`
	Advice        string   `json:"advice"`
	Source        Source   `json:"source"`
	EngineVersion string   `json:"engine_version"`
}

// Limit is a pass/fail threshold on a percentage metric.
type Limit struct {
	// AtLeast is true when the metric must be at or above Percent, as for
	// the share of finish-to-start links; otherwise it must be at or below.
	AtLeast bool    `json:"at_least"`
	Percent float64 `json:"percent"`
}

// Metric is one rule's count as a share of what it measured.
type Metric struct {
	ID          string  `json:"id"`
	Numerator   int     `json:"numerator"`
	Denominator int     `json:"denominator"`
	Percent     float64 `json:"percent"`
	Limit       *Limit  `json:"limit,omitempty"`
	// Pass is set only when the profile gives this metric a limit. A metric
	// with nothing to measure (a zero denominator) passes.
	Pass *bool `json:"pass,omitempty"`
}

// RuleStatus says whether a rule ran. A rule that could not run is reported
// as not evaluated with the reason, never as passing.
type RuleStatus struct {
	Rule      RuleID `json:"rule"`
	Evaluated bool   `json:"evaluated"`
	Reason    string `json:"reason,omitempty"`
}

// Result is one audit's output. Findings are ordered by severity, rule, task
// ID and link; Rules lists every rule in specification order.
type Result struct {
	Profile       string       `json:"profile"`
	Source        Source       `json:"source"`
	EngineVersion string       `json:"engine_version"`
	ReportingDate string       `json:"reporting_date,omitempty"`
	Findings      []Finding    `json:"findings"`
	Metrics       []Metric     `json:"metrics"`
	Rules         []RuleStatus `json:"rules"`
}
