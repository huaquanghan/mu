package audit

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/huaquanghan/mu/internal/clean"
	"github.com/huaquanghan/mu/internal/utils"
	"github.com/mattn/go-isatty"
)

// ExitError preserves report exit codes without terminating library callers.
type ExitError struct{ Code int }

func (e *ExitError) Error() string { return fmt.Sprintf("audit findings require exit code %d", e.Code) }
func (e *ExitError) ExitCode() int { return e.Code }

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
	if err := ValidateOptions(opts); err != nil {
		return err
	}

	if opts.JSON || opts.Report {
		return runReport(opts)
	}
	return runWizard(opts)
}

func ValidateOptions(opts Options) error {
	if opts.Report && opts.JSON {
		return fmt.Errorf("--report and --json are mutually exclusive")
	}
	if opts.DryRun && (opts.Report || opts.JSON) {
		return fmt.Errorf("--dry-run is only valid for the interactive audit workflow")
	}
	if len(opts.Include) > 0 {
		if _, err := clean.ResolveTargets(opts.Include); err != nil {
			return err
		}
	}
	return nil
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
		return &ExitError{Code: code}
	}
	return nil
}

func printHumanReport(rep Report) {
	fmt.Print("\n📋 Audit report\n\n")
	fmt.Printf("  Health score:     %d/100\n", rep.Health)
	fmt.Printf("  Disk / free:      %.0f%%\n", rep.DiskFreePctRoot)
	fmt.Printf("  Reclaimable:      %s\n", utils.HumanSize(rep.ReclaimableBytes))
	fmt.Println()
	if len(rep.Warnings) > 0 || len(rep.ScanErrors) > 0 {
		fmt.Println("  Scan warnings:")
		for _, warning := range append(append([]string(nil), rep.Warnings...), rep.ScanErrors...) {
			fmt.Printf("  • %s\n", warning)
		}
		fmt.Println()
	}

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
