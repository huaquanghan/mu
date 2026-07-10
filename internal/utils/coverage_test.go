package utils

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huaquanghan/mu/internal/command"
)

type trashRunnerStub struct {
	lookErr error
	runErr  error
}

func (s trashRunnerStub) Run(context.Context, string, ...string) (command.Result, error) {
	return command.Result{}, s.runErr
}
func (s trashRunnerStub) LookPath(string) (string, error) { return "gio", s.lookErr }

func TestSafeDeleteFallbackAndMissingPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".data"))
	ResetWhitelistCacheForTest()
	t.Cleanup(ResetWhitelistCacheForTest)
	trashRunner = trashRunnerStub{lookErr: errors.New("gio missing")}
	t.Cleanup(func() { trashRunner = command.ExecRunner{} })
	path := filepath.Join(home, "trash-me")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SafeDelete(path, false); err != nil {
		t.Fatal(err)
	}
	if PathExists(path) {
		t.Fatal("fallback did not move path")
	}
	if err := SafeDelete(filepath.Join(home, "missing"), false); err == nil {
		t.Fatal("expected missing path error")
	}
}

func TestSafeDeleteFallsBackWhenGioFails(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".data"))
	ResetWhitelistCacheForTest()
	t.Cleanup(ResetWhitelistCacheForTest)
	trashRunner = trashRunnerStub{runErr: errors.New("gio failure")}
	t.Cleanup(func() { trashRunner = command.ExecRunner{} })
	path := filepath.Join(home, "fallback")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SafeDelete(path, false); err != nil {
		t.Fatal(err)
	}
}

func TestLoggerRotationAndFormattingHelpers(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", root)
	logDir := filepath.Join(root, "mu")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(logDir, "operations.log")
	large, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := large.Truncate(logMaxBytes + 1); err != nil {
		t.Fatal(err)
	}
	_ = large.Close()
	if err := InitLogger(); err != nil {
		t.Fatal(err)
	}
	LogOp("test", "target")
	CloseLogger()
	if !PathExists(logPath + ".1") {
		t.Fatal("expected rotated operation log")
	}
	data, err := os.ReadFile(logPath)
	if err != nil || !strings.Contains(string(data), "success") {
		t.Fatalf("log data=%q err=%v", data, err)
	}
	if HumanSize(1024) != "1.0 KB" || HumanKB(1024) != "1.0 MB" || HumanKB(1024*1024) != "1.0 GB" {
		t.Fatal("unexpected human size formatting")
	}
}

func TestDirSizeCountsFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a"), []byte("1234"), 0o600); err != nil {
		t.Fatal(err)
	}
	size, err := DirSize(root)
	if err != nil || size != 4 {
		t.Fatalf("size=%d err=%v", size, err)
	}
}

func TestMountPointAndSharedFilesystemTrash(t *testing.T) {
	mount := t.TempDir()
	child := filepath.Join(mount, "dir", "file")
	if err := os.MkdirAll(filepath.Dir(child), 0o700); err != nil {
		t.Fatal(err)
	}
	mountInfo := filepath.Join(t.TempDir(), "mountinfo")
	line := "1 0 8:1 / " + strings.ReplaceAll(mount, " ", `\040`) + " rw - ext4 /dev/test rw\n"
	if err := os.WriteFile(mountInfo, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := mountPointForPath(child, mountInfo)
	if err != nil || got != mount {
		t.Fatalf("mount=%q err=%v", got, err)
	}
	shared := filepath.Join(mount, ".Trash")
	if err := os.Mkdir(shared, 0o777|os.ModeSticky); err != nil {
		t.Fatal(err)
	}
	trash, err := perFilesystemTrash(mount, 1234)
	if err != nil || trash != filepath.Join(shared, "1234") {
		t.Fatalf("trash=%q err=%v", trash, err)
	}
	if _, err := deviceID(mount); err != nil {
		t.Fatal(err)
	}
}
