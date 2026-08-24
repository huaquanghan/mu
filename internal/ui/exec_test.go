package ui

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestExecTerminalDispatchesViaExecMsg locks the contract that ExecTerminal
// routes fn through tea.Exec (its Cmd yields tea.execMsg), which is what
// triggers bubbletea's ReleaseTerminal before running and RestoreTerminal
// after — the mechanism that lets sudo read its password without racing the
// raw-mode readLoop.
func TestExecTerminalDispatchesViaExecMsg(t *testing.T) {
	cmd := ExecTerminal(func() error { return nil }, func(error) tea.Msg { return nil })
	if cmd == nil {
		t.Fatal("ExecTerminal returned nil Cmd")
	}
	if got := reflect.TypeOf(cmd()).Name(); got != "execMsg" {
		t.Fatalf("ExecTerminal cmd = %T (%s), want tea.execMsg", cmd(), got)
	}
}

// execProbeModel runs fn via ExecTerminal on Init and reports back.
type execProbeModel struct {
	cbErr error
}

type execProbeDoneMsg struct{ err error }

func (m execProbeModel) Init() tea.Cmd {
	return ExecTerminal(func() error {
		return errors.New("probe boom")
	}, func(err error) tea.Msg {
		return execProbeDoneMsg{err: err}
	})
}

func (m execProbeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case execProbeDoneMsg:
		m.cbErr = msg.err
		return m, tea.Quit
	}
	return m, nil
}

func (m execProbeModel) View() string { return "" }

// TestExecTerminalRunsFnEndToEnd runs a headless program so the full
// ReleaseTerminal → fn → RestoreTerminal round-trip executes; the callback
// must receive fn's error unchanged.
func TestExecTerminalRunsFnEndToEnd(t *testing.T) {
	p := tea.NewProgram(
		execProbeModel{},
		tea.WithInput(io.NopCloser(strings.NewReader(""))),
		tea.WithOutput(io.Discard),
	)
	final, err := p.Run()
	if err != nil {
		t.Fatalf("program: %v", err)
	}
	m, ok := final.(execProbeModel)
	if !ok {
		t.Fatalf("final = %T, want execProbeModel", final)
	}
	if m.cbErr == nil || m.cbErr.Error() != "probe boom" {
		t.Fatalf("callback err = %v, want probe boom", m.cbErr)
	}
}
