package status

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// CPUSample holds raw /proc/stat ticks.
type CPUSample struct {
	User, Nice, System, Idle, IOWait, IRQ, SoftIRQ, Steal uint64
}

// ReadCPU reads the first "cpu" line from /proc/stat.
func ReadCPU() (CPUSample, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return CPUSample{}, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		// fields[0] = "cpu", fields[1..] = user nice system idle iowait irq softirq steal guest guest_nice
		if len(fields) < 9 {
			return CPUSample{}, fmt.Errorf("unexpected /proc/stat format: %q", line)
		}
		parse := func(idx int) uint64 {
			v, _ := strconv.ParseUint(fields[idx], 10, 64)
			return v
		}
		return CPUSample{
			User:    parse(1),
			Nice:    parse(2),
			System:  parse(3),
			Idle:    parse(4),
			IOWait:  parse(5),
			IRQ:     parse(6),
			SoftIRQ: parse(7),
			Steal:   parse(8),
		}, nil
	}
	return CPUSample{}, fmt.Errorf("no cpu line found in /proc/stat")
}

// CPUPercent computes CPU usage % between two samples.
// Returns 0 if delta_total == 0 or if prev is zero-value (first sample).
func CPUPercent(prev, curr CPUSample) float64 {
	prevTotal := prev.User + prev.Nice + prev.System + prev.Idle + prev.IOWait + prev.IRQ + prev.SoftIRQ + prev.Steal
	currTotal := curr.User + curr.Nice + curr.System + curr.Idle + curr.IOWait + curr.IRQ + curr.SoftIRQ + curr.Steal

	// Zero prevTotal means this is the first sample; no delta available.
	if prevTotal == 0 {
		return 0
	}

	totalDelta := float64(currTotal - prevTotal)
	if totalDelta == 0 {
		return 0
	}

	prevIdle := prev.Idle + prev.IOWait
	currIdle := curr.Idle + curr.IOWait
	idleDelta := float64(currIdle - prevIdle)

	return (1 - idleDelta/totalDelta) * 100
}

// MemStats holds memory info.
type MemStats struct {
	TotalKB     uint64
	AvailableKB uint64
	SwapTotalKB uint64
	SwapFreeKB  uint64
}

// ReadMemory parses /proc/meminfo.
func ReadMemory() (MemStats, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return MemStats{}, err
	}
	defer f.Close()

	var m MemStats
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		val, _ := strconv.ParseUint(fields[1], 10, 64)
		switch fields[0] {
		case "MemTotal:":
			m.TotalKB = val
		case "MemAvailable:":
			m.AvailableKB = val
		case "SwapTotal:":
			m.SwapTotalKB = val
		case "SwapFree:":
			m.SwapFreeKB = val
		}
	}
	return m, scanner.Err()
}

// DiskStat holds per-mount disk info.
type DiskStat struct {
	Mount      string
	TotalBytes uint64
	FreeBytes  uint64
}

var skipFSTypes = map[string]bool{
	"tmpfs": true, "proc": true, "sysfs": true, "devtmpfs": true,
	"cgroup": true, "cgroup2": true, "debugfs": true, "squashfs": true,
	"devpts": true, "hugetlbfs": true, "mqueue": true, "pstore": true,
	"securityfs": true, "fusectl": true, "binfmt_misc": true,
}

// ReadDisk iterates /proc/mounts and calls syscall.Statfs on each.
// Deduplicates by device ID (Fsid).
func ReadDisk() ([]DiskStat, error) {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	type fsidKey [2]int32
	seen := make(map[fsidKey]bool)
	var result []DiskStat

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		mount := fields[1]
		fstype := fields[2]

		if skipFSTypes[fstype] {
			continue
		}

		var stat syscall.Statfs_t
		if err := syscall.Statfs(mount, &stat); err != nil {
			continue
		}

		key := fsidKey{stat.Fsid.X__val[0], stat.Fsid.X__val[1]}
		if seen[key] {
			continue
		}
		seen[key] = true

		result = append(result, DiskStat{
			Mount:      mount,
			TotalBytes: stat.Blocks * uint64(stat.Bsize),
			FreeBytes:  stat.Bavail * uint64(stat.Bsize),
		})
	}
	return result, scanner.Err()
}

// NetStat holds cumulative network bytes for one interface.
type NetStat struct {
	RxBytes uint64
	TxBytes uint64
}

// NetRate holds per-second byte rates for one interface.
type NetRate struct {
	RxBytesPerSec uint64
	TxBytesPerSec uint64
}

// ReadNetwork parses /proc/net/dev.
// Returns map[interfaceName]NetStat, skips "lo".
func ReadNetwork() (map[string]NetStat, error) {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	result := make(map[string]NetStat)
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if lineNum <= 2 {
			// Skip two header lines
			continue
		}
		line := scanner.Text()
		// Format: "  eth0: N N N N N N N N N N N N N N N N"
		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}
		iface := strings.TrimSpace(line[:colonIdx])
		if iface == "lo" {
			continue
		}
		rest := strings.TrimSpace(line[colonIdx+1:])
		fields := strings.Fields(rest)
		if len(fields) < 10 {
			continue
		}
		rxBytes, _ := strconv.ParseUint(fields[0], 10, 64)
		txBytes, _ := strconv.ParseUint(fields[8], 10, 64)
		result[iface] = NetStat{RxBytes: rxBytes, TxBytes: txBytes}
	}
	return result, scanner.Err()
}

// NetworkRates computes per-second rates from two NetStat snapshots.
// Returns 0 for interfaces with no previous reading.
func NetworkRates(prev, curr map[string]NetStat, elapsedSec float64) map[string]NetRate {
	if elapsedSec <= 0 {
		elapsedSec = 1
	}
	rates := make(map[string]NetRate)
	for iface, c := range curr {
		p, ok := prev[iface]
		if !ok {
			rates[iface] = NetRate{}
			continue
		}
		var rxDelta, txDelta uint64
		if c.RxBytes >= p.RxBytes {
			rxDelta = c.RxBytes - p.RxBytes
		}
		if c.TxBytes >= p.TxBytes {
			txDelta = c.TxBytes - p.TxBytes
		}
		rates[iface] = NetRate{
			RxBytesPerSec: uint64(float64(rxDelta) / elapsedSec),
			TxBytesPerSec: uint64(float64(txDelta) / elapsedSec),
		}
	}
	return rates
}
