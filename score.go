package main

import (
	"strings"
	"unicode"
)

// accentFolds maps the accented letters that show up in PT-BR job postings to
// their bare form, so "estágio" and "estagio" are the same token and a profile
// no longer has to list both.
var accentFolds = map[rune]rune{
	'á': 'a', 'à': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a',
	'é': 'e', 'è': 'e', 'ê': 'e', 'ë': 'e',
	'í': 'i', 'ì': 'i', 'î': 'i', 'ï': 'i',
	'ó': 'o', 'ò': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o',
	'ú': 'u', 'ù': 'u', 'û': 'u', 'ü': 'u',
	'ç': 'c', 'ñ': 'n',
}

// normalizeText lowercases, folds accents and turns every run of non
// alphanumeric characters into a single space, returning the result padded with
// a space on each end.
//
// Matching then becomes a search for " keyword ", which gives word-boundary
// semantics to single words and phrases alike with no regexp per keyword. The
// previous strings.Contains match was catastrophic on short entries: "ia" is
// inside experiência, tecnologia, ciência and dia; "ml" is inside html; "go" is
// inside algoritmo, jogo and categoria. Since hits saturate at 8, almost any
// long PT-BR description reached the cap and the ranking degenerated into a
// function of description length.
func normalizeText(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte(' ')
	pendingSpace := true
	for _, r := range strings.ToLower(s) {
		if folded, ok := accentFolds[r]; ok {
			r = folded
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			pendingSpace = false
			continue
		}
		if !pendingSpace {
			b.WriteByte(' ')
			pendingSpace = true
		}
	}
	if !pendingSpace {
		b.WriteByte(' ')
	}
	return b.String()
}

// normalizeKeyword is normalizeText without the padding, for the needle side.
func normalizeKeyword(s string) string {
	return strings.TrimSpace(normalizeText(s))
}

// matchesToken reports whether a normalized (padded) haystack contains kw as a
// whole word or whole phrase.
func matchesToken(padded, kw string) bool {
	if kw == "" {
		return false
	}
	return strings.Contains(padded, " "+kw+" ")
}

// matchesAnyToken is matchesToken over several keywords, which must already be
// normalized.
func matchesAnyToken(padded string, kws ...string) bool {
	for _, kw := range kws {
		if matchesToken(padded, kw) {
			return true
		}
	}
	return false
}

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
	// Accented and unaccented spellings of the same term are no longer listed
	// separately — normalizeKeyword folds them — and duplicates are dropped by
	// the scorer anyway. Short entries like "ia", "ml", "ai", "go" and "jr" are
	// safe here only because matching is token-based.
	"dev": {
		MinScore:    70,
		Preferred:   []string{"estágio", "intern", "júnior", "jr", "dados", "data", "data science", "analytics", "machine learning", "ml", "ia", "ai", "matemática", "math", "python", "svelte", "typescript", "go", "backend", "fullstack", "dev", "software"},
		RemoteBonus: 10,
		InternBonus: 15,
		CityBonus:   8,
		City:        []string{"belo horizonte", "bh"},
	},
	"data": {
		MinScore:    65,
		Preferred:   []string{"dados", "data", "data science", "analytics", "machine learning", "ml", "ia", "ai", "python", "matemática", "math"},
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
	text := normalizeText(j.Title + " " + j.Company + " " + strings.Join(j.Tags, " ") + " " + j.Raw + " " + j.Description)
	s := 20

	// Distinct keywords only. A term listed twice in a profile used to count
	// twice toward the cap — "júnior" was in the dev profile twice, so a junior
	// posting silently got +12 instead of +6, on top of the +8 bonus below for
	// the same concept. Folding also collapses the accented/unaccented pairs
	// the profiles used to carry.
	seen := map[string]bool{}
	hits := 0
	for _, kw := range h.opts.Preferred {
		k := normalizeKeyword(kw)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		if matchesToken(text, k) {
			hits++
		}
	}
	if hits > 8 {
		hits = 8
	}
	s += hits * 6

	if matchesAnyToken(text, "estagio", "intern", "trainee") {
		s += h.opts.InternBonus
	}
	if matchesAnyToken(text, "junior", "jr") {
		s += 8
	}
	if j.Remote {
		s += h.opts.RemoteBonus
	}
	location := normalizeText(j.Location)
	for _, c := range h.opts.City {
		if matchesToken(location, normalizeKeyword(c)) {
			s += h.opts.CityBonus
			break
		}
	}
	if matchesToken(text, "presencial") {
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
