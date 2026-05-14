package optimize

import (
	"fmt"
	"os"
	"os/exec"
	"slices"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/huaquanghan/mu/internal/ui"
	"github.com/huaquanghan/mu/internal/utils"
)

// Options controls optimize behavior.
type Options struct {
	DryRun bool
	Debug  bool
	Skip   []string
}

type step struct {
	id   string
	desc string
	run  func() error
}

func allSteps() []step {
	return []step{
		{"apt", "apt update && apt autoremove --purge", aptAutoremove},
		{"journal", "journalctl --vacuum-size=500M", journalVacuum},
		{"caches", "update icon/mime/font caches", updateCaches},
	}
}

// Run executes (or previews) the optimization steps.
func Run(opts Options) error {
	wl, err := utils.LoadWhitelist()
	if err != nil && opts.Debug {
		fmt.Fprintf(os.Stderr, "warn: whitelist: %v\n", err)
	}

	// Merge whitelist skip list with CLI --skip flag; CLI takes precedence for additions.
	skip := opts.Skip
	if wl != nil {
		for _, s := range wl.OptimizeSkip.Steps {
			if !slices.Contains(skip, s) {
				skip = append(skip, s)
			}
		}
	}

	steps := allSteps()

	fmt.Print("\n🔧 Optimize plan:\n\n")
	for _, s := range steps {
		if slices.Contains(skip, s.id) {
			fmt.Printf("  [skip]  %s\n", s.desc)
		} else {
			fmt.Printf("  [run]   %s\n", s.desc)
		}
	}

	if opts.DryRun {
		fmt.Println("\n⚠️  Dry run — nothing executed.")
		return nil
	}

	if !ui.Confirm("Proceed to optimize?") {
		fmt.Println("Aborted.")
		return nil
	}

	if err := utils.InitLogger(); err != nil && opts.Debug {
		fmt.Fprintf(os.Stderr, "warn: could not open log: %v\n", err)
	}
	defer utils.CloseLogger()

	var active []step
	for _, s := range steps {
		if !slices.Contains(skip, s.id) {
			active = append(active, s)
		}
	}

	p := tea.NewProgram(newOptimizeModel(active, opts.Debug))
	if _, err := p.Run(); err != nil {
		return err
	}

	fmt.Println("\n✅  Optimization complete.")
	return nil
}

// — Bubbletea model for spinner progress —

type stepDoneMsg struct {
	id  string
	err error
}

type optimizeModel struct {
	spinner spinner.Model
	steps   []step
	current int
	done    bool
	debug   bool
}

func newOptimizeModel(steps []step, debug bool) optimizeModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#0097A7"))
	return optimizeModel{spinner: sp, steps: steps, debug: debug}
}

func (m optimizeModel) Init() tea.Cmd {
	if len(m.steps) == 0 {
		return tea.Quit
	}
	return tea.Batch(m.spinner.Tick, m.runCurrentStep())
}

func (m optimizeModel) runCurrentStep() tea.Cmd {
	s := m.steps[m.current]
	return func() tea.Msg {
		err := s.run()
		return stepDoneMsg{id: s.id, err: err}
	}
}

func (m optimizeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case stepDoneMsg:
		utils.LogOp("optimize", msg.id)
		if msg.err != nil && m.debug {
			fmt.Fprintf(os.Stderr, "\nwarn: %s: %v\n", msg.id, msg.err)
		}
		m.current++
		if m.current >= len(m.steps) {
			m.done = true
			return m, tea.Quit
		}
		return m, m.runCurrentStep()

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
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
			out += fmt.Sprintf("  ✅ %s\n", s.desc)
		case i == m.current:
			out += fmt.Sprintf("  %s %s\n", m.spinner.View(), s.desc)
		default:
			out += fmt.Sprintf("  ⏳ %s\n", s.desc)
		}
	}
	hint := lipgloss.NewStyle().Padding(0, 2).Render("ctrl+c to cancel")
	return "\n\n" + out + "\n\n\n" + hint + "\n"
}

func aptAutoremove() error {
	for _, args := range [][]string{
		{"apt", "update"},
		{"apt", "autoremove", "--purge", "-y"},
	} {
		cmd := exec.Command("sudo", append([]string{"-n"}, args...)...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}

func journalVacuum() error {
	cmd := exec.Command("journalctl", "--vacuum-size=500M")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func updateCaches() error {
	for _, args := range [][]string{
		{"update-mime-database", "/usr/share/mime"},
		{"fc-cache", "-f"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run() // best-effort
	}
	return nil
}
