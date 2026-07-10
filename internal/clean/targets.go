package clean

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/huaquanghan/mu/internal/utils"
)

// CleanTarget describes one cleaning category with scan and execute functions.
type CleanTarget struct {
	ID           string
	Label        string
	Scan         func() int64 // returns size in bytes (0 if unavailable/skipped)
	Execute      func(dryRun bool) error
	RequiresSudo bool
	OptIn        bool // if true, only included when ID is in opts.Include
}

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

// userCacheTarget scans ~/.cache but excludes the thumbnails subdir
// and any path matching cache_skip (defaults + user config).
func userCacheTarget() CleanTarget {
	cacheHome := utils.XDGCacheHome()
	thumbDir := filepath.Join(cacheHome, "thumbnails")

	return CleanTarget{
		ID:    "user-cache",
		Label: "User Cache (~/.cache)",
		Scan: func() int64 {
			wl, _ := utils.LoadWhitelist()
			var patterns []string
			if wl != nil {
				patterns = wl.CacheSkip.Dirs
			}
			var total int64
			filepath.WalkDir(cacheHome, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil
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
			return total
		},
		Execute: func(dryRun bool) error {
			wl, _ := utils.LoadWhitelist()
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
				if err := utils.SafeDelete(entry, dryRun); err != nil && !dryRun {
					fmt.Printf("  warn: %v\n", err)
				}
			}
			return nil
		},
	}
}

// thumbnailsTarget targets ~/.cache/thumbnails.
func thumbnailsTarget() CleanTarget {
	thumbDir := filepath.Join(utils.XDGCacheHome(), "thumbnails")
	return CleanTarget{
		ID:    "thumbnails",
		Label: "Thumbnail Cache",
		Scan: func() int64 {
			sz, _ := utils.DirSize(thumbDir)
			return sz
		},
		Execute: func(dryRun bool) error {
			if !utils.PathExists(thumbDir) {
				return nil
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
		Scan: func() int64 {
			sz, _ := utils.DirSize(aptPath)
			return sz
		},
		Execute: func(dryRun bool) error {
			if dryRun {
				utils.LogOp("dry-run", "sudo apt clean")
				return nil
			}
			cmd := exec.Command("sudo", "apt", "clean")
			cmd.Stdout = nil
			cmd.Stderr = os.Stderr
			return cmd.Run()
		},
	}
}

// journalLogsTarget targets systemd journal logs.
func journalLogsTarget() CleanTarget {
	return CleanTarget{
		ID:    "journal-logs",
		Label: "Journal Logs",
		Scan: func() int64 {
			return JournalSize()
		},
		Execute: func(dryRun bool) error {
			if dryRun {
				utils.LogOp("dry-run", "journalctl --vacuum-time=30d")
				return nil
			}
			cmd := exec.Command("journalctl", "--vacuum-time=30d")
			cmd.Stdout = nil
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("journalctl vacuum: %w", err)
			}
			utils.LogOp("vacuum", "journalctl")
			return nil
		},
	}
}

// JournalSize parses journalctl --disk-usage to get journal size in bytes.
// Returns 0 if journalctl is unavailable or output cannot be parsed.
func JournalSize() int64 {
	out, err := exec.Command("journalctl", "--disk-usage").Output()
	if err != nil {
		return 0
	}
	var val float64
	var unit string
	fmt.Sscanf(string(out), "Archived and active journals take up %f%s", &val, &unit)
	switch {
	case len(unit) > 0 && unit[0] == 'G':
		return int64(val * 1024 * 1024 * 1024)
	case len(unit) > 0 && unit[0] == 'M':
		return int64(val * 1024 * 1024)
	case len(unit) > 0 && unit[0] == 'K':
		return int64(val * 1024)
	}
	return 0
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
