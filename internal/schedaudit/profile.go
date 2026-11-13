// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package schedaudit

import (
	"errors"
	"fmt"
	"math"
)

// Profile holds every threshold the rules use. Each finding names its
// profile and shows the threshold it was judged against.
type Profile struct {
	Name string `json:"name"`
	// LongLagDays: SQ-06 reports a lag greater than this, in working days.
	LongLagDays float64 `json:"long_lag_days"`
	// HighFloatDays: SQ-10 reports total float greater than this.
	HighFloatDays float64 `json:"high_float_days"`
	// LongDurationDays: SQ-12 reports a duration greater than this.
	LongDurationDays float64 `json:"long_duration_days"`
	// Limits gives pass/fail thresholds to metrics, keyed by metric ID
	// (LogicMetric or a rule ID). A metric without a limit is reported
	// without a pass or fail.
	Limits map[string]Limit `json:"limits,omitempty"`
}

// DefaultProfile is GoPMgr's shipped profile. Its limits are a product
// choice made on 2026-11-13, not a published standard: 44 working days for
// high float and long duration, as in the DCMA-style profile, and 10 working
// days (two working weeks) for a long lag. It sets no pass/fail limits on
// metrics. Each call returns a new value.
func DefaultProfile() Profile {
	return Profile{
		Name:             "GoPMgr default",
		LongLagDays:      10,
		HighFloatDays:    44,
		LongDurationDays: 44,
	}
}

// DCMAStyleProfile uses the thresholds commonly published for the DCMA
// 14-point assessment. It is DCMA-style only: GoPMgr makes no certification
// or conformance claim. Its lag limit is 0, so every positive lag counts, as
// in the DCMA lag test. Each call returns a new value.
func DCMAStyleProfile() Profile {
	return Profile{
		Name:             "DCMA-style",
		LongLagDays:      0,
		HighFloatDays:    44,
		LongDurationDays: 44,
		Limits: map[string]Limit{
			LogicMetric:                {Percent: 5},
			string(DanglingReference):  {Percent: 0},
			string(DependencyCycle):    {Percent: 0},
			string(Lead):               {Percent: 0},
			string(LongLag):            {Percent: 5},
			string(NonFinishToStart):   {AtLeast: true, Percent: 90},
			string(HardConstraint):     {Percent: 5},
			string(ConstraintViolated): {Percent: 0},
			string(HighFloat):          {Percent: 5},
			string(NegativeFloat):      {Percent: 0},
			string(LongDuration):       {Percent: 5},
			string(InvalidActuals):     {Percent: 0},
			string(UnresourcedTask):    {Percent: 5},
		},
	}
}

// ErrInvalidProfile reports a profile the audit cannot use.
var ErrInvalidProfile = errors.New("schedaudit: invalid profile")

func (p Profile) validate() error {
	if p.Name == "" {
		return fmt.Errorf("%w: it has no name", ErrInvalidProfile)
	}
	for name, v := range map[string]float64{
		"long lag": p.LongLagDays, "high float": p.HighFloatDays, "long duration": p.LongDurationDays,
	} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			return fmt.Errorf("%w: the %s limit must be a number of days, 0 or more", ErrInvalidProfile, name)
		}
	}
	for id, l := range p.Limits {
		if math.IsNaN(l.Percent) || l.Percent < 0 || l.Percent > 100 {
			return fmt.Errorf("%w: the limit on %s must be a percentage from 0 to 100", ErrInvalidProfile, id)
		}
	}
	return nil
}
