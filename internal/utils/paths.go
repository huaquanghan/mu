package utils

import (
	"os"
	"path/filepath"
	"strings"
)

// protectedPrefixes are never touched without explicit --force.
var protectedPrefixes = []string{
	"/boot", "/etc", "/usr", "/lib", "/lib64",
	"/bin", "/sbin", "/proc", "/sys", "/dev", "/run",
}

func IsProtected(path string) bool {
	clean := filepath.Clean(path)
	if clean == "/" {
		return true
	}
	for _, p := range protectedPrefixes {
		if clean == p || strings.HasPrefix(clean, p+"/") {
			return true
		}
	}
	return false
}

func XDGCacheHome() string {
	if d := os.Getenv("XDG_CACHE_HOME"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache")
}

func XDGConfigHome() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

func XDGDataHome() string {
	return xdgDataHome()
}
