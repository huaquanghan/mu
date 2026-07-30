package clean

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/huaquanghan/mu/internal/command"
)

const snapListFixture = `Name    Version   Rev   Tracking       Publisher   Notes
core20  20230801  1974  latest/stable  canonical*  base
lxd     5.21.1    27183 latest/stable  canonical*  disabled
firefox 123.0     3440  latest/stable  mozilla*    -
vlc     3.0.20    3078  latest/stable  videolan*   disabled
`

func TestSnapScanSizesArchiveInsteadOfMountedRevision(t *testing.T) {
	cleanRunner = cleanRunnerFunc{run: func(_ context.Context, name string, args ...string) (command.Result, error) {
		switch name {
		case "snap":
			return command.Result{Stdout: []byte(snapListFixture)}, nil
		case "du":
			path := args[len(args)-1]
			if path == "/var/lib/snapd/snaps/lxd_27183.snap" {
				return command.Result{Stdout: []byte("100 " + path + "\n")}, nil
			}
			if path == "/var/lib/snapd/snaps/vlc_3078.snap" {
				return command.Result{Stdout: []byte("200 " + path + "\n")}, nil
			}
			return command.Result{}, errors.New("unexpected snap size path: " + path)
		default:
			return command.Result{}, errors.New("unexpected command: " + strings.Join(append([]string{name}, args...), " "))
		}
	}}
	t.Cleanup(func() { cleanRunner = command.ExecRunner{} })

	size, err := snapTarget().Scan()
	if err != nil {
		t.Fatal(err)
	}
	if size != 300 {
		t.Fatalf("size=%d want 300", size)
	}
}

func TestParseDisabledSnaps_basic(t *testing.T) {
	revs := parseDisabledSnaps(snapListFixture)

	if len(revs) != 2 {
		t.Fatalf("expected 2 disabled revisions, got %d: %v", len(revs), revs)
	}

	// Check lxd
	if revs[0].name != "lxd" {
		t.Errorf("expected lxd, got %s", revs[0].name)
	}
	if revs[0].revision != "27183" {
		t.Errorf("expected revision 27183, got %s", revs[0].revision)
	}

	// Check vlc
	if revs[1].name != "vlc" {
		t.Errorf("expected vlc, got %s", revs[1].name)
	}
	if revs[1].revision != "3078" {
		t.Errorf("expected revision 3078, got %s", revs[1].revision)
	}
}

func TestParseDisabledSnaps_enabled_not_included(t *testing.T) {
	revs := parseDisabledSnaps(snapListFixture)
	for _, r := range revs {
		if r.name == "core20" || r.name == "firefox" {
			t.Errorf("enabled snap %s should not be in disabled list", r.name)
		}
	}
}

func TestParseDisabledSnaps_empty(t *testing.T) {
	revs := parseDisabledSnaps("")
	if len(revs) != 0 {
		t.Errorf("expected 0 revisions for empty input, got %d", len(revs))
	}
}

func TestParseDisabledSnaps_headerOnly(t *testing.T) {
	revs := parseDisabledSnaps("Name    Version   Rev   Tracking       Publisher   Notes\n")
	if len(revs) != 0 {
		t.Errorf("expected 0 revisions for header-only input, got %d", len(revs))
	}
}
