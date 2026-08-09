package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestCleanTitle_KeepsLevelWords(t *testing.T) {
	// Regression: cutting at "Sênior"/"Pleno"/"Júnior" used to truncate
	// legit titles like "Desenvolvedor(a) .NET Júnior" → ".NET".
	cases := map[string]string{
		"Desenvolvedor(a) .NET Júnior": "Desenvolvedor(a) .NET Júnior",
		"Desenvolvedor Sênior Python":  "Desenvolvedor Sênior Python",
		"📍 PRESENCIAL - SOMENTE PARA CANDIDATOS NO LOCAL Desenvolvedor Sênior Python": "Desenvolvedor Sênior Python",
		"PRESENCIAL Engenheiro de Software":                                           "Engenheiro de Software",
		"Vencida Analista de Dados":                                                   "Analista de Dados",
		"  Pleno  React Developer  ":                                                  "Pleno  React Developer",
	}
	for in, want := range cases {
		if got := cleanTitle(in); got != want {
			t.Errorf("cleanTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseCard_StripsBadgeSpans(t *testing.T) {
	html := `<a href="/jobs/123-desenvolvedor">
		<h3><span class="presential-only-badge">📍 PRESENCIAL</span> <span>Vencida</span> Desenvolvedor(a) .NET Júnior</h3>
	</a>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	sel := doc.Find("a[href^='/jobs/']").First()
	j := (&programathorCollector{}).parseCard(sel, "/jobs/123-desenvolvedor")

	if j.Title != "Desenvolvedor(a) .NET Júnior" {
		t.Errorf("Title = %q, want %q", j.Title, "Desenvolvedor(a) .NET Júnior")
	}
	if j.ID != "123" {
		t.Errorf("ID = %q, want %q", j.ID, "123")
	}
	if j.Source != "programathor" {
		t.Errorf("Source = %q, want programathor", j.Source)
	}
}

func TestExtractID(t *testing.T) {
	cases := map[string]string{
		"/jobs/123-desenvolvedor": "123",
		"/jobs/4567":              "4567",
	}
	for in, want := range cases {
		if got := extractID(in); got != want {
			t.Errorf("extractID(%q) = %q, want %q", in, got, want)
		}
	}
}

func writeProfiles(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	cfg := filepath.Join(dir, "tabelavagas")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	if content != "" {
		if err := os.WriteFile(filepath.Join(cfg, "profiles.toml"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("XDG_CONFIG_HOME", dir)
	return dir
}

func TestLoadCustomProfiles_MissingFile(t *testing.T) {
	writeProfiles(t, "")
	if got := loadCustomProfiles(); got != nil {
		t.Errorf("expected nil for missing file, got %v", got)
	}
}

func TestLoadCustomProfiles_Valid(t *testing.T) {
	writeProfiles(t, `
[profiles.meuperfil]
min_score = 55
keywords = ["python", "ml", "ia"]
remote_bonus = 10
intern_bonus = 15
city_bonus = 8
city = ["belo horizonte", "bh"]
`)
	p := loadCustomProfiles()["meuperfil"]
	if p.MinScore != 55 {
		t.Errorf("MinScore = %d, want 55", p.MinScore)
	}
	if len(p.Preferred) != 3 || p.Preferred[0] != "python" {
		t.Errorf("Preferred = %v, want [python ml ia]", p.Preferred)
	}
	if p.RemoteBonus != 10 || p.InternBonus != 15 || p.CityBonus != 8 {
		t.Errorf("bonus mismatch: remote=%d intern=%d city=%d", p.RemoteBonus, p.InternBonus, p.CityBonus)
	}
	if len(p.City) != 2 || p.City[0] != "belo horizonte" {
		t.Errorf("City = %v", p.City)
	}
}

func TestResolveProfile_Preferences(t *testing.T) {
	writeProfiles(t, `
[profiles.custom]
min_score = 30
keywords = ["elixir"]
`)
	if got := resolveProfile("custom").MinScore; got != 30 {
		t.Errorf("custom MinScore = %d, want 30", got)
	}
	// Built-ins are still available when no custom overrides them.
	if got := resolveProfile("data").MinScore; got != 65 {
		t.Errorf("data MinScore = %d, want 65", got)
	}
	// Unknown names fall back to dev.
	if got := resolveProfile("nao-existe").MinScore; got != 70 {
		t.Errorf("fallback MinScore = %d, want 70", got)
	}
}

func TestDefaultProfile_Env(t *testing.T) {
	t.Setenv("TABELAVAGAS_PROFILE", "")
	if got := defaultProfile(); got != "dev" {
		t.Errorf("defaultProfile() = %q, want dev", got)
	}
	t.Setenv("TABELAVAGAS_PROFILE", "data")
	if got := defaultProfile(); got != "data" {
		t.Errorf("defaultProfile() = %q, want data", got)
	}
}

func TestParseFlags_OnlyNew(t *testing.T) {
	f := parseFlags([]string{"--only-new", "5"})
	if !f.onlyNew {
		t.Error("onlyNew should be true")
	}
	if f.topN != 5 {
		t.Errorf("topN = %d, want 5", f.topN)
	}
}

func TestParseFlags_ProfilePrecedence(t *testing.T) {
	t.Setenv("TABELAVAGAS_PROFILE", "data")
	if got := parseFlags(nil).profile; got != "data" {
		t.Errorf("env profile = %q, want data", got)
	}
	if got := parseFlags([]string{"--profile", "fullstack"}).profile; got != "fullstack" {
		t.Errorf("flag profile = %q, want fullstack", got)
	}
}
