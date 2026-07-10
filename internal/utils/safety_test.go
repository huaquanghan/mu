package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestXDGRelativePathsAreIgnored(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", "relative/cache")
	t.Setenv("XDG_CONFIG_HOME", "relative/config")
	t.Setenv("XDG_DATA_HOME", "relative/data")
	if got := XDGCacheHome(); got != filepath.Join(home, ".cache") {
		t.Fatalf("cache home = %q", got)
	}
	if got := XDGConfigHome(); got != filepath.Join(home, ".config") {
		t.Fatalf("config home = %q", got)
	}
	if got := XDGDataHome(); got != filepath.Join(home, ".local", "share") {
		t.Fatalf("data home = %q", got)
	}
}

func TestValidateCleanupRootRejectsDangerousRoots(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, root := range []string{"relative", "/", home, filepath.Dir(home)} {
		if err := ValidateCleanupRoot(root); err == nil {
			t.Errorf("expected root %q to be rejected", root)
		}
	}
	safe := filepath.Join(home, ".cache")
	if err := ValidateCleanupRoot(safe); err != nil {
		t.Fatalf("safe root rejected: %v", err)
	}
}

func TestValidateCleanupCandidateRejectsEscapeAndTopSymlink(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(home, ".cache")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(home, "outside")
	if err := os.WriteFile(outside, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCleanupCandidate(root, outside); err == nil {
		t.Fatal("expected outside candidate rejection")
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCleanupCandidate(root, link); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink rejection, got %v", err)
	}
}

func TestValidateCleanupRootAndCandidateRejectSymlinkTraversal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	realRoot := filepath.Join(home, "real-cache")
	if err := os.MkdirAll(realRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	linkedRoot := filepath.Join(home, ".cache")
	if err := os.Symlink(realRoot, linkedRoot); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCleanupRoot(linkedRoot); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected linked root rejection, got %v", err)
	}

	root := filepath.Join(home, "safe-cache")
	external := filepath.Join(home, "external")
	if err := os.MkdirAll(external, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	parentLink := filepath.Join(root, "linked-parent")
	if err := os.Symlink(external, parentLink); err != nil {
		t.Fatal(err)
	}
	candidate := filepath.Join(parentLink, "cache")
	if err := os.Mkdir(candidate, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCleanupCandidate(root, candidate); err == nil || !strings.Contains(err.Error(), "traverses a symlink") {
		t.Fatalf("expected parent symlink rejection, got %v", err)
	}
}

func TestMalformedWhitelistFailsClosedEvenInDryRun(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	dir := filepath.Join(configRoot, "mu")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("not = [valid"), 0o600); err != nil {
		t.Fatal(err)
	}
	ResetWhitelistCacheForTest()
	t.Cleanup(ResetWhitelistCacheForTest)
	path := filepath.Join(t.TempDir(), "candidate")
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SafeDelete(path, true); err == nil || !strings.Contains(err.Error(), "invalid mu configuration") {
		t.Fatalf("expected fail-closed config error, got %v", err)
	}
	if !PathExists(path) {
		t.Fatal("candidate changed despite malformed config")
	}
}
