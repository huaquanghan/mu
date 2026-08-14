package clean

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/huaquanghan/mu/internal/command"
	"github.com/huaquanghan/mu/internal/ui"
	"github.com/huaquanghan/mu/internal/utils"
)

type cleanRunnerFunc struct {
	run  func(context.Context, string, ...string) (command.Result, error)
	look func(string) (string, error)
}

func (f cleanRunnerFunc) Run(ctx context.Context, name string, args ...string) (command.Result, error) {
	return f.run(ctx, name, args...)
}

func (f cleanRunnerFunc) LookPath(file string) (string, error) {
	if f.look == nil {
		return "/usr/bin/" + file, nil
	}
	return f.look(file)
}

func TestRunAutoremoveUsesPreviewAndAPTPolicyCommand(t *testing.T) {
	var calls [][]string
	var realHadDeadline bool
	cleanRunner = cleanRunnerFunc{run: func(ctx context.Context, name string, args ...string) (command.Result, error) {
		call := append([]string{name}, args...)
		calls = append(calls, call)
		if name == "apt-get" {
			return command.Result{Stdout: []byte("Remv old-lib [1]\n10 MB disk space will be freed.\n")}, nil
		}
		_, realHadDeadline = ctx.Deadline()
		return command.Result{}, nil
	}}
	t.Cleanup(func() { cleanRunner = command.ExecRunner{} })

	if err := RunAutoremove(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if err := RunAutoremove(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 {
		t.Fatalf("calls = %v", calls)
	}
	wantPreview := []string{"apt-get", "-s", "-o", "Debug::NoLocking=1", "autoremove", "--purge"}
	wantReal := []string{"sudo", "apt-get", "autoremove", "--purge", "-y"}
	if !slices.Equal(calls[0], wantPreview) || !slices.Equal(calls[1], wantReal) {
		t.Fatalf("calls = %v", calls)
	}
	if realHadDeadline {
		t.Fatal("active APT transaction must not use a short generic deadline")
	}
}

func TestSnapTargetPropagatesRevisionFailure(t *testing.T) {
	cleanRunner = cleanRunnerFunc{run: func(_ context.Context, name string, args ...string) (command.Result, error) {
		joined := name + " " + strings.Join(args, " ")
		switch {
		case joined == "snap list --all":
			return command.Result{Stdout: []byte(snapListFixture)}, nil
		case strings.HasPrefix(joined, "sudo snap remove"):
			return command.Result{}, errors.New("snapd unavailable")
		default:
			return command.Result{}, nil
		}
	}}
	t.Cleanup(func() { cleanRunner = command.ExecRunner{} })
	target := snapTarget()
	if err := target.Execute(false); err == nil || !strings.Contains(err.Error(), "snapd unavailable") {
		t.Fatalf("expected revision failure, got %v", err)
	}
}

func TestUserCacheRejectsTopLevelSymlinkInDryRun(t *testing.T) {
	home := t.TempDir()
	cacheRoot := filepath.Join(home, ".cache")
	configRoot := filepath.Join(home, ".config")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", cacheRoot)
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	if err := os.MkdirAll(cacheRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(home, "keep")
	if err := os.WriteFile(outside, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(cacheRoot, "ambiguous")); err != nil {
		t.Fatal(err)
	}
	if err := userCacheTarget().Execute(true); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink failure, got %v", err)
	}
	if !utils.PathExists(outside) {
		t.Fatal("symlink target changed")
	}
}

func TestExecuteAggregatesPartialFailures(t *testing.T) {
	results := []scanResult{
		{target: CleanTarget{ID: "ok", Label: "ok", Execute: func(bool) error { return nil }}, size: 100},
		{target: CleanTarget{ID: "bad", Label: "bad", Execute: func(bool) error { return errors.New("blocked") }}, size: 200},
	}
	if _, err := execute(ui.NewRun(io.Discard), results, Options{DryRun: true}); err == nil || !strings.Contains(err.Error(), "1 clean target") {
		t.Fatalf("expected aggregate failure, got %v", err)
	}
}

func TestResolveTargetsRejectsUnknownAndNonOptInIDs(t *testing.T) {
	if _, err := ResolveTargets([]string{"unknown"}); err == nil {
		t.Fatal("expected unknown include rejection")
	}
	if _, err := ResolveTargets([]string{"user-cache"}); err == nil {
		t.Fatal("expected non-opt-in include rejection")
	}
}

func TestRunFailsClosedOnMalformedConfigBeforeScanning(t *testing.T) {
	home := t.TempDir()
	configRoot := filepath.Join(home, ".config")
	cacheRoot := filepath.Join(home, ".cache")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("XDG_CACHE_HOME", cacheRoot)
	if err := os.MkdirAll(filepath.Join(configRoot, "mu"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cacheRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configRoot, "mu", "config.toml"), []byte("broken = ["), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(Options{DryRun: true}); err == nil || !strings.Contains(err.Error(), "invalid mu configuration") {
		t.Fatalf("expected config error, got %v", err)
	}
}

func TestRunDryRunCompletesWithReadOnlyScanners(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	if err := os.MkdirAll(filepath.Join(home, ".cache"), 0o700); err != nil {
		t.Fatal(err)
	}
	cleanRunner = cleanRunnerFunc{
		look: func(file string) (string, error) {
			if file == "snap" {
				return "", errors.New("missing")
			}
			return "/usr/bin/" + file, nil
		},
		run: func(_ context.Context, name string, args ...string) (command.Result, error) {
			if name == "env" && slices.Equal(args, []string{"LC_ALL=C", "journalctl", "--disk-usage"}) {
				return command.Result{Stdout: []byte("Archived and active journals take up 12.0M in the file system.\n")}, nil
			}
			if name == "apt-get" {
				return command.Result{Stdout: []byte("0 upgraded, 0 newly installed, 0 to remove.\n")}, nil
			}
			return command.Result{}, nil
		},
	}
	t.Cleanup(func() { cleanRunner = command.ExecRunner{} })
	if _, err := Run(Options{DryRun: true}); err != nil {
		t.Fatal(err)
	}
}

func TestSystemTargetsUseRunnerAndPropagateErrors(t *testing.T) {
	cleanRunner = cleanRunnerFunc{run: func(_ context.Context, name string, args ...string) (command.Result, error) {
		if name == "env" && slices.Equal(args, []string{"LC_ALL=C", "journalctl", "--disk-usage"}) {
			return command.Result{Stdout: []byte("Archived and active journals take up 1.5G in the file system.\n")}, nil
		}
		return command.Result{}, errors.New("command failed")
	}}
	t.Cleanup(func() { cleanRunner = command.ExecRunner{} })
	size, err := JournalSize()
	if err != nil || size != int64(1.5*1024*1024*1024) {
		t.Fatalf("journal size=%d err=%v", size, err)
	}
	if err := aptCacheTarget().Execute(true); err != nil {
		t.Fatal(err)
	}
	if err := aptCacheTarget().Execute(false); err == nil {
		t.Fatal("expected apt clean failure")
	}
	if err := journalLogsTarget().Execute(false); err == nil {
		t.Fatal("expected journal vacuum failure")
	}
	if _, ok := TargetByID("thumbnails"); !ok {
		t.Fatal("target lookup failed")
	}
	if _, ok := TargetByID("missing"); ok {
		t.Fatal("missing target unexpectedly found")
	}
}

func TestThumbnailAndSnapDryRunPaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	thumbs := filepath.Join(home, ".cache", "thumbnails")
	if err := os.MkdirAll(thumbs, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(thumbs, "x"), []byte("123"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := thumbnailsTarget()
	if size, err := target.Scan(); err != nil || size != 3 {
		t.Fatalf("thumbnail size=%d err=%v", size, err)
	}
	if err := target.Execute(true); err != nil {
		t.Fatal(err)
	}

	cleanRunner = cleanRunnerFunc{run: func(_ context.Context, name string, args ...string) (command.Result, error) {
		if name == "snap" {
			return command.Result{Stdout: []byte(snapListFixture)}, nil
		}
		if name == "du" {
			return command.Result{Stdout: []byte("10 /snap/x/1\n")}, nil
		}
		return command.Result{}, nil
	}}
	t.Cleanup(func() { cleanRunner = command.ExecRunner{} })
	snap := snapTarget()
	if size, err := snap.Scan(); err != nil || size != 20 {
		t.Fatalf("snap size=%d err=%v", size, err)
	}
	if err := snap.Execute(true); err != nil {
		t.Fatal(err)
	}
}

func TestJournalSizeOptionalAndParseErrors(t *testing.T) {
	cleanRunner = cleanRunnerFunc{
		look: func(string) (string, error) { return "", errors.New("missing") },
		run:  func(context.Context, string, ...string) (command.Result, error) { return command.Result{}, nil },
	}
	if size, err := JournalSize(); err != nil || size != 0 {
		t.Fatalf("optional journal size=%d err=%v", size, err)
	}
	cleanRunner = cleanRunnerFunc{run: func(context.Context, string, ...string) (command.Result, error) {
		return command.Result{Stdout: []byte("unparseable")}, nil
	}}
	t.Cleanup(func() { cleanRunner = command.ExecRunner{} })
	if _, err := JournalSize(); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestJournalTargetUsesSudoOnlyForRealVacuum(t *testing.T) {
	var calls [][]string
	cleanRunner = cleanRunnerFunc{run: func(_ context.Context, name string, args ...string) (command.Result, error) {
		calls = append(calls, append([]string{name}, args...))
		return command.Result{}, nil
	}}
	t.Cleanup(func() { cleanRunner = command.ExecRunner{} })
	target := journalLogsTarget()
	if !target.RequiresSudo {
		t.Fatal("journal target must advertise sudo")
	}
	if err := target.Execute(true); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 0 {
		t.Fatalf("dry-run executed command: %v", calls)
	}
	if err := target.Execute(false); err != nil {
		t.Fatal(err)
	}
	want := []string{"sudo", "journalctl", "--vacuum-time=30d"}
	if len(calls) != 1 || !slices.Equal(calls[0], want) {
		t.Fatalf("calls=%v want=%v", calls, want)
	}
}

func TestJournalSizeForcesCLocale(t *testing.T) {
	var call []string
	cleanRunner = cleanRunnerFunc{run: func(_ context.Context, name string, args ...string) (command.Result, error) {
		call = append([]string{name}, args...)
		return command.Result{Stdout: []byte("Archived and active journals take up 2.0M in the file system.\n")}, nil
	}}
	t.Cleanup(func() { cleanRunner = command.ExecRunner{} })
	if size, err := JournalSize(); err != nil || size != 2*1024*1024 {
		t.Fatalf("size=%d err=%v", size, err)
	}
	want := []string{"env", "LC_ALL=C", "journalctl", "--disk-usage"}
	if !slices.Equal(call, want) {
		t.Fatalf("call=%v want=%v", call, want)
	}
}

func TestUserCacheScanAllowsMissingRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "missing-cache"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	if size, err := userCacheTarget().Scan(); err != nil || size != 0 {
		t.Fatalf("size=%d err=%v", size, err)
	}
}
