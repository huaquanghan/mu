package clean

import "testing"

func TestDockerTarget_isOptIn(t *testing.T) {
	target := newDockerTarget()
	if !target.OptIn {
		t.Error("docker cache target should be opt-in")
	}
	if target.ID != "docker" {
		t.Errorf("unexpected id %q", target.ID)
	}
}
