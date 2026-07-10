package status

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type tickMsg time.Time

// Snapshot is the data structure for JSON output mode.
type Snapshot struct {
	CPUPercent float64            `json:"cpu_percent"`
	Memory     MemStats           `json:"memory"`
	Disks      []DiskStat         `json:"disks"`
	Network    map[string]NetRate `json:"network"`
	Health     int                `json:"health"`
	ScanErrors []string           `json:"scan_errors,omitempty"`
}

// Model is the Bubbletea model for the live dashboard.
type Model struct {
	prevCPU    CPUSample
	cpu        float64
	cpuReady   bool
	mem        MemStats
	disks      []DiskStat
	prevNets   map[string]NetStat
	netRates   map[string]NetRate
	health     int
	prevTime   time.Time
	scanErrors []string
	width      int
}

// NewModel returns a fresh Model.
func NewModel() Model {
	return Model{width: defaultTerm}
}

// Init schedules the first tick.
func (m Model) Init() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil

	case tickMsg:
		m.scanErrors = nil
		cpuAvailable := false
		memAvailable := false
		// 1. Read CPU; compute percent from prev
		curr, err := ReadCPU()
		if err == nil {
			if m.prevCPU != (CPUSample{}) {
				m.cpu = CPUPercent(m.prevCPU, curr)
				cpuAvailable = true
				m.cpuReady = true
			}
			m.prevCPU = curr
		} else {
			m.scanErrors = append(m.scanErrors, "cpu: "+err.Error())
		}
		// 2. Read memory
		mem, memErr := ReadMemory()
		if memErr != nil {
			m.scanErrors = append(m.scanErrors, "memory: "+memErr.Error())
		} else {
			m.mem = mem
			memAvailable = true
		}
		// 3. Read disk
		disks, diskErr := ReadDisk()
		m.disks = disks
		if diskErr != nil {
			m.scanErrors = append(m.scanErrors, "disk: "+diskErr.Error())
		}
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
		} else {
			m.scanErrors = append(m.scanErrors, "network: "+err.Error())
		}
		m.prevTime = time.Time(msg)
		// 5. Compute health
		m.health = HealthScoreAvailable(m.cpu, cpuAvailable, m.mem, memAvailable, m.disks)
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
	return renderDashboard(m)
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
