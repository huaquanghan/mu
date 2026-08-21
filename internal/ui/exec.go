package ui

import (
	"io"

	tea "github.com/charmbracelet/bubbletea"
)

// execFunc adapts a func() error to tea.ExecCommand so interactive work can
// run through tea.Exec. The Set* methods are no-ops: the wrapped work opens
// its own subprocesses with their own stdio (sudo reads the password prompt
// directly from the terminal).
type execFunc struct {
	fn func() error
}

func (e execFunc) Run() error          { return e.fn() }
func (e execFunc) SetStdin(io.Reader)  {}
func (e execFunc) SetStdout(io.Writer) {}
func (e execFunc) SetStderr(io.Writer) {}

// ExecTerminal runs fn via tea.Exec, which releases the terminal before fn
// runs and restores it after. Interactive children (sudo) read their password
// from the terminal; without the release, bubbletea's raw-mode input reader
// races the prompt and swallows keystrokes — the password fails, and worse
// with an IME like Unikey transforming keystrokes. cb receives fn's error (or
// the terminal-restore error).
func ExecTerminal(fn func() error, cb func(error) tea.Msg) tea.Cmd {
	return tea.Exec(execFunc{fn: fn}, cb)
}
