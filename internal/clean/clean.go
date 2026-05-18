package clean

import (
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
	includeSet := make(map[string]bool, len(opts.Include))
	for _, id := range opts.Include {
		includeSet[id] = true
	}

	all := AllTargets()

	// Filter: skip opt-in targets not in Include list
	var targets []CleanTarget
	for _, t := range all {
		if t.OptIn && !includeSet[t.ID] {
			continue
		}
		targets = append(targets, t)
	}

	fmt.Print("\n🔍 Scanning system...\n\n")

	type scanned struct {
		target CleanTarget
		size   int64
	}
	results := make([]scanned, 0, len(targets))
	var total int64
	for _, t := range targets {
		sz := t.Scan()
		results = append(results, scanned{target: t, size: sz})
		total += sz
		fmt.Printf("  %-40s %s\n", t.Label, utils.HumanSize(sz))
	}

	fmt.Println("  " + strings.Repeat("-", 50))
	fmt.Printf("  Potential space to free: %s\n", utils.HumanSize(total))

	if opts.DryRun {
		fmt.Println("\n⚠️  This is a DRY RUN. No files will be deleted.")
		fmt.Println("Run without --dry-run to proceed.")
		return nil
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
			fmt.Fprintf(os.Stderr, "  warn: %v\n", err)
			failed++
			continue
		}
		fmt.Println("  ✅ Done")
	}

	if failed > 0 {
		fmt.Fprintf(os.Stderr, "\n⚠️  %d target(s) had errors.\n", failed)
		return fmt.Errorf("%d clean target(s) failed", failed)
	}
	fmt.Println("\n✅  All done.")
	return nil
}

