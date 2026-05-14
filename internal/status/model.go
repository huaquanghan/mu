package status

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tickMsg time.Time

// Snapshot is the data structure for JSON output mode.
type Snapshot struct {
	CPUPercent float64            `json:"cpu_percent"`
	Memory     MemStats           `json:"memory"`
	Disks      []DiskStat         `json:"disks"`
	Network    map[string]NetRate `json:"network"`
	Health     int                `json:"health"`
}

// Model is the Bubbletea model for the live dashboard.
type Model struct {
	prevCPU  CPUSample
	cpu      float64
	mem      MemStats
	disks    []DiskStat
	prevNets map[string]NetStat
	nets     map[string]NetStat
	netRates map[string]NetRate
	health   int
	prevTime time.Time
	err      error
}

// NewModel returns a fresh Model.
func NewModel() Model { return Model{} }

// Init schedules the first tick.
func (m Model) Init() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		// 1. Read CPU; compute percent from prev
		curr, err := ReadCPU()
		if err == nil {
			m.cpu = CPUPercent(m.prevCPU, curr)
			m.prevCPU = curr
		} else {
			m.err = err
		}
		// 2. Read memory
		m.mem, _ = ReadMemory()
		// 3. Read disk
		m.disks, _ = ReadDisk()
		// 4. Read network; compute rates
		currNets, err := ReadNetwork()
		if err == nil {
			elapsed := time.Since(m.prevTime).Seconds()
			if elapsed <= 0 {
				elapsed = 1
			}
			if m.prevNets != nil {
				m.netRates = NetworkRates(m.prevNets, currNets, elapsed)
			}
			m.prevNets = currNets
		}
		m.prevTime = time.Time(msg)
		// 5. Compute health
		m.health = HealthScore(m.cpu, m.mem, m.disks)
		// 6. Schedule next tick
		return m, tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "Q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

// View renders the dashboard.
func (m Model) View() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#0097A7")).Render("  mu status")

	cpuLine := fmt.Sprintf("  CPU:    %.1f%%", m.cpu)

	usedKB := m.mem.TotalKB - m.mem.AvailableKB
	ramLine := fmt.Sprintf("  RAM:    %s / %s", humanKB(usedKB), humanKB(m.mem.TotalKB))

	var diskLines []string
	for _, d := range m.disks {
		freePercent := 0.0
		if d.TotalBytes > 0 {
			freePercent = float64(d.FreeBytes) / float64(d.TotalBytes) * 100
		}
		diskLines = append(diskLines, fmt.Sprintf("  Disk %-15s %s free (%.0f%%)",
			d.Mount, humanBytes(d.FreeBytes), freePercent))
	}

	var netLines []string
	for iface, rate := range m.netRates {
		if rate.RxBytesPerSec == 0 && rate.TxBytesPerSec == 0 {
			continue
		}
		netLines = append(netLines, fmt.Sprintf("  Net  %-10s ↑%s/s ↓%s/s",
			iface, humanBytes(rate.TxBytesPerSec), humanBytes(rate.RxBytesPerSec)))
	}

	healthColor := "#22C55E" // green
	if m.health < 60 {
		healthColor = "#F59E0B" // amber
	}
	if m.health < 30 {
		healthColor = "#EF4444" // red
	}
	badge := lipgloss.NewStyle().Foreground(lipgloss.Color(healthColor)).Bold(true).
		Render(fmt.Sprintf("  Health: %d/100", m.health))

	footer := lipgloss.NewStyle().Padding(0, 2).Render("q to quit")

	parts := []string{"", "", title, "", cpuLine, ramLine}
	parts = append(parts, diskLines...)
	parts = append(parts, netLines...)
	parts = append(parts, "", "", badge, "", "", "", footer)
	return strings.Join(parts, "\n")
}

func humanKB(kb uint64) string {
	if kb == 0 {
		return "0 B"
	}
	return humanBytes(kb * 1024)
}

func humanBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
