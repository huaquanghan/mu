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

	// Separate by source
	var aptNames, snapNames []string
	var allRemnants []string
	for _, pkg := range pkgs {
		switch pkg.Source {
		case "apt":
			aptNames = append(aptNames, pkg.Name)
		case "snap":
			snapNames = append(snapNames, pkg.Name)
		}
		allRemnants = append(allRemnants, pkg.RemnantsFound...)
	}

	if err := RemoveAPT(aptNames, opts.DryRun); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}
	if err := RemoveSnap(snapNames, opts.DryRun); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}
	if err := RemoveRemnants(allRemnants, opts.DryRun); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}

	fmt.Println("\nUninstall complete.")
	return nil
}
