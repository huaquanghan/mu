package utils

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/huaquanghan/mu/internal/command"
	"golang.org/x/sys/unix"
)

var (
	trashRunner   = command.Runner(command.ExecRunner{})
	renamePath    = renameNoReplace
	linkPath      = os.Link
	removePath    = os.Remove
	trashDeviceID = deviceID
	trashHomeDir  = func() (string, error) {
		dataHome := XDGDataHome()
		if err := os.MkdirAll(dataHome, 0o700); err != nil {
			return "", err
		}
		return dataHome, nil
	}
	trashMountFor = func(path string) (string, error) { return mountPointForPath(path, "/proc/self/mountinfo") }
)

// TrashRecoveryError reports where data can be recovered when metadata
// finalization and rollback both fail.
type TrashRecoveryError struct {
	OriginalPath string
	RecoveryPath string
	MetadataErr  error
	RollbackErr  error
}

func (e *TrashRecoveryError) Error() string {
	return fmt.Sprintf("trash metadata failed for %s; rollback failed; recover data from %s: metadata: %v; rollback: %v",
		e.OriginalPath, e.RecoveryPath, e.MetadataErr, e.RollbackErr)
}

func (e *TrashRecoveryError) Unwrap() error { return e.MetadataErr }

// SafeDelete moves path to trash (via gio or a FreeDesktop fallback).
func SafeDelete(path string, dryRun bool) error {
	wl, err := getWhitelist()
	if err != nil {
		return fmt.Errorf("invalid mu configuration: %w", err)
	}
	if IsWhitelisted(path, wl) {
		return fmt.Errorf("refused: %s is a protected path", path)
	}
	if _, err := os.Lstat(path); err != nil {
		return fmt.Errorf("inspect %s: %w", path, err)
	}
	if dryRun {
		LogOutcome("trash", path, "dry-run")
		return nil
	}

	if _, err := trashRunner.LookPath("gio"); err == nil {
		if _, runErr := trashRunner.Run(context.Background(), "gio", "trash", path); runErr == nil {
			LogOutcome("trash", path, "success")
			return nil
		}
	}
	recoveryPath, err := moveToTrash(path)
	if err != nil {
		LogOutcome("trash", path, "failure")
		if recoveryPath != "" {
			return fmt.Errorf("trash %s: %w", path, err)
		}
		return fmt.Errorf("trash %s: %w", path, err)
	}
	LogOutcome("trash", path, "success")
	return nil
}

type trashLocation struct {
	FilesDir string
	InfoDir  string
	PathInfo string
}

func moveToTrash(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	location, err := locateTrash(abs)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(location.FilesDir, 0o700); err != nil {
		return "", err
	}
	if err := os.MkdirAll(location.InfoDir, 0o700); err != nil {
		return "", err
	}

	base, tempInfo, err := reserveTrashName(location, filepath.Base(abs))
	if err != nil {
		return "", err
	}
	keepTemp := true
	defer func() {
		if keepTemp {
			_ = removePath(tempInfo)
		}
	}()

	metadata := fmt.Sprintf("[Trash Info]\nPath=%s\nDeletionDate=%s\n",
		percentEncodePath(location.PathInfo), time.Now().Format("2006-01-02T15:04:05"))
	file, err := os.OpenFile(tempInfo, os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", err
	}
	if _, err = file.WriteString(metadata); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}

	destination := filepath.Join(location.FilesDir, base)
	if err := renamePath(abs, destination); err != nil {
		return "", err
	}
	finalInfo := filepath.Join(location.InfoDir, base+".trashinfo")
	if err := linkPath(tempInfo, finalInfo); err != nil {
		if rollbackErr := renamePath(destination, abs); rollbackErr != nil {
			return destination, &TrashRecoveryError{
				OriginalPath: abs,
				RecoveryPath: destination,
				MetadataErr:  err,
				RollbackErr:  rollbackErr,
			}
		}
		return "", fmt.Errorf("finalize trash metadata: %w", err)
	}
	if err := removePath(tempInfo); err != nil {
		return destination, fmt.Errorf("remove temporary trash metadata %s: %w", tempInfo, err)
	}
	keepTemp = false
	return destination, nil
}

func locateTrash(path string) (trashLocation, error) {
	dataHome, err := trashHomeDir()
	if err != nil {
		return trashLocation{}, err
	}
	pathDevice, err := trashDeviceID(path)
	if err != nil {
		return trashLocation{}, err
	}
	homeDevice, err := trashDeviceID(dataHome)
	if err != nil {
		return trashLocation{}, err
	}
	if pathDevice == homeDevice {
		root := filepath.Join(XDGDataHome(), "Trash")
		return trashLocation{
			FilesDir: filepath.Join(root, "files"),
			InfoDir:  filepath.Join(root, "info"),
			PathInfo: path,
		}, nil
	}

	mount, err := trashMountFor(path)
	if err != nil {
		return trashLocation{}, err
	}
	root, err := perFilesystemTrash(mount, os.Getuid())
	if err != nil {
		return trashLocation{}, err
	}
	rel, err := filepath.Rel(mount, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return trashLocation{}, fmt.Errorf("path %s is outside mount %s", path, mount)
	}
	return trashLocation{
		FilesDir: filepath.Join(root, "files"),
		InfoDir:  filepath.Join(root, "info"),
		PathInfo: rel,
	}, nil
}

func deviceID(path string) (uint64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("device information unavailable for %s", path)
	}
	return uint64(stat.Dev), nil
}

func renameNoReplace(oldPath, newPath string) error {
	return unix.Renameat2(unix.AT_FDCWD, oldPath, unix.AT_FDCWD, newPath, unix.RENAME_NOREPLACE)
}

func perFilesystemTrash(mount string, uid int) (string, error) {
	shared := filepath.Join(mount, ".Trash")
	if info, err := os.Lstat(shared); err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 && info.Mode()&os.ModeSticky != 0 {
		path := filepath.Join(shared, strconv.Itoa(uid))
		if err := os.MkdirAll(path, 0o700); err == nil {
			return path, nil
		}
	}
	private := filepath.Join(mount, ".Trash-"+strconv.Itoa(uid))
	if err := os.MkdirAll(private, 0o700); err != nil {
		return "", fmt.Errorf("create filesystem trash %s: %w", private, err)
	}
	return private, nil
}

func mountPointForPath(path, mountInfoPath string) (string, error) {
	file, err := os.Open(mountInfoPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	best := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 5 {
			continue
		}
		mount := unescapeMount(fields[4])
		if path == mount || pathContains(mount, path) {
			if len(mount) > len(best) {
				best = mount
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if best == "" {
		return "", fmt.Errorf("no mount point found for %s", path)
	}
	return best, nil
}

func unescapeMount(value string) string {
	replacer := strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`)
	return replacer.Replace(value)
}

func reserveTrashName(location trashLocation, requested string) (string, string, error) {
	for attempt := 0; attempt < 10_000; attempt++ {
		base := requested
		if attempt > 0 {
			base = fmt.Sprintf("%s.%d", requested, attempt)
		}
		if _, err := os.Lstat(filepath.Join(location.FilesDir, base)); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", "", err
		}
		if _, err := os.Lstat(filepath.Join(location.InfoDir, base+".trashinfo")); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", "", err
		}
		temp := filepath.Join(location.InfoDir, base+".trashinfo.tmp")
		file, err := os.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", "", err
		}
		if err := file.Close(); err != nil {
			_ = os.Remove(temp)
			return "", "", err
		}
		return base, temp, nil
	}
	return "", "", fmt.Errorf("could not reserve a collision-free trash name for %s", requested)
}

func percentEncodePath(path string) string {
	return url.PathEscape(filepath.ToSlash(path))
}
