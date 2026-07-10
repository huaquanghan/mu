package status

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

const (
	colorCyan   = "#0097A7"
	colorGreen  = "#22C55E"
	colorAmber  = "#F59E0B"
	colorRed    = "#EF4444"
	colorEmpty  = "#374151"
	colorFaint  = "#9CA3AF"
	defaultTerm = 80
	labelWidth  = 8
	diskLabelW  = 14
)

// usageColor returns a stress color for fill percentage (higher = worse).
func usageColor(pct float64) string {
	switch {
	case pct >= 85:
		return colorRed
	case pct >= 60:
		return colorAmber
	default:
		return colorGreen
	}
}

// healthColor returns color for a 0–100 health score (higher = better).
func healthColor(score int) string {
	switch {
	case score < 30:
		return colorRed
	case score < 60:
		return colorAmber
	default:
		return colorGreen
	}
}

// healthLabel returns a short qualitative label for a health score.
func healthLabel(score int) string {
	switch {
	case score < 30:
		return "Poor"
	case score < 60:
		return "Fair"
	default:
		return "Good"
	}
}

// clampBarWidth picks a bar width from terminal width.
func clampBarWidth(termWidth int) int {
	if termWidth <= 0 {
		termWidth = defaultTerm
	}
	// left padding (2) + label (~14) + spaces + trailing values (~28)
	const fixed = 2 + diskLabelW + 4 + 28
	w := termWidth - fixed
	if w < 10 {
		return 10
	}
	if w > 40 {
		return 40
	}
	return w
}

// metricBar renders a solid usage bar at pct (0–100).
func metricBar(pct float64, width int, fullColor string) string {
	if width < 1 {
		width = 1
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := int(pct/100*float64(width) + 0.5)
	if filled > width {
		filled = width
	}
	if pct > 0 && filled == 0 {
		filled = 1
	}
	empty := width - filled
	fullStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(fullColor))
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorEmpty))
	return fullStyle.Render(strings.Repeat("█", filled)) + emptyStyle.Render(strings.Repeat("░", empty))
}

func padLabel(label string, width int) string {
	if utf8.RuneCountInString(label) >= width {
		return label
	}
	return label + strings.Repeat(" ", width-utf8.RuneCountInString(label))
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}

// renderDashboard builds the full status TUI view.
func renderDashboard(m Model) string {
	width := m.width
	if width <= 0 {
		width = defaultTerm
	}
	barW := clampBarWidth(width)

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorCyan))
	faintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorFaint))
	sectionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorFaint))

	header := titleStyle.Render("  mu status") + faintStyle.Render("  ·  live · 1s")

	// Health
	hColor := healthColor(m.health)
	healthBar := metricBar(float64(m.health), barW, hColor)
	healthText := lipgloss.NewStyle().Foreground(lipgloss.Color(hColor)).Bold(true).
		Render(fmt.Sprintf("%d/100  %s", m.health, healthLabel(m.health)))
	healthLine := fmt.Sprintf("  %s %s  %s", padLabel("Health", labelWidth), healthBar, healthText)

	// CPU
	var cpuLine string
	if !m.cpuReady {
		cpuLine = fmt.Sprintf("  %s %s", padLabel("CPU", labelWidth), faintStyle.Render("sampling…"))
	} else {
		cpuBar := metricBar(m.cpu, barW, usageColor(m.cpu))
		cpuLine = fmt.Sprintf("  %s %s  %.1f%%", padLabel("CPU", labelWidth), cpuBar, m.cpu)
	}

	// RAM
	var ramLine string
	if m.mem.TotalKB == 0 {
		ramLine = fmt.Sprintf("  %s %s", padLabel("RAM", labelWidth), faintStyle.Render("unavailable"))
	} else {
		usedKB := m.mem.TotalKB - m.mem.AvailableKB
		ramPct := float64(usedKB) / float64(m.mem.TotalKB) * 100
		ramBar := metricBar(ramPct, barW, usageColor(ramPct))
		ramLine = fmt.Sprintf("  %s %s  %s / %s  (%.0f%%)",
			padLabel("RAM", labelWidth), ramBar, humanKB(usedKB), humanKB(m.mem.TotalKB), ramPct)
	}

	// Swap
	var swapLine string
	if m.mem.SwapTotalKB == 0 {
		swapLine = fmt.Sprintf("  %s %s", padLabel("Swap", labelWidth), faintStyle.Render("none"))
	} else {
		swapUsed := m.mem.SwapTotalKB - m.mem.SwapFreeKB
		swapPct := float64(swapUsed) / float64(m.mem.SwapTotalKB) * 100
		swapBar := metricBar(swapPct, barW, usageColor(swapPct))
		swapLine = fmt.Sprintf("  %s %s  %s / %s  (%.0f%%)",
			padLabel("Swap", labelWidth), swapBar, humanKB(swapUsed), humanKB(m.mem.SwapTotalKB), swapPct)
	}

	var parts []string
	parts = append(parts, "", "", header, "", healthLine, "", cpuLine, ramLine, swapLine)

	// Disks
	if len(m.disks) > 0 {
		parts = append(parts, "", sectionStyle.Render("  Disks"))
		for _, d := range m.disks {
			usedPct := 0.0
			if d.TotalBytes > 0 {
				used := d.TotalBytes - d.FreeBytes
				usedPct = float64(used) / float64(d.TotalBytes) * 100
			}
			mount := truncateRunes(d.Mount, diskLabelW)
			bar := metricBar(usedPct, barW, usageColor(usedPct))
			parts = append(parts, fmt.Sprintf("  %s %s  used %.0f%%  ·  %s free",
				padLabel(mount, diskLabelW), bar, usedPct, humanBytes(d.FreeBytes)))
		}
	}

	// Network (active only)
	var netLines []string
	for iface, rate := range m.netRates {
		if rate.RxBytesPerSec == 0 && rate.TxBytesPerSec == 0 {
			continue
		}
		netLines = append(netLines, fmt.Sprintf("  %-10s  ↑ %-10s  ↓ %s",
			iface, humanBytes(rate.TxBytesPerSec)+"/s", humanBytes(rate.RxBytesPerSec)+"/s"))
	}
	sort.Strings(netLines)
	if len(netLines) > 0 {
		parts = append(parts, "", sectionStyle.Render("  Network"))
		parts = append(parts, netLines...)
	}

	for _, scanErr := range m.scanErrors {
		parts = append(parts, faintStyle.Render("  Unavailable: "+scanErr))
	}

	footer := faintStyle.Render("  q to quit")
	parts = append(parts, "", "", footer)
	return strings.Join(parts, "\n")
}
