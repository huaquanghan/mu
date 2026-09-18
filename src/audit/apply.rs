//! Apply path — port of `internal/audit/apply.go`: runs
//! `clean.Execute`/`optimize.RunStep` for each selected finding. The wizard
//! driver that calls this is deferred to the tui-port wave, but the dispatch
//! logic (dedupe, dry-run, outcome logging, policy skips) is complete.

use std::io::Write;

use super::Deps;
use super::findings::{Finding, parse_action};
use crate::error::{Error, Result, msg};
use crate::{clean, oplog, optimize};

/// `ApplyResult` — the outcome of applying one finding.
#[derive(Debug)]
pub struct ApplyResult {
    pub finding_id: String,
    pub action: String,
    pub err: Option<Error>,
    pub skipped: bool,
}

/// `Apply` runs `clean.Execute` / `optimize.RunStep` for each selected
/// finding. `dry_run` uses `Execute(true)` and skips real optimize steps
/// (logs only). `out` is Go's `io.Writer` (`os.Stdout` from the wizard).
pub fn apply(
    selected: &[Finding],
    dry_run: bool,
    debug: bool,
    out: &mut dyn Write,
) -> Vec<ApplyResult> {
    apply_in(selected, dry_run, debug, &Deps::real(), out)
}

/// Injectable `Apply` — `deps` supplies what Go reads from package vars:
/// the clean runner/targets, `optimize.RunStep`'s runner + `optimize_skip`
/// policy config, and the ops-log file `LogOutcome` appends to.
pub(crate) fn apply_in(
    selected: &[Finding],
    dry_run: bool,
    debug: bool,
    deps: &Deps,
    out: &mut dyn Write,
) -> Vec<ApplyResult> {
    // Dedupe actions (e.g. two findings same action).
    let mut seen = std::collections::HashSet::new();
    let mut results = Vec::new();

    for f in selected {
        if !f.selectable || f.action.is_empty() || f.action == "none" {
            oplog::log_outcome("audit", &f.action, "skipped");
            results.push(ApplyResult {
                finding_id: f.id.clone(),
                action: f.action.clone(),
                err: None,
                skipped: true,
            });
            continue;
        }
        if !seen.insert(f.action.clone()) {
            oplog::log_outcome("audit", &f.action, "skipped");
            results.push(ApplyResult {
                finding_id: f.id.clone(),
                action: f.action.clone(),
                err: None,
                skipped: true,
            });
            continue;
        }

        let (kind, id) = parse_action(&f.action);
        let _ = writeln!(out, "→ Applying {} ({})…", f.title, f.action);

        let (skipped, err) = match kind {
            "clean" => (false, apply_clean(id, dry_run, deps).err()),
            "optimize" => match apply_optimize(id, dry_run, debug, deps, out) {
                Ok(skipped) => (skipped, None),
                Err(e) => (false, Some(e)),
            },
            other => (
                false,
                Some(Error::Msg(format!("unknown action kind: {other}"))),
            ),
        };

        if let Some(e) = &err {
            let _ = writeln!(out, "  warn: {e}");
            oplog::log_outcome("audit", &f.action, "failure");
            if debug {
                eprintln!("audit apply {}: {}", f.action, e);
            }
        } else if skipped {
            let _ = writeln!(out, "  ⏭️ Skipped by optimize policy");
            oplog::log_outcome("audit", &f.action, "skipped");
        } else {
            let _ = writeln!(out, "  ✅ Done");
            let outcome = if dry_run { "dry-run" } else { "success" };
            oplog::log_outcome("audit", &f.action, outcome);
        }
        results.push(ApplyResult {
            finding_id: f.id.clone(),
            action: f.action.clone(),
            err,
            skipped,
        });
    }
    results
}

/// `applyClean` — `clean.TargetByID` + `Execute(dryRun)`. The lookup walks
/// `AllTargets()` like Go's `TargetByID`, here over the injected deps.
fn apply_clean(target_id: &str, dry_run: bool, deps: &Deps) -> Result<()> {
    let target = clean::all_targets_in(&deps.clean)
        .into_iter()
        .find(|t| t.id == target_id);
    match target {
        // Docker may be absent — the scan already ran; the target is missing.
        None => msg(format!("clean target {target_id:?} not available")),
        Some(t) => (t.execute)(dry_run),
    }
}

/// `applyOptimize` — dry-run logs only; otherwise `optimize.RunStep` into a
/// captured buffer (Go's `strings.Builder`), printed trimmed like Go.
/// `Ok(true)` = skipped by policy; `Ok(false)` = ran clean.
fn apply_optimize(
    step_id: &str,
    dry_run: bool,
    debug: bool,
    deps: &Deps,
    out: &mut dyn Write,
) -> Result<bool> {
    if dry_run {
        let _ = writeln!(out, "  [dry-run] would run optimize step {step_id}");
        return Ok(false);
    }
    let mut buf: Vec<u8> = Vec::new();
    let res = optimize::run_step_with(
        step_id,
        &optimize::Options {
            debug,
            ..optimize::Options::default()
        },
        &deps.optimize,
        &mut buf,
    );
    // `strings.TrimSpace(buf.String())` printed before the error check.
    let trimmed = String::from_utf8_lossy(&buf).trim().to_string();
    if !trimmed.is_empty() {
        let _ = writeln!(out, "{trimmed}");
    }
    res
}

/// `CountApplyErrors` — how many apply results failed.
pub fn count_apply_errors(rs: &[ApplyResult]) -> usize {
    rs.iter().filter(|r| r.err.is_some()).count()
}

#[cfg(test)]
mod tests {
    //! Port of `internal/audit/apply_test.go`.
    use super::super::test_support::*;
    use super::*;
    use std::fs;

    // TestApply_skipsOptimizeStepConfiguredInPolicy
    #[test]
    fn apply_skips_optimize_step_configured_in_policy() {
        let root = tempdir("applyskip");
        let cfg_dir = root.join("mu");
        fs::create_dir_all(&cfg_dir).unwrap();
        fs::write(
            cfg_dir.join("config.toml"),
            "[optimize_skip]\nsteps = [\"apt\"]\n",
        )
        .unwrap();
        let deps = deps_for_test(&root);

        let mut out: Vec<u8> = Vec::new();
        let results = apply_in(
            &[Finding {
                id: "optimize:apt".to_string(),
                title: "Unused packages can be removed".to_string(),
                action: "optimize:apt".to_string(),
                selectable: true,
                ..Finding::default()
            }],
            false,
            false,
            &deps,
            &mut out,
        );

        assert_eq!(results.len(), 1, "expected one result");
        assert!(results[0].skipped, "expected policy-skipped result");
        assert!(
            results[0].err.is_none(),
            "unexpected execution error: {:?}",
            results[0].err
        );
        let text = String::from_utf8_lossy(&out);
        assert!(
            text.contains("Skipped by optimize policy"),
            "expected policy skip message, got {text:?}"
        );
        fs::remove_dir_all(&root).ok();
    }

    // TestApplyDeduplicatesIdenticalActions
    #[test]
    fn apply_deduplicates_identical_actions() {
        let root = tempdir("applydedup");
        let deps = deps_for_test(&root);
        let mut out: Vec<u8> = Vec::new();
        let mk = |id: &str| Finding {
            id: id.to_string(),
            title: id.to_string(),
            action: "unknown:same".to_string(),
            selectable: true,
            ..Finding::default()
        };
        let results = apply_in(&[mk("first"), mk("second")], true, false, &deps, &mut out);
        assert!(
            results.len() == 2 && results[0].err.is_some() && results[1].skipped,
            "duplicate action was not skipped: {results:?}"
        );
        fs::remove_dir_all(&root).ok();
    }
}
