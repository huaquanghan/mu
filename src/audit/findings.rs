//! Findings model — port of `internal/audit/findings.go`: `Severity`, the
//! `Finding`/`Report` JSON schema, sort order, max-severity exit codes,
//! action parsing, and recommended-command generation.

use serde::Serialize;

use crate::status::model::serialize_go_f64;

/// `Severity` — how urgent a finding is. Serializes with Go's string values
/// (`"critical"`/`"warning"`/`"info"`). Go's zero value is `""`; production
/// code only ever assigns the three constants, so the enum is total here.
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq, Serialize)]
#[serde(rename_all = "lowercase")]
pub enum Severity {
    Critical,
    Warning,
    #[default]
    Info,
}

impl Severity {
    /// Go `string(f.Severity)` — the report badge text.
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Critical => "critical",
            Self::Warning => "warning",
            Self::Info => "info",
        }
    }
}

/// `severityRank` for sorting (higher = more urgent). Go's default branch
/// (unknown severity → 0) is unreachable for the closed enum.
fn severity_rank(s: Severity) -> i32 {
    match s {
        Severity::Critical => 3,
        Severity::Warning => 2,
        Severity::Info => 1,
    }
}

/// `Finding` — one audit result with optional remediation action.
/// Field order matches Go's struct so the serialized key order is identical.
#[derive(Debug, Clone, Default, Serialize)]
pub struct Finding {
    pub id: String,
    pub severity: Severity,
    pub title: String,
    pub detail: String,
    pub bytes: i64,
    /// `clean:<id>` | `optimize:<id>` | `none`.
    pub action: String,
    pub opt_in: bool,
    pub selectable: bool,
    pub default_selected: bool,
}

/// `Report` — the full audit snapshot for `--report` / `--json`.
///
/// `warnings`/`scan_errors` carry Go's `omitempty`. `findings` and
/// `recommended_commands` are plain `Vec`s: Go would emit `null` for a nil
/// slice, but `BuildReport` always populates both (findings always contains
/// at least `optimize:caches`), so `[]` is unreachable in real output.
#[derive(Debug, Clone, Default, Serialize)]
pub struct Report {
    pub health: i64,
    /// Go `encoding/json` float rules via [`serialize_go_f64`] (integral
    /// values emit without a fraction).
    #[serde(serialize_with = "serialize_go_f64")]
    pub disk_free_pct_root: f64,
    pub reclaimable_bytes: i64,
    pub findings: Vec<Finding>,
    pub recommended_commands: Vec<String>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub warnings: Vec<String>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub scan_errors: Vec<String>,
}

/// `SortFindings` — stable order by severity desc, bytes desc, actionable
/// findings first, then ID ascending. (`slice::sort_by` is stable, like
/// Go's `sort.SliceStable`.)
pub fn sort_findings(fs: &mut [Finding]) {
    fs.sort_by(|a, b| {
        severity_rank(b.severity)
            .cmp(&severity_rank(a.severity))
            .then_with(|| b.bytes.cmp(&a.bytes))
            .then_with(|| {
                let ai = a.action != "none" && !a.action.is_empty();
                let aj = b.action != "none" && !b.action.is_empty();
                aj.cmp(&ai)
            })
            .then_with(|| a.id.cmp(&b.id))
    });
}

/// `MaxSeverity` — the highest severity among findings, `None` when empty
/// (Go returns the zero-value `""`).
pub fn max_severity(fs: &[Finding]) -> Option<Severity> {
    let mut max = None;
    let mut max_r = 0;
    for f in fs {
        let r = severity_rank(f.severity);
        if r > max_r {
            max_r = r;
            max = Some(f.severity);
        }
    }
    max
}

/// `ExitCodeForReport`: 0 = clean/info only, 1 = warning, 2 = critical.
pub fn exit_code_for_report(fs: &[Finding]) -> i32 {
    match max_severity(fs) {
        Some(Severity::Critical) => 2,
        Some(Severity::Warning) => 1,
        _ => 0,
    }
}

/// `ParseAction` — splits `"clean:user-cache"` into kind and id.
pub fn parse_action(action: &str) -> (&str, &str) {
    if action.is_empty() || action == "none" {
        return ("none", "");
    }
    match action.split_once(':') {
        Some((kind, id)) => (kind, id),
        None => (action, ""),
    }
}

/// `RecommendedCommands` — suggested CLI commands from findings.
pub fn recommended_commands(fs: &[Finding]) -> Vec<String> {
    let mut cmds: Vec<String> = Vec::new();
    let mut clean_ids: Vec<&str> = Vec::new();
    let mut need_optimize_apt = false;
    for f in fs {
        let (kind, id) = parse_action(&f.action);
        match kind {
            "clean" => {
                if !id.is_empty() {
                    clean_ids.push(id);
                }
            }
            "optimize" if id == "apt" => need_optimize_apt = true,
            _ => {}
        }
    }
    if !clean_ids.is_empty() {
        cmds.push("mu clean --dry-run".to_string());
        // Only suggest --include for opt-in findings that are recommended
        // (default selected).
        let mut include: Vec<&str> = Vec::new();
        for f in fs {
            if f.opt_in && f.default_selected && !f.action.is_empty() && f.action != "none" {
                let (_, id) = parse_action(&f.action);
                if !id.is_empty() {
                    include.push(id);
                }
            }
        }
        if include.is_empty() {
            cmds.push("mu clean".to_string());
        } else {
            cmds.push(format!("mu clean --include={}", unique(&include).join(",")));
        }
    }
    if need_optimize_apt {
        cmds.push("mu optimize --dry-run".to_string());
        cmds.push("mu optimize".to_string());
    }
    if cmds.is_empty() {
        cmds.push("mu status".to_string());
    }
    cmds
}

/// `unique` — dedupe preserving first-occurrence order.
fn unique<'a>(input: &[&'a str]) -> Vec<&'a str> {
    let mut seen = std::collections::HashSet::with_capacity(input.len());
    let mut out = Vec::new();
    for s in input {
        if seen.insert(*s) {
            out.push(*s);
        }
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    fn finding(id: &str, severity: Severity, bytes: i64) -> Finding {
        Finding {
            id: id.to_string(),
            severity,
            bytes,
            ..Finding::default()
        }
    }

    // findings_test.go: TestSortFindings
    #[test]
    fn sort_findings_orders_by_severity_then_bytes() {
        let mut fs = vec![
            finding("a", Severity::Info, 100),
            finding("b", Severity::Critical, 10),
            finding("c", Severity::Warning, 50),
            finding("d", Severity::Warning, 200),
        ];
        sort_findings(&mut fs);
        assert_eq!(fs[0].id, "b", "expected critical first");
        assert_eq!(fs[1].id, "d", "expected larger warning second");
        assert_eq!(fs[2].id, "c", "expected smaller warning third");
    }

    // findings_test.go: TestExitCodeForReport
    #[test]
    fn exit_code_for_report_maps_max_severity() {
        assert_eq!(exit_code_for_report(&[]), 0, "empty should be 0");
        assert_eq!(
            exit_code_for_report(&[finding("x", Severity::Info, 0)]),
            0,
            "info only → 0"
        );
        assert_eq!(
            exit_code_for_report(&[finding("x", Severity::Warning, 0)]),
            1,
            "warning → 1"
        );
        assert_eq!(
            exit_code_for_report(&[finding("x", Severity::Critical, 0)]),
            2,
            "critical → 2"
        );
    }

    // findings_test.go: TestParseAction
    #[test]
    fn parse_action_splits_kind_and_id() {
        let (k, id) = parse_action("clean:user-cache");
        assert_eq!((k, id), ("clean", "user-cache"));
        let (k, id) = parse_action("none");
        assert_eq!((k, id), ("none", ""));
    }

    // findings_test.go: TestMaxSeverity
    #[test]
    fn max_severity_picks_highest() {
        assert_eq!(
            max_severity(&[
                finding("a", Severity::Info, 0),
                finding("b", Severity::Warning, 0),
            ]),
            Some(Severity::Warning)
        );
    }
}
