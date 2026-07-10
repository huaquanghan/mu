package clean

import (
	"slices"
	"testing"
)

func TestParseAutoremoveSimulationUsesOnlyAPTCandidates(t *testing.T) {
	fixture := `The following packages will be REMOVED:
  linux-image-6.8.0-50-generic linux-modules-6.8.0-50-generic
Remv linux-image-6.8.0-50-generic [6.8.0-50]
Purg linux-modules-6.8.0-50-generic [6.8.0-50]
Remv obsolete-lib [1.0]
Remv obsolete-lib [1.0]
After this operation, 240 MB disk space will be freed.
`
	packages, bytes := ParseAutoremoveSimulation(fixture)
	want := []string{"linux-image-6.8.0-50-generic", "linux-modules-6.8.0-50-generic", "obsolete-lib"}
	if !slices.Equal(packages, want) {
		t.Fatalf("packages = %v, want %v", packages, want)
	}
	if slices.Contains(packages, "linux-generic") {
		t.Fatal("meta-package not selected by APT must be preserved")
	}
	if bytes != 240_000_000 {
		t.Fatalf("bytes = %d, want 240000000", bytes)
	}
}

func TestParseAutoremoveSimulationNoCandidates(t *testing.T) {
	packages, bytes := ParseAutoremoveSimulation("0 upgraded, 0 newly installed, 0 to remove.\n")
	if len(packages) != 0 || bytes != 0 {
		t.Fatalf("got packages=%v bytes=%d", packages, bytes)
	}
}

func TestParseAutoremoveSimulationDoesNotTreatAdditionalUsageAsFreed(t *testing.T) {
	_, bytes := ParseAutoremoveSimulation("After this operation, 10 MB of additional disk space will be used.\n")
	if bytes != 0 {
		t.Fatalf("bytes = %d, want 0", bytes)
	}
}
