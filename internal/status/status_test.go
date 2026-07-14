package status

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCollectSnapshotSurfacesUnavailableMetrics(t *testing.T) {
	readCPUFn = func() (CPUSample, error) { return CPUSample{}, errors.New("cpu unavailable") }
	readMemoryFn = func() (MemStats, error) { return MemStats{}, errors.New("memory unavailable") }
	readDiskFn = func() ([]DiskStat, error) { return nil, errors.New("disk unavailable") }
	readNetworkFn = func() (map[string]NetStat, error) { return nil, errors.New("network unavailable") }
	t.Cleanup(resetStatusReaders)

	snapshot := CollectSnapshot(t.Context(), 0)
	joined := strings.Join(snapshot.ScanErrors, " ")
	for _, metric := range []string{"cpu unavailable", "memory unavailable", "disk unavailable", "network unavailable"} {
		if !strings.Contains(joined, metric) {
			t.Errorf("missing %q from %v", metric, snapshot.ScanErrors)
		}
	}
	if snapshot.Health != 0 {
		t.Fatalf("missing metrics produced health %d", snapshot.Health)
	}
}

func TestModelCPURecoveryRequiresTwoFreshSamples(t *testing.T) {
	samples := []struct {
		sample CPUSample
		err    error
	}{
		{sample: CPUSample{User: 10, Idle: 90}},
		{sample: CPUSample{User: 20, Idle: 180}},
		{err: errors.New("read failed")},
		{sample: CPUSample{User: 30, Idle: 270}},
		{sample: CPUSample{User: 40, Idle: 360}},
	}
	index := 0
	readCPUFn = func() (CPUSample, error) {
		result := samples[index]
		index++
		return result.sample, result.err
	}
	readMemoryFn = func() (MemStats, error) { return MemStats{TotalKB: 100, AvailableKB: 50}, nil }
	readDiskFn = func() ([]DiskStat, error) { return []DiskStat{{Mount: "/", TotalBytes: 100, FreeBytes: 50}}, nil }
	readNetworkFn = func() (map[string]NetStat, error) { return map[string]NetStat{}, nil }
	t.Cleanup(resetStatusReaders)

	m := NewModel()
	wantReady := []bool{false, true, false, false, true}
	for i, want := range wantReady {
		updated, _ := m.Update(tickMsg(time.Unix(int64(i+1), 0)))
		m = updated.(Model)
		if m.cpuReady != want {
			t.Fatalf("sample %d cpuReady=%v want=%v", i+1, m.cpuReady, want)
		}
		if i == 2 && m.prevCPU != (CPUSample{}) {
			t.Fatal("failed CPU read retained stale previous sample")
		}
	}
}

func resetStatusReaders() {
	readCPUFn = ReadCPU
	readMemoryFn = ReadMemory
	readDiskFn = ReadDisk
	readNetworkFn = ReadNetwork
}
