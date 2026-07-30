package clean

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/huaquanghan/mu/internal/utils"
)

type snapRevision struct {
	name     string
	revision string
}

// parseDisabledSnaps parses `snap list --all` output and returns disabled revisions.
// Columns: Name Version Rev Tracking Publisher Notes
// "disabled" appears in Notes (last column).
func parseDisabledSnaps(output string) []snapRevision {
	var result []snapRevision
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		if i == 0 {
			// Skip header line
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		// Expect at least 6 fields: Name Version Rev Tracking Publisher Notes
		if len(fields) < 6 {
			continue
		}
		notes := fields[len(fields)-1]
		if strings.Contains(notes, "disabled") {
			result = append(result, snapRevision{
				name:     fields[0],
				revision: fields[2],
			})
		}
	}
	return result
}

// snapTarget returns a CleanTarget for disabled snap revisions.
func snapTarget() CleanTarget {
	// Check if snap is installed
	_, snapErr := cleanRunner.LookPath("snap")

	return CleanTarget{
		ID:           "snap",
		Label:        "Snap Disabled Revisions",
		RequiresSudo: true,
		Scan: func() (int64, error) {
			if snapErr != nil {
				return 0, nil
			}
			result, err := cleanRunner.Run(context.Background(), "snap", "list", "--all")
			if err != nil {
				return 0, fmt.Errorf("snap list: %w", err)
			}
			revs := parseDisabledSnaps(string(result.Stdout))
			var total int64
			var scanErrors []error
			for _, r := range revs {
				snapPath := fmt.Sprintf("/var/lib/snapd/snaps/%s_%s.snap", r.name, r.revision)
				duResult, err := cleanRunner.Run(context.Background(), "du", "-sb", snapPath)
				if err != nil {
					scanErrors = append(scanErrors, fmt.Errorf("size %s: %w", snapPath, err))
					continue
				}
				parts := strings.Fields(string(duResult.Stdout))
				if len(parts) > 0 {
					if sz, err := strconv.ParseInt(parts[0], 10, 64); err == nil {
						total += sz
					}
				}
			}
			return total, errors.Join(scanErrors...)
		},
		Execute: func(dryRun bool) error {
			if snapErr != nil {
				return nil
			}
			result, err := cleanRunner.Run(context.Background(), "snap", "list", "--all")
			if err != nil {
				return fmt.Errorf("snap list: %w", err)
			}
			revs := parseDisabledSnaps(string(result.Stdout))
			var removeErrors []error
			for _, r := range revs {
				target := fmt.Sprintf("snap %s rev %s", r.name, r.revision)
				if dryRun {
					utils.LogOutcome("snap-remove", target, "dry-run")
					continue
				}
				if _, err := cleanRunner.Run(context.Background(), "sudo", "snap", "remove", "--revision", r.revision, r.name); err != nil {
					utils.LogOutcome("snap-remove", target, "failure")
					removeErrors = append(removeErrors, fmt.Errorf("remove snap %s rev %s: %w", r.name, r.revision, err))
					continue
				}
				utils.LogOutcome("snap-remove", target, "success")
			}
			return errors.Join(removeErrors...)
		},
	}
}
