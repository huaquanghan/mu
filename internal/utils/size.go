package utils

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func DirSize(path string) (int64, error) {
	var total int64
	var walkErrors []error
	err := filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			walkErrors = append(walkErrors, err)
			return nil
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	if err != nil {
		walkErrors = append(walkErrors, err)
	}
	return total, errors.Join(walkErrors...)
}

func HumanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func PathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// HumanKB formats a kilobyte count as a human-readable string.
func HumanKB(kb uint64) string {
	const unit = 1024
	if kb < unit {
		return fmt.Sprintf("%d KB", kb)
	}
	mb := float64(kb) / unit
	if mb < unit {
		return fmt.Sprintf("%.1f MB", mb)
	}
	return fmt.Sprintf("%.1f GB", mb/unit)
}
