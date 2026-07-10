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

	diskScore := 30.0 * rootDiskFreePercent(disks)

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

func rootDiskFreePercent(disks []DiskStat) float64 {
	for _, d := range disks {
		if d.Mount == "/" {
			if d.TotalBytes == 0 {
				return 0
			}
			return float64(d.FreeBytes) / float64(d.TotalBytes)
		}
	}
	return 0
}

// HealthScoreAvailable avoids awarding healthy points for unavailable metrics.
func HealthScoreAvailable(cpu float64, cpuAvailable bool, mem MemStats, memAvailable bool, disks []DiskStat) int {
	if !cpuAvailable {
		cpu = 100
	}
	if !memAvailable {
		mem = MemStats{TotalKB: 1, AvailableKB: 0, SwapTotalKB: 1, SwapFreeKB: 0}
	}
	return HealthScore(cpu, mem, disks)
}
