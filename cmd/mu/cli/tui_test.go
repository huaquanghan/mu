package cli

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestMenuNumericShortcuts(t *testing.T) {
	for key, want := range map[string]string{
		"1": "audit",
		"2": "clean",
		"3": "uninstall",
		"4": "optimize",
		"5": "status",
		"6": "quit",
	} {
		m, _ := mainMenuModel{}.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
		final, ok := m.(mainMenuModel)
		if !ok {
			t.Fatalf("key %s: unexpected model %T", key, m)
		}
		if final.chosen != want {
			t.Errorf("key %s chose %q, want %q", key, final.chosen, want)
		}
	}
}

func TestMenuBannerClearsOnFirstKeypress(t *testing.T) {
	m, _ := mainMenuModel{banner: "✅ Freed 1.2 GB"}.Update(tea.KeyMsg{Type: tea.KeyDown})
	final, ok := m.(mainMenuModel)
	if !ok {
		t.Fatalf("unexpected model %T", m)
	}
	if final.banner != "" || final.bannerErr {
		t.Errorf("banner not cleared: %q (err=%v)", final.banner, final.bannerErr)
	}
}

func TestMenuPersistsSnapshotWithoutReload(t *testing.T) {
	m := mainMenuModel{snapshot: &healthMsg{cpu: 12, memUsed: 1024, memTotal: 4096, diskFree: 50}}
	if cmd := m.Init(); cmd != nil {
		t.Error("Init should not reload health when a snapshot is present")
	}
}
