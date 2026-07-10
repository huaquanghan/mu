package status

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
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
		values := make([]uint64, 8)
		for i := range values {
			value, err := strconv.ParseUint(fields[i+1], 10, 64)
			if err != nil {
				return CPUSample{}, fmt.Errorf("parse /proc/stat field %d: %w", i+1, err)
			}
			values[i] = value
		}
		return CPUSample{
			User: values[0], Nice: values[1], System: values[2], Idle: values[3],
			IOWait: values[4], IRQ: values[5], SoftIRQ: values[6], Steal: values[7],
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

	if currTotal <= prevTotal {
		return 0
	}
	totalDelta := float64(currTotal - prevTotal)

	prevIdle := prev.Idle + prev.IOWait
	currIdle := curr.Idle + curr.IOWait
	if currIdle < prevIdle {
		return 0
	}
	idleDelta := float64(currIdle - prevIdle)

	percent := (1 - idleDelta/totalDelta) * 100
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return percent
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
		value, parseErr := strconv.ParseUint(fields[1], 10, 64)
		switch fields[0] {
		case "MemTotal:":
			if parseErr != nil {
				return MemStats{}, fmt.Errorf("parse MemTotal: %w", parseErr)
			}
			m.TotalKB = value
		case "MemAvailable:":
			if parseErr != nil {
				return MemStats{}, fmt.Errorf("parse MemAvailable: %w", parseErr)
			}
			m.AvailableKB = value
		case "SwapTotal:":
			if parseErr != nil {
				return MemStats{}, fmt.Errorf("parse SwapTotal: %w", parseErr)
			}
			m.SwapTotalKB = value
		case "SwapFree:":
			if parseErr != nil {
				return MemStats{}, fmt.Errorf("parse SwapFree: %w", parseErr)
			}
			m.SwapFreeKB = value
		}
	}
	if err := scanner.Err(); err != nil {
		return MemStats{}, err
	}
	if m.TotalKB == 0 || m.AvailableKB > m.TotalKB {
		return MemStats{}, fmt.Errorf("required memory metrics unavailable or invalid")
	}
	return m, nil
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
	"efivarfs": true, "tracefs": true, "configfs": true, "autofs": true,
	"rpc_pipefs": true, "nsfs": true, "fuse.portal": true, "bpf": true,
	"fuse.gvfsd-fuse": true,
}

type mountEntry struct {
	mount  string
	fstype string
}

// ReadDisk parses mountinfo and calls syscall.Statfs on each real filesystem.
// Deduplicates by device ID (Fsid).
func ReadDisk() ([]DiskStat, error) {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readDiskFrom(f, syscall.Statfs)
}

func readDiskFrom(reader io.Reader, statfs func(string, *syscall.Statfs_t) error) ([]DiskStat, error) {
	entries, err := parseMountInfo(reader)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].mount == "/" {
			return true
		}
		if entries[j].mount == "/" {
			return false
		}
		return len(entries[i].mount) < len(entries[j].mount)
	})

	type fsidKey [2]int32
	seen := make(map[fsidKey]bool)
	var result []DiskStat
	var scanErrors []error
	for _, entry := range entries {
		if skipFSTypes[entry.fstype] {
			continue
		}

		var stat syscall.Statfs_t
		if err := statfs(entry.mount, &stat); err != nil {
			scanErrors = append(scanErrors, fmt.Errorf("statfs %s: %w", entry.mount, err))
			continue
		}

		key := fsidKey{stat.Fsid.X__val[0], stat.Fsid.X__val[1]}
		if seen[key] {
			continue
		}
		if stat.Blocks == 0 || stat.Bsize <= 0 {
			continue
		}
		seen[key] = true

		result = append(result, DiskStat{
			Mount:      entry.mount,
			TotalBytes: stat.Blocks * uint64(stat.Bsize),
			FreeBytes:  stat.Bavail * uint64(stat.Bsize),
		})
	}
	return result, errors.Join(scanErrors...)
}

func parseMountInfo(reader io.Reader) ([]mountEntry, error) {
	var entries []mountEntry
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		left, right, ok := strings.Cut(scanner.Text(), " - ")
		if !ok {
			continue
		}
		leftFields := strings.Fields(left)
		rightFields := strings.Fields(right)
		if len(leftFields) < 5 || len(rightFields) < 1 {
			continue
		}
		entries = append(entries, mountEntry{mount: unescapeMountPath(leftFields[4]), fstype: rightFields[0]})
	}
	return entries, scanner.Err()
}

func unescapeMountPath(value string) string {
	return strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`).Replace(value)
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
	var parseErrors []error
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
			parseErrors = append(parseErrors, fmt.Errorf("unexpected /proc/net/dev format for %s", iface))
			continue
		}
		rxBytes, rxErr := strconv.ParseUint(fields[0], 10, 64)
		txBytes, txErr := strconv.ParseUint(fields[8], 10, 64)
		if rxErr != nil || txErr != nil {
			parseErrors = append(parseErrors, fmt.Errorf("parse network counters for %s: %w", iface, errors.Join(rxErr, txErr)))
			continue
		}
		result[iface] = NetStat{RxBytes: rxBytes, TxBytes: txBytes}
	}
	if err := scanner.Err(); err != nil {
		parseErrors = append(parseErrors, err)
	}
	return result, errors.Join(parseErrors...)
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
