package optimize

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStepIDs(t *testing.T) {
	ids := StepIDs()
	if len(ids) != 3 {
		t.Fatalf("expected 3 steps, got %v", ids)
	}
	want := []string{"apt", "journal", "caches"}
	for i, id := range want {
		if ids[i] != id {
			t.Errorf("StepIDs[%d]=%q want %q", i, ids[i], id)
		}
	}
}

func TestRunStep_unknown(t *testing.T) {
	_, err := RunStep("nope", Options{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error for unknown step")
	}
	if !strings.Contains(err.Error(), "unknown optimize step") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunStep_skipsConfiguredStep(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfgDir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "mu")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte("[optimize_skip]\nsteps = [\"apt\"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	skipped, err := RunStep("apt", Options{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("RunStep: %v", err)
	}
	if !skipped {
		t.Fatal("expected configured apt step to be skipped")
	}
}
