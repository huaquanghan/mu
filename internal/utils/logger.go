package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var logFile *os.File

const logMaxBytes = 10 * 1024 * 1024 // 10 MB

func InitLogger() error {
	if os.Getenv("MU_NO_OPLOG") == "1" {
		return nil
	}
	dir := filepath.Join(xdgDataHome(), "mu")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	logPath := filepath.Join(dir, "operations.log")
	if fi, err := os.Stat(logPath); err == nil && fi.Size() > logMaxBytes {
		_ = os.Rename(logPath, logPath+".1")
	}
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	logFile = f
	return nil
}

func LogOp(action, target string) {
	if logFile == nil {
		return
	}
	fmt.Fprintf(logFile, "%s  %-12s  %s\n", time.Now().Format(time.RFC3339), action, target)
}

func CloseLogger() {
	if logFile != nil {
		logFile.Close()
	}
}

func xdgDataHome() string {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share")
}
