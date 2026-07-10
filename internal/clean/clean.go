package clean

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/huaquanghan/mu/internal/ui"
	"github.com/huaquanghan/mu/internal/utils"
)

// Options controls clean behavior.
type Options struct {
	DryRun  bool
	Debug   bool
	Include []string // opt-in category IDs (e.g. ["browser-cache"])
	AutoYes bool     // skip confirmation prompt
}

// Run executes the clean workflow: scan → display → confirm → execute.
func Run(opts Options) error {
	if _, err := utils.LoadWhitelist(); err != nil {
		return fmt.Errorf("invalid mu configuration: %w", err)
	}
	if err := utils.ValidateCleanupRoot(utils.XDGCacheHome()); err != nil {
		return err
	}
	targets, err := ResolveTargets(opts.Include)
	if err != nil {
		return err
	}

	fmt.Print("\n🔍 Scanning system...\n\n")

	var total int64
	var scanErrors []error
	for _, t := range targets {
		sz, err := t.Scan()
		if err != nil {
			scanErrors = append(scanErrors, fmt.Errorf("scan %s: %w", t.ID, err))
		}
		total += sz
		fmt.Printf("  %-40s %s\n", t.Label, utils.HumanSize(sz))
		if err == nil && t.Preview != nil {
			items, previewErr := t.Preview()
			if previewErr != nil {
				scanErrors = append(scanErrors, fmt.Errorf("preview %s: %w", t.ID, previewErr))
			} else {
				for _, item := range items {
					fmt.Printf("    - %s\n", item)
				}
			}
		}
	}

	fmt.Println("  " + strings.Repeat("-", 50))
	fmt.Printf("  Potential space to free: %s\n", utils.HumanSize(total))
	if len(scanErrors) > 0 {
		return errors.Join(scanErrors...)
	}

	if opts.DryRun {
		fmt.Println("\n⚠️  This is a DRY RUN. No files will be deleted.")
		fmt.Println("Validating the exact cleanup actions without changing data.")
		return execute(targets, opts)
	}

	if !opts.AutoYes && !ui.Confirm("Proceed to clean?") {
		fmt.Println("Aborted.")
		return nil
	}

	return execute(targets, opts)
}

func execute(targets []CleanTarget, opts Options) error {
	if err := utils.InitLogger(); err != nil && opts.Debug {
		fmt.Fprintf(os.Stderr, "warn: could not open log: %v\n", err)
	}
	defer utils.CloseLogger()

	var failed int
	for _, t := range targets {
		fmt.Printf("→ Cleaning %s...\n", t.Label)
		if err := t.Execute(opts.DryRun); err != nil {
			utils.LogOutcome("clean", t.ID, "failure")
			fmt.Fprintf(os.Stderr, "  warn: %v\n", err)
			failed++
			continue
		}
		outcome := "success"
		if opts.DryRun {
			outcome = "dry-run"
		}
		utils.LogOutcome("clean", t.ID, outcome)
		fmt.Println("  ✅ Done")
	}

	if failed > 0 {
		fmt.Fprintf(os.Stderr, "\n⚠️  %d target(s) had errors.\n", failed)
		return fmt.Errorf("%d clean target(s) failed", failed)
	}
	fmt.Println("\n✅  All done.")
	return nil
}
