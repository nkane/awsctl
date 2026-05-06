package ui

import "github.com/charmbracelet/lipgloss"

// Theme bundles the lipgloss styles used across screens.
// Constructed once and passed to child models so styling stays consistent.
type Theme struct {
	Title      lipgloss.Style
	Subtle     lipgloss.Style
	StatusBar  lipgloss.Style
	StatusKey  lipgloss.Style
	StatusVal  lipgloss.Style
	UnsafeTag  lipgloss.Style
	SafeTag    lipgloss.Style
	ErrorTag   lipgloss.Style
	TabActive  lipgloss.Style
	TabInactiv lipgloss.Style
	Border     lipgloss.Style
}

// NewTheme returns the default theme.
func NewTheme() Theme {
	return Theme{
		Title:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")),
		Subtle:     lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
		StatusBar:  lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("252")).Padding(0, 1),
		StatusKey:  lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		StatusVal:  lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Bold(true),
		UnsafeTag:  lipgloss.NewStyle().Background(lipgloss.Color("196")).Foreground(lipgloss.Color("231")).Padding(0, 1).Bold(true),
		SafeTag:    lipgloss.NewStyle().Background(lipgloss.Color("28")).Foreground(lipgloss.Color("231")).Padding(0, 1),
		ErrorTag:   lipgloss.NewStyle().Foreground(lipgloss.Color("203")),
		TabActive:  lipgloss.NewStyle().Foreground(lipgloss.Color("231")).Background(lipgloss.Color("63")).Padding(0, 2),
		TabInactiv: lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Padding(0, 2),
		Border:     lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("241")),
	}
}
