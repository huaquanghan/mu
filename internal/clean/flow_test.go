package clean

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	sizeA int64 = 1 << 20
	sizeB int64 = 2 << 20
	sizeC int64 = 4 << 20
)

// fakeTargets builds three deterministic targets; pass an index >= 0 to make
// that target's Scan or Execute fail.
func fakeTargets(failScan, failExec int) []CleanTarget {
	sizes := []int64{sizeA, sizeB, sizeC}
	labels := []string{"Alpha Cache", "Beta Logs", "Gamma Temp"}
	targets := make([]CleanTarget, len(labels))
	for i := range labels {
		i := i
		targets[i] = CleanTarget{
			ID:      string([]byte{'a' + byte(i)}),
			Label:   labels[i],
			Scan:    func() (int64, error) { return sizes[i], nil },
			Execute: func(bool) error { return nil },
		}
		if i == failScan {
			targets[i].Scan = func() (int64, error) { return 0, errors.New("scan boom") }
		}
		if i == failExec {
			targets[i].Execute = func(bool) error { return errors.New("exec boom") }
		}
	}
	return targets
}

func newTestFlow(opts Options, failScan, failExec int) *flowModel {
	return newFlowModel(opts, fakeTargets(failScan, failExec))
}

func update(t *testing.T, m *flowModel, msgs ...tea.Msg) *flowModel {
	t.Helper()
	for _, msg := range msgs {
		mm, _ := m.Update(msg)
		var ok bool
		m, ok = mm.(*flowModel)
		if !ok {
			t.Fatalf("Update returned %T, want *flowModel", mm)
		}
	}
	return m
}

// scanAll drives the scan phase with real sizes, ending at the summary.
func scanAll(t *testing.T, m *flowModel) *flowModel {
	t.Helper()
	sizes := []int64{sizeA, sizeB, sizeC}
	for i := range m.targets {
		m = update(t, m, scanTargetMsg{idx: i, size: sizes[i]})
	}
	if m.state != stateSummary {
		t.Fatalf("after scan: state = %v, want stateSummary", m.state)
	}
	return m
}

// confirmYes walks summary → confirm → YES.
func confirmYes(t *testing.T, m *flowModel) *flowModel {
	t.Helper()
	return update(t, m,
		tea.KeyMsg{Type: tea.KeyEnter}, // summary → confirm
		tea.KeyMsg{Type: tea.KeyLeft},  // default is NO; move to YES
		tea.KeyMsg{Type: tea.KeyEnter}, // confirm
	)
}

func TestFlowHappyPath(t *testing.T) {
	m := confirmYes(t, scanAll(t, newTestFlow(Options{}, -1, -1)))
	if m.state != stateRunning {
		t.Fatalf("after YES: state = %v, want stateRunning", m.state)
	}
	if m.rows[0].status != rowRunning {
		t.Fatalf("row 0 = %v, want rowRunning", m.rows[0].status)
	}
	if !strings.Contains(m.View(), "Cleaning Alpha Cache") {
		t.Fatalf("running view missing progressive verb: %q", m.View())
	}

	m = update(t, m, itemDoneMsg{idx: 0})
	if m.rows[0].status != rowDone || m.rows[1].status != rowRunning {
		t.Fatalf("after item 0: rows = %v, %v; want rowDone, rowRunning", m.rows[0].status, m.rows[1].status)
	}
	if !strings.Contains(m.View(), "✓ Cleaned Alpha Cache") {
		t.Fatalf("running view missing past-tense line: %q", m.View())
	}

	m = update(t, m, itemDoneMsg{idx: 1}, itemDoneMsg{idx: 2})
	if m.state != stateDone {
		t.Fatalf("state = %v, want stateDone", m.state)
	}
	if m.summaryOut != "Freed 7.0 MB" {
		t.Fatalf("summaryOut = %q, want %q", m.summaryOut, "Freed 7.0 MB")
	}
	if !strings.Contains(m.View(), "✓ Freed 7.0 MB") {
		t.Fatalf("done view missing freed line: %q", m.View())
	}
}

func TestFlowDecline(t *testing.T) {
	m := scanAll(t, newTestFlow(Options{}, -1, -1))
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // summary → confirm
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // confirm with default NO
	if m.summaryOut != "Aborted." {
		t.Fatalf("summaryOut = %q, want %q", m.summaryOut, "Aborted.")
	}
}

func TestFlowDryRunSkipsConfirm(t *testing.T) {
	m := scanAll(t, newTestFlow(Options{DryRun: true}, -1, -1))
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // → running directly
	if m.state != stateRunning {
		t.Fatalf("state = %v, want stateRunning (dry-run skips confirm)", m.state)
	}
	m = update(t, m, itemDoneMsg{idx: 0}, itemDoneMsg{idx: 1}, itemDoneMsg{idx: 2})
	if m.summaryOut != "Dry run — 7.0 MB reclaimable" {
		t.Fatalf("summaryOut = %q, want %q", m.summaryOut, "Dry run — 7.0 MB reclaimable")
	}
}

func TestFlowAutoYesSkipsConfirm(t *testing.T) {
	m := scanAll(t, newTestFlow(Options{AutoYes: true}, -1, -1))
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // → running directly
	if m.state != stateRunning {
		t.Fatalf("state = %v, want stateRunning (--yes skips confirm)", m.state)
	}
	m = update(t, m, itemDoneMsg{idx: 0}, itemDoneMsg{idx: 1}, itemDoneMsg{idx: 2})
	if m.summaryOut != "Freed 7.0 MB" {
		t.Fatalf("summaryOut = %q, want %q", m.summaryOut, "Freed 7.0 MB")
	}
}

func TestFlowCtrlCCancelsAfterCurrent(t *testing.T) {
	m := confirmYes(t, scanAll(t, newTestFlow(Options{}, -1, -1)))
	m = update(t, m, itemDoneMsg{idx: 0})
	m = update(t, m, tea.KeyMsg{Type: tea.KeyCtrlC}) // while item 1 is running
	if !m.aborted {
		t.Fatal("expected aborted after ctrl+c")
	}
	m = update(t, m, itemDoneMsg{idx: 1}) // the in-flight item finishes
	if m.state != stateDone {
		t.Fatalf("state = %v, want stateDone", m.state)
	}
	if m.rows[2].status != rowSkipped {
		t.Fatalf("row 2 = %v, want rowSkipped", m.rows[2].status)
	}
	if m.summaryOut != "Aborted." {
		t.Fatalf("summaryOut = %q, want %q", m.summaryOut, "Aborted.")
	}
	if !strings.Contains(m.View(), "Cancelled") {
		t.Fatalf("done view missing cancelled note: %q", m.View())
	}
}

func TestFlowFailedItem(t *testing.T) {
	m := confirmYes(t, scanAll(t, newTestFlow(Options{}, -1, 1)))
	m = update(t, m, itemDoneMsg{idx: 0})
	m = update(t, m, itemDoneMsg{idx: 1, err: errors.New("exec boom")})
	if m.rows[1].status != rowFailed {
		t.Fatalf("row 1 = %v, want rowFailed", m.rows[1].status)
	}
	m = update(t, m, itemDoneMsg{idx: 2})
	if m.state != stateDone {
		t.Fatalf("state = %v, want stateDone", m.state)
	}
	if m.runErr == nil || !strings.Contains(m.runErr.Error(), "1 clean target(s) failed") {
		t.Fatalf("runErr = %v, want failure count", m.runErr)
	}
	if !strings.Contains(m.View(), "Failed Beta Logs") {
		t.Fatalf("done view missing failure: %q", m.View())
	}
}

func TestFlowScanErrorAborts(t *testing.T) {
	m := newTestFlow(Options{}, 0, -1)
	m = update(t, m,
		scanTargetMsg{idx: 0, err: errors.New("scan boom")},
		scanTargetMsg{idx: 1, size: sizeB},
		scanTargetMsg{idx: 2, size: sizeC},
	)
	if m.state != stateSummary {
		t.Fatalf("state = %v, want stateSummary", m.state)
	}
	if len(m.scanErrs) != 1 {
		t.Fatalf("scanErrs = %d, want 1", len(m.scanErrs))
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.runErr == nil {
		t.Fatal("expected runErr after scan failure")
	}
}

func TestFlowNarrowWidth(t *testing.T) {
	m := scanAll(t, newTestFlow(Options{}, -1, -1))
	m = update(t, m, tea.WindowSizeMsg{Width: 20})
	if !strings.Contains(m.View(), "Potential space to free:") {
		t.Fatalf("narrow summary missing total: %q", m.View())
	}
	m = confirmYes(t, m)
	m.View() // running view must not panic at width 20
	m = update(t, m, itemDoneMsg{idx: 0}, itemDoneMsg{idx: 1}, itemDoneMsg{idx: 2})
	m.View() // done view must not panic at width 20
}

func TestFlowInitEmptyTargets(t *testing.T) {
	m := newTestFlow(Options{}, -1, -1)
	m.targets = nil
	if cmd := m.Init(); cmd != nil {
		t.Fatal("empty targets must not start a scan")
	}
	if m.state != stateDone {
		t.Fatalf("state = %v, want stateDone", m.state)
	}
	if m.summaryOut != "Nothing to clean." {
		t.Fatalf("summaryOut = %q, want %q", m.summaryOut, "Nothing to clean.")
	}
}

func TestFlowInitStartsScan(t *testing.T) {
	m := newTestFlow(Options{}, -1, -1)
	if cmd := m.Init(); cmd == nil {
		t.Fatal("expected the spinner+scan batch cmd for non-empty targets")
	}
}

func TestScanCmdRunsTarget(t *testing.T) {
	m := newTestFlow(Options{}, -1, -1)
	msg := m.scanCmd(1)()
	sm, ok := msg.(scanTargetMsg)
	if !ok {
		t.Fatalf("scanCmd = %T, want scanTargetMsg", msg)
	}
	if sm.idx != 1 || sm.size != sizeB || sm.err != nil {
		t.Fatalf("scanTargetMsg = %+v, want idx 1 size %d no error", sm, sizeB)
	}
}

func TestScanCmdCollectsPreview(t *testing.T) {
	target := CleanTarget{
		ID:    "x",
		Label: "X Cache",
		Scan:  func() (int64, error) { return sizeA, nil },
		Preview: func() ([]string, error) {
			return []string{"/tmp/a"}, errors.New("preview boom")
		},
		Execute: func(bool) error { return nil },
	}
	m := newFlowModel(Options{}, []CleanTarget{target})
	msg := m.scanCmd(0)()
	sm, ok := msg.(scanTargetMsg)
	if !ok {
		t.Fatalf("scanCmd = %T, want scanTargetMsg", msg)
	}
	if len(sm.items) != 1 || sm.items[0] != "/tmp/a" {
		t.Fatalf("items = %v, want [/tmp/a]", sm.items)
	}
	if sm.previewErr == nil || sm.previewErr.Error() != "preview boom" {
		t.Fatalf("previewErr = %v, want preview boom", sm.previewErr)
	}
}

func TestQuitAbortReportsScanErrors(t *testing.T) {
	m := newTestFlow(Options{}, 0, -1)
	m = update(t, m,
		scanTargetMsg{idx: 0, err: errors.New("scan boom")},
		scanTargetMsg{idx: 1, size: sizeB},
		scanTargetMsg{idx: 2, size: sizeC},
	)
	if cmd := m.quitAbort(); cmd == nil {
		t.Fatal("quitAbort must return tea.Quit")
	}
	if m.runErr == nil || !strings.Contains(m.runErr.Error(), "scan boom") {
		t.Fatalf("runErr = %v, want joined scan error", m.runErr)
	}
}

func TestQuitAbortPlain(t *testing.T) {
	m := newTestFlow(Options{}, -1, -1)
	if cmd := m.quitAbort(); cmd == nil {
		t.Fatal("quitAbort must return tea.Quit")
	}
	if m.summaryOut != "Aborted." {
		t.Fatalf("summaryOut = %q, want %q", m.summaryOut, "Aborted.")
	}
}

func TestViewScanningProgress(t *testing.T) {
	m := newTestFlow(Options{}, -1, -1)
	if v := m.View(); !strings.Contains(v, "Scanning system") {
		t.Fatalf("initial scanning view missing title: %q", v)
	}
	m = update(t, m, scanTargetMsg{idx: 0, size: sizeA})
	if v := m.View(); !strings.Contains(v, "(1 of 3): Beta Logs") {
		t.Fatalf("scanning view missing progress + next label: %q", v)
	}
}

func TestViewConfirm(t *testing.T) {
	m := scanAll(t, newTestFlow(Options{}, -1, -1))
	m = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.state != stateConfirm {
		t.Fatalf("state = %v, want stateConfirm", m.state)
	}
	v := m.View()
	if !strings.Contains(v, "Proceed to clean?") || !strings.Contains(v, "3 item(s)") {
		t.Fatalf("confirm view missing prompt or count: %q", v)
	}
}

func TestTruncateUnclamped(t *testing.T) {
	m := newTestFlow(Options{}, -1, -1)
	if got := m.truncate("abcdef", 0); got != "abcdef" {
		t.Fatalf("truncate with width 0 = %q, want unchanged", got)
	}
}

func TestUpdateSpinnerTickAndUnknownMsgs(t *testing.T) {
	m := newTestFlow(Options{}, -1, -1) // scanning: spinner visible
	update(t, m, spinner.TickMsg{})     // must keep animating without panic
	update(t, m, struct{}{})            // unknown messages are ignored
	m = scanAll(t, m)
	update(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // → confirm (static screen)
	update(t, m, spinner.TickMsg{})              // tick dropped on static screens
}

// TestRunCmdReleasesTerminalForSudo is the regression guard for the Unikey
// sudo-password bug: a RequiresSudo target in a real (non-dry-run) run must be
// dispatched via tea.Exec (its Cmd yields tea.execMsg), which releases the
// terminal so sudo's password prompt doesn't race bubbletea's raw-mode
// readLoop. A plain goroutine returning itemDoneMsg would mean the readLoop
// still swallows the password. Non-sudo and dry-run targets keep the plain
// path.
func TestRunCmdReleasesTerminalForSudo(t *testing.T) {
	sudoTarget := CleanTarget{
		ID:           "apt",
		Label:        "APT Cache",
		RequiresSudo: true,
		Execute:      func(bool) error { return nil },
	}
	plainTarget := CleanTarget{
		ID:      "user-cache",
		Label:   "User cache",
		Execute: func(bool) error { return nil },
	}
	m := newFlowModel(Options{}, []CleanTarget{sudoTarget, plainTarget})
	m.results = []scanResult{
		{target: sudoTarget, size: sizeA},
		{target: plainTarget, size: sizeB},
	}

	if msg := m.runCmd(0)(); reflect.TypeOf(msg).Name() != "execMsg" {
		t.Fatalf("sudo target runCmd = %T (%s), want tea.execMsg (terminal released)", msg, reflect.TypeOf(msg).Name())
	}
	msg := m.runCmd(1)()
	if _, ok := msg.(itemDoneMsg); !ok {
		t.Fatalf("non-sudo target runCmd = %T, want itemDoneMsg", msg)
	}

	// Dry-run never executes sudo, so it must not take the tea.Exec path.
	dry := newFlowModel(Options{DryRun: true}, []CleanTarget{sudoTarget})
	dry.results = []scanResult{{target: sudoTarget, size: sizeA}}
	msg = dry.runCmd(0)()
	if _, ok := msg.(itemDoneMsg); !ok {
		t.Fatalf("dry-run sudo target runCmd = %T, want itemDoneMsg", msg)
	}
}
