package main

import "testing"

func TestParseFilter_ScoreAndRemote(t *testing.T) {
	f := parseFilter("remote score:70")
	if f.minScore != 70 {
		t.Errorf("minScore = %d, want 70", f.minScore)
	}
	if f.remote == nil || !*f.remote {
		t.Errorf("remote = %v, want true", f.remote)
	}
}

func TestParseFilter_Onsite(t *testing.T) {
	f := parseFilter("presencial")
	if f.remote == nil || *f.remote {
		t.Errorf("remote = %v, want false", f.remote)
	}
}

func TestParseFilter_SourceAndType(t *testing.T) {
	f := parseFilter("source:programathor tipo:junior")
	if f.source != "programathor" {
		t.Errorf("source = %q, want programathor", f.source)
	}
	if f.typ != "junior" {
		t.Errorf("typ = %q, want junior", f.typ)
	}
}

func TestParseFilter_Words(t *testing.T) {
	f := parseFilter("python svelte")
	if len(f.words) != 2 || f.words[0] != "python" || f.words[1] != "svelte" {
		t.Errorf("words = %v, want [python svelte]", f.words)
	}
}

func TestParseFilter_Empty(t *testing.T) {
	f := parseFilter("")
	if f.minScore != 0 || f.remote != nil || len(f.words) != 0 {
		t.Errorf("empty filter not neutral: %+v", f)
	}
}

func TestFilterMatches(t *testing.T) {
	base := Job{
		Source:   "remotive",
		Title:    "Senior Python Developer",
		Company:  "Acme",
		Location: "Remote",
		Remote:   true,
		Type:     "full_time",
		Tags:     []string{"python", "django"},
		Raw:      "Machine learning stack",
		Score:    85,
	}

	cases := []struct {
		name  string
		query string
		want  bool
	}{
		{"empty matches everything", "", true},
		{"word hits title", "python", true},
		{"word hits raw", "machine", true},
		{"missing word excluded", "ruby", false},
		{"all words required", "python ruby", false},
		{"score above threshold", "score:80", true},
		{"score below threshold excluded", "score:90", false},
		{"remote only includes remote", "remote", true},
		{"onsite excludes remote", "onsite", false},
		{"source match", "src:remotive", true},
		{"source mismatch excluded", "source:programathor", false},
		{"type match", "type:full_time", true},
		{"type mismatch excluded", "tipo:freelance", false},
	}
	for _, tc := range cases {
		if got := parseFilter(tc.query).matches(base); got != tc.want {
			t.Errorf("%s: parseFilter(%q).matches = %v, want %v", tc.name, tc.query, got, tc.want)
		}
	}
}
