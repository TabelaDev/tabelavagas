package main

import (
	"fmt"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := openStoreAt(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.close)
	return s
}

func TestTopAndTopUnnotified(t *testing.T) {
	s := newTestStore(t)
	jobs := []Job{
		{Source: "remotive", ID: "1", Title: "python machine learning dev", URL: "u1"},
		{Source: "remotive", ID: "2", Title: "java dev", URL: "u2"},
	}
	if _, err := s.save(jobs); err != nil {
		t.Fatal(err)
	}
	sc := &heuristicScorer{opts: builtinProfiles["dev"]}
	if err := scoreAll(s, sc); err != nil {
		t.Fatal(err)
	}

	all, err := s.top(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("top(10) = %d jobs, want 2", len(all))
	}
	if all[0].ID != "1" {
		t.Fatalf("top[0] = %q, want 1 (higher score)", all[0].ID)
	}

	unnotified, err := s.topUnnotified(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(unnotified) != 2 {
		t.Fatalf("topUnnotified(10) = %d jobs, want 2", len(unnotified))
	}

	if err := s.markNotified(all[:1]); err != nil {
		t.Fatal(err)
	}
	unnotified, err = s.topUnnotified(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(unnotified) != 1 {
		t.Fatalf("topUnnotified after mark = %d jobs, want 1", len(unnotified))
	}
	if unnotified[0].ID != "2" {
		t.Errorf("remaining unnotified = %q, want 2", unnotified[0].ID)
	}
	// top() still returns everything regardless of notified.
	all, _ = s.top(10)
	if len(all) != 2 {
		t.Errorf("top(10) after mark = %d jobs, want 2", len(all))
	}
}

func TestMarkNotified_Empty(t *testing.T) {
	s := newTestStore(t)
	if err := s.markNotified(nil); err != nil {
		t.Fatalf("markNotified(nil) error: %v", err)
	}
}

func TestTopFiltered_MinScore(t *testing.T) {
	s := newTestStore(t)
	jobs := []Job{
		{Source: "remotive", ID: "1", Title: "python machine learning dev", URL: "u1"},
		{Source: "remotive", ID: "2", Title: "java dev", URL: "u2"},
	}
	if _, err := s.save(jobs); err != nil {
		t.Fatal(err)
	}
	sc := &heuristicScorer{opts: builtinProfiles["dev"]}
	if err := scoreAll(s, sc); err != nil {
		t.Fatal(err)
	}

	// High bar: only the python/ML job (score ~38) clears it.
	high, err := s.topFiltered(10, 30, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(high) != 1 || high[0].ID != "1" {
		t.Fatalf("topFiltered(min=30) = %v, want only job 1", ids(high))
	}

	// Low bar: both pass.
	low, err := s.topFiltered(10, 1, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(low) != 2 {
		t.Fatalf("topFiltered(min=1) = %d jobs, want 2", len(low))
	}

	// min + only-new combined.
	if err := s.markNotified(high); err != nil {
		t.Fatal(err)
	}
	remaining, err := s.topFiltered(10, 30, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 0 {
		t.Fatalf("topFiltered(min=30, onlyNew) = %v, want none", ids(remaining))
	}
}

func TestSetVetoed_ExcludesFromTop(t *testing.T) {
	s := newTestStore(t)
	jobs := []Job{
		{Source: "remotive", ID: "1", Title: "python machine learning dev", URL: "u1"},
		{Source: "remotive", ID: "2", Title: "java dev", URL: "u2"},
	}
	if _, err := s.save(jobs); err != nil {
		t.Fatal(err)
	}
	if err := scoreAll(s, &heuristicScorer{opts: builtinProfiles["dev"]}); err != nil {
		t.Fatal(err)
	}

	if err := s.setVetoed("remotive", "1", true); err != nil {
		t.Fatal(err)
	}

	// CLI view (no vetoed): only job 2 remains.
	top, err := s.top(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(top) != 1 || top[0].ID != "2" {
		t.Fatalf("top after veto = %v, want only job 2", ids(top))
	}

	// TUI view including vetoed: both show, job 1 flagged.
	with, err := s.topFiltered(10, 0, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(with) != 2 {
		t.Fatalf("topFiltered(includeVetoed) = %d jobs, want 2", len(with))
	}
	found := false
	for _, j := range with {
		if j.ID == "1" && !j.Vetoed {
			t.Error("job 1 should be flagged vetoed")
		}
		if j.ID == "1" {
			found = true
		}
	}
	if !found {
		t.Error("job 1 missing from includeVetoed query")
	}

	// Un-veto restores it.
	if err := s.setVetoed("remotive", "1", false); err != nil {
		t.Fatal(err)
	}
	top, _ = s.top(10)
	if len(top) != 2 {
		t.Fatalf("top after unveto = %d jobs, want 2", len(top))
	}
}

func TestActivityLog(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 5; i++ {
		if err := s.addActivity("collect", fmt.Sprintf("run %d", i)); err != nil {
			t.Fatal(err)
		}
	}
	acts, err := s.recentActivity(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(acts) != 5 {
		t.Fatalf("recentActivity = %d, want 5", len(acts))
	}
	// Newest first.
	if acts[0].Detail != "run 4" {
		t.Errorf("first = %q, want run 4", acts[0].Detail)
	}
	// Limit works.
	acts, _ = s.recentActivity(2)
	if len(acts) != 2 || acts[0].Detail != "run 4" {
		t.Errorf("recentActivity(2) = %+v", acts)
	}
}

func TestActivityPrune(t *testing.T) {
	s := newTestStore(t)
	// Insert one entry, then backdate it beyond the 7-day window.
	if err := s.addActivity("open", "antiga"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE activity SET at = datetime('now', '-8 days')`); err != nil {
		t.Fatal(err)
	}
	// A new insert triggers the prune.
	if err := s.addActivity("open", "nova"); err != nil {
		t.Fatal(err)
	}
	acts, err := s.recentActivity(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(acts) != 1 || acts[0].Detail != "nova" {
		t.Fatalf("after prune = %+v, want only nova", acts)
	}
}

func ids(jobs []Job) []string {
	out := make([]string, len(jobs))
	for i, j := range jobs {
		out[i] = j.ID
	}
	return out
}
