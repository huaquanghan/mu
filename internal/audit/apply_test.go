package audit

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApply_skipsOptimizeStepConfiguredInPolicy(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfgDir := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "mu")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte("[optimize_skip]\nsteps = [\"apt\"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	results := Apply([]Finding{{
		ID:         "optimize:apt",
		Title:      "Unused packages can be removed",
		Action:     "optimize:apt",
		Selectable: true,
	}}, false, false, &out)

	if len(results) != 1 {
		t.Fatalf("expected one result, got %d", len(results))
	}
	if !results[0].Skipped {
		t.Fatal("expected policy-skipped result")
	}
	if results[0].Err != nil {
		t.Fatalf("unexpected execution error: %v", results[0].Err)
	}
	if !strings.Contains(out.String(), "Skipped by optimize policy") {
		t.Fatalf("expected policy skip message, got %q", out.String())
	}
}

func TestApplyDeduplicatesIdenticalActions(t *testing.T) {
	var out bytes.Buffer
	results := Apply([]Finding{
		{ID: "first", Title: "first", Action: "unknown:same", Selectable: true},
		{ID: "second", Title: "second", Action: "unknown:same", Selectable: true},
	}, true, false, &out)
	if len(results) != 2 || results[0].Err == nil || !results[1].Skipped {
		t.Fatalf("duplicate action was not skipped: %+v", results)
	}
}
