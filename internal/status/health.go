package status

import "math"

// HealthScore computes a 0–100 system health score.
// cpu: 0–100 (current CPU usage %)
// mem: current memory stats
// disks: current disk stats
func HealthScore(cpu float64, mem MemStats, disks []DiskStat) int {
	cpuScore := 30.0 * (1 - cpu/100)

	var ramScore float64
	if mem.TotalKB > 0 {
		ramScore = 30.0 * (float64(mem.AvailableKB) / float64(mem.TotalKB))
	}

	diskScore := 30.0 * minDiskFreePercent(disks)

	var swapScore float64
	if mem.SwapTotalKB == 0 {
		swapScore = 10.0
	} else {
		swapScore = 10.0 * (float64(mem.SwapFreeKB) / float64(mem.SwapTotalKB))
	}

	total := cpuScore + ramScore + diskScore + swapScore
	clamped := math.Max(0, math.Min(100, total))
	return int(math.Round(clamped))
}

func minDiskFreePercent(disks []DiskStat) float64 {
	if len(disks) == 0 {
		return 1.0
	}
	min := 1.0
	for _, d := range disks {
		if d.TotalBytes == 0 {
			return 0
		}
		pct := float64(d.FreeBytes) / float64(d.TotalBytes)
		if pct < min {
			min = pct
		}
	}
	return min
}
