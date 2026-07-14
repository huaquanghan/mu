package status

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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
	label = truncateWidth(label, width)
	if lipgloss.Width(label) >= width {
		return label
	}
	return label + strings.Repeat(" ", width-lipgloss.Width(label))
}

func truncateWidth(s string, max int) string {
	if max <= 0 {
		return ""
	}
	return ansi.Truncate(s, max, "…")
}

func metricLine(label string, preferredLabelWidth int, pct float64, color, wideText, primaryText string, width int) string {
	if width <= 0 {
		width = defaultTerm
	}
	labelWidth := preferredLabelWidth
	maxLabelWidth := width - 3 - lipgloss.Width(primaryText)
	if labelWidth > maxLabelWidth {
		labelWidth = maxLabelWidth
	}
	if labelWidth < 1 {
		labelWidth = 1
	}
	prefix := "  " + padLabel(label, labelWidth)
	withBar := func(text string) (string, bool) {
		barWidth := width - lipgloss.Width(prefix) - 3 - lipgloss.Width(text)
		if barWidth > 40 {
			barWidth = 40
		}
		if barWidth < 1 {
			return "", false
		}
		return prefix + " " + metricBar(pct, barWidth, color) + "  " + text, true
	}
	if line, ok := withBar(wideText); ok {
		return line
	}
	if wideText != primaryText {
		if line, ok := withBar(primaryText); ok {
			return line
		}
	}
	return truncateWidth(prefix+" "+primaryText, width)
}

// renderDashboard builds the full status TUI view.
func renderDashboard(m Model) string {
	width := m.width
	if width <= 0 {
		width = defaultTerm
	}
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorCyan))
	faintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorFaint))
	sectionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorFaint))

	header := titleStyle.Render("  mu status") + faintStyle.Render("  ·  live · 1s")

	// Health
	hColor := healthColor(m.health)
	healthText := lipgloss.NewStyle().Foreground(lipgloss.Color(hColor)).Bold(true).
		Render(fmt.Sprintf("%d/100  %s", m.health, healthLabel(m.health)))
	healthLine := metricLine("Health", labelWidth, float64(m.health), hColor, healthText, healthText, width)

	// CPU
	var cpuLine string
	if !m.cpuReady {
		cpuLine = fmt.Sprintf("  %s %s", padLabel("CPU", labelWidth), faintStyle.Render("sampling…"))
	} else {
		primary := fmt.Sprintf("%.1f%%", m.cpu)
		cpuLine = metricLine("CPU", labelWidth, m.cpu, usageColor(m.cpu), primary, primary, width)
	}

	// RAM
	var ramLine string
	if m.mem.TotalKB == 0 {
		ramLine = fmt.Sprintf("  %s %s", padLabel("RAM", labelWidth), faintStyle.Render("unavailable"))
	} else {
		usedKB := m.mem.TotalKB - m.mem.AvailableKB
		ramPct := float64(usedKB) / float64(m.mem.TotalKB) * 100
		primary := fmt.Sprintf("%.0f%%", ramPct)
		wide := fmt.Sprintf("%s / %s  (%s)", humanKB(usedKB), humanKB(m.mem.TotalKB), primary)
		ramLine = metricLine("RAM", labelWidth, ramPct, usageColor(ramPct), wide, primary, width)
	}

	// Swap
	var swapLine string
	if m.mem.SwapTotalKB == 0 {
		swapLine = fmt.Sprintf("  %s %s", padLabel("Swap", labelWidth), faintStyle.Render("none"))
	} else {
		swapUsed := m.mem.SwapTotalKB - m.mem.SwapFreeKB
		swapPct := float64(swapUsed) / float64(m.mem.SwapTotalKB) * 100
		primary := fmt.Sprintf("%.0f%%", swapPct)
		wide := fmt.Sprintf("%s / %s  (%s)", humanKB(swapUsed), humanKB(m.mem.SwapTotalKB), primary)
		swapLine = metricLine("Swap", labelWidth, swapPct, usageColor(swapPct), wide, primary, width)
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
			primary := fmt.Sprintf("used %.0f%%", usedPct)
			wide := fmt.Sprintf("%s  ·  %s free", primary, humanBytes(d.FreeBytes))
			parts = append(parts, metricLine(d.Mount, diskLabelW, usedPct, usageColor(usedPct), wide, primary, width))
		}
	}

	// Network (active only)
	var netLines []string
	for iface, rate := range m.netRates {
		if rate.RxBytesPerSec == 0 && rate.TxBytesPerSec == 0 {
			continue
		}
		netLines = append(netLines, truncateWidth(fmt.Sprintf("  %-10s  ↑ %-10s  ↓ %s",
			iface, humanBytes(rate.TxBytesPerSec)+"/s", humanBytes(rate.RxBytesPerSec)+"/s"), width))
	}
	sort.Strings(netLines)
	if len(netLines) > 0 {
		parts = append(parts, "", sectionStyle.Render("  Network"))
		parts = append(parts, netLines...)
	}

	for _, scanErr := range m.scanErrors {
		parts = append(parts, faintStyle.Render(truncateWidth("  Unavailable: "+scanErr, width)))
	}

	footer := faintStyle.Render("  q to quit")
	parts = append(parts, "", "", footer)
	for i := range parts {
		parts[i] = truncateWidth(parts[i], width)
	}
	return strings.Join(parts, "\n")
}
