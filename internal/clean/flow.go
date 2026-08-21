package clean

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/huaquanghan/mu/internal/ui"
	"github.com/huaquanghan/mu/internal/utils"
	"github.com/mattn/go-isatty"
)

// interactive reports whether both stdin and stdout are real terminals.
// When false (pipes, CI, editors), Run falls back to plain line output.
func interactive() bool {
	return isatty.IsTerminal(os.Stdin.Fd()) && isatty.IsTerminal(os.Stdout.Fd())
}

// flowState is the clean workflow's current screen.
type flowState uint8

const (
	stateScanning flowState = iota
	stateSummary
	stateConfirm
	stateRunning
	stateDone
)

type rowStatus uint8

const (
	rowPending rowStatus = iota
	rowRunning
	rowDone
	rowFailed
	rowSkipped
)

type runRow struct {
	label  string
	size   int64
	status rowStatus
	err    error
}

// scanTargetMsg reports one target's scan completion. The view only
// advances when this arrives — it is a real completion, not a timer.
type scanTargetMsg struct {
	idx        int
	size       int64
	items      []string
	err        error
	previewErr error
}

// itemDoneMsg reports one target's execute completion.
type itemDoneMsg struct {
	idx int
	err error
}

type flowModel struct {
	opts    Options
	targets []CleanTarget
	spinner spinner.Model
	state   flowState
	width   int // terminal width from tea.WindowSizeMsg; default 80

	results  []scanResult
	total    int64
	scanErrs []error

	confirmCursor int // 0=YES 1=NO; defaults to NO
	rows          []runRow
	freed         int64
	failed        int
	aborted       bool // ctrl+c mid-run: finish current item, skip the rest
	started       time.Time

	summaryOut string // "Freed X" / "Dry run — X reclaimable" / "Aborted."
	runErr     error
}

func newFlowModel(opts Options, targets []CleanTarget) *flowModel {
	return &flowModel{
		opts:          opts,
		targets:       targets,
		spinner:       ui.NewSpinner(),
		state:         stateScanning,
		width:         80,
		confirmCursor: 1,
		started:       time.Now(),
	}
}

// runFlow renders the clean workflow as one alt-screen Bubble Tea program,
// so every screen fully replaces the previous one (no leftover frames).
func runFlow(opts Options, targets []CleanTarget) (string, error) {
	if err := utils.InitLogger(); err != nil && opts.Debug {
		fmt.Fprintf(os.Stderr, "warn: could not open log: %v\n", err)
	}
	defer utils.CloseLogger()

	m := newFlowModel(opts, targets)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return "", err
	}
	return m.summaryOut, m.runErr
}

func (m *flowModel) Init() tea.Cmd {
	if len(m.targets) == 0 {
		m.state = stateDone
		m.summaryOut = "Nothing to clean."
		return nil
	}
	return tea.Batch(m.spinner.Tick, m.scanCmd(0))
}

// scanCmd scans one target; the next one is only issued after this completes.
func (m *flowModel) scanCmd(i int) tea.Cmd {
	return func() tea.Msg {
		t := m.targets[i]
		sz, err := t.Scan()
		msg := scanTargetMsg{idx: i, size: sz, err: err}
		if err == nil && t.Preview != nil {
			items, previewErr := t.Preview()
			msg.items = items
			msg.previewErr = previewErr
		}
		return msg
	}
}

// runCmd executes one target; the next one is only issued after this completes.
func (m *flowModel) runCmd(i int) tea.Cmd {
	t := m.results[i].target
	return func() tea.Msg {
		return itemDoneMsg{idx: i, err: t.Execute(m.opts.DryRun)}
	}
}

func (m *flowModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case spinner.TickMsg:
		// Keep the spinner animating only while it's visible (scanning and
		// running). On static screens (summary/confirm/done) drop the tick so
		// the view stops re-rendering ~13×/s and time-based text stays fixed.
		if m.state != stateScanning && m.state != stateRunning {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case scanTargetMsg:
		return m, m.onScanTarget(msg)
	case itemDoneMsg:
		return m, m.onItemDone(msg)
	case tea.KeyMsg:
		return m, m.handleKey(msg)
	}
	return m, nil
}

func (m *flowModel) onScanTarget(msg scanTargetMsg) tea.Cmd {
	m.results = append(m.results, scanResult{
		target: m.targets[msg.idx],
		size:   msg.size,
		items:  msg.items,
	})
	if msg.err != nil {
		m.scanErrs = append(m.scanErrs, fmt.Errorf("scan %s: %w", m.targets[msg.idx].ID, msg.err))
	} else {
		m.total += msg.size
		if msg.previewErr != nil {
			m.scanErrs = append(m.scanErrs, fmt.Errorf("preview %s: %w", m.targets[msg.idx].ID, msg.previewErr))
		}
	}
	if msg.idx+1 < len(m.targets) {
		return m.scanCmd(msg.idx + 1)
	}
	m.state = stateSummary
	return nil
}

func (m *flowModel) onItemDone(msg itemDoneMsg) tea.Cmd {
	row := &m.rows[msg.idx]
	if msg.err != nil {
		row.status = rowFailed
		row.err = msg.err
		m.failed++
		utils.LogOutcome("clean", m.results[msg.idx].target.ID, "failure")
	} else {
		row.status = rowDone
		m.freed += row.size
		outcome := "success"
		if m.opts.DryRun {
			outcome = "dry-run"
		}
		utils.LogOutcome("clean", m.results[msg.idx].target.ID, outcome)
	}
	next := msg.idx + 1
	if next < len(m.rows) && !m.aborted {
		m.rows[next].status = rowRunning
		return m.runCmd(next)
	}
	m.finishRun(next)
	return nil
}

func (m *flowModel) finishRun(next int) {
	for i := next; i < len(m.rows); i++ {
		m.rows[i].status = rowSkipped
	}
	m.state = stateDone
	switch {
	case m.aborted:
		m.summaryOut = "Aborted."
	case m.failed > 0:
		m.runErr = fmt.Errorf("%d clean target(s) failed", m.failed)
		if m.opts.DryRun {
			m.summaryOut = fmt.Sprintf("Dry run — %s reclaimable (%d failed)", utils.HumanSize(m.total), m.failed)
		} else {
			m.summaryOut = fmt.Sprintf("Freed %s — %d item(s) failed", utils.HumanSize(m.freed), m.failed)
		}
	case m.opts.DryRun:
		m.summaryOut = fmt.Sprintf("Dry run — %s reclaimable", utils.HumanSize(m.total))
	default:
		m.summaryOut = fmt.Sprintf("Freed %s", utils.HumanSize(m.freed))
	}
}

func (m *flowModel) handleKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "ctrl+c":
		switch m.state {
		case stateRunning:
			m.aborted = true // finish the in-flight item, skip the rest
		case stateDone:
			return tea.Quit
		default:
			return m.quitAbort()
		}
	case "q", "esc":
		switch m.state {
		case stateRunning:
			// ignore: only ctrl+c cancels a run in progress
		case stateDone:
			return tea.Quit
		default:
			return m.quitAbort()
		}
	case "enter", " ":
		switch m.state {
		case stateSummary:
			if len(m.scanErrs) > 0 {
				m.runErr = errors.Join(m.scanErrs...)
				return tea.Quit
			}
			if m.opts.DryRun || m.opts.AutoYes {
				m.startRun()
				return tea.Batch(m.spinner.Tick, m.runCmd(0))
			}
			m.state = stateConfirm
		case stateConfirm:
			if m.confirmCursor == 1 { // NO — declined
				m.summaryOut = "Aborted."
				return tea.Quit
			}
			m.startRun()
			return tea.Batch(m.spinner.Tick, m.runCmd(0))
		case stateDone:
			return tea.Quit
		}
	case "left", "h", "right", "l", "tab":
		if m.state == stateConfirm {
			m.confirmCursor = 1 - m.confirmCursor
		}
	}
	return nil
}

// quitAbort exits with a scan-error report when the scan finished with
// errors, matching the Enter key's behavior on the error summary; otherwise
// it records a plain "Aborted." summary.
func (m *flowModel) quitAbort() tea.Cmd {
	if m.state == stateSummary && len(m.scanErrs) > 0 {
		m.runErr = errors.Join(m.scanErrs...)
	} else {
		m.summaryOut = "Aborted."
	}
	return tea.Quit
}

func (m *flowModel) startRun() {
	m.rows = make([]runRow, len(m.results))
	for i, res := range m.results {
		m.rows[i] = runRow{label: res.target.Label, size: res.size, status: rowPending}
	}
	m.rows[0].status = rowRunning
	m.state = stateRunning
}

func (m *flowModel) runCount() int {
	n := 0
	for i := range m.rows {
		switch m.rows[i].status {
		case rowDone:
			n++
		}
	}
	return n
}

func (m *flowModel) View() string {
	switch m.state {
	case stateScanning:
		return m.viewScanning()
	case stateSummary:
		return m.viewSummary()
	case stateConfirm:
		return m.viewConfirm()
	case stateRunning:
		return m.viewRunning()
	default:
		return m.viewDone()
	}
}

func (m *flowModel) viewScanning() string {
	s := "\n\n  " + m.spinner.View() + " Scanning system"
	done := len(m.results)
	if done > 0 {
		label := ""
		if done < len(m.targets) {
			label = ": " + m.targets[done].Label
		}
		s += fmt.Sprintf("  (%d of %d)%s", done, len(m.targets), label)
	}
	s += "\n\n\n" + ui.StyleFaint.Render("  q or ctrl+c: cancel") + "\n"
	return s
}

func (m *flowModel) viewSummary() string {
	// Column widths come from the data, clamped to the terminal width —
	// nothing hard-coded, so narrow terminals truncate instead of wrapping.
	sizeW := len(utils.HumanSize(m.total))
	for _, res := range m.results {
		if w := len(utils.HumanSize(res.size)); w > sizeW {
			sizeW = w
		}
	}
	labelW := 0
	for _, res := range m.results {
		if w := lipgloss.Width(res.target.Label); w > labelW {
			labelW = w
		}
	}
	if max := m.width - sizeW - 6; labelW > max && max > 4 {
		labelW = max
	}
	if labelW < 1 {
		labelW = 1
	}

	s := "\n\n  " + ui.StyleBoldPrimary.Render("Scan complete") + "\n\n"
	pad := func(display string) string {
		p := labelW - lipgloss.Width(display) + 2
		if p < 0 {
			p = 0
		}
		return strings.Repeat(" ", p)
	}
	s += ui.StyleFaint.Render("  CATEGORY"+pad("CATEGORY")+fmt.Sprintf("%*s", sizeW, "SIZE")) + "\n"
	for _, res := range m.results {
		label := ansi.Truncate(res.target.Label, labelW, "…")
		s += "  " + label + pad(label) + fmt.Sprintf("%*s", sizeW, utils.HumanSize(res.size)) + "\n"
		for _, item := range res.items {
			s += ui.StyleFaint.Render("    - "+m.truncate(item, m.width-8)) + "\n"
		}
	}
	s += "  " + strings.Repeat("─", labelW+sizeW+2) + "\n"
	prefix := ui.StyleBoldPrimary.Render("Potential space to free:")
	s += "  " + prefix + pad(prefix) + fmt.Sprintf("%*s", sizeW, utils.HumanSize(m.total)) + "\n"

	if m.opts.DryRun {
		s += ui.StyleFaint.Render("  DRY RUN — no files will be deleted") + "\n"
	}
	if len(m.scanErrs) > 0 {
		for _, e := range m.scanErrs {
			s += "  " + ui.MarkError(m.truncate(e.Error(), m.width-6)) + "\n"
		}
		s += "\n\n\n" + ui.StyleFaint.Render("  Enter or q: exit") + "\n"
		return s
	}
	s += "\n\n\n" + ui.StyleFaint.Render("  Enter: continue  •  q: quit") + "\n"
	return s
}

func (m *flowModel) viewConfirm() string {
	yesBtn, noBtn := ui.RenderButtons(m.confirmCursor)
	s := "\n\n  " + ui.StyleBoldPrimary.Render("Proceed to clean?") + "\n\n"
	s += ui.StyleFaint.Render(fmt.Sprintf("  %d item(s) — %s", len(m.results), utils.HumanSize(m.total))) + "\n\n"
	s += "  " + yesBtn + "  " + noBtn + "\n"
	s += "\n\n\n" + ui.StyleFaint.Render("  ←/→ navigate  •  Enter: confirm  •  q: quit") + "\n"
	return s
}

func (m *flowModel) viewRunning() string {
	s := "\n\n  " + ui.StyleBoldPrimary.Render("Cleaning")
	started := 0
	for i := range m.rows {
		switch m.rows[i].status {
		case rowRunning, rowDone, rowFailed:
			started++
		}
	}
	s += "   " + ui.StyleFaint.Render(fmt.Sprintf("%d of %d", started, len(m.rows))) + "\n\n"
	for i := range m.rows {
		row := &m.rows[i]
		label := m.truncate(row.label, m.width-14)
		size := utils.HumanSize(row.size)
		var line string
		switch row.status {
		case rowPending:
			line = ui.StyleFaint.Render("  " + label + "  " + size)
		case rowRunning:
			line = "  " + m.spinner.View() + " Cleaning " + label + "  " + size
		case rowDone:
			line = "  " + ui.MarkSuccess("Cleaned "+label+"  "+size)
		case rowFailed:
			line = "  " + ui.MarkError("Failed "+label+" — "+m.truncate(row.err.Error(), m.width-8))
		case rowSkipped:
			line = ui.StyleFaint.Render("  Skipped " + label + " (cancelled)")
		}
		s += line + "\n"
	}
	s += "\n\n\n" + ui.StyleFaint.Render("  ctrl+c: cancel after current item") + "\n"
	return s
}

func (m *flowModel) viewDone() string {
	s := "\n\n  " + ui.StyleBoldPrimary.Render("Done") + "\n\n"
	switch {
	case m.aborted:
		s += ui.StyleFaint.Render(fmt.Sprintf("  Cancelled — %d of %d items cleaned", m.runCount(), len(m.rows))) + "\n"
	case m.failed > 0:
		s += "  " + ui.MarkError(m.summaryOut) + "\n"
	default:
		s += "  " + ui.MarkSuccess(m.summaryOut) + "\n"
	}
	took := time.Since(m.started)
	tookStr := took.Round(time.Second).String()
	if took < time.Second {
		tookStr = "<1s"
	}
	s += ui.StyleFaint.Render("  Took "+tookStr) + "\n"
	for i := range m.rows {
		if m.rows[i].status == rowFailed {
			s += "  " + ui.MarkError(fmt.Sprintf("Failed %s — %s", m.rows[i].label, m.rows[i].err)) + "\n"
		}
	}
	s += "\n\n\n" + ui.StyleFaint.Render("  Enter or q: exit  •  rerun: choose Clean in the menu (or `mu clean`)") + "\n"
	return s
}

// truncate shortens s to width columns on narrow terminals, leaving it
// untouched when the terminal is wide enough.
func (m *flowModel) truncate(s string, width int) string {
	if width >= 1 {
		return ansi.Truncate(s, width, "…")
	}
	return s
}
