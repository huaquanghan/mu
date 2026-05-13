package uninstall

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/huaquanghan/mu/internal/utils"
)

type loadedMsg struct{ pkgs []Package }

type phase string

const (
	phaseLoading phase = "loading"
	phaseList    phase = "list"
	phaseConfirm phase = "confirm"
	phaseDone    phase = "done"
)

// spinnerFrames are the loading animation frames.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type tickMsg struct{}

// uninstallModel is the top-level Bubbletea model for mu uninstall.
type uninstallModel struct {
	phase       phase
	spinnerIdx  int
	items       []pkgItem
	cursor      int
	selected    map[string]bool
	opts        Options
	err         error
	windowWidth int
	windowH     int
	scrollOff   int // top visible index
}

// pkgItem holds a package and its selection state.
type pkgItem struct {
	pkg      Package
	selected bool
}

var (
	purple    = lipgloss.Color("#7C3AED")
	gray      = lipgloss.Color("#6B7280")
	checkMark = lipgloss.NewStyle().Foreground(purple).Render("✓")
)

func newModel(opts Options) uninstallModel {
	return uninstallModel{
		phase:     phaseLoading,
		selected:  map[string]bool{},
		opts:      opts,
		windowH:   24,
		windowWidth: 80,
	}
}

func (m uninstallModel) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		func() tea.Msg {
			pkgs, err := Discover()
			if err != nil {
				return loadedMsg{nil}
			}
			for i := range pkgs {
				pkgs[i].RemnantsFound = FindRemnants(pkgs[i].Name)
				pkgs[i].RemnantsKB = RemnantSize(pkgs[i].RemnantsFound) / 1024
			}
			return loadedMsg{pkgs}
		},
	)
}

func tickCmd() tea.Cmd {
	return func() tea.Msg { return tickMsg{} }
}

func (m uninstallModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.windowH = msg.Height
		m.windowWidth = msg.Width
		return m, nil
	}

	switch m.phase {
	case phaseLoading:
		switch msg := msg.(type) {
		case loadedMsg:
			items := make([]pkgItem, len(msg.pkgs))
			for i, p := range msg.pkgs {
				items[i] = pkgItem{pkg: p}
			}
			m.items = items
			m.phase = phaseList
			return m, nil
		case tickMsg:
			m.spinnerIdx = (m.spinnerIdx + 1) % len(spinnerFrames)
			return m, tickCmd()
		case tea.KeyMsg:
			if msg.String() == "ctrl+c" || msg.String() == "q" {
				return m, tea.Quit
			}
		}

	case phaseList:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "ctrl+c", "q":
				return m, tea.Quit
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
					if m.cursor < m.scrollOff {
						m.scrollOff = m.cursor
					}
				}
			case "down", "j":
				if m.cursor < len(m.items)-1 {
					m.cursor++
					visibleH := m.listHeight()
					if m.cursor >= m.scrollOff+visibleH {
						m.scrollOff = m.cursor - visibleH + 1
					}
				}
			case " ":
				if m.cursor >= 0 && m.cursor < len(m.items) {
					name := m.items[m.cursor].pkg.Name
					m.selected[name] = !m.selected[name]
					m.items[m.cursor].selected = m.selected[name]
				}
			case "enter":
				nActual := 0
				for _, v := range m.selected {
					if v {
						nActual++
					}
				}
				if nActual == 0 {
					return m, nil
				}
				m.phase = phaseConfirm
				return m, nil
			}
		}

	case phaseConfirm:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "ctrl+c", "q", "n", "N":
				return m, tea.Quit
			case "y", "Y":
				m.phase = phaseDone
				return m, tea.Quit
			}
		}
	}

	return m, nil
}

// listHeight returns the number of visible list rows.
func (m uninstallModel) listHeight() int {
	// reserve rows: 3 header + 1 footer
	h := m.windowH - 4
	if h < 5 {
		h = 5
	}
	return h
}

func (m uninstallModel) View() string {
	switch m.phase {
	case phaseLoading:
		frame := lipgloss.NewStyle().Foreground(purple).Render(spinnerFrames[m.spinnerIdx])
		return fmt.Sprintf("\n  %s Loading packages...\n", frame)

	case phaseList:
		nSelected := 0
		for _, v := range m.selected {
			if v {
				nSelected++
			}
		}
		header := lipgloss.NewStyle().Bold(true).Foreground(purple).
			Render(fmt.Sprintf("  mu uninstall  (%d selected)", nSelected))

		var sb strings.Builder
		sb.WriteString(header + "\n")
		sb.WriteString(lipgloss.NewStyle().Faint(true).
			Render("  ↑/↓: navigate  Space: select  Enter: confirm  q: quit") + "\n\n")

		visH := m.listHeight()
		end := m.scrollOff + visH
		if end > len(m.items) {
			end = len(m.items)
		}

		for i := m.scrollOff; i < end; i++ {
			it := m.items[i]
			totalKB := it.pkg.InstalledKB + it.pkg.RemnantsKB
			size := utils.HumanSize(totalKB * 1024)
			sourceStr := lipgloss.NewStyle().Foreground(gray).Render("[" + it.pkg.Source + "]")
			sizeStr := lipgloss.NewStyle().Faint(true).Render(size)

			check := "  "
			if it.selected {
				check = checkMark + " "
			}

			line := fmt.Sprintf("%s%s %s %s  %s", check, it.pkg.Name, sourceStr, sizeStr, lipgloss.NewStyle().Faint(true).Render(it.pkg.Version))

			if i == m.cursor {
				line = lipgloss.NewStyle().Background(lipgloss.Color("#1F2937")).Foreground(lipgloss.Color("#F9FAFB")).Render("  " + strings.TrimLeft(line, " "))
			}
			sb.WriteString("  " + line + "\n")
		}

		if len(m.items) > visH {
			sb.WriteString(lipgloss.NewStyle().Faint(true).
				Render(fmt.Sprintf("  [%d-%d of %d]", m.scrollOff+1, end, len(m.items))) + "\n")
		}

		return sb.String()

	case phaseConfirm:
		var sb strings.Builder
		sb.WriteString(lipgloss.NewStyle().Bold(true).Render("\n  Will remove:\n"))
		for _, it := range m.items {
			if !m.selected[it.pkg.Name] {
				continue
			}
			sb.WriteString(fmt.Sprintf("    • %s (%s)\n", it.pkg.Name, it.pkg.Source))
			for _, r := range it.pkg.RemnantsFound {
				sb.WriteString(fmt.Sprintf("      - %s\n", r))
			}
		}
		sb.WriteString("\n  Press y to confirm, n/q to abort\n")
		return sb.String()

	case phaseDone:
		return "\n  Done.\n"
	}
	return ""
}

// selectedPackages returns packages the user selected.
func (m uninstallModel) selectedPackages() []Package {
	var result []Package
	for _, it := range m.items {
		if m.selected[it.pkg.Name] {
			result = append(result, it.pkg)
		}
	}
	return result
}
