// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package kernel

import "errors"

// levelResources levels with the default horizon and strategy and reports
// false only for a dependency cycle, the outcome most leveling tests check.
// capacities follows DetectOverallocations' convention (missing = 1.0).
func levelResources(tasks map[string]*Task, capacities map[string]float64) bool {
	return levelResourcesWithPlan(tasks, capacityPlanFromMap(capacities))
}

// levelResourcesWithPlan is levelResources with a full capacity plan.
func levelResourcesWithPlan(tasks map[string]*Task, plan ResourceCapacityPlan) bool {
	_, err := LevelResourcesWithOptions(tasks, plan, LevelingOptions{})
	return !errors.Is(err, ErrSchedulingCycle)
}
