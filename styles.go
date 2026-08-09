package main

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/ianptkcs/tabelatuiui"
)

// App-specific styles on top of tabelatuiui's shared chrome (called as
// theme.Header/Footer/Panel/Title/Dim and the semantic helpers directly).
// Colors live in the theme resolved in theme.go.

func mutedStyle() lipgloss.Style   { return theme.Muted() }
func successStyle() lipgloss.Style { return theme.Success() }
func warningStyle() lipgloss.Style { return theme.Warning() }
func errorStyle() lipgloss.Style   { return theme.Error() }
func infoStyle() lipgloss.Style    { return theme.Info() }

// cardTitleStyle is the job title in an unselected card: bold, plain text
// (no accent), so the colored score badge stays the only pop.
func cardTitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(theme.Text)
}

// selCardStyle is the full-width highlight for the focused card: the DMS
// accent as background, base text on top — the same solid-rectangle pattern
// tabelakanban uses for its selected cards.
func selCardStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(theme.Base).
		Background(theme.Primary)
}

// baseTextStyle is plain body text on the panel base background.
func baseTextStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(theme.Text).
		Background(theme.Base)
}

// filterBarStyle is the search line shown in filter mode: a left accent edge
// (Primary border) with no background fill, so nested colored segments render
// cleanly.
func filterBarStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(theme.Text).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(theme.Primary).
		Width(width)
}

// filterPromptStyle is the search glyph ("⌕ ") at the start of the filter bar.
func filterPromptStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(theme.Primary)
}

// filterQueryStyle is the typed query text (with the ▮ cursor).
func filterQueryStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(theme.Text)
}

func padLines(s string, width int) string {
	if width < 0 {
		width = 0
	}
	return tuiui.PadLines(s, width)
}

func wrapText(s string, width int) string {
	return tuiui.WrapText(s, width)
}

func padToHeight(s string, lines int) string {
	if lines < 0 {
		lines = 0
	}
	return tuiui.PadToHeight(s, lines)
}
