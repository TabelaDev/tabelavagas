package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCompanies_Greenhouse(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "tabelavagas")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "sources.toml"), []byte(`[greenhouse]
companies = ["gitlab", "vercel"]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)

	got := loadCompanies("greenhouse")
	if len(got) != 2 || got[0] != "gitlab" || got[1] != "vercel" {
		t.Fatalf("loadCompanies = %v, want [gitlab vercel]", got)
	}
	if loadCompanies("lever") != nil {
		t.Error("lever should return nil (not configured)")
	}
}

func TestLoadCompanies_MissingFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if got := loadCompanies("greenhouse"); got != nil {
		t.Errorf("missing config should yield nil, got %v", got)
	}
}

func TestHTMLToText(t *testing.T) {
	html := "<h1>Dev</h1><p>Backend <b>Go</b> engineer</p><ul><li>Kubernetes</li></ul>"
	got := htmlToText(html)
	if !strings.Contains(got, "Backend Go engineer") || !strings.Contains(got, "Kubernetes") {
		t.Errorf("htmlToText = %q", got)
	}
	if htmlToText("") != "" {
		t.Error("empty html should yield empty")
	}
}
