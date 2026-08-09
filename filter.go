package main

import (
	"strconv"
	"strings"
)

// bracket is one score tier in the TUI column view.
type bracket struct {
	label string
	jobs  []Job
}

// jobFilter is the live TUI filter. Tokens (whitespace-separated, case
// insensitive) AND together:
//
//	remote | onsite | presencial  modo de trabalho
//	score:NN | min:NN             score mínimo
//	source:NAME | src:NAME        fonte (remotive/programathor)
//	type:TYPE | tipo:TYPE         nível/contrato (estágio, junior, clt, ...)
//	qualquer outra palavra        busca por substring no título/empresa/tags
type jobFilter struct {
	minScore int
	remote   *bool
	source   string
	typ      string
	words    []string
}

func parseFilter(q string) jobFilter {
	var f jobFilter
	for _, tok := range strings.Fields(strings.ToLower(q)) {
		switch {
		case tok == "remote":
			b := true
			f.remote = &b
		case tok == "onsite" || tok == "presencial":
			b := false
			f.remote = &b
		case strings.HasPrefix(tok, "score:") || strings.HasPrefix(tok, "min:"):
			if _, v, ok := strings.Cut(tok, ":"); ok {
				if n, err := strconv.Atoi(v); err == nil {
					f.minScore = n
				}
			}
		case strings.HasPrefix(tok, "source:") || strings.HasPrefix(tok, "src:"):
			if _, v, ok := strings.Cut(tok, ":"); ok && v != "" {
				f.source = v
			}
		case strings.HasPrefix(tok, "type:") || strings.HasPrefix(tok, "tipo:"):
			if _, v, ok := strings.Cut(tok, ":"); ok && v != "" {
				f.typ = v
			}
		default:
			f.words = append(f.words, tok)
		}
	}
	return f
}

func (f jobFilter) matches(j Job) bool {
	if f.minScore > 0 && j.Score < f.minScore {
		return false
	}
	if f.remote != nil && j.Remote != *f.remote {
		return false
	}
	if f.source != "" && j.Source != f.source {
		return false
	}
	if f.typ != "" {
		needle := strings.ToLower(f.typ)
		hay := strings.ToLower(j.Type + " " + j.Title + " " + strings.Join(j.Tags, " "))
		if !strings.Contains(hay, needle) {
			return false
		}
	}
	if len(f.words) > 0 {
		hay := strings.ToLower(j.Title + " " + j.Company + " " + j.Location + " " +
			strings.Join(j.Tags, " ") + " " + j.Raw)
		for _, w := range f.words {
			if !strings.Contains(hay, w) {
				return false
			}
		}
	}
	return true
}
