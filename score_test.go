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

	// Same job, but one says "presencial" in the title.
	//
	// This test used to expect s1+1, and the comment explained why: matching was
	// substring-based, so "presencial" scored a hit for the "ia" keyword (+6)
	// that all but cancelled the -5 penalty. With token matching the penalty
	// stands on its own.
	j1 := Job{Title: "python developer"}
	j2 := Job{Title: "python developer presencial"}

	s1 := sc.Score(j1)
	s2 := sc.Score(j2)

	if s2 != s1-5 {
		t.Errorf("expected s2 = s1-5 (presencial penalty), got s1=%d s2=%d", s1, s2)
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

func TestJobHash_ScoringRelevantFields(t *testing.T) {
	base := Job{Source: "remotive", ID: "1", Title: "dev"}

	for _, mutate := range []struct {
		name string
		to   Job
	}{
		{"remote", func() Job { j := base; j.Remote = true; return j }()},
		{"type", func() Job { j := base; j.Type = "clt"; return j }()},
		{"deadline", func() Job { j := base; j.Deadline = "2026-09-01"; return j }()},
		// buildPrompt sends these two, so the cache key has to move when they
		// do. setDetails fills Description in after the first sync.
		{"description", func() Job { j := base; j.Description = "Vaga remota de backend"; return j }()},
		{"salary", func() Job { j := base; j.Salary = "R$ 8.000"; return j }()},
	} {
		if jobHash(base) == jobHash(mutate.to) {
			t.Errorf("jobHash should differ when %s changes (cache staleness)", mutate.name)
		}
	}
}

func TestNormalizeText(t *testing.T) {
	cases := map[string]string{
		"Estágio em Ciência de Dados": " estagio em ciencia de dados ",
		"Node.js / TypeScript":        " node js typescript ",
		"":                            " ",
	}
	for in, want := range cases {
		if got := normalizeText(in); got != want {
			t.Errorf("normalizeText(%q) = %q, want %q", in, got, want)
		}
	}
}

// The short entries in the dev profile ("ia", "ml", "ai", "go", "jr") are the
// reason matching had to become token-based: every one of them is a substring
// of a common Portuguese word.
func TestKeywordsDoNotMatchInsideWords(t *testing.T) {
	text := normalizeText("Temos experiência em tecnologia, ciência e algoritmos. Escrevemos HTML todo dia e jogamos na categoria certa.")
	for _, kw := range []string{"ia", "ml", "ai", "go", "jr"} {
		if matchesToken(text, kw) {
			t.Errorf("keyword %q matched inside a word: %s", kw, text)
		}
	}
	// ...but a standalone occurrence still counts.
	if !matchesToken(normalizeText("Vaga de IA generativa"), "ia") {
		t.Error(`"ia" should match when it stands alone`)
	}
	if !matchesToken(normalizeText("Backend em Go"), "go") {
		t.Error(`"go" should match when it stands alone`)
	}
}

// A long PT-BR description with no real match used to saturate the keyword cap
// purely through substrings, landing near the top of the range.
func TestNoiseDescriptionDoesNotSaturate(t *testing.T) {
	sc := &heuristicScorer{opts: builtinProfiles["dev"]}
	noise := Job{
		Title: "Auxiliar administrativo",
		Description: "Buscamos profissional com experiência em rotinas administrativas, " +
			"conhecimento em tecnologia, ciência da organização, atendimento no dia a dia, " +
			"categoria de documentos e diálogo com fornecedores. Jornada presencial.",
	}
	real := Job{Title: "Desenvolvedor Backend Python Júnior", Remote: true}

	noiseScore := sc.Score(noise)
	realScore := sc.Score(real)

	if noiseScore >= realScore {
		t.Errorf("noise (%d) should not outscore a real match (%d)", noiseScore, realScore)
	}
	if noiseScore > 30 {
		t.Errorf("noise scored %d; substring matching used to push it near the cap", noiseScore)
	}
}

// A term listed twice in a profile used to count twice toward the 8-hit cap.
func TestDuplicateKeywordCountsOnce(t *testing.T) {
	once := &heuristicScorer{opts: scoreOptions{Preferred: []string{"python"}}}
	twice := &heuristicScorer{opts: scoreOptions{Preferred: []string{"python", "Python", "pythón"}}}

	j := Job{Title: "Desenvolvedor Python"}
	if a, b := once.Score(j), twice.Score(j); a != b {
		t.Errorf("duplicate keywords changed the score: %d vs %d", a, b)
	}
}
