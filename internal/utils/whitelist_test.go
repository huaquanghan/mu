package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsWhitelisted_ProtectedPaths(t *testing.T) {
	wl := defaultWhitelist()
	protected := []string{"/", "/etc", "/etc/passwd", "/boot", "/usr/bin/env"}
	for _, p := range protected {
		if !IsWhitelisted(p, wl) {
			t.Errorf("expected %s to be whitelisted (protected)", p)
		}
	}
}

func TestIsWhitelisted_SafePaths(t *testing.T) {
	wl := defaultWhitelist()
	safe := []string{"/tmp/foo", "/home/user/.cache/something", "/var/cache/apt"}
	for _, p := range safe {
		if IsWhitelisted(p, wl) {
			t.Errorf("expected %s to NOT be whitelisted", p)
		}
	}
}

func TestLoadWhitelist_NoUserConfig(t *testing.T) {
	// Override XDG_CONFIG_HOME to a temp dir with no config
	tmp, err := os.MkdirTemp("", "mu-wl-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	t.Setenv("XDG_CONFIG_HOME", tmp)

	wl, err := LoadWhitelist()
	if err != nil {
		t.Fatalf("LoadWhitelist: %v", err)
	}
	if !IsWhitelisted("/etc", wl) {
		t.Error("default whitelist must protect /etc")
	}
}

func TestLoadWhitelist_UserOverride(t *testing.T) {
	tmp, err := os.MkdirTemp("", "mu-wl-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	t.Setenv("XDG_CONFIG_HOME", tmp)

	cfgDir := filepath.Join(tmp, "mu")
	os.MkdirAll(cfgDir, 0o700)
	os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(`
[optimize_skip]
steps = ["apt"]
`), 0o600)

	wl, err := LoadWhitelist()
	if err != nil {
		t.Fatalf("LoadWhitelist with override: %v", err)
	}
	if len(wl.OptimizeSkip.Steps) != 1 || wl.OptimizeSkip.Steps[0] != "apt" {
		t.Errorf("expected OptimizeSkip.Steps=[apt], got %v", wl.OptimizeSkip.Steps)
	}
}

func TestMatchCacheSkip_TopLevel(t *testing.T) {
	home := "/home/u/.cache"
	patterns := []string{"go-build", "pip"}
	if !MatchCacheSkip(filepath.Join(home, "go-build"), home, patterns) {
		t.Error("expected go-build to match")
	}
	if !MatchCacheSkip(filepath.Join(home, "go-build", "x", "y"), home, patterns) {
		t.Error("expected go-build child to match")
	}
	if MatchCacheSkip(filepath.Join(home, "thumbnails"), home, patterns) {
		t.Error("thumbnails should not match")
	}
}

func TestMatchCacheSkip_Glob(t *testing.T) {
	home := "/home/u/.cache"
	patterns := []string{"mozilla/firefox/*/startupCache"}
	path := filepath.Join(home, "mozilla", "firefox", "abc.default", "startupCache")
	if !MatchCacheSkip(path, home, patterns) {
		t.Error("expected nested firefox startupCache to match glob")
	}
	if MatchCacheSkip(filepath.Join(home, "mozilla", "firefox", "abc.default", "cache2"), home, patterns) {
		t.Error("cache2 should not match startupCache pattern")
	}
}

func TestShouldSkipCacheTopLevel(t *testing.T) {
	patterns := []string{"go-build", "mozilla/firefox/*/startupCache", "pip"}
	if !ShouldSkipCacheTopLevel("go-build", patterns) {
		t.Error("go-build should be skipped at top level")
	}
	if !ShouldSkipCacheTopLevel("mozilla", patterns) {
		t.Error("mozilla should be skipped (first segment of nested pattern)")
	}
	if ShouldSkipCacheTopLevel("thumbnails", patterns) {
		t.Error("thumbnails should not be skipped via cache_skip")
	}
}

func TestDefaultWhitelist_HasGoBuildSkip(t *testing.T) {
	wl := defaultWhitelist()
	found := false
	for _, d := range wl.CacheSkip.Dirs {
		if d == "go-build" {
			found = true
			break
		}
	}
	if !found {
		t.Error("default cache_skip must include go-build")
	}
}
