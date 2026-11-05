package ui

import (
	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
)

// Only ANSI colors, so the board inherits the terminal's palette.
var (
	accent     = lipgloss.Color("4")
	muted      = lipgloss.Color("8")
	errColor   = lipgloss.Color("1")
	tabActive  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(accent).Padding(0, 1)
	tabIdle    = lipgloss.NewStyle().Foreground(muted).Padding(0, 1)
	brand      = lipgloss.NewStyle().Bold(true).Foreground(accent).Padding(0, 1)
	faint      = lipgloss.NewStyle().Foreground(muted)
	errStyle   = lipgloss.NewStyle().Foreground(errColor)
	panel      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(muted)
	popupStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(accent).Padding(0, 1)
)

func tableStyles() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.Foreground(muted).BorderStyle(lipgloss.NormalBorder()).BorderForeground(muted).BorderBottom(true)
	s.Selected = s.Selected.Foreground(lipgloss.Color("0")).Background(accent).Bold(false)
	return s
}
