package status

import (
	"errors"
	"strings"
	"testing"
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

func resetStatusReaders() {
	readCPUFn = ReadCPU
	readMemoryFn = ReadMemory
	readDiskFn = ReadDisk
	readNetworkFn = ReadNetwork
}
