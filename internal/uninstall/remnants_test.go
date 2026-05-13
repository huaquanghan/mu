package uninstall

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRemnants_FindsExistingDirs(t *testing.T) {
	tmp, err := os.MkdirTemp("", "mu-remnants-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	// Override XDG dirs via env
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))

	// Create a fake remnant dir
	configDir := filepath.Join(tmp, "config", "testpkg")
	os.MkdirAll(configDir, 0o755)

	remnants := FindRemnants("testpkg")
	found := false
	for _, r := range remnants {
		if r == configDir {
			found = true
		}
	}
	if !found {
		t.Errorf("expected %s in remnants, got %v", configDir, remnants)
	}
}

func TestRemnantSize_SkipsVarLib(t *testing.T) {
	paths := []string{"/var/lib/somepackage", "/tmp/mu-test-remnant"}
	// /var/lib should be skipped, /tmp path doesn't exist so also 0
	size := RemnantSize(paths)
	if size != 0 {
		t.Errorf("expected 0, got %d", size)
	}
}
