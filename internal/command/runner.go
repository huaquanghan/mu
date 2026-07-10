package command

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
)

// Result captures a completed command without requiring callers to inspect
// exec.ExitError directly.
type Result struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// Runner executes external commands. Implementations must honor ctx while the
// command is active.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (Result, error)
	LookPath(file string) (string, error)
}

// ExecRunner executes commands through os/exec.
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) (Result, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), ExitCode: 0}
	if err == nil {
		return result, nil
	}

	result.ExitCode = -1
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
	}
	return result, err
}

func (ExecRunner) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}
