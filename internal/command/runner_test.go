package command

import (
	"context"
	"errors"
	"testing"
)

func TestExecRunnerCapturesOutputAndExitCode(t *testing.T) {
	result, err := (ExecRunner{}).Run(context.Background(), "sh", "-c", "printf out; printf err >&2; exit 7")
	if err == nil {
		t.Fatal("expected command failure")
	}
	if string(result.Stdout) != "out" || string(result.Stderr) != "err" || result.ExitCode != 7 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestExecRunnerHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := (ExecRunner{}).Run(ctx, "sh", "-c", "sleep 10")
	if err == nil || !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("expected cancellation, result=%+v err=%v", result, err)
	}
}
