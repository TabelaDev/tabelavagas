package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// sourcesConfig mirrors the per-provider company lists in
// ~/.config/tabelhavagas/sources.toml.
type sourcesConfig struct {
	Greenhouse sourceGroup `toml:"greenhouse"`
}

type sourceGroup struct {
	Companies []string `toml:"companies"`
}

// defaultSourcesPath returns the user config path for sources.toml,
// honoring XDG_CONFIG_HOME when set.
func defaultSourcesPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "tabelhavagas", "sources.toml")
}

// loadCompanies returns the configured company list for an API provider
// (e.g. "greenhouse"). Missing config or provider yields nil.
func loadCompanies(provider string) []string {
	data, err := os.ReadFile(defaultSourcesPath())
	if err != nil {
		return nil
	}
	var cfg sourcesConfig
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		fmt.Fprintf(stderr(), "aviso: %s inválido: %v\n", defaultSourcesPath(), err)
		return nil
	}
	switch provider {
	case "greenhouse":
		return cfg.Greenhouse.Companies
	}
	return nil
}
