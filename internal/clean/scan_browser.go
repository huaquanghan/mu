package clean

import (
	"fmt"
	"os"
	"path/filepath"

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
		Scan: func() int64 {
			var total int64
			for _, p := range browserCachePaths() {
				sz, err := utils.DirSize(p)
				if err == nil {
					total += sz
				}
			}
			return total
		},
		Execute: func(dryRun bool) error {
			for _, p := range browserCachePaths() {
				if err := utils.SafeDelete(p, dryRun); err != nil {
					fmt.Printf("  warn: %v\n", err)
				}
			}
			return nil
		},
	}
}
