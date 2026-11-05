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
	keyStyle   = lipgloss.NewStyle().Bold(true).Foreground(accent)
	groupStyle = lipgloss.NewStyle().Bold(true).Foreground(accent)
	errStyle   = lipgloss.NewStyle().Foreground(errColor)
	panel      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(muted)
	popupStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(accent).Padding(0, 1)
)

func tableStyles() table.Styles {
	// No cell padding: setRows pads the cells itself, so a group row can
	// run across the whole width without gaps between columns.
	s := table.DefaultStyles()
	s.Cell = s.Cell.Padding(0)
	s.Header = s.Header.Padding(0)
	s.Header = s.Header.Foreground(muted).BorderStyle(lipgloss.NormalBorder()).BorderForeground(muted).BorderBottom(true)
	s.Selected = s.Selected.Foreground(lipgloss.Color("0")).Background(accent).Bold(false)
	return s
}
