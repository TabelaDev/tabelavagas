package main

import (
	"github.com/charmbracelet/bubbles/key"
)

// tabelavagas' keybindings, declared once and shared by the key dispatch in
// handleBodyKey/handleSidebarKey (key.Matches), the footer hints
// (tuiui.Footer) and the help modal (tuiui.HelpModal) — the hints can never
// drift out of sync with what Update actually matches.
var (
	keyQuit      = key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "sair"))
	keyHelp      = key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "keybindings"))
	keyRefresh   = key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "recarregar"))
	keyFilter    = key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filtro"))
	keyMoveDown  = key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j", "próxima"))
	keyMoveUp    = key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k", "anterior"))
	keyMoveLeft  = key.NewBinding(key.WithKeys("h", "left"), key.WithHelp("h", "faixa esq"))
	keyMoveRight = key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("l", "faixa dir"))
	keyTop       = key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "topo"))
	keyBottom    = key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "fim"))
	keyPageUp    = key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "página ↑"))
	keyPageDown  = key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "página ↓"))
	keyOpen      = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "abrir URL"))
	keyDetail    = key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "detalhe"))
	keyTiers     = key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "faixas"))
	keySidebar   = key.NewBinding(key.WithKeys("ctrl+e"), key.WithHelp("ctrl+e", "perfil"))
	keyVeto      = key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "vetar"))
	keyShowVeto  = key.NewBinding(key.WithKeys("V"), key.WithHelp("V", "vetadas"))
	keyCollect   = key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "coletar"))
	keyNotify    = key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "notificar"))
	keyLLM       = key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "llm"))
	keyLogs      = key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "logs"))
)

// appKeymap is the full list of bindings the footer hints and the help modal
// render from.
var appKeymap = []key.Binding{
	keyMoveDown, keyMoveUp, keyMoveLeft, keyMoveRight, keyTop, keyBottom,
	keyPageUp, keyPageDown, keyOpen, keyDetail, keyTiers, keySidebar, keyVeto,
	keyShowVeto, keyCollect, keyNotify, keyLLM, keyFilter, keyLogs, keyRefresh,
	keyHelp, keyQuit,
}
