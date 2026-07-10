package audit

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestValidateOptionsRejectsConflictingAndMeaninglessFlags(t *testing.T) {
	for _, options := range []Options{
		{Report: true, JSON: true},
		{Report: true, DryRun: true},
		{JSON: true, DryRun: true},
		{Include: []string{"unknown"}},
	} {
		if err := ValidateOptions(options); err == nil {
			t.Fatalf("expected options rejection: %+v", options)
		}
	}
}

func TestAuditApplyCannotReportSuccessByInterruptingActiveMaintenance(t *testing.T) {
	m := newAuditModel(Options{})
	m.phase = phaseApply
	updated, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = updated.(auditModel)
	if cmd != nil || m.done || !strings.Contains(m.status, "cannot be interrupted safely") {
		t.Fatalf("unexpected interrupt state: done=%v status=%q", m.done, m.status)
	}
	if !strings.Contains(m.View(), "waiting for completion") {
		t.Fatalf("active-operation notice missing: %q", m.View())
	}
}

func TestExitErrorCarriesReportCode(t *testing.T) {
	var err error = &ExitError{Code: 2}
	var exitCoder interface{ ExitCode() int }
	if !errors.As(err, &exitCoder) || exitCoder.ExitCode() != 2 {
		t.Fatalf("exit code mapping failed: %v", err)
	}
}

func TestReportJSONKeepsExistingFieldsAndAddsScanErrors(t *testing.T) {
	data, err := json.Marshal(Report{Health: 80, DiskFreePctRoot: 40, ReclaimableBytes: 12, ScanErrors: []string{"cpu"}})
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, field := range []string{"health", "disk_free_pct_root", "reclaimable_bytes", "findings", "recommended_commands", "scan_errors"} {
		if !strings.Contains(text, `"`+field+`"`) {
			t.Errorf("missing JSON field %q in %s", field, text)
		}
	}
}
