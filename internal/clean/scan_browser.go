package clean

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/huaquanghan/mu/internal/utils"
)

// browserCachePaths returns all browser cache paths that exist on the system.
func browserCachePaths() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	candidates := []string{
		filepath.Join(home, ".config", "google-chrome", "Default", "Cache"),
		filepath.Join(home, ".config", "Code", "CachedData"),
		filepath.Join(home, ".config", "Code", "CachedExtensions"),
	}

	// Firefox: ~/.mozilla/firefox/*/cache2 (may match multiple profiles)
	firefoxGlob := filepath.Join(home, ".mozilla", "firefox", "*", "cache2")
	matches, _ := filepath.Glob(firefoxGlob)
	candidates = append(candidates, matches...)

	var existing []string
	for _, p := range candidates {
		if utils.PathExists(p) {
			existing = append(existing, p)
		}
	}
	return existing
}

// browserCacheTarget returns a CleanTarget for browser caches (opt-in).
func browserCacheTarget() CleanTarget {
	return CleanTarget{
		ID:    "browser-cache",
		Label: "Browser Caches (Chrome/Firefox/VSCode)",
		OptIn: true,
		Scan: func() (int64, error) {
			var total int64
			var scanErrors []error
			for _, p := range browserCachePaths() {
				sz, err := utils.DirSize(p)
				if err == nil {
					total += sz
				} else {
					scanErrors = append(scanErrors, err)
				}
			}
			return total, errors.Join(scanErrors...)
		},
		Execute: func(dryRun bool) error {
			var deleteErrors []error
			for _, p := range browserCachePaths() {
				root := browserCleanupRoot(p)
				if err := utils.ValidateCleanupCandidate(root, p); err != nil {
					deleteErrors = append(deleteErrors, err)
					continue
				}
				if err := utils.SafeDelete(p, dryRun); err != nil {
					deleteErrors = append(deleteErrors, err)
				}
			}
			return errors.Join(deleteErrors...)
		},
	}
}

func browserCleanupRoot(path string) string {
	home, _ := os.UserHomeDir()
	mozilla := filepath.Join(home, ".mozilla")
	if path == mozilla || strings.HasPrefix(path, mozilla+string(filepath.Separator)) {
		return mozilla
	}
	return filepath.Join(home, ".config")
}
