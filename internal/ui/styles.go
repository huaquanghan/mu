package ui

import (
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

// Shared mu palette. Every screen draws from these tokens so styling stays
// consistent — no ad-hoc ANSI codes scattered through views.
const (
	ColorPrimary  = "#0097A7" // mu cyan: titles, highlights, active items
	ColorDanger   = "#EF4444" // failures, errors
	ColorSuccess  = "#22C55E" // completed items
	ColorInactive = "#374151" // inactive button background
	ColorMuted    = "#9CA3AF" // inactive button foreground
)

var (
	// StyleBoldPrimary renders titles and highlights.
	StyleBoldPrimary = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorPrimary))
	// StyleFaint renders secondary text and hints.
	StyleFaint = lipgloss.NewStyle().Faint(true)
	// StyleButtonOn / StyleButtonOff render the YES/NO button pair.
	StyleButtonOn  = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color(ColorPrimary)).Foreground(lipgloss.Color("#FFFFFF")).Padding(0, 2)
	StyleButtonOff = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color(ColorInactive)).Foreground(lipgloss.Color(ColorMuted)).Padding(0, 2)
)

// MarkSuccess prefixes a completed item with a green check.
func MarkSuccess(text string) string {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorSuccess)).Render("✓ " + text)
}

// MarkError prefixes a failed item with a red cross.
func MarkError(text string) string {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorDanger)).Render("✗ " + text)
}

// NewSpinner returns mu's standard spinner.
func NewSpinner() spinner.Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPrimary))
	return sp
}
