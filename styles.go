package main

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/ianptkcs/tabelatuiui"
)

func headerStyle(width int) lipgloss.Style { return theme.Header(width) }
func footerStyle(width int) lipgloss.Style { return theme.Footer(width) }
func panelStyle(focused bool) lipgloss.Style {
	return theme.Panel(focused)
}
func dimStyle() lipgloss.Style     { return theme.Dim() }
func successStyle() lipgloss.Style { return theme.Success() }
func warningStyle() lipgloss.Style { return theme.Warning() }

func padToHeight(s string, lines int) string {
	if lines < 0 {
		lines = 0
	}
	return tuiui.PadToHeight(s, lines)
}
