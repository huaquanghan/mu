package audit

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/huaquanghan/mu/internal/ui"
	"github.com/huaquanghan/mu/internal/utils"
)

type phase string

const (
	phaseScan     phase = "scan"
	phaseFindings phase = "findings"
	phaseConfirm  phase = "confirm"
	phaseApply    phase = "apply"
	phaseRescore  phase = "rescore"
)

var (
	cyan      = lipgloss.Color("#0097A7")
	titleSt   = lipgloss.NewStyle().Bold(true).Foreground(cyan)
	faintSt   = lipgloss.NewStyle().Faint(true)
	critSt    = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Bold(true)
	warnSt    = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Bold(true)
	infoSt    = lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
	selectedS = lipgloss.NewStyle().Foreground(cyan).Bold(true)
)

type scannedMsg struct {
	snap Snapshot
	rep  Report
}
type appliedMsg struct {
	results []ApplyResult
	before  Report
}
type rescoredMsg struct {
	before, after Report
}

type auditModel struct {
	phase    phase
	opts     Options
	snap     Snapshot
	report   Report
	after    Report
	findings []Finding
	// selection mirrors findings indices that are selectable
	cursor   int
	selected map[int]bool // index into findings
	confirm  int          // 0=YES 1=NO
	applying bool
	results  []ApplyResult
	width    int
	height   int
	done     bool
	status   string
}

func newAuditModel(opts Options) auditModel {
	return auditModel{
		phase:    phaseScan,
		opts:     opts,
		selected: map[int]bool{},
		confirm:  1, // default NO
		width:    80,
		height:   24,
	}
}

func (m auditModel) Init() tea.Cmd {
	return m.startScan()
}

func (m auditModel) startScan() tea.Cmd {
	return func() tea.Msg {
		// progressive messages via separate channel not available in sync scan;
		// collect with progress discarded for TUI (spinner shows "Scanning…")
		snap := Collect(nil)
		rep := BuildReport(snap, m.opts.Include)
		return scannedMsg{snap: snap, rep: rep}
	}
}

func (m auditModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case scannedMsg:
		m.snap = msg.snap
		m.report = msg.rep
		m.findings = msg.rep.Findings
		m.selected = map[int]bool{}
		for i, f := range m.findings {
			if f.Selectable && f.DefaultSelected {
				m.selected[i] = true
			}
		}
		m.cursor = firstSelectable(m.findings, 0)
		m.phase = phaseFindings
		return m, nil

	case appliedMsg:
		m.results = msg.results
		m.applying = false
		m.phase = phaseRescore
		return m, func() tea.Msg {
			snap := Collect(nil)
			after := BuildReport(snap, m.opts.Include)
			return rescoredMsg{before: msg.before, after: after}
		}

	case rescoredMsg:
		m.after = msg.after
		m.report = msg.before
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m auditModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch m.phase {
	case phaseScan:
		if key == "ctrl+c" || key == "q" {
			m.done = true
			return m, tea.Quit
		}

	case phaseFindings:
		switch key {
		case "ctrl+c", "q":
			m.done = true
			return m, tea.Quit
		case "up", "k":
			m.cursor = prevSelectable(m.findings, m.cursor)
		case "down", "j":
			m.cursor = nextSelectable(m.findings, m.cursor)
		case " ":
			if m.cursor >= 0 && m.cursor < len(m.findings) && m.findings[m.cursor].Selectable {
				m.selected[m.cursor] = !m.selected[m.cursor]
			}
		case "a":
			// toggle all safe (non-opt-in selectable)
			allOn := true
			for i, f := range m.findings {
				if f.Selectable && !f.OptIn && !m.selected[i] {
					allOn = false
					break
				}
			}
			for i, f := range m.findings {
				if f.Selectable && !f.OptIn {
					m.selected[i] = !allOn
				}
			}
		case "enter":
			if m.countSelected() == 0 {
				m.status = "Select at least one finding, or q to quit."
				return m, nil
			}
			m.status = ""
			m.phase = phaseConfirm
			m.confirm = 1
		}

	case phaseConfirm:
		switch key {
		case "ctrl+c", "q":
			m.phase = phaseFindings
		case "left", "h", "right", "l", "tab":
			m.confirm = 1 - m.confirm
		case "enter":
			if m.confirm != 0 {
				m.phase = phaseFindings
				return m, nil
			}
			// YES
			m.phase = phaseApply
			m.applying = true
			selected := m.selectedFindings()
			before := m.report
			dry := m.opts.DryRun
			return m, func() tea.Msg {
				if err := utils.InitLogger(); err != nil && m.opts.Debug {
					fmt.Fprintf(os.Stderr, "warn: log: %v\n", err)
				}
				defer utils.CloseLogger()
				// capture apply output via discarding verbose to model — use stdout
				results := Apply(selected, dry, m.opts.Debug, os.Stdout)
				return appliedMsg{results: results, before: before}
			}
		}

	case phaseApply:
		if key == "ctrl+c" {
			m.status = "Active maintenance cannot be interrupted safely; waiting for completion."
			return m, nil
		}

	case phaseRescore:
		if key == "enter" || key == "q" || key == "ctrl+c" {
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m auditModel) countSelected() int {
	n := 0
	for i, on := range m.selected {
		if on && i < len(m.findings) && m.findings[i].Selectable {
			n++
		}
	}
	return n
}

func (m auditModel) selectedBytes() int64 {
	var b int64
	for i, on := range m.selected {
		if on && i < len(m.findings) {
			b += m.findings[i].Bytes
		}
	}
	return b
}

func (m auditModel) selectedFindings() []Finding {
	var out []Finding
	for i, f := range m.findings {
		if m.selected[i] && f.Selectable {
			out = append(out, f)
		}
	}
	return out
}

func (m auditModel) View() string {
	switch m.phase {
	case phaseScan:
		return "\n\n  " + titleSt.Render("mu audit") + "\n\n  🔍 Scanning system…\n\n\n" +
			faintSt.Render("  q to quit") + "\n"
	case phaseFindings:
		return m.viewFindings()
	case phaseConfirm:
		return m.viewConfirm()
	case phaseApply:
		notice := "  Please wait."
		if m.status != "" {
			notice += "\n  " + m.status
		}
		return "\n\n  " + titleSt.Render("Applying fixes…") + "\n\n" + notice + "\n\n\n" +
			faintSt.Render("  active package operations are allowed to finish") + "\n"
	case phaseRescore:
		return m.viewRescore()
	default:
		return ""
	}
}

func (m auditModel) viewFindings() string {
	var b strings.Builder
	b.WriteString("\n\n  " + titleSt.Render("mu audit — findings") + "\n")
	b.WriteString(faintSt.Render(fmt.Sprintf("  Health %d/100  •  Disk / %.0f%% free  •  Reclaimable %s",
		m.report.Health, m.report.DiskFreePctRoot, utils.HumanSize(m.report.ReclaimableBytes))) + "\n\n")

	if len(m.findings) == 0 {
		b.WriteString("  ✅ No issues found.\n\n\n")
		b.WriteString(faintSt.Render("  q to quit") + "\n")
		return b.String()
	}

	for i, f := range m.findings {
		mark := " "
		if f.Selectable {
			if m.selected[i] {
				mark = "✓"
			} else {
				mark = " "
			}
		} else {
			mark = "·"
		}
		line := fmt.Sprintf("%s [%s] %s", mark, f.Severity, f.Title)
		if f.Bytes > 0 {
			line += "  " + utils.HumanSize(f.Bytes)
		}
		if f.OptIn {
			line += " (opt-in)"
		}
		if !f.Selectable {
			line += " (info)"
		}

		style := infoSt
		switch f.Severity {
		case SeverityCritical:
			style = critSt
		case SeverityWarning:
			style = warnSt
		}
		if i == m.cursor {
			b.WriteString("  " + selectedS.Render("▶ "+line) + "\n")
		} else {
			b.WriteString("  " + style.Render("  "+line) + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Selected: %d  •  ~%s\n", m.countSelected(), utils.HumanSize(m.selectedBytes())))
	if m.status != "" {
		b.WriteString("  " + m.status + "\n")
	}
	b.WriteString("\n\n")
	b.WriteString(faintSt.Render("  j/k move  •  space toggle  •  a safe-all  •  enter continue  •  q quit") + "\n")
	return b.String()
}

func (m auditModel) viewConfirm() string {
	var b strings.Builder
	b.WriteString("\n\n  " + titleSt.Render("Confirm apply") + "\n\n")
	if m.opts.DryRun {
		b.WriteString("  Mode: DRY RUN (no changes)\n\n")
	}
	b.WriteString("  Will apply:\n")
	for _, f := range m.selectedFindings() {
		b.WriteString(fmt.Sprintf("    • %s (%s)\n", f.Title, f.Action))
	}
	b.WriteString("\n  Sudo may be required for apt/snap/kernel/journal actions.\n")
	b.WriteString("  User files go to trash when cleaned.\n\n")
	yes, no := ui.RenderButtons(m.confirm)
	b.WriteString("  " + yes + "  " + no + "\n\n\n")
	b.WriteString(faintSt.Render("  ←/→  •  Enter  •  q back") + "\n")
	return b.String()
}

func (m auditModel) viewRescore() string {
	var b strings.Builder
	b.WriteString("\n\n  " + titleSt.Render("Audit complete") + "\n\n")
	failed := CountApplyErrors(m.results)
	if failed > 0 {
		b.WriteString(fmt.Sprintf("  ⚠️  %d action(s) had errors.\n\n", failed))
	} else if m.opts.DryRun {
		b.WriteString("  Dry run finished — nothing modified.\n\n")
	} else {
		b.WriteString("  ✅ Actions finished.\n\n")
	}
	b.WriteString(fmt.Sprintf("  Health:      %d → %d\n", m.report.Health, m.after.Health))
	b.WriteString(fmt.Sprintf("  Disk free:   %.0f%% → %.0f%%\n", m.report.DiskFreePctRoot, m.after.DiskFreePctRoot))
	b.WriteString(fmt.Sprintf("  Reclaimable: %s → %s\n",
		utils.HumanSize(m.report.ReclaimableBytes),
		utils.HumanSize(m.after.ReclaimableBytes)))
	b.WriteString("\n\n")
	b.WriteString(faintSt.Render("  Enter or q to exit") + "\n")
	return b.String()
}

func firstSelectable(fs []Finding, from int) int {
	for i := from; i < len(fs); i++ {
		if fs[i].Selectable {
			return i
		}
	}
	for i := 0; i < len(fs); i++ {
		if fs[i].Selectable {
			return i
		}
	}
	if len(fs) > 0 {
		return 0
	}
	return 0
}

func nextSelectable(fs []Finding, cur int) int {
	for i := cur + 1; i < len(fs); i++ {
		if fs[i].Selectable {
			return i
		}
	}
	return cur
}

func prevSelectable(fs []Finding, cur int) int {
	for i := cur - 1; i >= 0; i-- {
		if fs[i].Selectable {
			return i
		}
	}
	return cur
}

func runWizard(opts Options) error {
	m := newAuditModel(opts)
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return err
	}
	am, ok := final.(auditModel)
	if !ok {
		return nil
	}
	if n := CountApplyErrors(am.results); n > 0 {
		return fmt.Errorf("%d audit action(s) failed", n)
	}
	return nil
}
