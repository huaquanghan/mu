package utils

import "testing"

func TestIsProtected_SystemPaths(t *testing.T) {
	cases := []struct {
		path      string
		protected bool
	}{
		{"/", true},
		{"/boot", true},
		{"/boot/grub/grub.cfg", true},
		{"/etc", true},
		{"/etc/ssh/sshd_config", true},
		{"/usr", true},
		{"/usr/bin/ls", true},
		{"/proc/1/cmdline", true},
		{"/sys/class/net", true},
		{"/tmp/mu-test", false},
		{"/home/user/.cache/foo", false},
		{"/var/cache/apt", false},
		{"/run/docker.sock", true},
	}
	for _, c := range cases {
		got := IsProtected(c.path)
		if got != c.protected {
			t.Errorf("IsProtected(%q) = %v, want %v", c.path, got, c.protected)
		}
	}
}
