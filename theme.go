package main

import (
	"path/filepath"

	"github.com/ianptkcs/tabelatuiui"
)

var (
	dmsSettingsPath = tuiui.EnvOr("TABELAVAGAS_DMS_SETTINGS", filepath.Join(tuiui.HomeDir(), ".config", "DankMaterialShell", "settings.json"))
	fallbackAccent  = tuiui.EnvOr("TABELAVAGAS_ACCENT", "mauve")
	theme           = tuiui.ResolveTheme(dmsSettingsPath, fallbackAccent)
)
