package status

import (
	"encoding/json"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"
)

// Options controls status behavior.
type Options struct {
	Debug bool
}

// Run launches the live status dashboard.
// If stdout is not a TTY, prints a JSON snapshot and exits.
func Run(opts Options) error {
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		// JSON mode: collect one snapshot (with 1s CPU delta)
		s1, err := ReadCPU()
		if err != nil {
			return err
		}
		time.Sleep(time.Second)
		s2, _ := ReadCPU()
		cpu := CPUPercent(s1, s2)
		mem, _ := ReadMemory()
		disks, _ := ReadDisk()
		n1, _ := ReadNetwork()
		time.Sleep(time.Second)
		n2, _ := ReadNetwork()
		rates := NetworkRates(n1, n2, 1.0)
		health := HealthScore(cpu, mem, disks)
		snap := Snapshot{
			CPUPercent: cpu,
			Memory:     mem,
			Disks:      disks,
			Network:    rates,
			Health:     health,
		}
		return json.NewEncoder(os.Stdout).Encode(snap)
	}
	m := NewModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
