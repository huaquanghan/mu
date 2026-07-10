package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// protectedPrefixes are never touched without explicit --force.
var protectedPrefixes = []string{
	"/boot", "/etc", "/usr", "/lib", "/lib64",
	"/bin", "/sbin", "/proc", "/sys", "/dev", "/run",
}

func IsProtected(path string) bool {
	clean := filepath.Clean(path)
	if clean == "/" {
		return true
	}
	for _, p := range protectedPrefixes {
		if clean == p || strings.HasPrefix(clean, p+"/") {
			return true
		}
	}
	return false
}

func XDGCacheHome() string {
	home, _ := os.UserHomeDir()
	return xdgHome("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
}

func XDGConfigHome() string {
	home, _ := os.UserHomeDir()
	return xdgHome("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
}

func XDGDataHome() string {
	return xdgDataHome()
}

func xdgHome(name, fallback string) string {
	if value := os.Getenv(name); filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	return filepath.Clean(fallback)
}

// ValidateCleanupRoot rejects roots whose contents cannot be safely treated as
// disposable cache data.
func ValidateCleanupRoot(root string) error {
	if root == "" || !filepath.IsAbs(root) {
		return fmt.Errorf("cleanup root must be absolute: %q", root)
	}
	cleanRoot := filepath.Clean(root)
	if cleanRoot == "/" || IsProtected(cleanRoot) {
		return fmt.Errorf("unsafe cleanup root: %s", cleanRoot)
	}
	if info, err := os.Lstat(cleanRoot); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("cleanup root is a symlink: %s", cleanRoot)
		}
		resolved, err := filepath.EvalSymlinks(cleanRoot)
		if err != nil {
			return fmt.Errorf("resolve cleanup root %s: %w", cleanRoot, err)
		}
		if filepath.Clean(resolved) != cleanRoot {
			return fmt.Errorf("cleanup root traverses a symlink: %s", cleanRoot)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect cleanup root %s: %w", cleanRoot, err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	cleanHome := filepath.Clean(home)
	if cleanRoot == cleanHome || pathContains(cleanRoot, cleanHome) {
		return fmt.Errorf("cleanup root must not be home or its ancestor: %s", cleanRoot)
	}
	return nil
}

// ValidateCleanupCandidate ensures candidate stays inside root and rejects a
// top-level symlink, whose target may escape the declared cleanup boundary.
func ValidateCleanupCandidate(root, candidate string) error {
	if err := ValidateCleanupRoot(root); err != nil {
		return err
	}
	if candidate == "" || !filepath.IsAbs(candidate) {
		return fmt.Errorf("cleanup candidate must be absolute: %q", candidate)
	}
	cleanRoot := filepath.Clean(root)
	cleanCandidate := filepath.Clean(candidate)
	if cleanCandidate == cleanRoot || !pathContains(cleanRoot, cleanCandidate) {
		return fmt.Errorf("cleanup candidate %s is outside root %s", cleanCandidate, cleanRoot)
	}
	info, err := os.Lstat(cleanCandidate)
	if err != nil {
		return fmt.Errorf("inspect cleanup candidate %s: %w", cleanCandidate, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("cleanup candidate is a top-level symlink: %s", cleanCandidate)
	}
	resolved, err := filepath.EvalSymlinks(cleanCandidate)
	if err != nil {
		return fmt.Errorf("resolve cleanup candidate %s: %w", cleanCandidate, err)
	}
	if filepath.Clean(resolved) != cleanCandidate {
		return fmt.Errorf("cleanup candidate traverses a symlink: %s", cleanCandidate)
	}
	return nil
}

func pathContains(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
