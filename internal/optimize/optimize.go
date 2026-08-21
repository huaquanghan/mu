package optimize

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/huaquanghan/mu/internal/clean"
	"github.com/huaquanghan/mu/internal/command"
	"github.com/huaquanghan/mu/internal/ui"
	"github.com/huaquanghan/mu/internal/utils"
	"github.com/mattn/go-isatty"
)

var (
	optimizeRunner = command.Runner(command.ExecRunner{})
	runAutoremove  = clean.RunAutoremove
)

// Options controls optimize behavior.
type Options struct {
	DryRun  bool
	Debug   bool
	Skip    []string
	AutoYes bool
}

type step struct {
	id   string
	desc string
	run  func(io.Writer) error
}

func allSteps() []step {
	return []step{
		{"apt", "apt-get update && apt-get autoremove --purge", aptAutoremove},
		{"journal", "journalctl --vacuum-size=500M", journalVacuum},
		{"caches", "update icon/mime/font caches", updateCaches},
	}
}

// StepIDs returns the IDs of all built-in optimize steps in order.
func StepIDs() []string {
	steps := allSteps()
	ids := make([]string, len(steps))
	for i, s := range steps {
		ids[i] = s.id
	}
	return ids
}

// RunStep runs a single optimize step by id (apt, journal, caches).
// It returns whether the step was skipped by policy and an error for an unknown or failed step.
func RunStep(id string, opts Options, out io.Writer) (skipped bool, err error) {
	if out == nil {
		out = io.Discard
	}

	var target step
	for _, s := range allSteps() {
		if s.id == id {
			target = s
			break
		}
	}
	if target.id == "" {
		return false, fmt.Errorf("unknown optimize step: %s", id)
	}
	skip, err := resolveSkip(opts)
	if err != nil {
		return false, err
	}
	if slices.Contains(skip, id) {
		return true, nil
	}
	return false, target.run(out)
}

// Run executes (or previews) the optimization steps.
// It returns a one-line summary (step counts, or "Aborted." when the user
// declines the confirmation prompt).
func Run(opts Options) (string, error) {
	if _, err := utils.LoadWhitelist(); err != nil {
		return "", fmt.Errorf("invalid mu configuration: %w", err)
	}
	skip, err := resolveSkip(opts)
	if err != nil {
		return "", err
	}
	steps := allSteps()

	r := ui.NewRun(os.Stdout)
	r.Section("Optimize plan")
	for _, s := range steps {
		if slices.Contains(skip, s.id) {
			r.Line("[skip]  %s", s.desc)
		} else {
			r.Line("[run]   %s", s.desc)
		}
	}

	if opts.DryRun {
		r.Faint("Dry run — nothing executed.")
		return "Dry run — nothing executed.", nil
	}

	if !opts.AutoYes && !ui.Confirm("Proceed to optimize?") {
		r.Faint("Aborted.")
		return "Aborted.", nil
	}

	if err := utils.InitLogger(); err != nil && opts.Debug {
		fmt.Fprintf(os.Stderr, "warn: could not open log: %v\n", err)
	}
	defer utils.CloseLogger()

	var programOptions []tea.ProgramOption
	interactive := isatty.IsTerminal(os.Stdin.Fd())
	if !interactive {
		programOptions = append(programOptions, tea.WithInput(nil))
	}
	model := newOptimizeModel(steps, skip, opts.Debug)
	model.interactive = interactive
	p := tea.NewProgram(model, programOptions...)
	final, err := p.Run()
	if err != nil {
		return "", err
	}
	model, ok := final.(optimizeModel)
	if !ok {
		return "", fmt.Errorf("unexpected optimize model result %T", final)
	}
	resultErr := stepErrors(model.results)
	if model.cancelled {
		resultErr = errors.Join(resultErr, fmt.Errorf("optimization stopped after the active step completed"))
	}
	if resultErr != nil {
		return "", resultErr
	}

	var succeeded, failed, skipped int
	for _, res := range model.results {
		switch res.Status {
		case StepSuccess:
			succeeded++
		case StepFailed:
			failed++
		case StepSkipped:
			skipped++
		}
	}
	summary := fmt.Sprintf("Optimization complete — %d succeeded, %d failed, %d skipped", succeeded, failed, skipped)
	if model.cancelled {
		summary += " (stopped early)"
	}
	r.Summary(summary)
	return summary, nil
}

func resolveSkip(opts Options) ([]string, error) {
	wl, err := utils.LoadWhitelist()
	if err != nil {
		return nil, fmt.Errorf("invalid mu configuration: %w", err)
	}

	// Merge whitelist skip list with CLI --skip flag; CLI takes precedence for additions.
	skip := append([]string(nil), opts.Skip...)
	if wl != nil {
		for _, s := range wl.OptimizeSkip.Steps {
			if !slices.Contains(skip, s) {
				skip = append(skip, s)
			}
		}
	}
	valid := StepIDs()
	for _, id := range skip {
		if !slices.Contains(valid, id) {
			return nil, fmt.Errorf("unknown optimize skip ID: %s", id)
		}
	}
	return skip, nil
}

// — Bubbletea model for spinner progress —

type stepDoneMsg struct {
	id     string
	err    error
	output string
	status StepStatus
}

type StepStatus string

const (
	StepSuccess StepStatus = "success"
	StepFailed  StepStatus = "failed"
	StepSkipped StepStatus = "skipped"
)

type StepResult struct {
	ID     string
	Status StepStatus
	Err    error
}

type optimizeModel struct {
	spinner       spinner.Model
	steps         []step
	current       int
	done          bool
	debug         bool
	skip          []string
	interactive   bool
	results       []StepResult
	stopRequested bool
	cancelled     bool
}

func newOptimizeModel(steps []step, skip []string, debug bool) optimizeModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#0097A7"))
	return optimizeModel{spinner: sp, steps: steps, skip: skip, debug: debug}
}

func (m optimizeModel) Init() tea.Cmd {
	if len(m.steps) == 0 {
		return tea.Quit
	}
	return tea.Batch(m.spinner.Tick, m.runCurrentStep())
}

func (m optimizeModel) runCurrentStep() tea.Cmd {
	s := m.steps[m.current]
	if slices.Contains(m.skip, s.id) {
		return func() tea.Msg { return stepDoneMsg{id: s.id, status: StepSkipped} }
	}
	if m.interactive {
		// Wrap the step in ui.ExecTerminal so bubbletea releases the terminal
		// before sudo reads its password — without the release, the raw-mode
		// readLoop swallows keystrokes and the sudo prompt always fails.
		var outBuf strings.Builder
		return ui.ExecTerminal(func() error {
			return s.run(&outBuf)
		}, func(err error) tea.Msg {
			output := strings.TrimRight(outBuf.String(), "\n")
			if len(output) > 4096 {
				output = output[:4096] + "\n... (truncated)"
			}
			status := StepSuccess
			if err != nil {
				status = StepFailed
			}
			return stepDoneMsg{id: s.id, err: err, output: output, status: status}
		})
	}
	return func() tea.Msg {
		var buf strings.Builder
		err := s.run(&buf)
		output := strings.TrimRight(buf.String(), "\n")
		if len(output) > 4096 {
			output = output[:4096] + "\n... (truncated)"
		}
		status := StepSuccess
		if err != nil {
			status = StepFailed
		}
		return stepDoneMsg{id: s.id, err: err, output: output, status: status}
	}
}

func (m optimizeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case stepDoneMsg:
		m.results = append(m.results, StepResult{ID: msg.id, Status: msg.status, Err: msg.err})
		utils.LogOutcome("optimize", msg.id, string(msg.status))
		var cmds []tea.Cmd
		if msg.output != "" {
			cmds = append(cmds, tea.Println(msg.output))
		}
		if msg.err != nil {
			cmds = append(cmds, tea.Println(fmt.Sprintf("failed: %s: %v", msg.id, msg.err)))
		}
		m.current++
		if m.current >= len(m.steps) {
			m.done = true
			cmds = append(cmds, tea.Quit)
			return m, tea.Batch(cmds...)
		}
		if m.stopRequested {
			for _, remaining := range m.steps[m.current:] {
				m.results = append(m.results, StepResult{ID: remaining.id, Status: StepSkipped})
				utils.LogOutcome("optimize", remaining.id, string(StepSkipped))
			}
			m.cancelled = true
			m.done = true
			cmds = append(cmds, tea.Quit)
			return m, tea.Batch(cmds...)
		}
		cmds = append(cmds, m.runCurrentStep())
		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.stopRequested = true
			return m, nil
		}
	}
	return m, nil
}

func (m optimizeModel) View() string {
	if m.done {
		return ""
	}
	var out string
	for i, s := range m.steps {
		switch {
		case i < m.current:
			status := m.results[i].Status
			symbol := "✅"
			if status == StepFailed {
				symbol = "❌"
			} else if status == StepSkipped {
				symbol = "⏭️"
			}
			out += fmt.Sprintf("  %s %s\n", symbol, s.desc)
		case i == m.current:
			out += fmt.Sprintf("  %s %s\n", m.spinner.View(), s.desc)
		default:
			out += fmt.Sprintf("  ⏳ %s\n", s.desc)
		}
	}
	hint := lipgloss.NewStyle().Padding(0, 2).Render("ctrl+c to cancel")
	if m.stopRequested {
		hint = lipgloss.NewStyle().Padding(0, 2).Render("stopping after the active step completes")
	}
	return "\n\n" + out + "\n\n\n" + hint + "\n"
}

func aptAutoremove(out io.Writer) error {
	result, err := optimizeRunner.Run(context.Background(), "sudo", "apt-get", "update")
	_, _ = out.Write(result.Stdout)
	_, _ = out.Write(result.Stderr)
	if err != nil {
		return fmt.Errorf("apt-get update: %w", err)
	}
	return runAutoremove(context.Background(), false)
}

func journalVacuum(out io.Writer) error {
	result, err := optimizeRunner.Run(context.Background(), "sudo", "journalctl", "--vacuum-size=500M")
	_, _ = out.Write(result.Stdout)
	_, _ = out.Write(result.Stderr)
	return err
}

func updateCaches(out io.Writer) error {
	var errs []error
	for _, args := range [][]string{
		{"sudo", "update-mime-database", "/usr/share/mime"},
		{"fc-cache", "-f"},
	} {
		result, err := optimizeRunner.Run(context.Background(), args[0], args[1:]...)
		_, _ = out.Write(result.Stdout)
		_, _ = out.Write(result.Stderr)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", strings.Join(args, " "), err))
		}
	}
	return errors.Join(errs...)
}

func stepErrors(results []StepResult) error {
	var errs []error
	for _, result := range results {
		if result.Status == StepFailed && result.Err != nil {
			errs = append(errs, fmt.Errorf("optimize step %s: %w", result.ID, result.Err))
		}
	}
	return errors.Join(errs...)
}
