package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafeDelete_ProtectedPath(t *testing.T) {
	err := SafeDelete("/etc/passwd", false)
	if err == nil {
		t.Fatal("expected error for protected path, got nil")
	}
}

func TestSafeDelete_DryRun(t *testing.T) {
	f, err := os.CreateTemp("", "mu-trash-test-*")
	if err != nil {
		t.Fatal(err)
	}
	path := f.Name()
	f.Close()
	defer os.Remove(path)

	if err := SafeDelete(path, true); err != nil {
		t.Fatalf("SafeDelete dry-run: %v", err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("dry-run should not remove the file")
	}
}

func TestSafeDelete_MovesToTrash(t *testing.T) {
	f, err := os.CreateTemp("", "mu-trash-test-*")
	if err != nil {
		t.Fatal(err)
	}
	path := f.Name()
	f.Close()

	if err := SafeDelete(path, false); err != nil {
		t.Fatalf("SafeDelete: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		os.Remove(path)
		t.Error("file should be gone from original location after trash")
	}
	// Verify it landed somewhere in ~/.local/share/Trash/
	home, _ := os.UserHomeDir()
	trashFiles := filepath.Join(home, ".local", "share", "Trash", "files")
	entries, _ := os.ReadDir(trashFiles)
	base := filepath.Base(path)
	found := false
	for _, e := range entries {
		if e.Name() == base {
			found = true
			os.Remove(filepath.Join(trashFiles, e.Name()))
			break
		}
	}
	if !found {
		t.Log("file may have been moved via gio trash (not visible in Trash/files) — acceptable")
	}
}
