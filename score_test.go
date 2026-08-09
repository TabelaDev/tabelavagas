package main

import (
	"testing"
)

func TestHeuristicScore_BasicRange(t *testing.T) {
	sc := &heuristicScorer{opts: builtinProfiles["dev"]}

	// Empty job should score low but not zero
	j := Job{Title: "test"}
	score := sc.Score(j)
	if score < 0 || score > 100 {
		t.Errorf("score out of range: %d", score)
	}
}

func TestHeuristicScore_RemoteBonus(t *testing.T) {
	sc := &heuristicScorer{opts: builtinProfiles["dev"]}

	j1 := Job{Title: "developer", Remote: false}
	j2 := Job{Title: "developer", Remote: true}

	s1 := sc.Score(j1)
	s2 := sc.Score(j2)

	if s2 <= s1 {
		t.Errorf("remote job should score higher: %d vs %d", s2, s1)
	}
}

func TestHeuristicScore_InternBonus(t *testing.T) {
	sc := &heuristicScorer{opts: builtinProfiles["dev"]}

	j1 := Job{Title: "developer"}
	j2 := Job{Title: "estágio developer"}

	s1 := sc.Score(j1)
	s2 := sc.Score(j2)

	if s2 <= s1 {
		t.Errorf("intern job should score higher: %d vs %d", s2, s1)
	}
}

func TestHeuristicScore_CityBonus(t *testing.T) {
	sc := &heuristicScorer{opts: builtinProfiles["dev"]}

	j1 := Job{Title: "developer", Location: "São Paulo"}
	j2 := Job{Title: "developer", Location: "Belo Horizonte"}

	s1 := sc.Score(j1)
	s2 := sc.Score(j2)

	if s2 <= s1 {
		t.Errorf("BH job should score higher: %d vs %d", s2, s1)
	}
}

func TestHeuristicScore_PresencialPenalty(t *testing.T) {
	sc := &heuristicScorer{opts: builtinProfiles["dev"]}

	// Same job, but one has "presencial" in title
	// Note: "presencial" contains "ia" which is a keyword, so it gets a hit.
	// The presencial penalty (-5) should still apply.
	j1 := Job{Title: "python developer"}
	j2 := Job{Title: "python developer presencial"}

	s1 := sc.Score(j1)
	s2 := sc.Score(j2)

	// s2 should be s1 + 6 (ia hit) - 5 (presencial) = s1 + 1
	if s2 != s1+1 {
		t.Errorf("expected s2 = s1+1 (ia hit - presencial penalty), got s1=%d s2=%d", s1, s2)
	}
}

func TestHeuristicScore_Keywords(t *testing.T) {
	sc := &heuristicScorer{opts: builtinProfiles["dev"]}

	j1 := Job{Title: "developer"}
	j2 := Job{Title: "python developer"}

	s1 := sc.Score(j1)
	s2 := sc.Score(j2)

	if s2 <= s1 {
		t.Errorf("keyword match should score higher: %d vs %d", s2, s1)
	}
}

func TestHeuristicScore_Capped(t *testing.T) {
	sc := &heuristicScorer{opts: builtinProfiles["dev"]}

	// Job with many keywords should still be capped at 100
	j := Job{
		Title:    "estágio python machine learning ia dados svelte typescript go backend fullstack dev software",
		Remote:   true,
		Location: "Belo Horizonte",
	}
	score := sc.Score(j)
	if score > 100 {
		t.Errorf("score should be capped at 100: %d", score)
	}
}

func TestParseLLMScore_ValidJSON(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{`{"score": 85, "reason": "good match"}`, 85},
		{`{"score": 0, "reason": "bad"}`, 0},
		{`{"score": 100, "reason": "perfect"}`, 100},
		{"```json\n{\"score\": 72, \"reason\": \"ok\"}\n```", 72},
	}

	for _, tt := range tests {
		got, err := parseLLMScore(tt.input)
		if err != nil {
			t.Errorf("parseLLMScore(%q) error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseLLMScore(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestParseLLMScore_Invalid(t *testing.T) {
	_, err := parseLLMScore("not a score")
	if err == nil {
		t.Error("expected error for invalid input")
	}
}

func TestJobHash_DifferentJobs(t *testing.T) {
	j1 := Job{Source: "remotive", ID: "1", Title: "dev"}
	j2 := Job{Source: "remotive", ID: "2", Title: "dev"}

	h1 := jobHash(j1)
	h2 := jobHash(j2)

	if h1 == h2 {
		t.Error("different jobs should have different hashes")
	}
}

func TestJobHash_SameJob(t *testing.T) {
	j1 := Job{Source: "remotive", ID: "1", Title: "dev"}
	j2 := Job{Source: "remotive", ID: "1", Title: "dev"}

	h1 := jobHash(j1)
	h2 := jobHash(j2)

	if h1 != h2 {
		t.Error("same job should have same hash")
	}
}
