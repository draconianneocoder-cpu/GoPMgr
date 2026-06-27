// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package db

import "testing"

func TestResourceCalendarCRUD(t *testing.T) {
	d := newBackupTestDB(t)
	p, err := d.UpsertProject(Project{Name: "Resource Calendars"})
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}

	saved, err := d.SaveResourceCalendar(ResourceCalendar{
		ProjectID:       p.ID,
		Name:            "Alice part time",
		Resource:        "alice",
		DefaultCapacity: 1,
		WeeklyCapacity:  map[int]float64{0: 0.5, 4: 0.5},
		Overrides:       map[int]float64{3: 0},
		SkillTags:       []string{"piping", "qa"},
		Notes:           map[int]string{3: "medical leave"},
	})
	if err != nil {
		t.Fatalf("SaveResourceCalendar: %v", err)
	}
	if saved.ID == "" {
		t.Fatal("SaveResourceCalendar did not assign an ID")
	}
	if saved.CreatedAt.IsZero() || saved.UpdatedAt.IsZero() {
		t.Fatalf("timestamps not populated: %+v", saved)
	}
	if saved.WeeklyCapacity[0] != 0.5 || saved.Overrides[3] != 0 || saved.Notes[3] != "medical leave" {
		t.Fatalf("saved calendar mismatch: %+v", saved)
	}

	list, err := d.ListResourceCalendars(p.ID)
	if err != nil {
		t.Fatalf("ListResourceCalendars: %v", err)
	}
	if len(list) != 1 || list[0].ID != saved.ID {
		t.Fatalf("list = %+v, want saved calendar", list)
	}

	saved.DefaultCapacity = 0.75
	saved.SkillTags = []string{"controls"}
	updated, err := d.SaveResourceCalendar(saved)
	if err != nil {
		t.Fatalf("SaveResourceCalendar update: %v", err)
	}
	if updated.DefaultCapacity != 0.75 || len(updated.SkillTags) != 1 || updated.SkillTags[0] != "controls" {
		t.Fatalf("updated calendar mismatch: %+v", updated)
	}

	if err := d.DeleteResourceCalendar(saved.ID); err != nil {
		t.Fatalf("DeleteResourceCalendar: %v", err)
	}
	list, err = d.ListResourceCalendars(p.ID)
	if err != nil {
		t.Fatalf("ListResourceCalendars after delete: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("after delete list = %+v, want empty", list)
	}
}

func TestResourceCalendarTableExists(t *testing.T) {
	d := newBackupTestDB(t)

	cols, err := d.columnSet("resource_calendars")
	if err != nil {
		t.Fatalf("columnSet resource_calendars: %v", err)
	}
	for _, name := range []string{"id", "project_id", "resource", "name", "default_capacity", "weekly_capacity", "overrides", "skill_tags", "notes"} {
		if _, ok := cols[name]; !ok {
			t.Fatalf("resource_calendars.%s column missing", name)
		}
	}
}
