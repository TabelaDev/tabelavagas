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

func TestParseCard_IconFields(t *testing.T) {
	// Real Programathor markup: the Font Awesome icon lives in an <i> child,
	// the span text is the label. Match on the icon class, not the text.
	html := `<a href="/jobs/456-senior-dev">
		<h3>Desenvolvedor(a) Full Stack Sênior</h3>
		<div class="cell-list-content-icon">
			<span><i class="fa fa-briefcase"></i>Almeida Kruger</span>
			<span><i class="fas fa-map-marker-alt"></i>Paraná (Presencial)</span>
			<span><i class="fa fa-building"></i>Grande empresa</span>
			<span><i class="far fa-money-bill-alt"></i>Até R$2.500</span>
			<span><i class="fas fa-tag"></i>Python</span>
			<span><i class="fas fa-tag"></i>React</span>
		</div>
	</a>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	j := (&programathorCollector{}).parseCard(doc.Find("a").First(), "/jobs/456-senior-dev")

	if j.Company != "Almeida Kruger" {
		t.Errorf("Company = %q, want %q", j.Company, "Almeida Kruger")
	}
	if j.Location != "Paraná" {
		t.Errorf("Location = %q, want %q (sem hint presencial)", j.Location, "Paraná")
	}
	if j.Remote {
		t.Error("Remote should be false for a presencial job")
	}
	if len(j.Tags) != 2 || j.Tags[0] != "Python" || j.Tags[1] != "React" {
		t.Errorf("Tags = %v, want [Python React]", j.Tags)
	}
	if strings.Contains(j.Raw, "Python") {
		t.Errorf("Raw must not duplicate tags: %q", j.Raw)
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
	cfg := filepath.Join(dir, "tabelhavagas")
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
	t.Setenv("TABELHAVAGAS_PROFILE", "")
	if got := defaultProfile(); got != "dev" {
		t.Errorf("defaultProfile() = %q, want dev", got)
	}
	t.Setenv("TABELHAVAGAS_PROFILE", "data")
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
	if !f.topNSet {
		t.Error("topNSet should be true when N is explicit")
	}
}

func TestParseFlags_ExplicitTen(t *testing.T) {
	// Regression: `notify 10` must keep 10, not fall back to the default.
	f := parseFlags([]string{"10"})
	if !f.topNSet || f.topN != 10 {
		t.Errorf("topNSet=%v topN=%d, want true/10", f.topNSet, f.topN)
	}
}

func TestNotifyCount(t *testing.T) {
	if got := notifyCount(parseFlags(nil)); got != 5 {
		t.Errorf("notifyCount(default) = %d, want 5", got)
	}
	if got := notifyCount(parseFlags([]string{"10"})); got != 10 {
		t.Errorf("notifyCount(10) = %d, want 10", got)
	}
	if got := notifyCount(parseFlags([]string{"3"})); got != 3 {
		t.Errorf("notifyCount(3) = %d, want 3", got)
	}
}

func TestParseFlags_ProfilePrecedence(t *testing.T) {
	t.Setenv("TABELHAVAGAS_PROFILE", "data")
	if got := parseFlags(nil).profile; got != "data" {
		t.Errorf("env profile = %q, want data", got)
	}
	if got := parseFlags([]string{"--profile", "fullstack"}).profile; got != "fullstack" {
		t.Errorf("flag profile = %q, want fullstack", got)
	}
}
