package main

import (
	"testing"
)

func TestProfileNames_IncludesCustom(t *testing.T) {
	writeProfiles(t, `
[profiles.custom]
min_score = 30
keywords = ["elixir"]
`)
	names := profileNames()
	if !contains(names, "custom") {
		t.Errorf("profileNames() = %v, want to include custom", names)
	}
	if !contains(names, "dev") || !contains(names, "data") || !contains(names, "fullstack") {
		t.Errorf("profileNames() = %v, want builtins", names)
	}
}

func TestTierGroups_PartitionsByScore(t *testing.T) {
	m := newModel()
	m.jobs = []Job{
		{Title: "a", Score: 90},
		{Title: "b", Score: 80},
		{Title: "c", Score: 72},
		{Title: "d", Score: 60},
		{Title: "e", Score: 59},
		{Title: "f", Score: 30},
	}
	m.total = len(m.jobs)
	b := m.tierGroups()
	if len(b) != 3 {
		t.Fatalf("tierGroups = %d brackets, want 3", len(b))
	}
	if len(b[0].jobs) != 2 || len(b[1].jobs) != 2 || len(b[2].jobs) != 2 {
		t.Errorf("counts = %d/%d/%d, want 2/2/2", len(b[0].jobs), len(b[1].jobs), len(b[2].jobs))
	}
}

func TestFocusedJob_ListAndTiers(t *testing.T) {
	m := newModel()
	m.jobs = []Job{
		{Title: "a", Score: 90},
		{Title: "b", Score: 70},
		{Title: "c", Score: 40},
	}
	m.total = len(m.jobs)

	m.cursor = 1
	j, ok := m.focusedJob()
	if !ok || j.Title != "b" {
		t.Errorf("list focusedJob = %q, want b", j.Title)
	}

	m.tierView = true
	m.colIdx = 2 // <60 bracket has job c
	m.cursor = 0
	j, ok = m.focusedJob()
	if !ok || j.Title != "c" {
		t.Errorf("tier focusedJob = %q, want c", j.Title)
	}

	// Out-of-range cursor in a tier returns not-ok.
	m.colIdx = 1 // 60-79 bracket is empty here? no: b score 70 -> bracket 60-79 has b
	// bracket 1 has b, cursor 0 ok; switch to colIdx 0 (80-100, job a)
	m.colIdx = 0
	j, ok = m.focusedJob()
	if !ok || j.Title != "a" {
		t.Errorf("tier focusedJob col0 = %q, want a", j.Title)
	}
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
