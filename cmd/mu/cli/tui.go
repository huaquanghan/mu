package cli

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/huaquanghan/mu/internal/status"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7C3AED")).
			Padding(0, 1)

	itemStyle     = lipgloss.NewStyle().Padding(0, 2)
	selectedStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(lipgloss.Color("#7C3AED")).
			Bold(true)
	snapshotStyle = lipgloss.NewStyle().Faint(true).Padding(0, 2)
)

type menuItem struct {
	label   string
	desc    string
	command string
}

var menuItems = []menuItem{
	{"Clean", "Free disk space (cache, apt, snap, journal, kernels)", "clean"},
	{"Uninstall", "Remove apps and all their remnants", "uninstall"},
	{"Optimize", "Run system maintenance tasks", "optimize"},
	{"Status", "Live system dashboard", "status"},
	{"Quit", "Exit mu", "quit"},
}

type healthMsg struct {
	cpu     float64
	memUsed uint64
	memTotal uint64
	diskFree float64
}

type mainMenuModel struct {
	cursor   int
	chosen   string
	snapshot *healthMsg
}

func (m mainMenuModel) Init() tea.Cmd {
	return func() tea.Msg {
		s1, err := status.ReadCPU()
		if err != nil {
			return healthMsg{}
		}
		time.Sleep(time.Second)
		s2, _ := status.ReadCPU()
		cpu := status.CPUPercent(s1, s2)
		mem, _ := status.ReadMemory()
		disks, _ := status.ReadDisk()

		diskFree := 100.0
		for _, d := range disks {
			if d.Mount == "/" && d.TotalBytes > 0 {
				diskFree = float64(d.FreeBytes) / float64(d.TotalBytes) * 100
			}
		}
		return healthMsg{
			cpu:      cpu,
			memUsed:  mem.TotalKB - mem.AvailableKB,
			memTotal: mem.TotalKB,
			diskFree: diskFree,
		}
	}
}

func (m mainMenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case healthMsg:
		m.snapshot = &msg
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(menuItems)-1 {
				m.cursor++
			}
		case "enter", " ":
			m.chosen = menuItems[m.cursor].command
			return m, tea.Quit
		}
	}
	return m, nil
}

const banner = `░█▄▒▄█░█▒█░░
░█▒▀▒█░▀▄█▒░`

func (m mainMenuModel) View() string {
	bannerStr := lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED")).Bold(true).Render(banner)
	s := "  " + strings.ReplaceAll(bannerStr, "\n", "\n  ") + "\n"
	s += titleStyle.Render("  Mole Ubuntu — safe system cleaner") + "\n"

	if m.snapshot == nil {
		s += snapshotStyle.Render("Loading...") + "\n\n"
	} else {
		snap := m.snapshot
		var cpuStr, ramStr, diskStr string
		if snap.memTotal == 0 {
			cpuStr, ramStr, diskStr = "---", "---", "---"
		} else {
			cpuStr = fmt.Sprintf("CPU: %.0f%%", snap.cpu)
			ramStr = fmt.Sprintf("RAM: %s/%s", humanKB(snap.memUsed), humanKB(snap.memTotal))
			diskStr = fmt.Sprintf("Disk /: %.0f%% free", snap.diskFree)
		}
		s += snapshotStyle.Render(fmt.Sprintf("%s  %s  %s", cpuStr, ramStr, diskStr)) + "\n\n"
	}

	for i, item := range menuItems {
		if i == m.cursor {
			s += selectedStyle.Render(fmt.Sprintf("▶ %-14s %s", item.label, item.desc)) + "\n"
		} else {
			s += itemStyle.Render(fmt.Sprintf("  %-14s %s", item.label, item.desc)) + "\n"
		}
	}
	s += "\n" + itemStyle.Render("↑/↓ or j/k to navigate  •  Enter to select  •  q to quit")
	return s
}

func humanKB(kb uint64) string {
	const unit = 1024
	if kb < unit {
		return fmt.Sprintf("%d KB", kb)
	}
	mb := float64(kb) / unit
	if mb < unit {
		return fmt.Sprintf("%.1f MB", mb)
	}
	return fmt.Sprintf("%.1f GB", mb/unit)
}

func runTUI() error {
	m := mainMenuModel{}
	p := tea.NewProgram(m, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return err
	}
	final, ok := result.(mainMenuModel)
	if !ok || final.chosen == "" || final.chosen == "quit" {
		return nil
	}
	// Dispatch to subcommand after TUI exits
	switch final.chosen {
	case "clean":
		return runClean()
	case "uninstall":
		return runUninstall()
	case "optimize":
		return runOptimize()
	case "status":
		return runStatus()
	}
	return nil
}
