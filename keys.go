package main

import (
	"path/filepath"

	"github.com/charmbracelet/bubbles/key"
	"github.com/TAbelhaDev/tabelhatuiui"
)

// reg is tabelhavagas' single source of truth for keybindings: defaults
// registered below, overrides persisted to ~/.config/tabelhavagas/keybindings.json.
// Resolve() returns the effective binding, shared by dispatch, footer and
// help modal — a user rebind via the settings modal applies to all at once.
var reg = tuiui.NewKeyRegistry(filepath.Join(tuiui.ConfigDir(), "tabelhavagas", "keybindings.json"))

func init() {
	reg.RegisterMany(
		tuiui.Action{ID: "quit", Help: "sair", Keys: []string{"q", "ctrl+c"}},
		tuiui.Action{ID: "help", Help: "keybindings", Keys: []string{"?"}},
		tuiui.Action{ID: "settings", Help: "rebind keys", Keys: []string{","}},
		tuiui.Action{ID: "refresh", Help: "recarregar", Keys: []string{"r"}},
		tuiui.Action{ID: "reload", Help: "recarregar config", Keys: []string{"f5"}},
		tuiui.Action{ID: "filter", Help: "filtro", Keys: []string{"/"}},
		tuiui.Action{ID: "move-down", Help: "próxima", Keys: []string{"j", "down"}, Label: "j"},
		tuiui.Action{ID: "move-up", Help: "anterior", Keys: []string{"k", "up"}, Label: "k"},
		tuiui.Action{ID: "move-left", Help: "faixa esq", Keys: []string{"h", "left"}, Label: "h"},
		tuiui.Action{ID: "move-right", Help: "faixa dir", Keys: []string{"l", "right"}, Label: "l"},
		tuiui.Action{ID: "top", Help: "topo", Keys: []string{"g", "home"}, Label: "g"},
		tuiui.Action{ID: "bottom", Help: "fim", Keys: []string{"G", "end"}, Label: "G"},
		tuiui.Action{ID: "page-up", Help: "página ↑", Keys: []string{"pgup"}},
		tuiui.Action{ID: "page-down", Help: "página ↓", Keys: []string{"pgdown"}},
		tuiui.Action{ID: "open", Help: "abrir URL", Keys: []string{"enter"}},
		tuiui.Action{ID: "detail", Help: "detalhe", Keys: []string{"o"}},
		tuiui.Action{ID: "tiers", Help: "faixas", Keys: []string{"t"}},
		tuiui.Action{ID: "sidebar", Help: "perfil", Keys: []string{"ctrl+e"}},
		tuiui.Action{ID: "veto", Help: "vetar", Keys: []string{"x"}},
		tuiui.Action{ID: "show-veto", Help: "vetadas", Keys: []string{"V"}},
		tuiui.Action{ID: "collect", Help: "coletar", Keys: []string{"c"}},
		tuiui.Action{ID: "notify", Help: "notificar", Keys: []string{"n"}},
		tuiui.Action{ID: "llm", Help: "llm", Keys: []string{"m"}},
		tuiui.Action{ID: "logs", Help: "logs", Keys: []string{"L"}},
	)
}

// resolve is a short alias so Update reads like the old named keys.
func resolve(id string) key.Binding { return reg.Resolve(id) }

// bindingsOf returns the resolved bindings for a list of action IDs — used by
// the help modal's sections so they reflect rebinds live.
func bindingsOf(ids ...string) []key.Binding {
	out := make([]key.Binding, 0, len(ids))
	for _, id := range ids {
		out = append(out, reg.Resolve(id))
	}
	return out
}
