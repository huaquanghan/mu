package clean

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/huaquanghan/mu/internal/utils"
)

var freedSpacePattern = regexp.MustCompile(`(?i)after this operation,\s*([0-9]+(?:\.[0-9]+)?)\s*([kmgt]?b)\s+of additional disk space will be used|(?i)([0-9]+(?:\.[0-9]+)?)\s*([kmgt]?b)\s+disk space will be freed`)

// ParseAutoremoveSimulation extracts APT's complete removal candidate set and
// estimated freed bytes from apt-get -s autoremove --purge output.
func ParseAutoremoveSimulation(output string) ([]string, int64) {
	seen := map[string]bool{}
	var packages []string
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 || (fields[0] != "Remv" && fields[0] != "Purg") {
			continue
		}
		if !seen[fields[1]] {
			seen[fields[1]] = true
			packages = append(packages, fields[1])
		}
	}

	var bytes int64
	for _, line := range strings.Split(output, "\n") {
		match := freedSpacePattern.FindStringSubmatch(strings.TrimSpace(line))
		if len(match) == 0 {
			continue
		}
		value, unit := match[3], match[4]
		if value == "" {
			// "additional disk space will be used" is not reclaimable.
			continue
		}
		bytes = parseAPTSize(value, unit)
	}
	return packages, bytes
}

func parseAPTSize(value, unit string) int64 {
	number, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	multiplier := float64(1)
	switch strings.ToUpper(unit) {
	case "KB":
		multiplier = 1000
	case "MB":
		multiplier = 1000 * 1000
	case "GB":
		multiplier = 1000 * 1000 * 1000
	case "TB":
		multiplier = 1000 * 1000 * 1000 * 1000
	}
	return int64(number * multiplier)
}

func simulateAutoremove(ctx context.Context) ([]string, int64, error) {
	result, err := cleanRunner.Run(ctx, "apt-get", "-s", "-o", "Debug::NoLocking=1", "autoremove", "--purge")
	if err != nil {
		return nil, 0, fmt.Errorf("apt-get autoremove preview: %w: %s", err, strings.TrimSpace(string(result.Stderr)))
	}
	packages, bytes := ParseAutoremoveSimulation(string(result.Stdout))
	return packages, bytes, nil
}

// RunAutoremove executes APT's own autoremove policy. Active package
// transactions intentionally have no short timeout.
func RunAutoremove(ctx context.Context, dryRun bool) error {
	if dryRun {
		previewCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		packages, _, err := simulateAutoremove(previewCtx)
		if err != nil {
			return err
		}
		utils.LogOutcome("apt-autoremove", strings.Join(packages, ","), "dry-run")
		return nil
	}
	result, err := cleanRunner.Run(ctx, "sudo", "apt-get", "autoremove", "--purge", "-y")
	if err != nil {
		utils.LogOutcome("apt-autoremove", "apt policy", "failure")
		return fmt.Errorf("apt-get autoremove: %w: %s", err, strings.TrimSpace(string(result.Stderr)))
	}
	utils.LogOutcome("apt-autoremove", "apt policy", "success")
	return nil
}

// kernelsTarget keeps the stable target ID for CLI compatibility while using
// APT policy for the complete autoremove candidate set.
func kernelsTarget() CleanTarget {
	var once sync.Once
	var packages []string
	var bytes int64
	var scanErr error
	load := func() {
		once.Do(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			packages, bytes, scanErr = simulateAutoremove(ctx)
		})
	}
	return CleanTarget{
		ID:           "kernels",
		Label:        "APT Autoremove Candidates",
		RequiresSudo: true,
		Scan: func() (int64, error) {
			load()
			return bytes, scanErr
		},
		Preview: func() ([]string, error) {
			load()
			return append([]string(nil), packages...), scanErr
		},
		Execute: func(dryRun bool) error {
			return RunAutoremove(context.Background(), dryRun)
		},
	}
}
