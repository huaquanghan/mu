package utils

import (
	_ "embed"
	"path/filepath"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
)

//go:embed default-whitelist.toml
var defaultWhitelistTOML []byte

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
	if _, err := toml.Decode(string(defaultWhitelistTOML), wl); err != nil {
		// fallback: safe minimum if TOML is somehow corrupt
		wl.ProtectedPaths.System = []string{
			"/", "/boot", "/etc", "/usr", "/lib", "/lib64",
			"/bin", "/sbin", "/proc", "/sys", "/dev", "/run",
		}
		wl.ProtectedPaths.ProtectRunningKernel = true
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

var (
	wlOnce   sync.Once
	wlCached *Whitelist
)

// getWhitelist returns a process-wide cached whitelist (defaults + user config).
func getWhitelist() *Whitelist {
	wlOnce.Do(func() {
		wl, err := LoadWhitelist()
		if err != nil || wl == nil {
			wlCached = defaultWhitelist()
			return
		}
		wlCached = wl
	})
	return wlCached
}

// ResetWhitelistCacheForTest clears the cached whitelist so tests can inject config.
// Not for production use.
func ResetWhitelistCacheForTest() {
	wlOnce = sync.Once{}
	wlCached = nil
}

// IsWhitelisted returns true if path is protected by the system or user whitelist.
func IsWhitelisted(path string, wl *Whitelist) bool {
	if IsProtected(path) {
		return true
	}
	if wl == nil {
		return false
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

// MatchCacheSkip reports whether path under cacheHome matches any cache_skip pattern.
// Patterns are relative to cacheHome (e.g. "go-build", "mozilla/firefox/*/startupCache").
func MatchCacheSkip(path, cacheHome string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}
	cleanPath := filepath.Clean(path)
	cleanHome := filepath.Clean(cacheHome)
	if cleanPath == cleanHome {
		return false
	}
	rel, err := filepath.Rel(cleanHome, cleanPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	relSlash := filepath.ToSlash(rel)

	for _, pat := range patterns {
		pat = strings.TrimSpace(filepath.ToSlash(pat))
		if pat == "" || pat == "." {
			continue
		}
		if cachePathMatches(relSlash, pat) {
			return true
		}
	}
	return false
}

// cachePathMatches checks if relative path rel matches pattern, or is under a
// non-glob prefix that fully covers a directory, or has an ancestor that matches.
func cachePathMatches(rel, pattern string) bool {
	// Exact or under a plain (non-glob) prefix: "go-build" matches "go-build" and "go-build/x"
	if !strings.ContainsAny(pattern, "*?[") {
		if rel == pattern || strings.HasPrefix(rel, pattern+"/") {
			return true
		}
		return false
	}

	// Glob pattern: match full relative path
	if ok, _ := filepath.Match(pattern, rel); ok {
		return true
	}

	// Segment-wise match when lengths equal
	relParts := strings.Split(rel, "/")
	patParts := strings.Split(pattern, "/")
	if len(patParts) == len(relParts) {
		all := true
		for i := range patParts {
			ok, err := filepath.Match(patParts[i], relParts[i])
			if err != nil || !ok {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}

	// Ancestor of rel matches pattern (e.g. pattern matches a parent dir)
	cur := rel
	for {
		parent := filepath.ToSlash(filepath.Dir(cur))
		if parent == "." || parent == cur {
			break
		}
		if ok, _ := filepath.Match(pattern, parent); ok {
			return true
		}
		// Also plain prefix of pattern without globs in remaining... skip
		cur = parent
	}
	return false
}

// ShouldSkipCacheTopLevel reports whether a top-level cache entry name should be
// left entirely alone (exact denylist name or first segment of any pattern).
func ShouldSkipCacheTopLevel(name string, patterns []string) bool {
	name = filepath.Base(name)
	for _, pat := range patterns {
		pat = strings.TrimSpace(filepath.ToSlash(pat))
		if pat == "" {
			continue
		}
		first := pat
		if i := strings.IndexByte(pat, '/'); i >= 0 {
			first = pat[:i]
		}
		if first == name {
			return true
		}
		if strings.ContainsAny(first, "*?[") {
			if ok, _ := filepath.Match(first, name); ok {
				return true
			}
		}
	}
	return false
}
