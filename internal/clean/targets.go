package clean

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/huaquanghan/mu/internal/command"
	"github.com/huaquanghan/mu/internal/utils"
)

// CleanTarget describes one cleaning category with scan and execute functions.
type CleanTarget struct {
	ID           string
	Label        string
	Scan         func() (int64, error)
	Preview      func() ([]string, error)
	Execute      func(dryRun bool) error
	RequiresSudo bool
	OptIn        bool // if true, only included when ID is in opts.Include
}

var cleanRunner = command.Runner(command.ExecRunner{})

// AllTargets returns all built-in clean targets in display order.
// Docker target is included only if the Docker socket is present.
func AllTargets() []CleanTarget {
	targets := []CleanTarget{
		userCacheTarget(),
		thumbnailsTarget(),
		aptCacheTarget(),
		journalLogsTarget(),
		snapTarget(),
		kernelsTarget(),
		browserCacheTarget(),
	}
	if dt := dockerTarget(); dt != nil {
		targets = append(targets, *dt)
	}
	return targets
}

// ResolveTargets validates include IDs and returns the enabled target set.
func ResolveTargets(include []string) ([]CleanTarget, error) {
	all := AllTargets()
	byID := make(map[string]CleanTarget, len(all))
	for _, target := range all {
		byID[target.ID] = target
	}
	includeSet := make(map[string]bool, len(include))
	for _, id := range include {
		target, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("unknown clean include ID: %s", id)
		}
		if !target.OptIn {
			return nil, fmt.Errorf("clean include ID is not opt-in: %s", id)
		}
		includeSet[id] = true
	}
	var targets []CleanTarget
	for _, target := range all {
		if target.OptIn && !includeSet[target.ID] {
			continue
		}
		targets = append(targets, target)
	}
	return targets, nil
}

// userCacheTarget scans ~/.cache but excludes the thumbnails subdir
// and any path matching cache_skip (defaults + user config).
func userCacheTarget() CleanTarget {
	cacheHome := utils.XDGCacheHome()
	thumbDir := filepath.Join(cacheHome, "thumbnails")

	return CleanTarget{
		ID:    "user-cache",
		Label: "User Cache (~/.cache)",
		Scan: func() (int64, error) {
			if err := utils.ValidateCleanupRoot(cacheHome); err != nil {
				return 0, err
			}
			if _, err := os.Lstat(cacheHome); errors.Is(err, os.ErrNotExist) {
				return 0, nil
			} else if err != nil {
				return 0, err
			}
			wl, err := utils.LoadWhitelist()
			if err != nil {
				return 0, err
			}
			var patterns []string
			if wl != nil {
				patterns = wl.CacheSkip.Dirs
			}
			var total int64
			err = filepath.WalkDir(cacheHome, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				// Skip thumbnails entirely to avoid double-counting
				if path == thumbDir {
					return filepath.SkipDir
				}
				if path != cacheHome && utils.MatchCacheSkip(path, cacheHome, patterns) {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				if !d.IsDir() {
					info, err := d.Info()
					if err == nil {
						total += info.Size()
					}
				}
				return nil
			})
			return total, err
		},
		Execute: func(dryRun bool) error {
			if err := utils.ValidateCleanupRoot(cacheHome); err != nil {
				return err
			}
			wl, err := utils.LoadWhitelist()
			if err != nil {
				return err
			}
			var patterns []string
			if wl != nil {
				patterns = wl.CacheSkip.Dirs
			}
			// Delete files inside cache dir but skip thumbnails (handled separately)
			// and denylisted cache_skip entries.
			entries, err := filepath.Glob(filepath.Join(cacheHome, "*"))
			if err != nil {
				return err
			}
			var deleteErrors []error
			for _, entry := range entries {
				base := filepath.Base(entry)
				if base == "thumbnails" {
					continue
				}
				if utils.ShouldSkipCacheTopLevel(base, patterns) {
					continue
				}
				if utils.MatchCacheSkip(entry, cacheHome, patterns) {
					continue
				}
				if err := utils.ValidateCleanupCandidate(cacheHome, entry); err != nil {
					deleteErrors = append(deleteErrors, err)
					continue
				}
				if err := utils.SafeDelete(entry, dryRun); err != nil {
					deleteErrors = append(deleteErrors, err)
				}
			}
			return errors.Join(deleteErrors...)
		},
	}
}

// thumbnailsTarget targets ~/.cache/thumbnails.
func thumbnailsTarget() CleanTarget {
	thumbDir := filepath.Join(utils.XDGCacheHome(), "thumbnails")
	return CleanTarget{
		ID:    "thumbnails",
		Label: "Thumbnail Cache",
		Scan: func() (int64, error) {
			if !utils.PathExists(thumbDir) {
				return 0, nil
			}
			return utils.DirSize(thumbDir)
		},
		Execute: func(dryRun bool) error {
			if !utils.PathExists(thumbDir) {
				return nil
			}
			if err := utils.ValidateCleanupCandidate(utils.XDGCacheHome(), thumbDir); err != nil {
				return err
			}
			return utils.SafeDelete(thumbDir, dryRun)
		},
	}
}

// aptCacheTarget targets /var/cache/apt/archives.
func aptCacheTarget() CleanTarget {
	const aptPath = "/var/cache/apt/archives"
	return CleanTarget{
		ID:           "apt-cache",
		Label:        "APT Package Cache",
		RequiresSudo: true,
		Scan: func() (int64, error) {
			size, _ := utils.DirSize(aptPath)
			return size, nil
		},
		Execute: func(dryRun bool) error {
			if dryRun {
				utils.LogOutcome("apt-clean", "apt cache", "dry-run")
				return nil
			}
			result, err := cleanRunner.Run(context.Background(), "sudo", "apt-get", "clean")
			if len(result.Stderr) > 0 {
				_, _ = os.Stderr.Write(result.Stderr)
			}
			return err
		},
	}
}

// journalLogsTarget targets systemd journal logs.
func journalLogsTarget() CleanTarget {
	return CleanTarget{
		ID:           "journal-logs",
		Label:        "Journal Logs",
		RequiresSudo: true,
		Scan: func() (int64, error) {
			return JournalSize()
		},
		Execute: func(dryRun bool) error {
			if dryRun {
				utils.LogOutcome("journal-vacuum", "30d", "dry-run")
				return nil
			}
			result, err := cleanRunner.Run(context.Background(), "sudo", "journalctl", "--vacuum-time=30d")
			if len(result.Stderr) > 0 {
				_, _ = os.Stderr.Write(result.Stderr)
			}
			if err != nil {
				return fmt.Errorf("journalctl vacuum: %w", err)
			}
			utils.LogOutcome("journal-vacuum", "30d", "success")
			return nil
		},
	}
}

// JournalSize parses journalctl --disk-usage to get journal size in bytes.
// A missing journalctl is optional; command and parse failures are returned.
func JournalSize() (int64, error) {
	if _, err := cleanRunner.LookPath("journalctl"); err != nil {
		return 0, nil
	}
	result, err := cleanRunner.Run(context.Background(), "env", "LC_ALL=C", "journalctl", "--disk-usage")
	if err != nil {
		return 0, err
	}
	var val float64
	var unit string
	if _, err := fmt.Sscanf(string(result.Stdout), "Archived and active journals take up %f%s", &val, &unit); err != nil {
		return 0, fmt.Errorf("parse journal disk usage: %w", err)
	}
	switch {
	case len(unit) > 0 && unit[0] == 'G':
		return int64(val * 1024 * 1024 * 1024), nil
	case len(unit) > 0 && unit[0] == 'M':
		return int64(val * 1024 * 1024), nil
	case len(unit) > 0 && unit[0] == 'K':
		return int64(val * 1024), nil
	}
	return 0, fmt.Errorf("unknown journal size unit: %q", strings.TrimSpace(unit))
}

// TargetByID returns the clean target with the given ID, if present.
func TargetByID(id string) (CleanTarget, bool) {
	for _, t := range AllTargets() {
		if t.ID == id {
			return t, true
		}
	}
	return CleanTarget{}, false
}
