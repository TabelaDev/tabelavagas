package main

import (
	"github.com/ianptkcs/tabelatuiui"
)

// theme mirrors the installed DankMaterialShell's own configured accent
// (falling back to a manually chosen Catppuccin accent when DMS isn't
// installed/configured). TABELAVAGAS_DMS_SETTINGS/TABELAVAGAS_ACCENT env
// vars override the defaults; see tabelatuiui.NewThemeFromEnv.
var theme = tuiui.NewThemeFromEnv("TABELAVAGAS")
