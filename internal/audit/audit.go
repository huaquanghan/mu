package audit

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/huaquanghan/mu/internal/utils"
	"github.com/mattn/go-isatty"
)

// Options controls audit behavior.
type Options struct {
	Report  bool
	JSON    bool
	DryRun  bool
	Debug   bool
	Include []string // pre-select opt-in clean IDs
}

// Run executes audit: report/json modes or interactive wizard.
func Run(opts Options) error {
	// Non-TTY without explicit wizard intent → report
	if !opts.Report && !opts.JSON && !isatty.IsTerminal(os.Stdout.Fd()) {
		opts.Report = true
	}

	if opts.JSON || opts.Report {
		return runReport(opts)
	}
	return runWizard(opts)
}

func runReport(opts Options) error {
	var progress ProgressFunc
	if opts.Report && !opts.JSON {
		progress = func(msg string) {
			fmt.Fprintf(os.Stderr, "  %s\n", msg)
		}
		fmt.Fprintln(os.Stderr, "\n🔍 Auditing system…")
	}

	snap := Collect(progress)
	rep := BuildReport(snap, opts.Include)

	if opts.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			return err
		}
	} else {
		printHumanReport(rep)
	}

	code := ExitCodeForReport(rep.Findings)
	if code != 0 {
		os.Exit(code)
	}
	return nil
}

func printHumanReport(rep Report) {
	fmt.Print("\n📋 Audit report\n\n")
	fmt.Printf("  Health score:     %d/100\n", rep.Health)
	fmt.Printf("  Disk / free:      %.0f%%\n", rep.DiskFreePctRoot)
	fmt.Printf("  Reclaimable:      %s\n", utils.HumanSize(rep.ReclaimableBytes))
	fmt.Println()

	if len(rep.Findings) == 0 {
		fmt.Println("  ✅ No issues found. System looks clean.")
		fmt.Println()
		return
	}

	fmt.Println("  Findings:")
	for _, f := range rep.Findings {
		badge := string(f.Severity)
		size := ""
		if f.Bytes > 0 {
			size = "  " + utils.HumanSize(f.Bytes)
		}
		sel := ""
		if f.Selectable && f.DefaultSelected {
			sel = " [recommended]"
		} else if f.OptIn {
			sel = " [opt-in]"
		} else if !f.Selectable {
			sel = " [guide]"
		}
		fmt.Printf("  • %-8s %s%s%s\n", badge, f.Title, size, sel)
		if f.Detail != "" {
			fmt.Printf("           %s\n", f.Detail)
		}
	}

	fmt.Println()
	fmt.Println("  Recommended commands:")
	for _, c := range rep.RecommendedCommands {
		fmt.Printf("    %s\n", c)
	}
	fmt.Println()
	fmt.Println("  Run `mu audit` (interactive) to select and apply fixes.")
	fmt.Println()
}
