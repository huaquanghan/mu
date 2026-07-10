package clean

import (
	"context"
	"errors"
	"testing"

	"github.com/huaquanghan/mu/internal/command"
)

func TestDockerTarget_isOptIn(t *testing.T) {
	target := newDockerTarget()
	if !target.OptIn {
		t.Error("docker cache target should be opt-in")
	}
	if target.ID != "docker" {
		t.Errorf("unexpected id %q", target.ID)
	}
}

func TestParseDockerBuildCacheSizes(t *testing.T) {
	fixture := "not-json\n" +
		`{"Type":"Images","ReclaimableSize":"9GB"}` + "\n" +
		`{"Type":"Build Cache","ReclaimableSize":"1.5GB"}` + "\n"
	if got := parseDockerBuildCacheSize(fixture); got != int64(1.5*1024*1024*1024) {
		t.Fatalf("size=%d", got)
	}
	for input, want := range map[string]int64{"512MB": 512 * 1024 * 1024, "2KB": 2048, "7B": 7, "0B": 0, "bad": 0} {
		if got := parseDockerSize(input); got != want {
			t.Errorf("parseDockerSize(%q)=%d want %d", input, got, want)
		}
	}
}

func TestDockerTargetPropagatesScanAndExecuteFailures(t *testing.T) {
	failed := false
	cleanRunner = cleanRunnerFunc{run: func(_ context.Context, _ string, args ...string) (command.Result, error) {
		if len(args) > 0 && args[0] == "system" {
			return command.Result{Stdout: []byte(`{"Type":"Build Cache","ReclaimableSize":"2MB"}`)}, nil
		}
		if failed {
			return command.Result{}, errors.New("docker failed")
		}
		return command.Result{}, nil
	}}
	t.Cleanup(func() { cleanRunner = command.ExecRunner{} })
	target := newDockerTarget()
	size, err := target.Scan()
	if err != nil || size != 2*1024*1024 {
		t.Fatalf("size=%d err=%v", size, err)
	}
	if err := target.Execute(true); err != nil {
		t.Fatal(err)
	}
	failed = true
	if err := target.Execute(false); err == nil {
		t.Fatal("expected prune failure")
	}
}
