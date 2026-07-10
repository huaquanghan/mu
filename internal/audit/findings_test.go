package audit

import "testing"

func TestSortFindings(t *testing.T) {
	fs := []Finding{
		{ID: "a", Severity: SeverityInfo, Bytes: 100},
		{ID: "b", Severity: SeverityCritical, Bytes: 10},
		{ID: "c", Severity: SeverityWarning, Bytes: 50},
		{ID: "d", Severity: SeverityWarning, Bytes: 200},
	}
	SortFindings(fs)
	if fs[0].ID != "b" {
		t.Fatalf("expected critical first, got %v", fs[0].ID)
	}
	if fs[1].ID != "d" {
		t.Fatalf("expected larger warning second, got %v", fs[1].ID)
	}
	if fs[2].ID != "c" {
		t.Fatalf("expected smaller warning third, got %v", fs[2].ID)
	}
}

func TestExitCodeForReport(t *testing.T) {
	if ExitCodeForReport(nil) != 0 {
		t.Fatal("empty should be 0")
	}
	if ExitCodeForReport([]Finding{{Severity: SeverityInfo}}) != 0 {
		t.Fatal("info only → 0")
	}
	if ExitCodeForReport([]Finding{{Severity: SeverityWarning}}) != 1 {
		t.Fatal("warning → 1")
	}
	if ExitCodeForReport([]Finding{{Severity: SeverityCritical}}) != 2 {
		t.Fatal("critical → 2")
	}
}

func TestParseAction(t *testing.T) {
	k, id := ParseAction("clean:user-cache")
	if k != "clean" || id != "user-cache" {
		t.Fatalf("got %s %s", k, id)
	}
	k, id = ParseAction("none")
	if k != "none" || id != "" {
		t.Fatalf("got %s %s", k, id)
	}
}

func TestMaxSeverity(t *testing.T) {
	if MaxSeverity([]Finding{
		{Severity: SeverityInfo},
		{Severity: SeverityWarning},
	}) != SeverityWarning {
		t.Fatal("expected warning")
	}
}
