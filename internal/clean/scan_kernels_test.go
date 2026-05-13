package clean

import (
	"testing"
)

const kernelFixture = `linux-image-6.8.0-57-generic	123456
linux-image-6.8.0-50-generic	120000
linux-headers-6.8.0-57-generic	50000
linux-headers-6.8.0-50-generic	48000
linux-image-6.5.0-41-generic	115000
`

func TestParseKernelPackages_excludesRunning(t *testing.T) {
	running := "6.8.0-57-generic"
	pkgs, total := ParseKernelPackages(kernelFixture, running)

	// Running kernel packages must be excluded
	for _, p := range pkgs {
		if contains(p, running) {
			t.Errorf("running kernel package included: %s", p)
		}
	}

	// Should include old packages
	expected := []string{
		"linux-image-6.8.0-50-generic",
		"linux-headers-6.8.0-50-generic",
		"linux-image-6.5.0-41-generic",
	}
	if len(pkgs) != len(expected) {
		t.Errorf("expected %d packages, got %d: %v", len(expected), len(pkgs), pkgs)
	}

	// Size: (120000 + 48000 + 115000) * 1024
	expectedTotal := int64((120000 + 48000 + 115000) * 1024)
	if total != expectedTotal {
		t.Errorf("expected total %d, got %d", expectedTotal, total)
	}
}

func TestParseKernelPackages_emptyOutput(t *testing.T) {
	pkgs, total := ParseKernelPackages("", "6.8.0-57-generic")
	if len(pkgs) != 0 {
		t.Errorf("expected 0 packages, got %d", len(pkgs))
	}
	if total != 0 {
		t.Errorf("expected total 0, got %d", total)
	}
}

func TestParseKernelPackages_allRunning(t *testing.T) {
	fixture := "linux-image-6.8.0-57-generic\t100000\nlinux-headers-6.8.0-57-generic\t50000\n"
	pkgs, total := ParseKernelPackages(fixture, "6.8.0-57-generic")
	if len(pkgs) != 0 {
		t.Errorf("expected 0 packages, got %d: %v", len(pkgs), pkgs)
	}
	if total != 0 {
		t.Errorf("expected total 0, got %d", total)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
