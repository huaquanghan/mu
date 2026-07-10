package audit

import (
	"sort"
	"strings"
)

// Severity ranks how urgent a finding is.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityWarning  Severity = "warning"
	SeverityInfo     Severity = "info"
)

// severityRank for sorting (higher = more urgent).
func severityRank(s Severity) int {
	switch s {
	case SeverityCritical:
		return 3
	case SeverityWarning:
		return 2
	case SeverityInfo:
		return 1
	default:
		return 0
	}
}

// Finding is one audit result with optional remediation action.
type Finding struct {
	ID              string   `json:"id"`
	Severity        Severity `json:"severity"`
	Title           string   `json:"title"`
	Detail          string   `json:"detail"`
	Bytes           int64    `json:"bytes"`
	Action          string   `json:"action"` // clean:<id> | optimize:<id> | none
	OptIn           bool     `json:"opt_in"`
	Selectable      bool     `json:"selectable"`
	DefaultSelected bool     `json:"default_selected"`
}

// Report is the full audit snapshot for --report / --json.
type Report struct {
	Health              int       `json:"health"`
	DiskFreePctRoot     float64   `json:"disk_free_pct_root"`
	ReclaimableBytes    int64     `json:"reclaimable_bytes"`
	Findings            []Finding `json:"findings"`
	RecommendedCommands []string  `json:"recommended_commands"`
	Warnings            []string  `json:"warnings,omitempty"`
	ScanErrors          []string  `json:"scan_errors,omitempty"`
}

// SortFindings orders by severity, then bytes descending, then selectable actions first.
func SortFindings(fs []Finding) {
	sort.SliceStable(fs, func(i, j int) bool {
		ri, rj := severityRank(fs[i].Severity), severityRank(fs[j].Severity)
		if ri != rj {
			return ri > rj
		}
		if fs[i].Bytes != fs[j].Bytes {
			return fs[i].Bytes > fs[j].Bytes
		}
		ai := fs[i].Action != "none" && fs[i].Action != ""
		aj := fs[j].Action != "none" && fs[j].Action != ""
		if ai != aj {
			return ai
		}
		return fs[i].ID < fs[j].ID
	})
}

// MaxSeverity returns the highest severity among findings, or empty if none.
func MaxSeverity(fs []Finding) Severity {
	var max Severity
	maxR := 0
	for _, f := range fs {
		if r := severityRank(f.Severity); r > maxR {
			maxR = r
			max = f.Severity
		}
	}
	return max
}

// ExitCodeForReport: 0 = clean/info only, 1 = warning, 2 = critical.
func ExitCodeForReport(fs []Finding) int {
	switch MaxSeverity(fs) {
	case SeverityCritical:
		return 2
	case SeverityWarning:
		return 1
	default:
		return 0
	}
}

// ParseAction splits "clean:user-cache" into kind and id.
func ParseAction(action string) (kind, id string) {
	if action == "" || action == "none" {
		return "none", ""
	}
	kind, id, ok := strings.Cut(action, ":")
	if !ok {
		return action, ""
	}
	return kind, id
}

// RecommendedCommands builds suggested CLI commands from findings.
func RecommendedCommands(fs []Finding) []string {
	var cmds []string
	var cleanIDs []string
	needOptimizeApt := false
	for _, f := range fs {
		kind, id := ParseAction(f.Action)
		switch kind {
		case "clean":
			if id != "" {
				cleanIDs = append(cleanIDs, id)
			}
		case "optimize":
			if id == "apt" {
				needOptimizeApt = true
			}
		}
	}
	if len(cleanIDs) > 0 {
		cmds = append(cmds, "mu clean --dry-run")
		// only suggest --include for opt-in findings that are recommended (default selected)
		var include []string
		for _, f := range fs {
			if f.OptIn && f.DefaultSelected && f.Action != "" && f.Action != "none" {
				_, id := ParseAction(f.Action)
				if id != "" {
					include = append(include, id)
				}
			}
		}
		if len(include) > 0 {
			cmds = append(cmds, "mu clean --include="+strings.Join(unique(include), ","))
		} else {
			cmds = append(cmds, "mu clean")
		}
	}
	if needOptimizeApt {
		cmds = append(cmds, "mu optimize --dry-run")
		cmds = append(cmds, "mu optimize")
	}
	if len(cmds) == 0 {
		cmds = append(cmds, "mu status")
	}
	return cmds
}

func unique(in []string) []string {
	seen := make(map[string]bool, len(in))
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
