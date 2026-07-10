package status

import "testing"

func TestHealthScore_MissingDiskDoesNotLookFullyHealthy(t *testing.T) {
	mem := MemStats{TotalKB: 16000000, AvailableKB: 16000000, SwapTotalKB: 0, SwapFreeKB: 0}
	score := HealthScore(0, mem, nil)
	if score != 70 {
		t.Errorf("expected 70 without a root disk metric, got %d", score)
	}
}

func TestHealthScore_MaxStress(t *testing.T) {
	mem := MemStats{TotalKB: 16000000, AvailableKB: 0, SwapTotalKB: 4000000, SwapFreeKB: 0}
	disks := []DiskStat{{Mount: "/", TotalBytes: 1000, FreeBytes: 0}}
	score := HealthScore(100, mem, disks)
	if score != 0 {
		t.Errorf("expected 0, got %d", score)
	}
}

func TestHealthScore_MidRange(t *testing.T) {
	mem := MemStats{TotalKB: 16000000, AvailableKB: 8000000, SwapTotalKB: 4000000, SwapFreeKB: 2000000}
	disks := []DiskStat{{Mount: "/", TotalBytes: 1000000, FreeBytes: 500000}}
	score := HealthScore(50, mem, disks)
	if score < 40 || score > 60 {
		t.Errorf("expected mid-range score (~50), got %d", score)
	}
}

func TestHealthScoreUsesRootFilesystemOnly(t *testing.T) {
	mem := MemStats{TotalKB: 100, AvailableKB: 100}
	disks := []DiskStat{
		{Mount: "/", TotalBytes: 100, FreeBytes: 100},
		{Mount: "/sys/firmware/efi/efivars", TotalBytes: 100, FreeBytes: 0},
	}
	if score := HealthScore(0, mem, disks); score != 100 {
		t.Fatalf("pseudo mount affected root health: %d", score)
	}
}

func TestHealthScoreAvailableDoesNotRewardMissingMetrics(t *testing.T) {
	if score := HealthScoreAvailable(0, false, MemStats{}, false, nil); score != 0 {
		t.Fatalf("missing metrics produced healthy score %d", score)
	}
}
