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
	LogOutcome(action, target, "success")
}

// LogOutcome records the final outcome of an operation. Callers must only log
// success after the operation completes.
func LogOutcome(action, target, outcome string) {
	if logFile == nil {
		return
	}
	fmt.Fprintf(logFile, "%s  %-12s  %-8s  %s\n", time.Now().Format(time.RFC3339), action, outcome, target)
}

func CloseLogger() {
	if logFile != nil {
		logFile.Close()
	}
}

func xdgDataHome() string {
	home, _ := os.UserHomeDir()
	return xdgHome("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
}
