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
	phaseSearch  phase = "search"
	phaseConfirm phase = "confirm"
	phaseDone    phase = "done"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type tickMsg struct{}

type uninstallModel struct {
	phase       phase
	spinnerIdx  int
	allItems    []pkgItem // full loaded list
	items       []pkgItem // filtered view
	query       string
	loaded      bool
	cursor      int
	selected    map[string]bool
	opts        Options
	windowWidth int
	windowH     int
	scrollOff   int
}

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
		phase:       phaseSearch,
		selected:    map[string]bool{},
		opts:        opts,
		windowH:     24,
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

func filterItems(all []pkgItem, query string) []pkgItem {
	if query == "" {
		return nil
	}
	q := strings.ToLower(query)
	var result []pkgItem
	for _, it := range all {
		if strings.Contains(strings.ToLower(it.pkg.Name), q) {
			result = append(result, it)
		}
	}
	return result
}

func (m uninstallModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.windowH = msg.Height
		m.windowWidth = msg.Width
		return m, nil

	case loadedMsg:
		items := make([]pkgItem, len(msg.pkgs))
		for i, p := range msg.pkgs {
			items[i] = pkgItem{pkg: p, selected: m.selected[p.Name]}
		}
		m.allItems = items
		m.loaded = true
		m.items = filterItems(m.allItems, m.query)
		m.cursor = 0
		m.scrollOff = 0
		return m, nil

	case tickMsg:
		if !m.loaded {
			m.spinnerIdx = (m.spinnerIdx + 1) % len(spinnerFrames)
			return m, tickCmd()
		}
		return m, nil
	}

	switch m.phase {
	case phaseSearch:
		msg, ok := msg.(tea.KeyMsg)
		if !ok {
			break
		}
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "backspace":
			if len(m.query) > 0 {
				m.query = m.query[:len(m.query)-1]
				m.items = filterItems(m.allItems, m.query)
				m.cursor = 0
				m.scrollOff = 0
			}

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
				visH := m.listHeight()
				if m.cursor >= m.scrollOff+visH {
					m.scrollOff = m.cursor - visH + 1
				}
			}

		case " ":
			if m.cursor >= 0 && m.cursor < len(m.items) {
				name := m.items[m.cursor].pkg.Name
				m.selected[name] = !m.selected[name]
				for i := range m.allItems {
					if m.allItems[i].pkg.Name == name {
						m.allItems[i].selected = m.selected[name]
						break
					}
				}
				m.items = filterItems(m.allItems, m.query)
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

		default:
			if len(msg.Runes) > 0 {
				m.query += string(msg.Runes)
				m.items = filterItems(m.allItems, m.query)
				m.cursor = 0
				m.scrollOff = 0
			}
		}

	case phaseConfirm:
		msg, ok := msg.(tea.KeyMsg)
		if !ok {
			break
		}
		switch msg.String() {
		case "ctrl+c", "q", "n", "N":
			return m, tea.Quit
		case "y", "Y":
			m.phase = phaseDone
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m uninstallModel) listHeight() int {
	h := m.windowH - 7 // header + hint + blank + search + blank + footer + scroll
	if h < 3 {
		h = 3
	}
	return h
}

func (m uninstallModel) View() string {
	switch m.phase {
	case phaseSearch:
		nSelected := 0
		for _, v := range m.selected {
			if v {
				nSelected++
			}
		}

		header := lipgloss.NewStyle().Bold(true).Foreground(purple).
			Render(fmt.Sprintf("  mu uninstall  (%d selected)", nSelected))
		hint := lipgloss.NewStyle().Faint(true).
			Render("  Space: select  Enter: confirm  q: quit")

		spinStr := ""
		if !m.loaded {
			frame := lipgloss.NewStyle().Foreground(purple).Render(spinnerFrames[m.spinnerIdx])
			spinStr = "  " + frame
		}
		searchLine := lipgloss.NewStyle().Foreground(purple).
			Render(fmt.Sprintf("  Search: %s█%s", m.query, spinStr))

		var sb strings.Builder
		sb.WriteString(header + "\n")
		sb.WriteString(hint + "\n\n")
		sb.WriteString(searchLine + "\n\n")

		switch {
		case m.query == "":
			sb.WriteString(lipgloss.NewStyle().Faint(true).
				Render("  Type to search installed packages...") + "\n")
		case len(m.items) == 0:
			sb.WriteString(lipgloss.NewStyle().Faint(true).
				Render("  No packages found.") + "\n")
		default:
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
				if m.selected[it.pkg.Name] {
					check = checkMark + " "
				}
				line := fmt.Sprintf("%s%s %s %s  %s", check, it.pkg.Name, sourceStr, sizeStr,
					lipgloss.NewStyle().Faint(true).Render(it.pkg.Version))
				if i == m.cursor {
					line = lipgloss.NewStyle().
						Background(lipgloss.Color("#1F2937")).
						Foreground(lipgloss.Color("#F9FAFB")).
						Render("  " + strings.TrimLeft(line, " "))
				}
				sb.WriteString("  " + line + "\n")
			}
			if len(m.items) > visH {
				sb.WriteString(lipgloss.NewStyle().Faint(true).
					Render(fmt.Sprintf("  [%d-%d of %d]", m.scrollOff+1, end, len(m.items))) + "\n")
			}
		}

		return sb.String()

	case phaseConfirm:
		var sb strings.Builder
		sb.WriteString(lipgloss.NewStyle().Bold(true).Render("\n  Will remove:\n"))
		for name, sel := range m.selected {
			if !sel {
				continue
			}
			for _, it := range m.allItems {
				if it.pkg.Name != name {
					continue
				}
				sb.WriteString(fmt.Sprintf("    • %s (%s)\n", it.pkg.Name, it.pkg.Source))
				for _, r := range it.pkg.RemnantsFound {
					sb.WriteString(fmt.Sprintf("      - %s\n", r))
				}
				break
			}
		}
		sb.WriteString("\n  Press y to confirm, n/q to abort\n")
		return sb.String()

	case phaseDone:
		return "\n  Done.\n"
	}
	return ""
}

func (m uninstallModel) selectedPackages() []Package {
	var result []Package
	for _, it := range m.allItems {
		if m.selected[it.pkg.Name] {
			result = append(result, it.pkg)
		}
	}
	return result
}
