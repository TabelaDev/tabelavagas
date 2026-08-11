package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeConfigFixture points XDG_CONFIG_HOME at a temp dir and writes
// config.toml into ~/.config/tabelavagas. An empty body leaves the file out.
func writeConfigFixture(t *testing.T, body string) string {
	t.Helper()
	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", base)
	cfg = nil
	settings = defaultConfig()
	// settings is package state: restore it so a test that changes a value
	// doesn't leak it into whatever runs next.
	t.Cleanup(func() {
		cfg = nil
		settings = defaultConfig()
		apply()
	})

	dir := filepath.Join(base, "tabelavagas")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.toml")
	if body != "" {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func TestLoadSettingsWithoutFileUsesDefaults(t *testing.T) {
	writeConfigFixture(t, "")
	if err := loadSettings(); err != nil {
		t.Fatalf("loadSettings() = %v, want nil", err)
	}
	if settings.Notify.Count != 5 || settings.LLM.Workers != 6 {
		t.Fatalf("settings = %+v, want the defaults", settings)
	}
	if sidebarW() != 22 || cardH() != 4 {
		t.Fatalf("layout = %d/%d, want 22/4", sidebarW(), cardH())
	}
}

func TestLoadSettingsOverridesOnlyWhatFileSets(t *testing.T) {
	writeConfigFixture(t, "[notify]\ncount = 12\n")
	if err := loadSettings(); err != nil {
		t.Fatal(err)
	}
	if settings.Notify.Count != 12 {
		t.Fatalf("Notify.Count = %d, want 12", settings.Notify.Count)
	}
	// Untouched keys in the same section, and other sections, keep defaults.
	if settings.Notify.Binary != "dms" || settings.Notify.TimeoutMS != 8000 {
		t.Fatalf("notify = %+v, want the remaining defaults", settings.Notify)
	}
	if settings.LLM.Model != "deepseek-chat" {
		t.Fatalf("LLM.Model = %q, want the default", settings.LLM.Model)
	}
}

func TestDurationsParseFromTOML(t *testing.T) {
	writeConfigFixture(t, "[collector]\ngreenhouse_delay = \"1s\"\nhttp_timeout = \"5s\"\n")
	if err := loadSettings(); err != nil {
		t.Fatal(err)
	}
	if got := settings.Collector.GreenhouseDelay.Duration; got != time.Second {
		t.Fatalf("GreenhouseDelay = %v, want 1s", got)
	}
	// apply() must rebuild the shared client with the configured timeout.
	if httpClient.Timeout != 5*time.Second {
		t.Fatalf("httpClient.Timeout = %v, want 5s", httpClient.Timeout)
	}
}

// Env still wins over the file, and is read at the point of use so it applies
// however late it is set.
func TestEnvBeatsConfigFile(t *testing.T) {
	writeConfigFixture(t, "default_profile = \"data\"\n[llm]\nmodel = \"do-arquivo\"\n[database]\npath = \"/do/arquivo.db\"\n")
	if err := loadSettings(); err != nil {
		t.Fatal(err)
	}
	if defaultProfile() != "data" || llmModel() != "do-arquivo" || defaultDBPath() != "/do/arquivo.db" {
		t.Fatalf("file values not in effect: %q %q %q", defaultProfile(), llmModel(), defaultDBPath())
	}

	t.Setenv("TABELAVAGAS_PROFILE", "do-env")
	t.Setenv("TABELAVAGAS_LLM_MODEL", "modelo-do-env")
	t.Setenv("TABELAVAGAS_DB", "/do/env.db")
	if got := defaultProfile(); got != "do-env" {
		t.Fatalf("defaultProfile() = %q, want the env value", got)
	}
	if got := llmModel(); got != "modelo-do-env" {
		t.Fatalf("llmModel() = %q, want the env value", got)
	}
	if got := defaultDBPath(); got != "/do/env.db" {
		t.Fatalf("defaultDBPath() = %q, want the env value", got)
	}
}

func TestDBPathExpandsHome(t *testing.T) {
	writeConfigFixture(t, "[database]\npath = \"~/vagas.db\"\n")
	if err := loadSettings(); err != nil {
		t.Fatal(err)
	}
	if got := defaultDBPath(); got == "~/vagas.db" || got[0] == '~' {
		t.Fatalf("defaultDBPath() = %q, want the ~ expanded", got)
	}
}

func TestNormalizeClampsUnusableValues(t *testing.T) {
	tests := []struct {
		name  string
		input config
		check func(t *testing.T, got config)
	}{
		{
			name:  "zero workers would stall LLM scoring",
			input: config{LLM: llmConfig{Workers: 0}},
			check: func(t *testing.T, got config) {
				if got.LLM.Workers != 6 {
					t.Fatalf("Workers = %d, want 6", got.LLM.Workers)
				}
			},
		},
		{
			name:  "detail min above max has no valid width",
			input: config{Layout: layoutConfig{DetailMin: 90, DetailMax: 40}},
			check: func(t *testing.T, got config) {
				if got.Layout.DetailMin != 32 || got.Layout.DetailMax != 64 {
					t.Fatalf("detail = %d..%d, want the defaults", got.Layout.DetailMin, got.Layout.DetailMax)
				}
			},
		},
		{
			name:  "zero notify count would send an empty digest",
			input: config{Notify: notifyConfig{Count: 0}},
			check: func(t *testing.T, got config) {
				if got.Notify.Count != 5 {
					t.Fatalf("Notify.Count = %d, want 5", got.Notify.Count)
				}
			},
		},
		{
			name:  "zero timeouts would mean no timeout at all",
			input: config{},
			check: func(t *testing.T, got config) {
				if got.Collector.HTTPTimeout.Duration != 30*time.Second {
					t.Fatalf("Collector.HTTPTimeout = %v, want 30s", got.Collector.HTTPTimeout.Duration)
				}
				if got.LLM.HTTPTimeout.Duration != 30*time.Second {
					t.Fatalf("LLM.HTTPTimeout = %v, want 30s", got.LLM.HTTPTimeout.Duration)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, normalize(tt.input))
		})
	}
}

func TestReloadSettingsKeepsValuesOnMalformedFile(t *testing.T) {
	path := writeConfigFixture(t, "[notify]\ncount = 7\n")
	if err := loadSettings(); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte("[notify\ncount ="), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := reloadSettings(); err == nil {
		t.Fatal("reloadSettings() on malformed TOML = nil, want a parse error")
	}
	if settings.Notify.Count != 7 {
		t.Fatalf("Notify.Count = %d, want the previous 7", settings.Notify.Count)
	}
}

// notifyCount used to be one of three independent 5s (this one, plus two in
// the TUI's runNotify) — they all read [notify].count now.
func TestNotifyCountFollowsConfig(t *testing.T) {
	writeConfigFixture(t, "[notify]\ncount = 3\n")
	if err := loadSettings(); err != nil {
		t.Fatal(err)
	}
	if got := notifyCount(cmdFlags{}); got != 3 {
		t.Fatalf("notifyCount() = %d, want 3", got)
	}
	// An explicit N on the command line still wins.
	if got := notifyCount(cmdFlags{topN: 11, topNSet: true}); got != 11 {
		t.Fatalf("notifyCount(--top 11) = %d, want 11", got)
	}
}
