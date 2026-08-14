package ui

import (
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
)

var (
	runSectionStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorPrimary))
	runFaintStyle   = lipgloss.NewStyle().Faint(true)
	runSummaryStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorPrimary))
)

// Run renders a subcommand run in mu's consistent style: section headers,
// content lines, an animated spinner during long work, and a final summary.
// Non-terminal output degrades to plain text.
type Run struct {
	w io.Writer
}

// NewRun creates a run renderer writing to w (use os.Stdout for CLI runs).
func NewRun(w io.Writer) *Run {
	return &Run{w: w}
}

// Section prints a section header line.
func (r *Run) Section(title string) {
	fmt.Fprintln(r.w, "\n"+runSectionStyle.Render("  "+title))
}

// Line prints an indented content line.
func (r *Run) Line(format string, args ...any) {
	fmt.Fprintln(r.w, "  "+fmt.Sprintf(format, args...))
}

// Faint prints an indented, dimmed note line.
func (r *Run) Faint(format string, args ...any) {
	fmt.Fprintln(r.w, runFaintStyle.Render("  "+fmt.Sprintf(format, args...)))
}

// Summary prints the final bold summary line.
func (r *Run) Summary(text string) {
	fmt.Fprintln(r.w, "\n"+runSummaryStyle.Render("  "+text))
}

// Spinner runs fn while rendering an animated spinner labeled with label,
// ending on a check (✅) or cross (❌) frame depending on fn's error.
// Non-terminal output falls back to a plain "label..." line while fn runs.
func (r *Run) Spinner(label string, fn func() error) error {
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		fmt.Fprintln(r.w, "  "+label+"...")
		return fn()
	}
	m := spinnerRunModel{spinner: NewSpinner(), label: label, fn: fn}
	final, err := tea.NewProgram(m).Run()
	if err != nil {
		// The spinner is cosmetic; run the work directly on failure.
		return fn()
	}
	if fm, ok := final.(spinnerRunModel); ok {
		return fm.err
	}
	return nil
}

type spinnerDoneMsg struct {
	err error
}

type spinnerRunModel struct {
	spinner spinner.Model
	label   string
	fn      func() error
	done    bool
	err     error
}

func (m spinnerRunModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		return spinnerDoneMsg{err: m.fn()}
	})
}

func (m spinnerRunModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case spinnerDoneMsg:
		m.done = true
		m.err = msg.err
		return m, tea.Quit
	}
	return m, nil
}

func (m spinnerRunModel) View() string {
	mark := m.spinner.View()
	if m.done {
		if m.err != nil {
			mark = "❌"
		} else {
			mark = "✅"
		}
	}
	return "\n  " + mark + " " + m.label + "\n"
}
