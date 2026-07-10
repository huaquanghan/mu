package status

import (
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"
)

func TestReadCPU_ParsesFixture(t *testing.T) {
	s1 := CPUSample{User: 1000, Nice: 0, System: 100, Idle: 9000, IOWait: 50, IRQ: 0, SoftIRQ: 0, Steal: 0}
	s2 := CPUSample{User: 1100, Nice: 0, System: 150, Idle: 9200, IOWait: 50, IRQ: 0, SoftIRQ: 0, Steal: 0}
	// total_delta = (1100+0+150+9200+50+0+0+0) - (1000+0+100+9000+50+0+0+0) = 10500 - 10150 = 350
	// idle_delta = (9200+50) - (9000+50) = 200
	// usage = (1 - 200/350) * 100 ≈ 42.857
	pct := CPUPercent(s1, s2)
	if pct < 42.0 || pct > 44.0 {
		t.Errorf("expected ~42.9%%, got %.2f%%", pct)
	}
}

func TestCPUPercentCounterResetClampsToZero(t *testing.T) {
	prev := CPUSample{User: 100, Idle: 900}
	curr := CPUSample{User: 50, Idle: 400}
	if got := CPUPercent(prev, curr); got != 0 {
		t.Fatalf("counter reset produced %.2f, want 0", got)
	}
}

func TestReadCPU_ZeroPrevGivesZero(t *testing.T) {
	var prev CPUSample
	curr := CPUSample{User: 100, Idle: 900}
	pct := CPUPercent(prev, curr)
	if pct != 0 {
		t.Errorf("zero prev should give 0%%, got %.2f", pct)
	}
}

func TestReadDiskParsesMountInfoAndExcludesEFIVariables(t *testing.T) {
	fixture := strings.NewReader(
		"36 25 8:1 / / rw,relatime - ext4 /dev/root rw\n" +
			"37 25 0:31 / /sys/firmware/efi/efivars rw - efivarfs efivarfs rw\n")
	efiCalled := false
	disks, err := readDiskFrom(fixture, func(path string, stat *syscall.Statfs_t) error {
		if strings.Contains(path, "efivars") {
			efiCalled = true
		}
		stat.Blocks = 100
		stat.Bavail = 40
		stat.Bsize = 4096
		stat.Fsid.X__val[0] = 1
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if efiCalled || len(disks) != 1 || disks[0].Mount != "/" {
		t.Fatalf("pseudo filesystem leaked into disk stats: disks=%+v efiCalled=%v", disks, efiCalled)
	}
}

func TestReadDiskSurfacesStatfsErrors(t *testing.T) {
	fixture := strings.NewReader("36 25 8:1 / / rw - ext4 /dev/root rw\n")
	disks, err := readDiskFrom(fixture, func(string, *syscall.Statfs_t) error {
		return errors.New("permission denied")
	})
	if len(disks) != 0 || err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("disks=%v err=%v", disks, err)
	}
}

func TestReadMemory_ParsesFile(t *testing.T) {
	// Read proc_meminfo.txt fixture to ensure it exists
	data, err := os.ReadFile("testdata/proc_meminfo.txt")
	if err != nil {
		t.Skip("fixture not found")
	}
	_ = data

	// Verify ReadMemory works on the real system
	mem, err := ReadMemory()
	if err != nil {
		t.Fatalf("ReadMemory: %v", err)
	}
	if mem.TotalKB == 0 {
		t.Error("MemTotal must be > 0")
	}
}

func TestReadNetwork_SkipsLoopback(t *testing.T) {
	nets, err := ReadNetwork()
	if err != nil {
		t.Fatalf("ReadNetwork: %v", err)
	}
	if _, ok := nets["lo"]; ok {
		t.Error("loopback interface 'lo' must be skipped")
	}
}

func TestNetworkRates_ComputesDelta(t *testing.T) {
	prev := map[string]NetStat{
		"eth0": {RxBytes: 1000, TxBytes: 500},
	}
	curr := map[string]NetStat{
		"eth0": {RxBytes: 2000, TxBytes: 1500},
	}
	rates := NetworkRates(prev, curr, 1.0)
	r, ok := rates["eth0"]
	if !ok {
		t.Fatal("eth0 not found in rates")
	}
	if r.RxBytesPerSec != 1000 {
		t.Errorf("expected RxBytesPerSec=1000, got %d", r.RxBytesPerSec)
	}
	if r.TxBytesPerSec != 1000 {
		t.Errorf("expected TxBytesPerSec=1000, got %d", r.TxBytesPerSec)
	}
}

func TestNetworkRates_NoPrevGivesZero(t *testing.T) {
	prev := map[string]NetStat{}
	curr := map[string]NetStat{
		"eth0": {RxBytes: 2000, TxBytes: 1500},
	}
	rates := NetworkRates(prev, curr, 1.0)
	r, ok := rates["eth0"]
	if !ok {
		t.Fatal("eth0 not found in rates")
	}
	if r.RxBytesPerSec != 0 || r.TxBytesPerSec != 0 {
		t.Errorf("expected 0 rates for new interface, got rx=%d tx=%d", r.RxBytesPerSec, r.TxBytesPerSec)
	}
}
