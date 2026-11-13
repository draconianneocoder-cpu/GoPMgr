// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package schedaudit

import "gopmgr/internal/kernel"

// cloneTask returns a copy of t that shares no slice with it. Every slice
// field of kernel.Task is copied; TestCloneTaskCoversEveryReferenceField
// fails when kernel.Task gains a reference field this function does not
// know about.
func cloneTask(t kernel.Task) kernel.Task {
	c := t
	c.Precedents = cloneSlice(t.Precedents)
	c.Links = cloneSlice(t.Links)
	c.WorkDays = cloneSlice(t.WorkDays)
	c.PlannedWorkSegments = cloneSlice(t.PlannedWorkSegments)
	c.Assignments = cloneSlice(t.Assignments)
	for i := range c.Assignments {
		c.Assignments[i].SkillTags = cloneSlice(t.Assignments[i].SkillTags)
	}
	return c
}

// cloneSlice copies s, keeping a nil slice nil and an empty one empty.
func cloneSlice[T any](s []T) []T {
	if s == nil {
		return nil
	}
	return append(make([]T, 0, len(s)), s...)
}
