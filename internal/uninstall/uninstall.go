package uninstall

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/huaquanghan/mu/internal/utils"
)

// Options controls uninstall behavior.
type Options struct {
	DryRun bool
	Debug  bool
}

// Run launches the interactive uninstall TUI.
func Run(opts Options) error {
	if _, err := utils.LoadWhitelist(); err != nil {
		return fmt.Errorf("invalid mu configuration: %w", err)
	}
	if err := utils.InitLogger(); err != nil && opts.Debug {
		fmt.Fprintf(os.Stderr, "warn: could not open log: %v\n", err)
	}
	defer utils.CloseLogger()

	m := newModel(opts)
	p := tea.NewProgram(m, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return err
	}

	final, ok := result.(uninstallModel)
	if !ok || final.phase != phaseDone {
		return nil // user quit or aborted
	}

	pkgs := final.selectedPackages()
	if len(pkgs) == 0 {
		fmt.Println("Nothing to remove.")
		return nil
	}

	installed := make([]Package, 0, len(final.allItems))
	for _, item := range final.allItems {
		installed = append(installed, item.pkg)
	}
	results := RemoveSelected(pkgs, installed, opts.DryRun)
	if err := RemovalErrors(results); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return err
	}

	fmt.Println("\nUninstall complete.")
	return nil
}
