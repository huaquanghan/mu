package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// SafeDelete moves path to trash (via gio or FreeDesktop fallback).
// Returns an error if path is a protected system path or user-whitelisted.
// Does nothing if dryRun is true.
func SafeDelete(path string, dryRun bool) error {
	if IsWhitelisted(path, getWhitelist()) {
		return fmt.Errorf("refused: %s is a protected path", path)
	}
	if dryRun {
		LogOp("dry-trash", path)
		return nil
	}
	if _, err := exec.LookPath("gio"); err == nil {
		if err := exec.Command("gio", "trash", path).Run(); err == nil {
			LogOp("trash", path)
			return nil
		}
	}
	if err := moveToTrash(path); err != nil {
		return fmt.Errorf("trash %s: %w", path, err)
	}
	LogOp("trash", path)
	return nil
}

func moveToTrash(path string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	trashFiles := filepath.Join(home, ".local", "share", "Trash", "files")
	trashInfoDir := filepath.Join(home, ".local", "share", "Trash", "info")
	if err := os.MkdirAll(trashFiles, 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(trashInfoDir, 0o700); err != nil {
		return err
	}

	base := filepath.Base(path)
	dst := filepath.Join(trashFiles, base)
	if PathExists(dst) {
		base = fmt.Sprintf("%s.%d", base, time.Now().UnixNano())
		dst = filepath.Join(trashFiles, base)
	}

	if err := os.Rename(path, dst); err != nil {
		return err
	}

	abs, _ := filepath.Abs(path)
	trashinfo := fmt.Sprintf("[Trash Info]\nPath=%s\nDeletionDate=%s\n",
		abs, time.Now().Format("2006-01-02T15:04:05"))
	return os.WriteFile(
		filepath.Join(trashInfoDir, base+".trashinfo"),
		[]byte(trashinfo),
		0o600,
	)
}
