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
