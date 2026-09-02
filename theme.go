package main

import (
	"github.com/TAbelhaDev/tabelhatuiui"
)

// theme mirrors the installed DankMaterialShell's own configured accent
// (falling back to a manually chosen Catppuccin accent when DMS isn't
// installed/configured). TABELHAVAGAS_DMS_SETTINGS/TABELHAVAGAS_ACCENT env
// vars override the defaults; see tabelhatuiui.NewThemeFromEnv.
var theme = tuiui.NewThemeFromEnv("TABELHAVAGAS")
