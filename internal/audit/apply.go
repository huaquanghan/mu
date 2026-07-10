package audit

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/huaquanghan/mu/internal/clean"
	"github.com/huaquanghan/mu/internal/optimize"
	"github.com/huaquanghan/mu/internal/utils"
)

// ApplyResult is the outcome of applying one finding.
type ApplyResult struct {
	FindingID string
	Action    string
	Err       error
	Skipped   bool
}

// Apply runs clean.Execute / optimize.RunStep for each selected finding.
// dryRun uses Execute(true) and skips real optimize steps (logs only).
func Apply(selected []Finding, dryRun bool, debug bool, out io.Writer) []ApplyResult {
	if out == nil {
		out = os.Stdout
	}

	// Dedupe actions (e.g. two findings same action)
	seen := make(map[string]bool)
	var results []ApplyResult

	for _, f := range selected {
		if !f.Selectable || f.Action == "" || f.Action == "none" {
			utils.LogOutcome("audit", f.Action, "skipped")
			results = append(results, ApplyResult{FindingID: f.ID, Action: f.Action, Skipped: true})
			continue
		}
		if seen[f.Action] {
			utils.LogOutcome("audit", f.Action, "skipped")
			results = append(results, ApplyResult{FindingID: f.ID, Action: f.Action, Skipped: true})
			continue
		}
		seen[f.Action] = true

		kind, id := ParseAction(f.Action)
		fmt.Fprintf(out, "→ Applying %s (%s)…\n", f.Title, f.Action)

		var (
			skipped bool
			err     error
		)
		switch kind {
		case "clean":
			err = applyClean(id, dryRun)
		case "optimize":
			skipped, err = applyOptimize(id, dryRun, debug, out)
		default:
			err = fmt.Errorf("unknown action kind: %s", kind)
		}

		if err != nil {
			fmt.Fprintf(out, "  warn: %v\n", err)
			utils.LogOutcome("audit", f.Action, "failure")
			if debug {
				fmt.Fprintf(os.Stderr, "audit apply %s: %v\n", f.Action, err)
			}
		} else if skipped {
			fmt.Fprintln(out, "  ⏭️ Skipped by optimize policy")
			utils.LogOutcome("audit", f.Action, "skipped")
		} else {
			fmt.Fprintln(out, "  ✅ Done")
			outcome := "success"
			if dryRun {
				outcome = "dry-run"
			}
			utils.LogOutcome("audit", f.Action, outcome)
		}
		results = append(results, ApplyResult{FindingID: f.ID, Action: f.Action, Err: err, Skipped: skipped})
	}
	return results
}

func applyClean(targetID string, dryRun bool) error {
	t, ok := clean.TargetByID(targetID)
	if !ok {
		// Docker may be absent — try scan all again already did; missing target
		return fmt.Errorf("clean target %q not available", targetID)
	}
	if err := t.Execute(dryRun); err != nil {
		return err
	}
	return nil
}

func applyOptimize(stepID string, dryRun bool, debug bool, out io.Writer) (bool, error) {
	if dryRun {
		fmt.Fprintf(out, "  [dry-run] would run optimize step %s\n", stepID)
		return false, nil
	}
	var buf strings.Builder
	skipped, err := optimize.RunStep(stepID, optimize.Options{Debug: debug}, &buf)
	if s := strings.TrimSpace(buf.String()); s != "" {
		fmt.Fprintln(out, s)
	}
	if err != nil {
		return false, err
	}
	if skipped {
		return true, nil
	}
	return false, nil
}

// CountApplyErrors returns how many apply results failed.
func CountApplyErrors(rs []ApplyResult) int {
	n := 0
	for _, r := range rs {
		if r.Err != nil {
			n++
		}
	}
	return n
}
