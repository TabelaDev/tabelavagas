package main

import (
	"time"

	"github.com/ianptkcs/tabelatuiui"
)

// config is tabelavagas' settings schema, read from
// ~/.config/tabelavagas/config.toml. Every field falls back to defaultConfig
// when the file leaves it out.
//
// profiles.toml and sources.toml stay separate files: those are catalogs
// (what a "dev" profile scores, which companies to scrape), not settings.
// This file only points at the default profile by name.
//
// Environment variables still win over the file, resolved at the point of use
// (see llmBaseURL/llmModel, defaultProfile, defaultDBPath). That keeps the
// one-off override flow working exactly as before, and keeps the secret
// (TABELAVAGAS_LLM_API_KEY) env-only — it never gets a config key.
type config struct {
	DefaultProfile string          `toml:"default_profile"`
	Database       databaseConfig  `toml:"database"`
	LLM            llmConfig       `toml:"llm"`
	Collector      collectorConfig `toml:"collector"`
	Layout         layoutConfig    `toml:"layout"`
	Notify         notifyConfig    `toml:"notify"`
}

type databaseConfig struct {
	// Path accepts a leading "~". Read only at startup — changing it needs a
	// restart, since the store is opened once.
	Path string `toml:"path"`
}

type llmConfig struct {
	BaseURL     string  `toml:"base_url"`
	Model       string  `toml:"model"`
	Temperature float64 `toml:"temperature"`
	MaxTokens   int     `toml:"max_tokens"`
	// Workers is the size of the scoring pool. Read when a scoring run
	// starts, so a reload only affects the next run.
	Workers     int      `toml:"workers"`
	HTTPTimeout duration `toml:"http_timeout"`
}

type collectorConfig struct {
	HTTPTimeout          duration `toml:"http_timeout"`
	GreenhouseDelay      duration `toml:"greenhouse_delay"`
	ProgramathorMaxPages int      `toml:"programathor_max_pages"`
	ProgramathorDelay    duration `toml:"programathor_delay"`
}

type layoutConfig struct {
	SidebarWidth int `toml:"sidebar_width"`
	CardHeight   int `toml:"card_height"`
	DetailMin    int `toml:"detail_min"`
	DetailMax    int `toml:"detail_max"`
}

type notifyConfig struct {
	// Binary is the notifier to shell out to; when it isn't on PATH the app
	// falls back to printing the digest on stdout.
	Binary    string `toml:"binary"`
	TimeoutMS int    `toml:"timeout_ms"`
	AppName   string `toml:"app_name"`
	// Count is how many jobs a notification carries when no explicit N is
	// given on the command line.
	Count int `toml:"count"`
}

// duration wraps time.Duration so TOML can express it as "30s" instead of a
// raw nanosecond count.
type duration struct{ time.Duration }

func (d *duration) UnmarshalText(text []byte) error {
	parsed, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}
	d.Duration = parsed
	return nil
}

func defaultConfig() config {
	return config{
		DefaultProfile: "dev",
		Database:       databaseConfig{Path: "~/.local/state/tabelavagas/vagas.db"},
		LLM: llmConfig{
			BaseURL:     "https://api.deepseek.com/v1",
			Model:       "deepseek-chat",
			Temperature: 0.1,
			MaxTokens:   100,
			Workers:     6,
			HTTPTimeout: duration{30 * time.Second},
		},
		Collector: collectorConfig{
			HTTPTimeout:          duration{30 * time.Second},
			GreenhouseDelay:      duration{500 * time.Millisecond},
			ProgramathorMaxPages: 20,
			ProgramathorDelay:    duration{400 * time.Millisecond},
		},
		Layout: layoutConfig{
			SidebarWidth: 22,
			CardHeight:   4,
			DetailMin:    32,
			DetailMax:    64,
		},
		Notify: notifyConfig{
			Binary:    "dms",
			TimeoutMS: 8000,
			AppName:   "tabelavagas",
			Count:     5,
		},
	}
}

// configPath is resolved lazily, not in a package-level var: an init-time var
// would freeze XDG_CONFIG_HOME before main (or a test) could set it.
func configPath() string { return tuiui.ConfigPath("tabelavagas", "config.toml") }

var cfg *tuiui.Config[config]

// settings is the normalized snapshot the app reads from.
var settings = defaultConfig()

// Env overrides are resolved at the point of use, not folded into `settings`
// at load time: precedence is env > config.toml > default, and reading the
// variable when the value is actually needed keeps it live regardless of when
// it was set. TABELAVAGAS_LLM_API_KEY is deliberately absent — a secret has
// no config key, it stays env-only.
func llmBaseURL() string { return envOr("TABELAVAGAS_LLM_BASEURL", settings.LLM.BaseURL) }
func llmModel() string   { return envOr("TABELAVAGAS_LLM_MODEL", settings.LLM.Model) }

// normalize clamps values the app cannot run on: a zero worker count would
// stall LLM scoring, and a detail panel with min > max has no valid width.
func normalize(c config) config {
	d := defaultConfig()
	if c.LLM.Workers < 1 {
		c.LLM.Workers = d.LLM.Workers
	}
	if c.LLM.MaxTokens < 1 {
		c.LLM.MaxTokens = d.LLM.MaxTokens
	}
	if c.LLM.HTTPTimeout.Duration <= 0 {
		c.LLM.HTTPTimeout = d.LLM.HTTPTimeout
	}
	if c.Collector.HTTPTimeout.Duration <= 0 {
		c.Collector.HTTPTimeout = d.Collector.HTTPTimeout
	}
	if c.Collector.GreenhouseDelay.Duration < 0 {
		c.Collector.GreenhouseDelay = d.Collector.GreenhouseDelay
	}
	if c.Collector.ProgramathorDelay.Duration < 0 {
		c.Collector.ProgramathorDelay = d.Collector.ProgramathorDelay
	}
	if c.Collector.ProgramathorMaxPages < 1 {
		c.Collector.ProgramathorMaxPages = d.Collector.ProgramathorMaxPages
	}
	if c.Layout.SidebarWidth < 10 {
		c.Layout.SidebarWidth = d.Layout.SidebarWidth
	}
	if c.Layout.CardHeight < 1 {
		c.Layout.CardHeight = d.Layout.CardHeight
	}
	if c.Layout.DetailMin < 1 || c.Layout.DetailMax < c.Layout.DetailMin {
		c.Layout.DetailMin, c.Layout.DetailMax = d.Layout.DetailMin, d.Layout.DetailMax
	}
	if c.Notify.Count < 1 {
		c.Notify.Count = d.Notify.Count
	}
	if c.Notify.Binary == "" {
		c.Notify.Binary = d.Notify.Binary
	}
	if c.Notify.TimeoutMS < 1 {
		c.Notify.TimeoutMS = d.Notify.TimeoutMS
	}
	if c.DefaultProfile == "" {
		c.DefaultProfile = d.DefaultProfile
	}
	if c.Database.Path == "" {
		c.Database.Path = d.Database.Path
	}
	return c
}

// apply refreshes the derived state that can't just read settings on demand.
func apply() {
	// httpClient is a package var built at init with the default timeout;
	// rebuild it now that the configured one is known.
	httpClient = newHTTPClient(settings.Collector.HTTPTimeout.Duration)
}

// loadSettings reads config.toml once at startup. A missing file is not an
// error — the app runs on defaults.
func loadSettings() error {
	cfg = tuiui.NewConfig(configPath(), defaultConfig())
	err := cfg.Load()
	settings = normalize(cfg.Get())
	apply()
	return err
}

// reloadSettings re-reads config.toml, reporting whether anything changed. On
// a parse error Config keeps the previous values, so settings stays valid.
// Note that the store is not reopened: [database].path needs a restart.
func reloadSettings() (bool, error) {
	if cfg == nil {
		return true, loadSettings()
	}
	changed, err := cfg.Reload()
	settings = normalize(cfg.Get())
	apply()
	return changed, err
}
