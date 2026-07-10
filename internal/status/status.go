package status

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"
)

var (
	readCPUFn     = ReadCPU
	readMemoryFn  = ReadMemory
	readDiskFn    = ReadDisk
	readNetworkFn = ReadNetwork
)

// Options controls status behavior.
type Options struct {
	Debug bool
	JSON  bool // force JSON output regardless of TTY
}

// Run launches the live status dashboard.
// Outputs JSON if --json is set or stdout is not a TTY.
func Run(opts Options) error {
	if opts.JSON || !isatty.IsTerminal(os.Stdout.Fd()) {
		snap := CollectSnapshot(context.Background(), time.Second)
		return json.NewEncoder(os.Stdout).Encode(snap)
	}
	m := NewModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// CollectSnapshot gathers one scriptable status sample and reports every
// unavailable metric in ScanErrors.
func CollectSnapshot(ctx context.Context, interval time.Duration) Snapshot {
	var snap Snapshot
	cpuAvailable := false
	s1, err := readCPUFn()
	if err != nil {
		snap.ScanErrors = append(snap.ScanErrors, "cpu: "+err.Error())
	} else if err := waitContext(ctx, interval); err != nil {
		snap.ScanErrors = append(snap.ScanErrors, "cpu: "+err.Error())
	} else if s2, secondErr := readCPUFn(); secondErr != nil {
		snap.ScanErrors = append(snap.ScanErrors, "cpu: "+secondErr.Error())
	} else {
		snap.CPUPercent = CPUPercent(s1, s2)
		cpuAvailable = true
	}

	memAvailable := false
	if mem, memErr := readMemoryFn(); memErr != nil {
		snap.ScanErrors = append(snap.ScanErrors, "memory: "+memErr.Error())
	} else {
		snap.Memory = mem
		memAvailable = true
	}
	if disks, diskErr := readDiskFn(); diskErr != nil {
		snap.Disks = disks
		snap.ScanErrors = append(snap.ScanErrors, "disk: "+diskErr.Error())
	} else {
		snap.Disks = disks
	}

	n1, netErr := readNetworkFn()
	if netErr != nil {
		snap.ScanErrors = append(snap.ScanErrors, "network: "+netErr.Error())
	} else if err := waitContext(ctx, interval); err != nil {
		snap.ScanErrors = append(snap.ScanErrors, "network: "+err.Error())
	} else if n2, secondErr := readNetworkFn(); secondErr != nil {
		snap.ScanErrors = append(snap.ScanErrors, "network: "+secondErr.Error())
	} else {
		seconds := interval.Seconds()
		if seconds <= 0 {
			seconds = 1
		}
		snap.Network = NetworkRates(n1, n2, seconds)
	}
	snap.Health = HealthScoreAvailable(snap.CPUPercent, cpuAvailable, snap.Memory, memAvailable, snap.Disks)
	return snap
}

func waitContext(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		return nil
	}
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("metric collection canceled: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}
