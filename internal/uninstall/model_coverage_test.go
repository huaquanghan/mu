package uninstall

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestUninstallModelSearchConfirmAndRenderFlow(t *testing.T) {
	m := newModel(Options{})
	if view := m.View(); !strings.Contains(view, "Type to search") {
		t.Fatalf("initial view=%q", view)
	}
	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 10})
	m = updated.(uninstallModel)
	if cmd != nil || m.windowWidth != 100 {
		t.Fatal("window size not applied")
	}
	updated, cmd = m.Update(tickMsg{})
	m = updated.(uninstallModel)
	if cmd == nil || m.spinnerIdx != 1 {
		t.Fatal("loading spinner did not advance")
	}
	m.query = "app"
	updated, _ = m.Update(loadedMsg{pkgs: []Package{
		{Name: "app", Source: "apt", Version: "1", InstalledKB: 10},
		{Name: "app", Source: "snap", Version: "2", InstalledKB: 20},
		{Name: "other", Source: "apt"},
	}})
	m = updated.(uninstallModel)
	if view := m.View(); !strings.Contains(view, "[apt]") || !strings.Contains(view, "[snap]") {
		t.Fatalf("search results missing: %q", view)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(uninstallModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(uninstallModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(uninstallModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(uninstallModel)
	if m.phase != phaseConfirm || !strings.Contains(m.View(), "Will remove") {
		t.Fatalf("confirm phase=%s view=%q", m.phase, m.View())
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = updated.(uninstallModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(uninstallModel)
	if m.phase != phaseDone || !strings.Contains(m.View(), "Confirmed") || len(m.selectedPackages()) != 2 {
		t.Fatalf("done phase=%s selected=%v", m.phase, m.selectedPackages())
	}
}

func TestUninstallModelEmptyNoMatchAndNavigationBranches(t *testing.T) {
	m := newModel(Options{})
	if m.Init() == nil {
		t.Fatal("expected init command")
	}
	m.windowH = 20
	if got := m.listHeight(); got != 13 {
		t.Fatalf("list height=%d", got)
	}
	m.windowH = 5
	if got := m.listHeight(); got != 3 {
		t.Fatalf("minimum list height=%d", got)
	}
	m.loaded = true
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(uninstallModel)
	if m.phase != phaseSearch {
		t.Fatal("empty selection should remain in search")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m = updated.(uninstallModel)
	if !strings.Contains(m.View(), "No packages found") {
		t.Fatalf("no-match view=%q", m.View())
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = updated.(uninstallModel)
	if m.query != "" {
		t.Fatal("backspace did not clear query")
	}
	m.phase = phaseConfirm
	m.confirmCursor = 0
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = updated.(uninstallModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(uninstallModel)
	if m.phase != phaseSearch {
		t.Fatal("NO should return to search")
	}
	m.phase = phaseConfirm
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	m = updated.(uninstallModel)
	if m.phase != phaseConfirm || cmd == nil {
		t.Fatal("q should quit from confirm")
	}
}
