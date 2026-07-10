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
	sz, err := target.Scan()
	if err != nil {
		t.Fatal(err)
	}
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
	sz, err := target.Scan()
	if err != nil {
		t.Fatal(err)
	}
	if sz != 0 {
		t.Errorf("expected 0 for empty home, got %d", sz)
	}
}

func TestBrowserCacheTargetDryRunValidatesAndKeepsCache(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	cache := filepath.Join(home, ".config", "Code", "CachedData")
	if err := os.MkdirAll(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := browserCacheTarget().Execute(true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cache); err != nil {
		t.Fatal("dry-run removed browser cache")
	}
	if got := browserCleanupRoot(cache); got != filepath.Join(home, ".config") {
		t.Fatalf("cleanup root=%q", got)
	}
	mozilla := filepath.Join(home, ".mozilla", "firefox", "x", "cache2")
	if got := browserCleanupRoot(mozilla); got != filepath.Join(home, ".mozilla") {
		t.Fatalf("mozilla root=%q", got)
	}
}
