package clean

import (
	"fmt"
	"os/exec"
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
	_, snapErr := exec.LookPath("snap")

	return CleanTarget{
		ID:           "snap",
		Label:        "Snap Disabled Revisions",
		RequiresSudo: true,
		Scan: func() int64 {
			if snapErr != nil {
				return 0
			}
			out, err := exec.Command("snap", "list", "--all").Output()
			if err != nil {
				return 0
			}
			revs := parseDisabledSnaps(string(out))
			var total int64
			for _, r := range revs {
				snapPath := fmt.Sprintf("/snap/%s/%s", r.name, r.revision)
				duOut, err := exec.Command("du", "-sb", snapPath).Output()
				if err != nil {
					continue
				}
				parts := strings.Fields(string(duOut))
				if len(parts) > 0 {
					if sz, err := strconv.ParseInt(parts[0], 10, 64); err == nil {
						total += sz
					}
				}
			}
			return total
		},
		Execute: func(dryRun bool) error {
			if snapErr != nil {
				return nil
			}
			out, err := exec.Command("snap", "list", "--all").Output()
			if err != nil {
				return fmt.Errorf("snap list: %w", err)
			}
			revs := parseDisabledSnaps(string(out))
			for _, r := range revs {
				target := fmt.Sprintf("snap %s rev %s", r.name, r.revision)
				if dryRun {
					utils.LogOp("dry-run", target)
					continue
				}
				cmd := exec.Command("sudo", "snap", "remove", "--revision", r.revision, r.name)
				if err := cmd.Run(); err != nil {
					fmt.Printf("  warn: remove snap %s rev %s: %v\n", r.name, r.revision, err)
					continue
				}
				utils.LogOp("snap-remove", target)
			}
			return nil
		},
	}
}
