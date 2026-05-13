package utils

import (
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Whitelist holds path protection rules and skip lists.
type Whitelist struct {
	ProtectedPaths struct {
		System               []string `toml:"system"`
		ProtectRunningKernel bool     `toml:"protect_running_kernel"`
	} `toml:"protected_paths"`
	CacheSkip struct {
		Dirs []string `toml:"dirs"`
	} `toml:"cache_skip"`
	OptimizeSkip struct {
		Steps []string `toml:"steps"`
	} `toml:"optimize_skip"`
}

func defaultWhitelist() *Whitelist {
	wl := &Whitelist{}
	wl.ProtectedPaths.System = []string{
		"/", "/boot", "/etc", "/usr", "/lib", "/lib64",
		"/bin", "/sbin", "/proc", "/sys", "/dev", "/run",
	}
	wl.ProtectedPaths.ProtectRunningKernel = true
	wl.CacheSkip.Dirs = []string{
		"mozilla/firefox/*/startupCache",
		"google-chrome/Default/Session*",
	}
	return wl
}

// LoadWhitelist returns defaults merged with user overrides from ~/.config/mu/config.toml.
func LoadWhitelist() (*Whitelist, error) {
	wl := defaultWhitelist()
	userCfg := filepath.Join(XDGConfigHome(), "mu", "config.toml")
	if !PathExists(userCfg) {
		return wl, nil
	}
	var override Whitelist
	if _, err := toml.DecodeFile(userCfg, &override); err != nil {
		return wl, err
	}
	if len(override.ProtectedPaths.System) > 0 {
		wl.ProtectedPaths.System = append(wl.ProtectedPaths.System, override.ProtectedPaths.System...)
	}
	if len(override.CacheSkip.Dirs) > 0 {
		wl.CacheSkip.Dirs = append(wl.CacheSkip.Dirs, override.CacheSkip.Dirs...)
	}
	if len(override.OptimizeSkip.Steps) > 0 {
		wl.OptimizeSkip.Steps = override.OptimizeSkip.Steps
	}
	return wl, nil
}

// IsWhitelisted returns true if path is protected by the system or user whitelist.
func IsWhitelisted(path string, wl *Whitelist) bool {
	if IsProtected(path) {
		return true
	}
	clean := filepath.Clean(path)
	for _, p := range wl.ProtectedPaths.System {
		pp := filepath.Clean(p)
		if clean == pp || strings.HasPrefix(clean, pp+"/") {
			return true
		}
	}
	return false
}
