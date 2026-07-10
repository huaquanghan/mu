package clean

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/huaquanghan/mu/internal/utils"
)

func TestUserCacheTarget_skipsDenylisted(t *testing.T) {
	tmp := t.TempDir()
	cacheHome := filepath.Join(tmp, ".cache")
	t.Setenv("XDG_CACHE_HOME", cacheHome)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, ".config"))
	utils.ResetWhitelistCacheForTest()
	t.Cleanup(utils.ResetWhitelistCacheForTest)

	// Safe-to-delete junk
	junk := filepath.Join(cacheHome, "some-app")
	if err := os.MkdirAll(junk, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(junk, "a.bin"), []byte("junk data here"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Denylisted go-build (must not be counted or deleted)
	goBuild := filepath.Join(cacheHome, "go-build")
	if err := os.MkdirAll(goBuild, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goBuild, "pkg"), []byte("important build cache"), 0o644); err != nil {
		t.Fatal(err)
	}

	target := userCacheTarget()
	sz, err := target.Scan()
	if err != nil {
		t.Fatal(err)
	}
	// only junk file size should count
	if sz < int64(len("junk data here")) {
		t.Fatalf("expected scan to include junk size, got %d", sz)
	}
	// go-build content must not inflate size beyond junk (+small overhead none)
	if sz > int64(len("junk data here"))+64 {
		t.Fatalf("scan size %d looks like it included go-build", sz)
	}

	if err := target.Execute(true); err != nil {
		t.Fatalf("dry-run execute: %v", err)
	}
	// After dry-run both should still exist
	if !utils.PathExists(goBuild) {
		t.Error("go-build must remain after dry-run")
	}
	if !utils.PathExists(junk) {
		t.Error("junk must remain after dry-run")
	}

	// Non-dry-run: junk trashed, go-build kept
	if err := target.Execute(false); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !utils.PathExists(goBuild) {
		t.Error("go-build must not be deleted")
	}
	if utils.PathExists(junk) {
		t.Error("junk cache entry should have been trashed")
	}
}
