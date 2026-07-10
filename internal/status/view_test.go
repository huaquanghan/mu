package status

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestUsageColorThresholds(t *testing.T) {
	cases := []struct {
		pct  float64
		want string
	}{
		{0, colorGreen},
		{59.9, colorGreen},
		{60, colorAmber},
		{84.9, colorAmber},
		{85, colorRed},
		{100, colorRed},
	}
	for _, tc := range cases {
		if got := usageColor(tc.pct); got != tc.want {
			t.Errorf("usageColor(%.1f) = %s, want %s", tc.pct, got, tc.want)
		}
	}
}

func TestHealthColorAndLabel(t *testing.T) {
	if healthColor(100) != colorGreen || healthLabel(100) != "Good" {
		t.Fatalf("healthy score: color=%s label=%s", healthColor(100), healthLabel(100))
	}
	if healthColor(45) != colorAmber || healthLabel(45) != "Fair" {
		t.Fatalf("mid score: color=%s label=%s", healthColor(45), healthLabel(45))
	}
	if healthColor(10) != colorRed || healthLabel(10) != "Poor" {
		t.Fatalf("low score: color=%s label=%s", healthColor(10), healthLabel(10))
	}
}

func TestMetricBarWidthAndFill(t *testing.T) {
	bar := metricBar(50, 20, colorGreen)
	// Strip ANSI: count █ and ░ runes via strings after lipgloss may wrap SGR.
	// Full bar string contains exactly 20 block characters when SGR codes stripped.
	plain := stripANSI(bar)
	if utf8.RuneCountInString(plain) != 20 {
		t.Fatalf("bar rune count = %d, want 20; plain=%q", utf8.RuneCountInString(plain), plain)
	}
	full := strings.Count(plain, "█")
	empty := strings.Count(plain, "░")
	if full != 10 || empty != 10 {
		t.Fatalf("50%% of 20: full=%d empty=%d", full, empty)
	}

	tiny := stripANSI(metricBar(1, 20, colorGreen))
	if strings.Count(tiny, "█") != 1 {
		t.Fatalf("nonzero tiny fill should be at least 1, got %q", tiny)
	}

	zero := stripANSI(metricBar(0, 10, colorGreen))
	if strings.Count(zero, "█") != 0 || strings.Count(zero, "░") != 10 {
		t.Fatalf("0%% bar unexpected: %q", zero)
	}
}

func TestClampBarWidth(t *testing.T) {
	if got := clampBarWidth(0); got < 10 || got > 40 {
		t.Fatalf("default width out of range: %d", got)
	}
	if got := clampBarWidth(20); got != 10 {
		t.Fatalf("narrow terminal: got %d want 10", got)
	}
	if got := clampBarWidth(200); got != 40 {
		t.Fatalf("wide terminal: got %d want 40", got)
	}
}

func TestRenderDashboardSamplingAndSwapNone(t *testing.T) {
	m := Model{
		width:    80,
		cpuReady: false,
		health:   80,
		mem:      MemStats{TotalKB: 16 * 1024 * 1024, AvailableKB: 8 * 1024 * 1024},
	}
	view := renderDashboard(m)
	if !strings.Contains(view, "sampling…") {
		t.Fatalf("expected sampling state, got:\n%s", view)
	}
	if !strings.Contains(view, "Swap") || !strings.Contains(view, "none") {
		t.Fatalf("expected swap none, got:\n%s", view)
	}
	if !strings.Contains(view, "Good") {
		t.Fatalf("expected health label Good, got:\n%s", view)
	}
	if !strings.HasPrefix(view, "\n\n") {
		t.Fatal("view must start with top padding blank lines")
	}
	if !strings.Contains(view, "q to quit") {
		t.Fatal("missing quit hint")
	}
}

func TestRenderDashboardCPUReadyAndDisks(t *testing.T) {
	m := Model{
		width:    100,
		cpuReady: true,
		cpu:      42.5,
		health:   50,
		mem: MemStats{
			TotalKB:     8 * 1024 * 1024,
			AvailableKB: 2 * 1024 * 1024,
			SwapTotalKB: 2 * 1024 * 1024,
			SwapFreeKB:  1 * 1024 * 1024,
		},
		disks: []DiskStat{
			{Mount: "/", TotalBytes: 1000, FreeBytes: 400},
			{Mount: "/very/long/mount/path/name", TotalBytes: 2000, FreeBytes: 1000},
		},
		netRates: map[string]NetRate{
			"eth0": {RxBytesPerSec: 1024, TxBytesPerSec: 512},
			"wlan0": {},
		},
		scanErrors: []string{"disk: permission denied"},
	}
	view := renderDashboard(m)
	for _, want := range []string{
		"42.5%",
		"Fair",
		"Disks",
		"used 60%",
		"Network",
		"eth0",
		"Unavailable: disk: permission denied",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q in view:\n%s", want, view)
		}
	}
	if strings.Contains(view, "wlan0") {
		t.Fatal("zero-rate iface should be hidden")
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("hello", 10); got != "hello" {
		t.Fatalf("short: %q", got)
	}
	if got := truncateRunes("abcdefghij", 5); utf8.RuneCountInString(got) != 5 {
		t.Fatalf("truncated length: %q", got)
	}
	if !strings.HasSuffix(truncateRunes("abcdefghij", 5), "…") {
		t.Fatalf("expected ellipsis: %q", truncateRunes("abcdefghij", 5))
	}
}

// stripANSI removes common SGR sequences for length assertions.
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j < len(s) {
				i = j + 1
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}
