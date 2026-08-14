package optimize

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/huaquanghan/mu/internal/clean"
	"github.com/huaquanghan/mu/internal/command"
)

type optimizeRunnerFunc func(context.Context, string, ...string) (command.Result, error)

func (f optimizeRunnerFunc) Run(ctx context.Context, name string, args ...string) (command.Result, error) {
	return f(ctx, name, args...)
}

func (f optimizeRunnerFunc) LookPath(file string) (string, error) { return "/usr/bin/" + file, nil }

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

func TestOptimizeModelRecordsFailedAndSuccessfulSteps(t *testing.T) {
	steps := []step{{id: "bad", desc: "bad"}, {id: "good", desc: "good"}}
	m := newOptimizeModel(steps, nil, false)
	updated, _ := m.Update(stepDoneMsg{id: "bad", status: StepFailed, err: errors.New("blocked")})
	m = updated.(optimizeModel)
	if !strings.Contains(m.View(), "❌") {
		t.Fatalf("failed step not visible: %q", m.View())
	}
	updated, _ = m.Update(stepDoneMsg{id: "good", status: StepSuccess})
	m = updated.(optimizeModel)
	if len(m.results) != 2 || m.results[0].Status != StepFailed || m.results[1].Status != StepSuccess {
		t.Fatalf("results = %+v", m.results)
	}
	if err := stepErrors(m.results); err == nil || !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("expected aggregate error, got %v", err)
	}
}

func TestOptimizeModelRecordsSkippedStep(t *testing.T) {
	m := newOptimizeModel([]step{{id: "apt", desc: "apt"}}, []string{"apt"}, false)
	msg := m.runCurrentStep()().(stepDoneMsg)
	if msg.status != StepSkipped {
		t.Fatalf("status = %s", msg.status)
	}
}

func TestOptimizeModelStopsAfterActiveStepAndSkipsRemaining(t *testing.T) {
	m := newOptimizeModel([]step{{id: "active"}, {id: "later"}, {id: "last"}}, nil, false)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = updated.(optimizeModel)
	if cmd != nil || !m.stopRequested {
		t.Fatal("stop request should wait for the active step")
	}
	updated, cmd = m.Update(stepDoneMsg{id: "active", status: StepSuccess})
	m = updated.(optimizeModel)
	if cmd == nil || !m.cancelled || len(m.results) != 3 {
		t.Fatalf("unexpected stopped state: %+v", m)
	}
	if m.results[0].Status != StepSuccess || m.results[1].Status != StepSkipped || m.results[2].Status != StepSkipped {
		t.Fatalf("step outcomes = %+v", m.results)
	}
}

func TestUpdateCachesAggregatesFailuresAndContinues(t *testing.T) {
	var calls [][]string
	optimizeRunner = optimizeRunnerFunc(func(_ context.Context, name string, args ...string) (command.Result, error) {
		calls = append(calls, append([]string{name}, args...))
		if name == "sudo" {
			return command.Result{}, errors.New("mime failed")
		}
		return command.Result{}, nil
	})
	t.Cleanup(func() { optimizeRunner = command.ExecRunner{} })
	err := updateCaches(&bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "mime failed") {
		t.Fatalf("expected cache error, got %v", err)
	}
	want := [][]string{{"sudo", "update-mime-database", "/usr/share/mime"}, {"fc-cache", "-f"}}
	if len(calls) != len(want) || !slices.Equal(calls[0], want[0]) || !slices.Equal(calls[1], want[1]) {
		t.Fatalf("independent step did not continue: %v", calls)
	}
}

func TestOptimizeRejectsUnknownSkip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := Run(Options{DryRun: true, Skip: []string{"unknown"}}); err == nil {
		t.Fatal("expected unknown skip error")
	}
}

func TestRunStepFailsClosedOnMalformedConfig(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	if err := os.MkdirAll(filepath.Join(root, "mu"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "mu", "config.toml"), []byte("broken = ["), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := RunStep("caches", Options{}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "invalid mu configuration") {
		t.Fatalf("expected fail-closed config error, got %v", err)
	}
}

func TestRunAllSkippedCompletesAndRecordsSkippedStates(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := Run(Options{AutoYes: true, Skip: []string{"apt", "journal", "caches"}}); err != nil {
		t.Fatal(err)
	}
}

func TestRunReturnsNonzeroAfterIndependentStepFailure(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	optimizeRunner = optimizeRunnerFunc(func(_ context.Context, _ string, _ ...string) (command.Result, error) {
		return command.Result{}, errors.New("cache refresh failed")
	})
	t.Cleanup(func() { optimizeRunner = command.ExecRunner{} })
	_, err := Run(Options{AutoYes: true, Skip: []string{"apt", "journal"}})
	if err == nil || !strings.Contains(err.Error(), "cache refresh failed") {
		t.Fatalf("expected optimize failure, got %v", err)
	}
}

func TestAptAndJournalStepsUseInjectedRunners(t *testing.T) {
	var calls [][]string
	optimizeRunner = optimizeRunnerFunc(func(_ context.Context, name string, args ...string) (command.Result, error) {
		calls = append(calls, append([]string{name}, args...))
		return command.Result{Stdout: []byte("ok\n")}, nil
	})
	runAutoremove = func(context.Context, bool) error {
		calls = append(calls, []string{"autoremove"})
		return nil
	}
	t.Cleanup(func() {
		optimizeRunner = command.ExecRunner{}
		runAutoremove = clean.RunAutoremove
	})
	var out bytes.Buffer
	if err := aptAutoremove(&out); err != nil {
		t.Fatal(err)
	}
	if err := journalVacuum(&out); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"sudo", "apt-get", "update"},
		{"autoremove"},
		{"sudo", "journalctl", "--vacuum-size=500M"},
	}
	if len(calls) != len(want) || !slices.Equal(calls[0], want[0]) || !slices.Equal(calls[1], want[1]) || !slices.Equal(calls[2], want[2]) {
		t.Fatalf("calls=%v", calls)
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
