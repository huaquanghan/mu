package utils

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeDelete_ProtectedPath(t *testing.T) {
	err := SafeDelete("/etc/passwd", false)
	if err == nil {
		t.Fatal("expected error for protected path, got nil")
	}
}

func TestLocateTrashUsesPerFilesystemTrashAcrossDevices(t *testing.T) {
	home := t.TempDir()
	mount := t.TempDir()
	path := filepath.Join(mount, "data.txt")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	trashHomeDir = func() (string, error) { return home, nil }
	trashDeviceID = func(candidate string) (uint64, error) {
		if candidate == home {
			return 1, nil
		}
		return 2, nil
	}
	trashMountFor = func(string) (string, error) { return mount, nil }
	t.Cleanup(resetTrashHooks)

	location, err := locateTrash(path)
	if err != nil {
		t.Fatal(err)
	}
	wantRoot := filepath.Join(mount, ".Trash-"+stringInt(os.Getuid()))
	if location.FilesDir != filepath.Join(wantRoot, "files") || location.PathInfo != "data.txt" {
		t.Fatalf("unexpected location: %+v", location)
	}
}

func TestMoveToTrashCollisionAndEncodedMetadata(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".data"))
	trashHomeDir = func() (string, error) { return home, nil }
	trashDeviceID = func(string) (uint64, error) { return 1, nil }
	t.Cleanup(resetTrashHooks)

	path := filepath.Join(home, "file name")
	if err := os.WriteFile(path, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	trashRoot := filepath.Join(home, ".data", "Trash")
	if err := os.MkdirAll(filepath.Join(trashRoot, "files"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(trashRoot, "info"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(trashRoot, "files", "file name"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	recovery, err := moveToTrash(path)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(recovery) != "file name.1" {
		t.Fatalf("collision name = %q", filepath.Base(recovery))
	}
	metadata, err := os.ReadFile(filepath.Join(trashRoot, "info", "file name.1.trashinfo"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(metadata), "Path=%2F") || !strings.Contains(string(metadata), "%20") {
		t.Fatalf("path is not percent encoded: %q", metadata)
	}
}

func TestRenameNoReplaceNeverOverwritesExistingTrashEntry(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")
	if err := os.WriteFile(source, []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("destination"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := renameNoReplace(source, destination); err == nil {
		t.Fatal("expected collision error")
	}
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != "destination" || !PathExists(source) {
		t.Fatalf("destination=%q sourceExists=%v err=%v", data, PathExists(source), err)
	}
}

func TestMoveToTrashRollsBackWhenMetadataFinalizationFails(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dataHome := filepath.Join(home, ".data")
	t.Setenv("XDG_DATA_HOME", dataHome)
	if err := os.Mkdir(dataHome, 0o700); err != nil {
		t.Fatal(err)
	}
	trashHomeDir = func() (string, error) { return dataHome, nil }
	trashDeviceID = func(string) (uint64, error) { return 1, nil }
	linkPath = func(string, string) error { return errors.New("metadata full") }
	t.Cleanup(resetTrashHooks)
	path := filepath.Join(home, "rollback")
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if recovery, err := moveToTrash(path); err == nil || recovery != "" {
		t.Fatalf("expected rollback error with no recovery path, recovery=%q err=%v", recovery, err)
	}
	if !PathExists(path) {
		t.Fatal("original was not restored")
	}
}

func TestMoveToTrashReportsRecoveryPathWhenRollbackFails(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dataHome := filepath.Join(home, ".data")
	t.Setenv("XDG_DATA_HOME", dataHome)
	if err := os.Mkdir(dataHome, 0o700); err != nil {
		t.Fatal(err)
	}
	trashHomeDir = func() (string, error) { return dataHome, nil }
	trashDeviceID = func(string) (uint64, error) { return 1, nil }
	linkPath = func(string, string) error { return errors.New("metadata full") }
	renames := 0
	renamePath = func(oldPath, newPath string) error {
		renames++
		if renames == 2 {
			return errors.New("rollback blocked")
		}
		return os.Rename(oldPath, newPath)
	}
	t.Cleanup(resetTrashHooks)
	path := filepath.Join(home, "recover")
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	recovery, err := moveToTrash(path)
	var recoveryErr *TrashRecoveryError
	if !errors.As(err, &recoveryErr) || recovery == "" || recoveryErr.RecoveryPath != recovery {
		t.Fatalf("expected recovery error, recovery=%q err=%v", recovery, err)
	}
	if !PathExists(recovery) {
		t.Fatal("recovery path does not contain moved data")
	}
}

func resetTrashHooks() {
	renamePath = renameNoReplace
	linkPath = os.Link
	removePath = os.Remove
	trashDeviceID = deviceID
	trashHomeDir = func() (string, error) {
		dataHome := XDGDataHome()
		if err := os.MkdirAll(dataHome, 0o700); err != nil {
			return "", err
		}
		return dataHome, nil
	}
	trashMountFor = func(path string) (string, error) { return mountPointForPath(path, "/proc/self/mountinfo") }
}

func stringInt(value int) string {
	return fmt.Sprintf("%d", value)
}

func TestSafeDelete_UserProtectedPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	cfgDir := filepath.Join(tmp, "mu")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	// Protect a custom path under /tmp-like area via absolute path in user config.
	// Use a unique directory that is not a system path.
	protected := filepath.Join(tmp, "keep-me")
	if err := os.MkdirAll(protected, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(protected, "data.txt")
	if err := os.WriteFile(marker, []byte("safe"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := "[protected_paths]\nsystem = [\"" + protected + "\"]\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	err := SafeDelete(marker, false)
	if err == nil {
		t.Fatal("expected error for user-whitelisted path, got nil")
	}
	if _, statErr := os.Stat(marker); os.IsNotExist(statErr) {
		t.Error("user-protected file must not be deleted")
	}
}

func TestSafeDeleteReloadsWhitelistForEveryOperation(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	candidate := filepath.Join(root, "candidate")
	if err := os.WriteFile(candidate, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SafeDelete(candidate, true); err != nil {
		t.Fatal(err)
	}
	configDir := filepath.Join(root, "mu")
	if err := os.Mkdir(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	config := fmt.Sprintf("[protected_paths]\nsystem = [%q]\n", candidate)
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SafeDelete(candidate, true); err == nil || !strings.Contains(err.Error(), "protected path") {
		t.Fatalf("expected reloaded protection rule, got %v", err)
	}
}

func TestSafeDeleteRejectsAncestorOfProtectedPathInDryRunAndRealMode(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, ".config"))
	parent := filepath.Join(root, ".cache", "app")
	protected := filepath.Join(parent, "keep")
	if err := os.MkdirAll(protected, 0o700); err != nil {
		t.Fatal(err)
	}
	configDir := filepath.Join(root, ".config", "mu")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	config := fmt.Sprintf("[protected_paths]\nsystem = [%q]\n", protected)
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, dryRun := range []bool{true, false} {
		if err := SafeDelete(parent, dryRun); err == nil || !strings.Contains(err.Error(), "protected path") {
			t.Fatalf("dryRun=%v err=%v", dryRun, err)
		}
		if !PathExists(protected) {
			t.Fatalf("dryRun=%v changed protected data", dryRun)
		}
	}
}

func TestPerFilesystemTrashFallsBackFromInvalidSharedDirectory(t *testing.T) {
	for name, setup := range map[string]func(*testing.T, string){
		"regular-file": func(t *testing.T, path string) {
			if err := os.WriteFile(path, []byte("unsafe"), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"symlink": func(t *testing.T, path string) {
			if err := os.Symlink(t.TempDir(), path); err != nil {
				t.Fatal(err)
			}
		},
		"no-sticky-bit": func(t *testing.T, path string) {
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			mount := t.TempDir()
			setup(t, filepath.Join(mount, ".Trash"))
			uid := os.Getuid()
			got, err := perFilesystemTrash(mount, uid)
			want := filepath.Join(mount, ".Trash-"+stringInt(uid))
			if err != nil || got != want {
				t.Fatalf("trash=%q want=%q err=%v", got, want, err)
			}
			if err := ensurePrivateTrashDir(got, uid); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPerFilesystemTrashRejectsInvalidPrivateDirectory(t *testing.T) {
	uid := os.Getuid()
	for name, tc := range map[string]struct {
		uid   int
		setup func(*testing.T, string)
	}{
		"regular-file": {uid: uid, setup: func(t *testing.T, path string) {
			if err := os.WriteFile(path, []byte("unsafe"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		"symlink": {uid: uid, setup: func(t *testing.T, path string) {
			if err := os.Symlink(t.TempDir(), path); err != nil {
				t.Fatal(err)
			}
		}},
		"mode-0755": {uid: uid, setup: func(t *testing.T, path string) {
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		"foreign-owner": {uid: uid + 1, setup: func(t *testing.T, path string) {
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(name, func(t *testing.T) {
			mount := t.TempDir()
			path := filepath.Join(mount, ".Trash-"+stringInt(tc.uid))
			tc.setup(t, path)
			if _, err := perFilesystemTrash(mount, tc.uid); err == nil {
				t.Fatal("expected invalid private trash rejection")
			}
		})
	}
}

func TestMoveToTrashRejectsUnsafeSubdirectoriesBeforeMovingSource(t *testing.T) {
	for _, subdir := range []string{"files", "info"} {
		for name, setup := range map[string]func(*testing.T, string){
			"regular-file": func(t *testing.T, path string) {
				if err := os.WriteFile(path, []byte("unsafe"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			"symlink": func(t *testing.T, path string) {
				if err := os.Symlink(t.TempDir(), path); err != nil {
					t.Fatal(err)
				}
			},
			"mode-0755": func(t *testing.T, path string) {
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(path, 0o755); err != nil {
					t.Fatal(err)
				}
			},
		} {
			t.Run(subdir+"/"+name, func(t *testing.T) {
				home := t.TempDir()
				mount := t.TempDir()
				source := filepath.Join(mount, "payload")
				if err := os.WriteFile(source, []byte("keep"), 0o600); err != nil {
					t.Fatal(err)
				}
				trashHomeDir = func() (string, error) { return home, nil }
				trashDeviceID = func(path string) (uint64, error) {
					if path == home {
						return 1, nil
					}
					return 2, nil
				}
				trashMountFor = func(string) (string, error) { return mount, nil }
				t.Cleanup(resetTrashHooks)

				root := filepath.Join(mount, ".Trash-"+stringInt(os.Getuid()))
				if err := os.Mkdir(root, 0o700); err != nil {
					t.Fatal(err)
				}
				if subdir == "info" {
					if err := os.Mkdir(filepath.Join(root, "files"), 0o700); err != nil {
						t.Fatal(err)
					}
				}
				setup(t, filepath.Join(root, subdir))
				if _, err := moveToTrash(source); err == nil {
					t.Fatal("expected unsafe trash directory rejection")
				}
				data, err := os.ReadFile(source)
				if err != nil || string(data) != "keep" {
					t.Fatalf("source changed: data=%q err=%v", data, err)
				}
			})
		}
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
