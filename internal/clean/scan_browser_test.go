package clean

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBrowserCacheTarget_scan(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Create a fake Chrome cache file
	cacheDir := filepath.Join(tmp, ".config", "google-chrome", "Default", "Cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	testFile := filepath.Join(cacheDir, "testfile")
	if err := os.WriteFile(testFile, []byte("hello cache"), 0o644); err != nil {
		t.Fatal(err)
	}

	target := browserCacheTarget()
	sz := target.Scan()
	if sz <= 0 {
		t.Errorf("expected non-zero scan size, got %d", sz)
	}
}

func TestBrowserCacheTarget_isOptIn(t *testing.T) {
	target := browserCacheTarget()
	if !target.OptIn {
		t.Error("browser cache target should be opt-in")
	}
}

func TestBrowserCacheTarget_scanEmpty(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// No cache directories created
	target := browserCacheTarget()
	sz := target.Scan()
	if sz != 0 {
		t.Errorf("expected 0 for empty home, got %d", sz)
	}
}
