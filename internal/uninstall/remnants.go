package uninstall

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/huaquanghan/mu/internal/utils"
)

// knownAliases maps package names to their actual folder name in ~/.config etc.
var knownAliases = map[string]string{
	"code":                 "Code",
	"google-chrome-stable": "google-chrome",
	"google-chrome":        "google-chrome",
	"chromium-browser":     "chromium",
	"vscode":               "Code",
	"slack-desktop":        "Slack",
	"discord":              "discord",
	"spotify-client":       "spotify",
}

// FindRemnants returns existing remnant directories for a package.
func FindRemnants(name string) []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	_ = home // used implicitly via XDG helpers which call os.UserHomeDir internally

	// Collect candidate names (original + alias)
	names := []string{name}
	if alias, ok := knownAliases[name]; ok && alias != name {
		names = append(names, alias)
	}

	var found []string
	seen := map[string]bool{}

	for _, n := range names {
		candidates := []string{
			filepath.Join(utils.XDGConfigHome(), n),
			filepath.Join(utils.XDGDataHome(), n),
			filepath.Join(utils.XDGCacheHome(), n),
			filepath.Join("/var/lib", n),
		}
		for _, p := range candidates {
			if seen[p] {
				continue
			}
			seen[p] = true
			if utils.PathExists(p) {
				found = append(found, p)
			}
		}
	}
	return found
}

// RemnantSize returns the total size in bytes of user-owned remnant dirs.
// Skips /var/lib paths (would need sudo).
func RemnantSize(paths []string) int64 {
	var total int64
	for _, p := range paths {
		if strings.HasPrefix(p, "/var/") {
			continue // skip root-owned
		}
		size, err := utils.DirSize(p)
		if err == nil {
			total += size
		}
	}
	return total
}
