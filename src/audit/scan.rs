//! Snapshot collection and findings rules — port of `internal/audit/scan.go`.
//!
//! `Collect` gathers clean-target sizes and health metrics into a raw
//! [`Snapshot`]; `BuildFindings` applies the severity rules and
//! `BuildReport` wraps everything into the `--report`/`--json` schema.
//!
//! Go's `CollectContext` cancellation (a `ctx.Err()` check between target
//! scans) is dropped — Ctrl-C kills the CLI process outright, and the only
//! observable difference was an extra scan_error entry that cannot occur
//! without a context to cancel.

use std::time::Duration;

use super::Deps;
use super::findings::{Finding, Report, Severity, recommended_commands, sort_findings};
use crate::clean;
use crate::status::{self, MemStats};

/// `bytes500MiB`.
pub const BYTES_500_MIB: i64 = 500 * 1024 * 1024;
/// `bytes2GiB`.
pub const BYTES_2_GIB: i64 = 2 * 1024 * 1024 * 1024;
/// `bytes1GiB`.
pub const BYTES_1_GIB: i64 = 1024 * 1024 * 1024;

/// Gap between the two CPU samples (Go `time.Sleep(200 * time.Millisecond)`).
const CPU_SAMPLE_GAP: Duration = Duration::from_millis(200);

/// `TargetSize` — one clean category scan result.
#[derive(Debug, Default)]
pub struct TargetSize {
    pub id: &'static str,
    pub label: &'static str,
    pub bytes: i64,
    pub opt_in: bool,
    pub requires_sudo: bool,
}

/// `Snapshot` — raw data collected before rule evaluation.
#[derive(Debug, Default)]
pub struct Snapshot {
    pub targets: Vec<TargetSize>,
    pub health: i64,
    pub disk_free_pct_root: f64,
    pub mem_avail_pct: f64,
    pub swap_used_pct: f64,
    pub journal_bytes: i64,
    pub apt_autoremove_n: usize,
    pub cpu_percent: f64,
    pub scan_errors: Vec<String>,
    pub warnings: Vec<String>,
    /// `rootDiskAvailable` — unexported in Go: callers cannot fabricate the
    /// root-disk metric being present (it is what gates the critical
    /// disk-pressure finding).
    pub(crate) root_disk_available: bool,
}

/// `Collect` gathers cleanup and health signals. `progress` may be `None`
/// (Go's nil `ProgressFunc`).
pub fn collect(progress: Option<&dyn Fn(&str)>) -> Snapshot {
    collect_in(&mut Deps::real(), progress)
}

/// Injectable `CollectContext` — the readers/targets Go resolves through
/// `internal/status` and `internal/clean` package functions come from `deps`
/// so tests can substitute fakes without touching the process environment.
pub(crate) fn collect_in(deps: &mut Deps, progress: Option<&dyn Fn(&str)>) -> Snapshot {
    let report = |msg: &str| {
        if let Some(p) = progress {
            p(msg);
        }
    };

    let mut snap = Snapshot::default();

    report("Scanning clean categories…");
    for t in clean::all_targets_in(&deps.clean) {
        report(&format!("Scanning {}…", t.id));
        // Go records `Bytes: sz` even on scan error — the partial size.
        let (bytes, scan_err) = (t.scan)();
        if let Some(e) = scan_err {
            snap.scan_errors.push(format!("{}: {}", t.id, e));
        }
        let preview = if t.id == "kernels" {
            t.preview.as_ref()
        } else {
            None
        };
        snap.targets.push(TargetSize {
            id: t.id,
            label: t.label,
            bytes,
            opt_in: t.opt_in,
            requires_sudo: t.requires_sudo,
        });
        if let Some(preview) = preview {
            let (items, preview_err) = preview();
            match preview_err {
                Some(e) => snap.scan_errors.push(format!("kernels preview: {e}")),
                None => snap.apt_autoremove_n = items.len(),
            }
        }
    }

    report("Reading health metrics…");
    let mut cpu_available = false;
    match (deps.readers.cpu)() {
        Ok(s1) => {
            std::thread::sleep(CPU_SAMPLE_GAP);
            match (deps.readers.cpu)() {
                Err(second_err) => snap.scan_errors.push(format!("cpu: {second_err}")),
                Ok(s2) => {
                    snap.cpu_percent = status::cpu_percent(s1, s2);
                    cpu_available = true;
                }
            }
        }
        Err(e) => snap.scan_errors.push(format!("cpu: {e}")),
    }
    let (mem, mem_available) = match (deps.readers.memory)() {
        Err(mem_err) => {
            snap.scan_errors.push(format!("memory: {mem_err}"));
            (MemStats::default(), false)
        }
        Ok(mem) => (mem, true),
    };
    let (disks, disk_err) = (deps.readers.disk)();
    if let Some(disk_err) = disk_err {
        snap.scan_errors.push(format!("disk: {disk_err}"));
    }
    snap.health =
        status::health_score_available(snap.cpu_percent, cpu_available, mem, mem_available, &disks);

    snap.disk_free_pct_root = 0.0;
    let mut root_found = false;
    for d in &disks {
        if d.mount == "/" && d.total_bytes > 0 {
            snap.disk_free_pct_root = d.free_bytes as f64 / d.total_bytes as f64 * 100.0;
            snap.root_disk_available = true;
            root_found = true;
        }
    }
    if !root_found {
        snap.scan_errors
            .push("disk: root filesystem metric unavailable".to_string());
    }
    if mem.total_kb > 0 {
        snap.mem_avail_pct = mem.available_kb as f64 / mem.total_kb as f64 * 100.0;
    }
    if mem.swap_total_kb > 0 {
        let used = mem.swap_total_kb - mem.swap_free_kb;
        snap.swap_used_pct = used as f64 / mem.swap_total_kb as f64 * 100.0;
    }

    report("Checking journal size…");
    match clean::journal_size(deps.clean.runner.as_ref()) {
        Ok(bytes) => snap.journal_bytes = bytes,
        Err(e) => snap.scan_errors.push(format!("journal: {e}")),
    }

    snap
}

/// `BuildFindings` applies severity rules to a snapshot.
/// `include_preselect` lists opt-in clean IDs to default-select when present.
pub fn build_findings(snap: &Snapshot, include_preselect: &[String]) -> Vec<Finding> {
    let include_set: std::collections::HashSet<&str> =
        include_preselect.iter().map(String::as_str).collect();

    let mut findings: Vec<Finding> = Vec::new();
    let mut reclaimable: i64 = 0;
    let mut journal_finding_emitted = false;

    // Disk pressure (info/banner style; may boost clean defaults).
    let disk_critical = snap.root_disk_available && snap.disk_free_pct_root < 10.0;
    let disk_warning = snap.root_disk_available && snap.disk_free_pct_root < 20.0 && !disk_critical;
    if disk_critical {
        findings.push(Finding {
            id: "health:disk-root".to_string(),
            severity: Severity::Critical,
            title: "Root filesystem almost full".to_string(),
            detail: format!(
                "/ is only {:.0}% free. Free space soon to avoid system issues.",
                snap.disk_free_pct_root
            ),
            bytes: 0,
            action: "none".to_string(),
            selectable: false,
            ..Finding::default()
        });
    } else if disk_warning {
        findings.push(Finding {
            id: "health:disk-root".to_string(),
            severity: Severity::Warning,
            title: "Root filesystem low on space".to_string(),
            detail: format!(
                "/ is {:.0}% free. Cleaning caches and packages is recommended.",
                snap.disk_free_pct_root
            ),
            bytes: 0,
            action: "none".to_string(),
            selectable: false,
            ..Finding::default()
        });
    }

    // Clean targets.
    let mut autoremove_finding_emitted = false;
    for t in &snap.targets {
        if t.bytes <= 0 && !(t.id == "kernels" && snap.apt_autoremove_n > 0) {
            continue;
        }
        if t.id == "kernels" {
            autoremove_finding_emitted = true;
        }
        // Journal: single finding via clean action (dedupe optimize:journal).
        if t.id == "journal-logs" {
            journal_finding_emitted = true;
        }

        let mut sev = Severity::Info;
        let mut def_sel;
        if t.bytes >= BYTES_2_GIB {
            sev = Severity::Warning;
            def_sel = !t.opt_in;
        } else if t.bytes >= BYTES_500_MIB {
            sev = Severity::Info;
            def_sel = !t.opt_in;
        } else {
            // small but non-zero
            def_sel = false;
        }

        // Disk pressure: auto-select safe (non-opt-in) reclaimable targets.
        if (disk_critical || disk_warning) && !t.opt_in && t.bytes > 0 {
            def_sel = true;
            if sev == Severity::Info && t.bytes >= BYTES_500_MIB {
                sev = Severity::Warning;
            }
        }

        if t.opt_in {
            def_sel = include_set.contains(t.id);
        }

        let mut detail = format!("Estimated reclaimable: space under {}.", t.label);
        if t.requires_sudo {
            detail.push_str(" Requires sudo.");
        }
        if t.opt_in {
            detail.push_str(" Opt-in category — not selected by default.");
        }

        findings.push(Finding {
            id: format!("clean:{}", t.id),
            severity: sev,
            title: t.label.to_string(),
            detail,
            bytes: t.bytes,
            action: format!("clean:{}", t.id),
            opt_in: t.opt_in,
            selectable: true,
            default_selected: def_sel,
        });
        reclaimable += t.bytes;
    }

    // Journal size if not already covered by clean target (e.g. scan
    // returned 0 but JournalSize large).
    if !journal_finding_emitted && snap.journal_bytes >= BYTES_1_GIB {
        findings.push(Finding {
            id: "clean:journal-logs".to_string(),
            severity: Severity::Warning,
            title: "Journal logs are large".to_string(),
            detail: "systemd journal exceeds 1 GiB. Vacuuming frees space (keeps recent \
                     logs."
                .to_string(),
            bytes: snap.journal_bytes,
            action: "clean:journal-logs".to_string(),
            selectable: true,
            default_selected: true,
            ..Finding::default()
        });
        reclaimable += snap.journal_bytes;
    } else if !journal_finding_emitted && snap.journal_bytes >= BYTES_500_MIB {
        findings.push(Finding {
            id: "clean:journal-logs".to_string(),
            severity: Severity::Info,
            title: "Journal logs moderately large".to_string(),
            detail: "Consider vacuuming journal to ~500M.".to_string(),
            bytes: snap.journal_bytes,
            action: "clean:journal-logs".to_string(),
            selectable: true,
            default_selected: false,
            ..Finding::default()
        });
    }

    // Boost journal finding severity if huge.
    for f in &mut findings {
        if f.id == "clean:journal-logs" && f.bytes >= BYTES_1_GIB {
            f.severity = Severity::Warning;
            if !f.opt_in {
                f.default_selected = true;
            }
        }
    }

    // Apt autoremove.
    if snap.apt_autoremove_n > 0 && !autoremove_finding_emitted {
        let def_sel = disk_critical || disk_warning;
        findings.push(Finding {
            id: "clean:kernels".to_string(),
            severity: Severity::Info,
            title: "Unused packages can be removed".to_string(),
            detail: format!(
                "apt reports {} auto-removable package(s). Uses apt autoremove --purge policy.",
                snap.apt_autoremove_n
            ),
            bytes: 0,
            action: "clean:kernels".to_string(),
            selectable: true,
            default_selected: def_sel,
            ..Finding::default()
        });
    }

    // Optional caches refresh (never default selected).
    findings.push(Finding {
        id: "optimize:caches".to_string(),
        severity: Severity::Info,
        title: "Refresh icon/font/MIME caches".to_string(),
        detail: "Low priority maintenance; rarely frees disk space.".to_string(),
        bytes: 0,
        action: "optimize:caches".to_string(),
        selectable: true,
        default_selected: false,
        ..Finding::default()
    });

    // Health score banner.
    if snap.health < 40 {
        findings.push(Finding {
            id: "health:score".to_string(),
            severity: Severity::Warning,
            title: format!("System health score is low ({}/100)", snap.health),
            detail: "Score weights CPU, RAM, disk free, and swap. Freeing disk often helps \
                     most."
                .to_string(),
            action: "none".to_string(),
            selectable: false,
            ..Finding::default()
        });
    } else if snap.health <= 60 {
        findings.push(Finding {
            id: "health:score".to_string(),
            severity: Severity::Info,
            title: format!("System health score is moderate ({}/100)", snap.health),
            detail: "No critical health issue; cleanup may still reclaim space.".to_string(),
            action: "none".to_string(),
            selectable: false,
            ..Finding::default()
        });
    }

    // RAM / swap — guide only.
    if snap.mem_avail_pct > 0.0 && snap.mem_avail_pct < 10.0 {
        findings.push(Finding {
            id: "health:ram".to_string(),
            severity: Severity::Info,
            title: "Low available memory".to_string(),
            detail: "Close heavy apps. Cleaning disk does not reliably free RAM.".to_string(),
            action: "none".to_string(),
            selectable: false,
            ..Finding::default()
        });
    }
    if snap.swap_used_pct > 50.0 {
        findings.push(Finding {
            id: "health:swap".to_string(),
            severity: Severity::Info,
            title: "Heavy swap usage".to_string(),
            detail: "System is paging to disk. Freeing RAM (close apps) helps more than \
                     cache cleanup."
                .to_string(),
            action: "none".to_string(),
            selectable: false,
            ..Finding::default()
        });
    }

    let _ = reclaimable;
    sort_findings(&mut findings);
    findings
}

/// `BuildReport` builds a full report from a live or test snapshot.
pub fn build_report(snap: &Snapshot, include: &[String]) -> Report {
    let fs = build_findings(snap, include);
    let mut reclaim: i64 = 0;
    for f in &fs {
        if f.selectable && f.action.starts_with("clean:") {
            reclaim += f.bytes;
        }
    }
    Report {
        health: snap.health,
        disk_free_pct_root: snap.disk_free_pct_root,
        reclaimable_bytes: reclaim,
        recommended_commands: recommended_commands(&fs),
        findings: fs,
        warnings: snap.warnings.clone(),
        scan_errors: snap.scan_errors.clone(),
    }
}

#[cfg(test)]
mod tests {
    //! Port of `internal/audit/scan_test.go`.
    use super::super::findings::exit_code_for_report;
    use super::*;

    fn target(id: &'static str, label: &'static str, bytes: i64, opt_in: bool) -> TargetSize {
        TargetSize {
            id,
            label,
            bytes,
            opt_in,
            requires_sudo: false,
        }
    }

    fn find<'a>(fs: &'a [Finding], id: &str) -> Option<&'a Finding> {
        fs.iter().find(|f| f.id == id)
    }

    // TestBuildFindings_largeUserCache
    #[test]
    fn build_findings_large_user_cache() {
        let snap = Snapshot {
            targets: vec![target(
                "user-cache",
                "User Cache",
                3 * BYTES_2_GIB / 2,
                false,
            )], // 3 GiB
            health: 80,
            disk_free_pct_root: 50.0,
            ..Snapshot::default()
        };
        let fs = build_findings(&snap, &[]);
        let found = find(&fs, "clean:user-cache").expect("expected clean:user-cache finding");
        assert_eq!(
            found.severity,
            Severity::Warning,
            "severity={:?} want warning",
            found.severity
        );
        assert!(
            found.default_selected,
            "expected default selected for large non-opt-in cache"
        );
    }

    // TestBuildFindings_browserOptIn
    #[test]
    fn build_findings_browser_opt_in() {
        let snap = Snapshot {
            targets: vec![target("browser-cache", "Browser", 2 * BYTES_1_GIB, true)],
            health: 80,
            disk_free_pct_root: 50.0,
            ..Snapshot::default()
        };
        let fs = build_findings(&snap, &[]);
        let found = find(&fs, "clean:browser-cache").expect("expected browser finding");
        assert!(
            !found.default_selected,
            "opt-in must not be default selected without --include"
        );

        let fs2 = build_findings(&snap, &["browser-cache".to_string()]);
        let found2 = find(&fs2, "clean:browser-cache").unwrap();
        assert!(
            found2.default_selected,
            "expected default selected when --include=browser-cache"
        );
    }

    // TestBuildFindings_diskCriticalSelectsClean
    #[test]
    fn build_findings_disk_critical_selects_clean() {
        let snap = Snapshot {
            targets: vec![target("thumbnails", "Thumbs", 100 * 1024 * 1024, false)],
            health: 30,
            disk_free_pct_root: 8.0,
            root_disk_available: true,
            ..Snapshot::default()
        };
        let fs = build_findings(&snap, &[]);
        let disk = find(&fs, "health:disk-root");
        let thumbs = find(&fs, "clean:thumbnails");
        assert!(
            matches!(disk, Some(d) if d.severity == Severity::Critical),
            "expected critical disk finding, got {disk:?}"
        );
        assert!(
            matches!(thumbs, Some(t) if t.default_selected),
            "expected thumbnails selected under disk pressure, got {thumbs:?}"
        );
    }

    // TestBuildFindings_journalDedupe
    #[test]
    fn build_findings_journal_dedupe() {
        let snap = Snapshot {
            targets: vec![target("journal-logs", "Journal", 2 * BYTES_1_GIB, false)],
            journal_bytes: 2 * BYTES_1_GIB,
            health: 70,
            disk_free_pct_root: 40.0,
            ..Snapshot::default()
        };
        let fs = build_findings(&snap, &[]);
        let mut n = 0;
        for f in &fs {
            if f.id == "clean:journal-logs" || f.action == "optimize:journal" {
                n += 1;
                assert_ne!(
                    f.action, "optimize:journal",
                    "should not emit optimize:journal when clean:journal-logs exists"
                );
            }
        }
        assert_eq!(n, 1, "expected exactly 1 journal finding, got {n}");
    }

    // TestBuildFindings_emptyHealthy
    #[test]
    fn build_findings_empty_healthy() {
        let snap = Snapshot {
            health: 90,
            disk_free_pct_root: 60.0,
            ..Snapshot::default()
        };
        let fs = build_findings(&snap, &[]);
        // may still have optimize:caches info
        for f in &fs {
            assert!(
                !f.default_selected,
                "nothing should be default selected on healthy empty system: {}",
                f.id
            );
        }
    }

    // TestBuildReport_reclaimable
    #[test]
    fn build_report_reclaimable() {
        let snap = Snapshot {
            targets: vec![target("user-cache", "Cache", BYTES_500_MIB, false)],
            health: 75,
            disk_free_pct_root: 40.0,
            ..Snapshot::default()
        };
        let rep = build_report(&snap, &[]);
        assert!(
            rep.reclaimable_bytes >= BYTES_500_MIB,
            "reclaimable={} want >= {BYTES_500_MIB}",
            rep.reclaimable_bytes
        );
        assert!(
            !rep.recommended_commands.is_empty(),
            "expected recommended commands"
        );
    }

    // TestBuildFindingsDeduplicatesAPTAutoRemoveAction
    #[test]
    fn build_findings_deduplicates_apt_autoremove_action() {
        let snap = Snapshot {
            targets: vec![target("kernels", "APT Autoremove Candidates", 100, false)],
            apt_autoremove_n: 3,
            health: 80,
            disk_free_pct_root: 50.0,
            ..Snapshot::default()
        };
        let findings = build_findings(&snap, &[]);
        let mut count = 0;
        for finding in &findings {
            if finding.action == "clean:kernels" || finding.action == "optimize:apt" {
                count += 1;
                assert_eq!(
                    finding.action, "clean:kernels",
                    "non-canonical APT action: {finding:?}"
                );
            }
        }
        assert_eq!(
            count, 1,
            "expected one APT action, got {count}: {findings:?}"
        );
    }

    // TestBuildReportCarriesScanErrorsAdditively
    #[test]
    fn build_report_carries_scan_errors_additively() {
        let report = build_report(
            &Snapshot {
                health: 50,
                disk_free_pct_root: 30.0,
                scan_errors: vec!["disk unavailable".to_string()],
                ..Snapshot::default()
            },
            &[],
        );
        assert_eq!(report.scan_errors, ["disk unavailable"]);
    }

    // TestMissingRootDiskDoesNotCreateCriticalFinding
    #[test]
    fn missing_root_disk_does_not_create_critical_finding() {
        let report = build_report(
            &Snapshot {
                health: 0,
                disk_free_pct_root: 0.0,
                scan_errors: vec!["disk: root filesystem metric unavailable".to_string()],
                ..Snapshot::default()
            },
            &[],
        );
        assert_eq!(
            report.disk_free_pct_root, 0.0,
            "disk_free_pct_root={}",
            report.disk_free_pct_root
        );
        for finding in &report.findings {
            assert!(
                finding.id != "health:disk-root" && finding.severity != Severity::Critical,
                "missing metric produced false critical finding: {finding:?}"
            );
        }
        let code = exit_code_for_report(&report.findings);
        assert_ne!(code, 2, "missing metric produced critical exit code {code}");
    }
}
