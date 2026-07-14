package uninstall

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/huaquanghan/mu/internal/command"
)

func TestParseAPTOutput_FilterInstalled(t *testing.T) {
	fixture := "install ok installed\t1234\tcurl\t7.88.1\n" +
		"deinstall ok config-files\t0\told-pkg\t1.0\n" +
		"install ok installed\t5678\twget\t1.21.3\n"
	pkgs := parseAPTOutput(fixture)
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 installed packages, got %d", len(pkgs))
	}
	if pkgs[0].Name != "curl" || pkgs[0].InstalledKB != 1234 {
		t.Errorf("unexpected first package: %+v", pkgs[0])
	}
	if pkgs[1].Name != "wget" {
		t.Errorf("unexpected second package: %+v", pkgs[1])
	}
}

func TestParseSnapOutput_SkipsHeader(t *testing.T) {
	fixture := "Name    Version   Rev   Tracking  Publisher  Notes\n" +
		"firefox  124.0    456   latest/stable  mozilla  -\n" +
		"vlc      3.0.21   2345  latest/stable  videolan -\n"
	pkgs := parseSnapOutput(fixture)
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 snap packages, got %d", len(pkgs))
	}
	if pkgs[0].Name != "firefox" || pkgs[0].Source != "snap" {
		t.Errorf("unexpected: %+v", pkgs[0])
	}
}

func TestDiscoverReturnsPartialPackagesAndJoinedErrors(t *testing.T) {
	for name, failCommand := range map[string]string{"apt-fails": "dpkg-query", "snap-fails": "snap"} {
		t.Run(name, func(t *testing.T) {
			uninstallRunner = uninstallRunnerFunc(func(_ context.Context, commandName string, _ ...string) (command.Result, error) {
				if commandName == failCommand {
					return command.Result{}, errors.New("source unavailable")
				}
				switch commandName {
				case "dpkg-query":
					return command.Result{Stdout: []byte("install ok installed\t2\tapt-app\t1\n")}, nil
				case "snap":
					return command.Result{Stdout: []byte("Name Version Rev Tracking Publisher Notes\nsnap-app 2 1 stable pub -\n")}, nil
				}
				return command.Result{}, nil
			})
			t.Cleanup(func() { uninstallRunner = command.ExecRunner{} })
			packages, err := Discover()
			if err == nil || !strings.Contains(err.Error(), "source unavailable") || len(packages) != 1 {
				t.Fatalf("packages=%+v err=%v", packages, err)
			}
		})
	}
}

func TestDiscoverReportsBothSourceFailures(t *testing.T) {
	uninstallRunner = uninstallRunnerFunc(func(_ context.Context, name string, _ ...string) (command.Result, error) {
		return command.Result{}, errors.New(name + " unavailable")
	})
	t.Cleanup(func() { uninstallRunner = command.ExecRunner{} })
	packages, err := Discover()
	if len(packages) != 0 || err == nil || !strings.Contains(err.Error(), "dpkg-query unavailable") || !strings.Contains(err.Error(), "snap unavailable") {
		t.Fatalf("packages=%+v err=%v", packages, err)
	}
}
