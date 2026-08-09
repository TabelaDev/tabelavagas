package main

import (
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
