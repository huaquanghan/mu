package audit

import "testing"

func TestBuildFindings_largeUserCache(t *testing.T) {
	snap := Snapshot{
		Targets: []TargetSize{
			{ID: "user-cache", Label: "User Cache", Bytes: 3 * bytes2GiB / 2, OptIn: false}, // 3 GiB
		},
		Health:          80,
		DiskFreePctRoot: 50,
	}
	fs := BuildFindings(snap, nil)
	var found *Finding
	for i := range fs {
		if fs[i].ID == "clean:user-cache" {
			found = &fs[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected clean:user-cache finding")
	}
	if found.Severity != SeverityWarning {
		t.Errorf("severity=%s want warning", found.Severity)
	}
	if !found.DefaultSelected {
		t.Error("expected default selected for large non-opt-in cache")
	}
}

func TestBuildFindings_browserOptIn(t *testing.T) {
	snap := Snapshot{
		Targets: []TargetSize{
			{ID: "browser-cache", Label: "Browser", Bytes: 2 * bytes1GiB, OptIn: true},
		},
		Health:          80,
		DiskFreePctRoot: 50,
	}
	fs := BuildFindings(snap, nil)
	var found *Finding
	for i := range fs {
		if fs[i].ID == "clean:browser-cache" {
			found = &fs[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected browser finding")
	}
	if found.DefaultSelected {
		t.Error("opt-in must not be default selected without --include")
	}

	fs2 := BuildFindings(snap, []string{"browser-cache"})
	for i := range fs2 {
		if fs2[i].ID == "clean:browser-cache" && !fs2[i].DefaultSelected {
			t.Error("expected default selected when --include=browser-cache")
		}
	}
}

func TestBuildFindings_diskCriticalSelectsClean(t *testing.T) {
	snap := Snapshot{
		Targets: []TargetSize{
			{ID: "thumbnails", Label: "Thumbs", Bytes: 100 * 1024 * 1024, OptIn: false},
		},
		Health:          30,
		DiskFreePctRoot: 8,
	}
	fs := BuildFindings(snap, nil)
	var disk, thumbs *Finding
	for i := range fs {
		switch fs[i].ID {
		case "health:disk-root":
			disk = &fs[i]
		case "clean:thumbnails":
			thumbs = &fs[i]
		}
	}
	if disk == nil || disk.Severity != SeverityCritical {
		t.Fatalf("expected critical disk finding, got %+v", disk)
	}
	if thumbs == nil || !thumbs.DefaultSelected {
		t.Fatalf("expected thumbnails selected under disk pressure, got %+v", thumbs)
	}
}

func TestBuildFindings_journalDedupe(t *testing.T) {
	snap := Snapshot{
		Targets: []TargetSize{
			{ID: "journal-logs", Label: "Journal", Bytes: 2 * bytes1GiB, OptIn: false},
		},
		JournalBytes:    2 * bytes1GiB,
		Health:          70,
		DiskFreePctRoot: 40,
	}
	fs := BuildFindings(snap, nil)
	n := 0
	for _, f := range fs {
		if f.ID == "clean:journal-logs" || f.Action == "optimize:journal" {
			n++
			if f.Action == "optimize:journal" {
				t.Error("should not emit optimize:journal when clean:journal-logs exists")
			}
		}
	}
	if n != 1 {
		t.Fatalf("expected exactly 1 journal finding, got %d", n)
	}
}

func TestBuildFindings_emptyHealthy(t *testing.T) {
	snap := Snapshot{
		Health:          90,
		DiskFreePctRoot: 60,
		Targets:         nil,
	}
	fs := BuildFindings(snap, nil)
	// may still have optimize:caches info
	for _, f := range fs {
		if f.DefaultSelected {
			t.Errorf("nothing should be default selected on healthy empty system: %s", f.ID)
		}
	}
}

func TestBuildReport_reclaimable(t *testing.T) {
	snap := Snapshot{
		Targets: []TargetSize{
			{ID: "user-cache", Label: "Cache", Bytes: bytes500MiB, OptIn: false},
		},
		Health:          75,
		DiskFreePctRoot: 40,
	}
	rep := BuildReport(snap, nil)
	if rep.ReclaimableBytes < bytes500MiB {
		t.Errorf("reclaimable=%d want >= %d", rep.ReclaimableBytes, bytes500MiB)
	}
	if len(rep.RecommendedCommands) == 0 {
		t.Error("expected recommended commands")
	}
}
