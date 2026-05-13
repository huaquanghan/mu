package status

import "testing"

func TestHealthScore_FullHealth(t *testing.T) {
	mem := MemStats{TotalKB: 16000000, AvailableKB: 16000000, SwapTotalKB: 0, SwapFreeKB: 0}
	score := HealthScore(0, mem, nil)
	if score != 100 {
		t.Errorf("expected 100, got %d", score)
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
