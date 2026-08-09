package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/BurntSushi/toml"
)

// profilesConfig mirrors the on-disk format of ~/.config/tabelavagas/profiles.toml.
type profilesConfig struct {
	Profiles map[string]tomlProfile `toml:"profiles"`
}

// tomlProfile is a custom profile as written in the TOML file.
type tomlProfile struct {
	MinScore    int      `toml:"min_score"`
	Keywords    []string `toml:"keywords"`
	RemoteBonus int      `toml:"remote_bonus"`
	InternBonus int      `toml:"intern_bonus"`
	CityBonus   int      `toml:"city_bonus"`
	City        []string `toml:"city"`
}

func (p tomlProfile) toScoreOptions() scoreOptions {
	return scoreOptions{
		MinScore:    p.MinScore,
		Preferred:   p.Keywords,
		RemoteBonus: p.RemoteBonus,
		InternBonus: p.InternBonus,
		CityBonus:   p.CityBonus,
		City:        p.City,
	}
}

// defaultProfilesPath returns the user config path for profiles.toml,
// honoring XDG_CONFIG_HOME when set.
func defaultProfilesPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "tabelavagas", "profiles.toml")
}

// loadCustomProfiles reads the user's profiles.toml and returns the custom
// profiles. A missing file yields an empty map; a malformed file is a warning
// (best-effort config, never fatal).
func loadCustomProfiles() map[string]scoreOptions {
	path := defaultProfilesPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cfg profilesConfig
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "aviso: %s inválido: %v\n", path, err)
		return nil
	}
	out := make(map[string]scoreOptions, len(cfg.Profiles))
	for name, p := range cfg.Profiles {
		out[name] = p.toScoreOptions()
	}
	return out
}

// resolveProfile returns the scoring profile for name, preferring custom
// profiles from profiles.toml over built-ins. Unknown names fall back to "dev".
func resolveProfile(name string) scoreOptions {
	custom := loadCustomProfiles()
	if p, ok := custom[name]; ok {
		return p
	}
	if p, ok := builtinProfiles[name]; ok {
		return p
	}
	return builtinProfiles["dev"]
}

// defaultProfile returns the effective profile name: TABELAVAGAS_PROFILE env
// if set, otherwise "dev".
func defaultProfile() string {
	if p := os.Getenv("TABELAVAGAS_PROFILE"); p != "" {
		return p
	}
	return "dev"
}

// profileNames lists every available profile (built-in + custom), sorted.
func profileNames() []string {
	names := map[string]bool{}
	for n := range builtinProfiles {
		names[n] = true
	}
	for n := range loadCustomProfiles() {
		names[n] = true
	}
	out := make([]string, 0, len(names))
	for n := range names {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
