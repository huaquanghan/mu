package uninstall

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/huaquanghan/mu/internal/command"
	"github.com/huaquanghan/mu/internal/utils"
)

type uninstallRunnerFunc func(context.Context, string, ...string) (command.Result, error)

func (f uninstallRunnerFunc) Run(ctx context.Context, name string, args ...string) (command.Result, error) {
	return f(ctx, name, args...)
}

func (f uninstallRunnerFunc) LookPath(file string) (string, error) { return "/usr/bin/" + file, nil }

type uninstallRunnerStub struct {
	run     uninstallRunnerFunc
	lookErr error
}

func (s uninstallRunnerStub) Run(ctx context.Context, name string, args ...string) (command.Result, error) {
	return s.run(ctx, name, args...)
}

func (s uninstallRunnerStub) LookPath(file string) (string, error) {
	if s.lookErr != nil {
		return "", s.lookErr
	}
	return "/usr/bin/" + file, nil
}

func TestRemoveSelectedRetainsSharedRemnantAfterAPTFails(t *testing.T) {
	var calls [][]string
	uninstallRunner = uninstallRunnerFunc(func(_ context.Context, name string, args ...string) (command.Result, error) {
		calls = append(calls, append([]string{name}, args...))
		if slices.Equal(args, []string{"apt-get", "purge", "-y", "shared"}) {
			return command.Result{}, errors.New("dpkg locked")
		}
		return command.Result{}, nil
	})
	t.Cleanup(func() { uninstallRunner = command.ExecRunner{} })
	shared := filepath.Join(t.TempDir(), "config", "shared")
	selected := []Package{
		{Name: "shared", Source: "apt", RemnantsFound: []string{shared}},
		{Name: "shared", Source: "snap", RemnantsFound: []string{shared}},
	}
	results := RemoveSelected(selected, selected, false)
	if len(results) != 2 || results[0].Err == nil || !results[1].Removed {
		t.Fatalf("unexpected results: %+v", results)
	}
	if !slices.Contains(results[0].RemnantsRetained, shared) || !slices.Contains(results[1].RemnantsRetained, shared) {
		t.Fatalf("shared remnant was not retained: %+v", results)
	}
	if len(calls) != 2 || !slices.Equal(calls[0], []string{"sudo", "apt-get", "purge", "-y", "shared"}) ||
		!slices.Equal(calls[1], []string{"sudo", "snap", "remove", "shared"}) {
		t.Fatalf("source-scoped calls = %v", calls)
	}
	if err := RemovalErrors(results); err == nil || !strings.Contains(err.Error(), "apt:shared") {
		t.Fatalf("expected aggregate package error, got %v", err)
	}
}

func TestRemoveSelectedDryRunRemovesOnlyManagedUniqueRemnant(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	remnant := filepath.Join(home, ".config", "solo")
	if err := os.MkdirAll(remnant, 0o700); err != nil {
		t.Fatal(err)
	}
	pkg := Package{Name: "solo", Source: "apt", RemnantsFound: []string{remnant}}
	results := RemoveSelected([]Package{pkg}, []Package{pkg}, true)
	if err := RemovalErrors(results); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(results[0].RemnantsRemoved, remnant) || !utils.PathExists(remnant) {
		t.Fatalf("dry-run result=%+v exists=%v", results[0], utils.PathExists(remnant))
	}
	if err := removePackage(Package{Name: "bad", Source: "unknown"}, false); err == nil {
		t.Fatal("expected unknown source error")
	}
}

func TestDiscoverUsesSourceSpecificCommandsAndSorts(t *testing.T) {
	uninstallRunner = uninstallRunnerStub{run: uninstallRunnerFunc(func(_ context.Context, name string, args ...string) (command.Result, error) {
		switch name {
		case "dpkg-query":
			return command.Result{Stdout: []byte("install ok installed\t2\tzeta\t1\n")}, nil
		case "snap":
			return command.Result{Stdout: []byte("Name Version Rev Tracking Publisher Notes\nalpha 2 1 stable pub -\n")}, nil
		case "du":
			return command.Result{Stdout: []byte("8 /snap/alpha/current\n")}, nil
		default:
			return command.Result{}, nil
		}
	})}
	t.Cleanup(func() { uninstallRunner = command.ExecRunner{} })
	packages, err := Discover()
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 2 || packages[0].Name != "alpha" || packages[0].InstalledKB != 8 || packages[1].Name != "zeta" {
		t.Fatalf("packages=%+v", packages)
	}
	uninstallRunner = uninstallRunnerStub{run: uninstallRunnerFunc(func(context.Context, string, ...string) (command.Result, error) {
		return command.Result{}, nil
	}), lookErr: errors.New("snap missing")}
	snaps, err := DiscoverSnap()
	if err != nil || len(snaps) != 0 {
		t.Fatalf("snaps=%v err=%v", snaps, err)
	}
}

func TestSameNameAPTAndSnapSelectionIsIsolated(t *testing.T) {
	m := newModel(Options{})
	m.query = "shared"
	updated, _ := m.Update(loadedMsg{pkgs: []Package{
		{Name: "shared", Source: "apt"},
		{Name: "shared", Source: "snap"},
	}})
	m = updated.(uninstallModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(uninstallModel)
	if !m.selected["apt:shared"] || m.selected["snap:shared"] {
		t.Fatalf("selection coupled by name: %v", m.selected)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(uninstallModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(uninstallModel)
	if !m.selected["apt:shared"] || !m.selected["snap:shared"] {
		t.Fatalf("expected independent selections: %v", m.selected)
	}
}

func TestUninstallFailsClosedOnMalformedConfig(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, ".config"))
	configDir := filepath.Join(root, ".config", "mu")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte("broken = ["), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Run(Options{DryRun: true}); err == nil || !strings.Contains(err.Error(), "invalid mu configuration") {
		t.Fatalf("expected config error, got %v", err)
	}
}
