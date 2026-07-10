package audit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/huaquanghan/mu/internal/clean"
	"github.com/huaquanghan/mu/internal/status"
)

const (
	bytes500MiB = 500 * 1024 * 1024
	bytes2GiB   = 2 * 1024 * 1024 * 1024
	bytes1GiB   = 1024 * 1024 * 1024
)

// TargetSize is one clean category scan result.
type TargetSize struct {
	ID           string
	Label        string
	Bytes        int64
	OptIn        bool
	RequiresSudo bool
}

// Snapshot is raw data collected before rule evaluation.
type Snapshot struct {
	Targets         []TargetSize
	Health          int
	DiskFreePctRoot float64
	MemAvailPct     float64
	SwapUsedPct     float64
	JournalBytes    int64
	AptAutoremoveN  int
	CPUPercent      float64
	ScanErrors      []string
	Warnings        []string
}

// ProgressFunc is called with a short status line during collection.
type ProgressFunc func(msg string)

// Collect gathers cleanup and health signals. progress may be nil.
func Collect(progress ProgressFunc) Snapshot {
	return CollectContext(context.Background(), progress)
}

// CollectContext gathers cleanup and health signals and honors cancellation
// between read-only scanner steps.
func CollectContext(ctx context.Context, progress ProgressFunc) Snapshot {
	report := func(msg string) {
		if progress != nil {
			progress(msg)
		}
	}

	var snap Snapshot

	report("Scanning clean categories…")
	for _, t := range clean.AllTargets() {
		if err := ctx.Err(); err != nil {
			snap.ScanErrors = append(snap.ScanErrors, err.Error())
			break
		}
		report(fmt.Sprintf("Scanning %s…", t.ID))
		sz, err := t.Scan()
		if err != nil {
			snap.ScanErrors = append(snap.ScanErrors, fmt.Sprintf("%s: %v", t.ID, err))
		}
		snap.Targets = append(snap.Targets, TargetSize{
			ID:           t.ID,
			Label:        t.Label,
			Bytes:        sz,
			OptIn:        t.OptIn,
			RequiresSudo: t.RequiresSudo,
		})
		if t.ID == "kernels" && t.Preview != nil {
			items, err := t.Preview()
			if err != nil {
				snap.ScanErrors = append(snap.ScanErrors, fmt.Sprintf("kernels preview: %v", err))
			} else {
				snap.AptAutoremoveN = len(items)
			}
		}
	}

	report("Reading health metrics…")
	cpuAvailable := false
	s1, err := status.ReadCPU()
	if err == nil {
		time.Sleep(200 * time.Millisecond)
		s2, secondErr := status.ReadCPU()
		if secondErr != nil {
			snap.ScanErrors = append(snap.ScanErrors, "cpu: "+secondErr.Error())
		} else {
			snap.CPUPercent = status.CPUPercent(s1, s2)
			cpuAvailable = true
		}
	} else {
		snap.ScanErrors = append(snap.ScanErrors, "cpu: "+err.Error())
	}
	mem, memErr := status.ReadMemory()
	if memErr != nil {
		snap.ScanErrors = append(snap.ScanErrors, "memory: "+memErr.Error())
	}
	disks, diskErr := status.ReadDisk()
	if diskErr != nil {
		snap.ScanErrors = append(snap.ScanErrors, "disk: "+diskErr.Error())
	}
	snap.Health = status.HealthScoreAvailable(snap.CPUPercent, cpuAvailable, mem, memErr == nil, disks)

	snap.DiskFreePctRoot = 0
	rootFound := false
	for _, d := range disks {
		if d.Mount == "/" && d.TotalBytes > 0 {
			snap.DiskFreePctRoot = float64(d.FreeBytes) / float64(d.TotalBytes) * 100
			rootFound = true
		}
	}
	if !rootFound {
		snap.ScanErrors = append(snap.ScanErrors, "disk: root filesystem metric unavailable")
	}
	if mem.TotalKB > 0 {
		snap.MemAvailPct = float64(mem.AvailableKB) / float64(mem.TotalKB) * 100
	}
	if mem.SwapTotalKB > 0 {
		used := mem.SwapTotalKB - mem.SwapFreeKB
		snap.SwapUsedPct = float64(used) / float64(mem.SwapTotalKB) * 100
	}

	report("Checking journal size…")
	snap.JournalBytes, err = clean.JournalSize()
	if err != nil {
		snap.ScanErrors = append(snap.ScanErrors, "journal: "+err.Error())
	}

	return snap
}

// BuildFindings applies severity rules to a snapshot.
// includePreselect lists opt-in clean IDs to default-select when present.
func BuildFindings(snap Snapshot, includePreselect []string) []Finding {
	includeSet := make(map[string]bool, len(includePreselect))
	for _, id := range includePreselect {
		includeSet[id] = true
	}

	var findings []Finding
	var reclaimable int64
	journalFindingEmitted := false

	// Disk pressure (info/banner style; may boost clean defaults)
	diskCritical := snap.DiskFreePctRoot < 10
	diskWarning := snap.DiskFreePctRoot < 20 && !diskCritical
	if diskCritical {
		findings = append(findings, Finding{
			ID:         "health:disk-root",
			Severity:   SeverityCritical,
			Title:      "Root filesystem almost full",
			Detail:     fmt.Sprintf("/ is only %.0f%% free. Free space soon to avoid system issues.", snap.DiskFreePctRoot),
			Bytes:      0,
			Action:     "none",
			Selectable: false,
		})
	} else if diskWarning {
		findings = append(findings, Finding{
			ID:         "health:disk-root",
			Severity:   SeverityWarning,
			Title:      "Root filesystem low on space",
			Detail:     fmt.Sprintf("/ is %.0f%% free. Cleaning caches and packages is recommended.", snap.DiskFreePctRoot),
			Bytes:      0,
			Action:     "none",
			Selectable: false,
		})
	}

	// Clean targets
	autoremoveFindingEmitted := false
	for _, t := range snap.Targets {
		if t.Bytes <= 0 && !(t.ID == "kernels" && snap.AptAutoremoveN > 0) {
			continue
		}
		if t.ID == "kernels" {
			autoremoveFindingEmitted = true
		}
		// Journal: single finding via clean action (dedupe optimize:journal)
		if t.ID == "journal-logs" {
			journalFindingEmitted = true
		}

		sev := SeverityInfo
		defSel := false
		if t.Bytes >= bytes2GiB {
			sev = SeverityWarning
			defSel = !t.OptIn
		} else if t.Bytes >= bytes500MiB {
			sev = SeverityInfo
			defSel = !t.OptIn
		} else {
			// small but non-zero
			defSel = false
		}

		// Disk pressure: auto-select safe (non-opt-in) reclaimable targets
		if (diskCritical || diskWarning) && !t.OptIn && t.Bytes > 0 {
			defSel = true
			if sev == SeverityInfo && t.Bytes >= bytes500MiB {
				sev = SeverityWarning
			}
		}

		if t.OptIn {
			defSel = includeSet[t.ID]
		}

		detail := fmt.Sprintf("Estimated reclaimable: space under %s.", t.Label)
		if t.RequiresSudo {
			detail += " Requires sudo."
		}
		if t.OptIn {
			detail += " Opt-in category — not selected by default."
		}

		findings = append(findings, Finding{
			ID:              "clean:" + t.ID,
			Severity:        sev,
			Title:           t.Label,
			Detail:          detail,
			Bytes:           t.Bytes,
			Action:          "clean:" + t.ID,
			OptIn:           t.OptIn,
			Selectable:      true,
			DefaultSelected: defSel,
		})
		reclaimable += t.Bytes
	}

	// Journal size if not already covered by clean target (e.g. scan returned 0 but JournalSize large)
	if !journalFindingEmitted && snap.JournalBytes >= bytes1GiB {
		findings = append(findings, Finding{
			ID:              "clean:journal-logs",
			Severity:        SeverityWarning,
			Title:           "Journal logs are large",
			Detail:          "systemd journal exceeds 1 GiB. Vacuuming frees space (keeps recent logs).",
			Bytes:           snap.JournalBytes,
			Action:          "clean:journal-logs",
			Selectable:      true,
			DefaultSelected: true,
		})
		reclaimable += snap.JournalBytes
	} else if !journalFindingEmitted && snap.JournalBytes >= bytes500MiB {
		findings = append(findings, Finding{
			ID:              "clean:journal-logs",
			Severity:        SeverityInfo,
			Title:           "Journal logs moderately large",
			Detail:          "Consider vacuuming journal to ~500M.",
			Bytes:           snap.JournalBytes,
			Action:          "clean:journal-logs",
			Selectable:      true,
			DefaultSelected: false,
		})
	}

	// Boost journal finding severity if huge
	for i := range findings {
		if findings[i].ID == "clean:journal-logs" && findings[i].Bytes >= bytes1GiB {
			findings[i].Severity = SeverityWarning
			if !findings[i].OptIn {
				findings[i].DefaultSelected = true
			}
		}
	}

	// Apt autoremove
	if snap.AptAutoremoveN > 0 && !autoremoveFindingEmitted {
		defSel := diskCritical || diskWarning
		findings = append(findings, Finding{
			ID:              "clean:kernels",
			Severity:        SeverityInfo,
			Title:           "Unused packages can be removed",
			Detail:          fmt.Sprintf("apt reports %d auto-removable package(s). Uses apt autoremove --purge policy.", snap.AptAutoremoveN),
			Bytes:           0,
			Action:          "clean:kernels",
			Selectable:      true,
			DefaultSelected: defSel,
		})
	}

	// Optional caches refresh (never default selected)
	findings = append(findings, Finding{
		ID:              "optimize:caches",
		Severity:        SeverityInfo,
		Title:           "Refresh icon/font/MIME caches",
		Detail:          "Low priority maintenance; rarely frees disk space.",
		Bytes:           0,
		Action:          "optimize:caches",
		Selectable:      true,
		DefaultSelected: false,
	})

	// Health score banner
	if snap.Health < 40 {
		findings = append(findings, Finding{
			ID:         "health:score",
			Severity:   SeverityWarning,
			Title:      fmt.Sprintf("System health score is low (%d/100)", snap.Health),
			Detail:     "Score weights CPU, RAM, disk free, and swap. Freeing disk often helps most.",
			Action:     "none",
			Selectable: false,
		})
	} else if snap.Health <= 60 {
		findings = append(findings, Finding{
			ID:         "health:score",
			Severity:   SeverityInfo,
			Title:      fmt.Sprintf("System health score is moderate (%d/100)", snap.Health),
			Detail:     "No critical health issue; cleanup may still reclaim space.",
			Action:     "none",
			Selectable: false,
		})
	}

	// RAM / swap — guide only
	if snap.MemAvailPct > 0 && snap.MemAvailPct < 10 {
		findings = append(findings, Finding{
			ID:         "health:ram",
			Severity:   SeverityInfo,
			Title:      "Low available memory",
			Detail:     "Close heavy apps. Cleaning disk does not reliably free RAM.",
			Action:     "none",
			Selectable: false,
		})
	}
	if snap.SwapUsedPct > 50 {
		findings = append(findings, Finding{
			ID:         "health:swap",
			Severity:   SeverityInfo,
			Title:      "Heavy swap usage",
			Detail:     "System is paging to disk. Freeing RAM (close apps) helps more than cache cleanup.",
			Action:     "none",
			Selectable: false,
		})
	}

	_ = reclaimable
	SortFindings(findings)
	return findings
}

// BuildReport builds a full report from a live or test snapshot.
func BuildReport(snap Snapshot, include []string) Report {
	fs := BuildFindings(snap, include)
	var reclaim int64
	for _, f := range fs {
		if f.Selectable && strings.HasPrefix(f.Action, "clean:") {
			reclaim += f.Bytes
		}
	}
	return Report{
		Health:              snap.Health,
		DiskFreePctRoot:     snap.DiskFreePctRoot,
		ReclaimableBytes:    reclaim,
		Findings:            fs,
		RecommendedCommands: RecommendedCommands(fs),
		Warnings:            append([]string(nil), snap.Warnings...),
		ScanErrors:          append([]string(nil), snap.ScanErrors...),
	}
}
