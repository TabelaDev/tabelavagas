package main

import "strings"

// Scorer rates a job 0–100 for the user's profile. Implementations may be
// local heuristic, LLM-backed, or anything in between.
type Scorer interface {
	Score(Job) int
}

// scoreOptions captures the profile the heuristic scorer uses to decide
// "worth it for me". Overridable via flags/env/toml config.
type scoreOptions struct {
	MinScore    int
	Preferred   []string
	RemoteBonus int
	InternBonus int
	CityBonus   int
	City        []string
}

// Built-in profiles — merged with user config when present.
var builtinProfiles = map[string]scoreOptions{
	"dev": {
		MinScore:    70,
		Preferred:   []string{"estágio", "estagio", "intern", "júnior", "junior", "jr", "júnior", "dados", "data", "data science", "analytics", "machine learning", "ml", "ia", "ai", "matemática", "matematica", "math", "python", "svelte", "typescript", "go", "backend", "fullstack", "dev", "software"},
		RemoteBonus: 10,
		InternBonus: 15,
		CityBonus:   8,
		City:        []string{"belo horizonte", "bh"},
	},
	"data": {
		MinScore:    65,
		Preferred:   []string{"dados", "data", "data science", "analytics", "machine learning", "ml", "ia", "ai", "python", "matemática", "matematica", "math"},
		RemoteBonus: 10,
		InternBonus: 15,
		CityBonus:   8,
		City:        []string{"belo horizonte", "bh"},
	},
	"fullstack": {
		MinScore:    60,
		Preferred:   []string{"fullstack", "full-stack", "full stack", "frontend", "react", "next", "vue", "node", "typescript", "javascript"},
		RemoteBonus: 10,
		InternBonus: 10,
		CityBonus:   8,
		City:        []string{"belo horizonte", "bh"},
	},
}

// heuristicScorer implements Scorer using a keyword/bonus model.
type heuristicScorer struct {
	opts scoreOptions
}

func (h *heuristicScorer) Score(j Job) int {
	text := strings.ToLower(j.Title + " " + j.Company + " " + strings.Join(j.Tags, " ") + " " + j.Raw + " " + j.Description)
	s := 20

	hits := 0
	for _, kw := range h.opts.Preferred {
		if strings.Contains(text, strings.ToLower(kw)) {
			hits++
		}
	}
	if hits > 8 {
		hits = 8
	}
	s += hits * 6

	if containsAny(text, []string{"estágio", "estagio", "intern"}) {
		s += h.opts.InternBonus
	}
	if containsAny(text, []string{"júnior", "junior", " jr ", "júnior"}) {
		s += 8
	}
	if j.Remote {
		s += h.opts.RemoteBonus
	}
	for _, c := range h.opts.City {
		if strings.Contains(strings.ToLower(j.Location), c) {
			s += h.opts.CityBonus
			break
		}
	}
	if strings.Contains(text, "presencial") {
		s -= 5
	}

	if s > 100 {
		s = 100
	}
	if s < 0 {
		s = 0
	}
	return s
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
