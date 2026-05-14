package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	confirmCyan    = lipgloss.Color("#0097A7")
	confirmActive  = lipgloss.NewStyle().Bold(true).Background(confirmCyan).Foreground(lipgloss.Color("#FFFFFF")).Padding(0, 2)
	confirmInactive = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("#374151")).Foreground(lipgloss.Color("#9CA3AF")).Padding(0, 2)
)

type confirmModel struct {
	prompt string
	cursor int // 0=YES 1=NO
	result bool
}

func (m confirmModel) Init() tea.Cmd { return nil }

func (m confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	msg2, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch msg2.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "left", "h", "right", "l", "tab":
		m.cursor = 1 - m.cursor
	case "enter":
		m.result = m.cursor == 0
		return m, tea.Quit
	}
	return m, nil
}

// RenderButtons returns styled YES and NO strings; cursor 0 = YES active.
func RenderButtons(cursor int) (yes, no string) {
	if cursor == 0 {
		return confirmActive.Render("YES"), confirmInactive.Render("NO")
	}
	return confirmInactive.Render("YES"), confirmActive.Render("NO")
}

func (m confirmModel) View() string {
	yesBtn, noBtn := RenderButtons(m.cursor)
	nav := lipgloss.NewStyle().Padding(0, 2).Render("←/→ navigate  •  Enter: confirm  •  q: quit")
	return "\n\n  " + m.prompt + "\n\n  " + yesBtn + "  " + noBtn + "\n\n\n" + nav + "\n"
}

// Confirm renders an interactive YES/NO button prompt. Returns true if YES was selected.
// Defaults to NO to prevent accidental destructive actions.
func Confirm(prompt string) bool {
	m, err := tea.NewProgram(confirmModel{prompt: prompt, cursor: 1}).Run()
	if err != nil {
		return false
	}
	final, ok := m.(confirmModel)
	return ok && final.result
}
