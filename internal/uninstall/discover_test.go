package uninstall

import "testing"

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
