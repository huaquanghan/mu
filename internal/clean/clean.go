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

type scanResult struct {
	target CleanTarget
	size   int64
	items  []string
}

// Run executes the clean workflow: scan → display → confirm → execute.
// It returns a one-line summary (freed space, reclaimable on dry runs, or
// "Aborted." when the user declines the confirmation prompt).
//
// On a real terminal the flow runs as a single Bubble Tea program
// (scanning → summary → confirm → running → done); on pipes/CI it falls
// back to plain line-by-line output.
func Run(opts Options) (string, error) {
	if _, err := utils.LoadWhitelist(); err != nil {
		return "", fmt.Errorf("invalid mu configuration: %w", err)
	}
	if err := utils.ValidateCleanupRoot(utils.XDGCacheHome()); err != nil {
		return "", err
	}
	targets, err := ResolveTargets(opts.Include)
	if err != nil {
		return "", err
	}

	if interactive() {
		return runFlow(opts, targets)
	}
	return runPlain(opts, targets)
}

// runPlain is the non-interactive fallback: line-by-line output, no
// animation, no prompts (pass --yes to run without a terminal).
func runPlain(opts Options, targets []CleanTarget) (string, error) {
	r := ui.NewRun(os.Stdout)

	var results []scanResult
	var total int64
	var scanErrors []error
	_ = r.Spinner("Scanning system", func() error {
		for _, t := range targets {
			sz, err := t.Scan()
			if err != nil {
				scanErrors = append(scanErrors, fmt.Errorf("scan %s: %w", t.ID, err))
			}
			total += sz
			res := scanResult{target: t, size: sz}
			if err == nil && t.Preview != nil {
				items, previewErr := t.Preview()
				if previewErr != nil {
					scanErrors = append(scanErrors, fmt.Errorf("preview %s: %w", t.ID, previewErr))
				} else {
					res.items = items
				}
			}
			results = append(results, res)
		}
		return nil
	})

	for _, res := range results {
		r.Line("%-40s %s", res.target.Label, utils.HumanSize(res.size))
		for _, item := range res.items {
			r.Line("  - %s", item)
		}
	}
	r.Line("%s", strings.Repeat("-", 50))
	r.Line("Potential space to free: %s", utils.HumanSize(total))
	if len(scanErrors) > 0 {
		return "", errors.Join(scanErrors...)
	}

	if opts.DryRun {
		r.Faint("This is a DRY RUN. No files will be deleted.")
		r.Faint("Validating the exact cleanup actions without changing data.")
		_, err := execute(r, results, opts)
		if err != nil {
			return "", err
		}
		summary := fmt.Sprintf("Dry run — %s reclaimable", utils.HumanSize(total))
		r.Summary(summary)
		return summary, nil
	}

	if !opts.AutoYes {
		r.Faint("Non-interactive: pass --yes to confirm, or --dry-run to preview.")
		r.Faint("Aborted.")
		return "Aborted.", nil
	}

	freed, err := execute(r, results, opts)
	if err != nil {
		return "", err
	}
	r.Summary(fmt.Sprintf("All done — freed %s", utils.HumanSize(freed)))
	return fmt.Sprintf("Freed %s", utils.HumanSize(freed)), nil
}

func execute(r *ui.Run, results []scanResult, opts Options) (int64, error) {
	if err := utils.InitLogger(); err != nil && opts.Debug {
		fmt.Fprintf(os.Stderr, "warn: could not open log: %v\n", err)
	}
	defer utils.CloseLogger()

	var freed int64
	var failed int
	for _, res := range results {
		t := res.target
		if err := r.Spinner("Cleaning "+t.Label, func() error {
			return t.Execute(opts.DryRun)
		}); err != nil {
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
		freed += res.size
	}

	if failed > 0 {
		return freed, fmt.Errorf("%d clean target(s) failed", failed)
	}
	return freed, nil
}
