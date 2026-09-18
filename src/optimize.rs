//! Optimize steps — port of `internal/optimize/optimize.go`.
//!
//! Go runs the steps through a Bubbletea program (`optimizeModel`); the
//! non-interactive path (`isatty(stdin)` false → `tea.WithInput(nil)`) is a
//! purely sequential executor: each step runs in order, failures do not stop
//! later steps, captured output is printed via `tea.Println`, and the joined
//! `stepErrors` becomes the command's nonzero exit. That is what this module
//! ports — the same sequential executor drives TTY and non-TTY runs (no
//! raw-mode input reader exists here, so Go's `ui.ExecTerminal` terminal
//! release has nothing to race against). The spinner checklist, ANSI styles,
//! and the ctrl+c "finish the active step then skip the rest" path are
//! TUI-phase concerns and are intentionally dropped; on a non-TTY Go cannot
//! receive `KeyMsg` at all, so `stopRequested` is unreachable there anyway.
//!
//! Go's swappable package vars become explicit injection: [`Deps`] mirrors
//! `optimizeRunner` (`command.Runner`) and `runAutoremove`
//! (`clean.RunAutoremove`), plus the `ui.Confirm` prompt and the XDG dirs that
//! `utils.LoadWhitelist`/`utils.InitLogger` read from the environment (tests
//! must not mutate env).

use std::io::{IsTerminal, Write};
use std::path::{Path, PathBuf};
use std::sync::Arc;

use crate::config;
use crate::error::{Error, Result, msg};
use crate::oplog;
use crate::runner::{CommandSpec, Output, ProcessRunner, RunError, Runner};
use crate::xdg;

/// `Options` — controls optimize behavior.
#[derive(Debug, Default, Clone)]
pub struct Options {
    /// `DryRun` — preview actions without making changes.
    pub dry_run: bool,
    /// `Debug` — verbose logging (warn when the ops log cannot open).
    pub debug: bool,
    /// `Skip` — step IDs from `--skip`, merged over `optimize_skip.steps`.
    pub skip: Vec<String>,
    /// `AutoYes` — skip the confirmation prompt (`--yes`).
    pub auto_yes: bool,
}

/// Dependencies that Go reads from package vars, env, and the terminal.
/// Production wiring lives in [`Deps::real`]; tests substitute fakes.
pub(crate) struct Deps {
    /// `optimizeRunner` — `command.ExecRunner` in production.
    pub runner: Arc<dyn Runner>,
    /// `runAutoremove` — `clean.RunAutoremove(ctx, dryRun)` in production.
    pub autoremove: Arc<dyn Fn(bool) -> Result<()> + Send + Sync>,
    /// `ui.Confirm` — returns true when the user picks YES.
    pub confirm: Arc<dyn Fn(&str) -> bool + Send + Sync>,
    /// `$XDG_CONFIG_HOME` — where `mu/config.toml` is loaded from.
    pub config_home: PathBuf,
    /// `$XDG_DATA_HOME` — where `mu/operations.log` is opened.
    pub data_home: PathBuf,
    /// `MU_NO_OPLOG=1` — disables `InitLogger`.
    pub no_oplog: bool,
}

impl Deps {
    /// Production dependencies, mirroring the Go package vars and env reads.
    pub(crate) fn real() -> Self {
        Self {
            runner: Arc::new(ProcessRunner::new()),
            // `clean.RunAutoremove` on clean's own `cleanRunner` — a separate
            // `ProcessRunner` from `optimizeRunner`, matching the separate Go
            // package vars.
            autoremove: Arc::new(|dry_run| {
                crate::clean::run_autoremove(&ProcessRunner::new(), dry_run)
            }),
            confirm: Arc::new(confirm_prompt),
            config_home: xdg::config_home(),
            data_home: xdg::data_home(),
            no_oplog: std::env::var_os("MU_NO_OPLOG").is_some_and(|v| v == "1"),
        }
    }
}

/// Go `func(io.Writer) error` — the step action writing captured
/// stdout/stderr to the sink.
type StepRun = Box<dyn Fn(&mut dyn Write) -> Result<()>>;

/// `step` — one optimize step: stable ID, plan description, action.
pub(crate) struct Step {
    pub id: &'static str,
    pub desc: &'static str,
    pub run: StepRun,
}

/// `allSteps` — apt, journal, caches, in that order.
fn all_steps(deps: &Deps) -> Vec<Step> {
    vec![
        Step {
            id: "apt",
            desc: "apt-get update && apt-get autoremove --purge",
            run: {
                let runner = deps.runner.clone();
                let autoremove = deps.autoremove.clone();
                Box::new(move |out: &mut dyn Write| {
                    apt_autoremove(runner.as_ref(), autoremove.as_ref(), out)
                })
            },
        },
        Step {
            id: "journal",
            desc: "journalctl --vacuum-size=500M",
            run: {
                let runner = deps.runner.clone();
                Box::new(move |out: &mut dyn Write| journal_vacuum(runner.as_ref(), out))
            },
        },
        Step {
            id: "caches",
            desc: "update icon/mime/font caches",
            run: {
                let runner = deps.runner.clone();
                Box::new(move |out: &mut dyn Write| update_caches(runner.as_ref(), out))
            },
        },
    ]
}

/// `StepIDs` — the IDs of all built-in optimize steps in order.
pub fn step_ids() -> Vec<&'static str> {
    // Go derives IDs from allSteps(); the deps do not affect the IDs.
    all_steps(&Deps::real()).iter().map(|s| s.id).collect()
}

/// `StepStatus` — per-step outcome, logged via `LogOutcome`.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum StepStatus {
    /// `StepSuccess`
    Success,
    /// `StepFailed`
    Failed,
    /// `StepSkipped`
    Skipped,
}

impl StepStatus {
    /// `string(msg.status)` — the outcome string logged to the ops log.
    fn as_str(self) -> &'static str {
        match self {
            Self::Success => "success",
            Self::Failed => "failed",
            Self::Skipped => "skipped",
        }
    }
}

/// `StepResult` — the recorded outcome of one step.
#[derive(Debug)]
pub struct StepResult {
    /// Step ID.
    pub id: String,
    /// Final status.
    pub status: StepStatus,
    /// Step error, when failed.
    pub err: Option<Error>,
}

/// `RunStep` — runs a single optimize step by id (apt, journal, caches).
///
/// Go returns `(skipped bool, err error)`; `skipped == true` only ever pairs
/// with a nil error, so `Result<bool>` is lossless: `Ok(true)` skipped by
/// policy, `Ok(false)` ran successfully, `Err` unknown or failed step.
/// `out` receives the step's captured output (`io.Discard` → `io::sink()`).
pub fn run_step(id: &str, opts: &Options, out: &mut dyn Write) -> Result<bool> {
    run_step_with(id, opts, &Deps::real(), out)
}

pub(crate) fn run_step_with(
    id: &str,
    opts: &Options,
    deps: &Deps,
    out: &mut dyn Write,
) -> Result<bool> {
    let steps = all_steps(deps);
    let target = steps
        .iter()
        .find(|s| s.id == id)
        .ok_or_else(|| Error::Msg(format!("unknown optimize step: {id}")))?;
    let skip = resolve_skip(opts, &deps.config_home)?;
    if skip.iter().any(|s| s == id) {
        return Ok(true);
    }
    (target.run)(out)?;
    Ok(false)
}

/// `Run` — executes (or previews) the optimization steps, returning the
/// one-line summary (`"Aborted."` when the user declines the prompt).
/// Step lines and the summary are also printed, like Go's `ui.Run`.
pub fn run(opts: &Options) -> Result<String> {
    let stdout = std::io::stdout();
    let mut out = stdout.lock();
    run_with(opts, &Deps::real(), &mut out)
}

/// TUI entry for optimize — port of Go's interactive path. Prints the plan,
/// shows a YES/NO confirm (default NO), then runs with `auto_yes=true` if
/// confirmed, or returns "Aborted." if declined. Wired by `tui::dispatch_subcommand`.
pub fn run_tui(opts: &Options) -> Result<String> {
    let stdout = std::io::stdout();
    let mut out = stdout.lock();
    // Reuse run_with but with a confirm closure that calls the TUI.
    let deps = Deps {
        confirm: Arc::new(|_prompt: &str| crate::tui::confirm_inline("Proceed to optimize?")),
        ..Deps::real()
    };
    run_with(opts, &deps, &mut out)
}

pub(crate) fn run_with(opts: &Options, deps: &Deps, out: &mut dyn Write) -> Result<String> {
    // Go loads the whitelist once up front (fail closed on malformed config)
    // and again inside resolveSkip.
    config::load_whitelist_from(&deps.config_home)
        .map_err(|e| e.with_context("invalid mu configuration".to_string()))?;
    let skip = resolve_skip(opts, &deps.config_home)?;
    let steps = all_steps(deps);

    // `ui.NewRun(os.Stdout)` — on a non-TTY lipgloss styles render as plain
    // text; the port always emits the plain form (animation dropped).
    let _ = writeln!(out, "\n  Optimize plan");
    for s in &steps {
        if skip.iter().any(|k| k == s.id) {
            let _ = writeln!(out, "  [skip]  {}", s.desc);
        } else {
            let _ = writeln!(out, "  [run]   {}", s.desc);
        }
    }

    if opts.dry_run {
        let _ = writeln!(out, "  Dry run — nothing executed.");
        return Ok("Dry run — nothing executed.".to_string());
    }

    if !opts.auto_yes && !(deps.confirm)("Proceed to optimize?") {
        let _ = writeln!(out, "  Aborted.");
        return Ok("Aborted.".to_string());
    }

    // `utils.InitLogger` + `defer utils.CloseLogger()` — the RAII guard lives
    // to scope end like Go's defer.
    if !deps.no_oplog
        && let Err(e) = oplog::init_logger_at(&deps.data_home)
        && opts.debug
    {
        eprintln!("warn: could not open log: {e}");
    }
    let _log = CloseLoggerGuard;
    let results = execute_steps(&steps, &skip, out);

    // Go: a failed model run returns the joined step errors and never prints
    // the summary.
    step_errors(&results)?;

    let (mut succeeded, mut failed, mut skipped) = (0, 0, 0);
    for res in &results {
        match res.status {
            StepStatus::Success => succeeded += 1,
            StepStatus::Failed => failed += 1,
            StepStatus::Skipped => skipped += 1,
        }
    }
    let summary = format!(
        "Optimization complete — {succeeded} succeeded, {failed} failed, {skipped} skipped"
    );
    let _ = writeln!(out, "\n  {summary}");
    Ok(summary)
}

/// `resolveSkip` — merges `opts.Skip` with the config's `optimize_skip.steps`
/// (CLI entries first, config additions deduplicated) and validates every ID
/// against [`step_ids`]. A malformed config fails closed.
fn resolve_skip(opts: &Options, config_home: &Path) -> Result<Vec<String>> {
    let wl = config::load_whitelist_from(config_home)
        .map_err(|e| e.with_context("invalid mu configuration".to_string()))?;

    let mut skip: Vec<String> = opts.skip.clone();
    for s in &wl.optimize_skip.steps {
        if !skip.contains(s) {
            skip.push(s.clone());
        }
    }
    let valid = step_ids();
    for id in &skip {
        if !valid.contains(&id.as_str()) {
            return msg(format!("unknown optimize skip ID: {id}"));
        }
    }
    Ok(skip)
}

/// The sequential half of `optimizeModel`: run each step in order, record
/// `{id, status, err}`, `LogOutcome` every completion, then emit the same
/// lines Go's `Update` emits via `tea.Println` — the step's captured output
/// (trailing newlines trimmed, truncated past 4096 bytes) and
/// `failed: {id}: {err}` on failure. Independent steps continue after a
/// failure; there is no `stopRequested` path (see module docs).
fn execute_steps(steps: &[Step], skip: &[String], out: &mut dyn Write) -> Vec<StepResult> {
    let mut results = Vec::with_capacity(steps.len());
    for s in steps {
        let (status, err, output) = if skip.iter().any(|k| k == s.id) {
            (StepStatus::Skipped, None, Vec::new())
        } else {
            let mut buf: Vec<u8> = Vec::new();
            let res = (s.run)(&mut buf);
            // strings.TrimRight(buf.String(), "\n") then a 4096-byte cut +
            // "\n... (truncated)" — byte-level, like Go string slicing.
            while buf.last() == Some(&b'\n') {
                buf.pop();
            }
            if buf.len() > 4096 {
                buf.truncate(4096);
                buf.extend_from_slice(b"\n... (truncated)");
            }
            match res {
                Ok(()) => (StepStatus::Success, None, buf),
                Err(e) => (StepStatus::Failed, Some(e), buf),
            }
        };
        // `tea.Println(fmt.Sprintf("failed: %s: %v", ...))` — formatted before
        // `err` moves into the result.
        let failed_line = err.as_ref().map(|e| format!("failed: {}: {}", s.id, e));
        results.push(StepResult {
            id: s.id.to_string(),
            status,
            err,
        });
        oplog::log_outcome("optimize", s.id, status.as_str());
        if !output.is_empty() {
            let _ = out.write_all(&output);
            let _ = out.write_all(b"\n");
        }
        if let Some(line) = failed_line {
            let _ = writeln!(out, "{line}");
        }
    }
    results
}

/// `stepErrors` — joins each failed step as `optimize step {id}: {err}`,
/// newline-separated like `errors.Join`.
fn step_errors(results: &[StepResult]) -> Result<()> {
    let mut errs = Vec::new();
    for result in results {
        if result.status == StepStatus::Failed
            && let Some(e) = &result.err
        {
            errs.push(format!("optimize step {}: {}", result.id, e));
        }
    }
    if errs.is_empty() {
        Ok(())
    } else {
        Err(Error::Msg(errs.join("\n")))
    }
}

/// `defer utils.CloseLogger()` — closes on drop at every return path.
struct CloseLoggerGuard;

impl Drop for CloseLoggerGuard {
    fn drop(&mut self) {
        oplog::close_logger();
    }
}

/// Interim `ui.Confirm`: the YES/NO button model is TUI-phase. On a pipe Go's
/// model can never get a key — it quits on EOF with the NO default — so a
/// non-terminal stdin declines without printing an unanswerable prompt. On a
/// TTY a plain line prompt keeps the NO default: EOF, empty input, or
/// anything but `y`/`yes` declines, matching `Confirm`'s `result == false`
/// exits (q, ctrl+c, error).
fn confirm_prompt(prompt: &str) -> bool {
    let stdin = std::io::stdin();
    if !stdin.is_terminal() {
        return false;
    }
    {
        let stdout = std::io::stdout();
        let mut o = stdout.lock();
        let _ = write!(o, "  {prompt} [y/N] ");
        let _ = o.flush();
    }
    let mut line = String::new();
    match stdin.read_line(&mut line) {
        Ok(0) | Err(_) => false,
        Ok(_) => matches!(line.trim().to_lowercase().as_str(), "y" | "yes"),
    }
}

/// Go callers write `result.Stdout`/`result.Stderr` unconditionally — the
/// buffers stay populated on `ExitError` and are empty on a spawn failure, so
/// a [`RunError`] carrying output writes it and one without writes nothing.
fn write_captured(out: &mut dyn Write, res: &std::result::Result<Output, RunError>) {
    let output = match res {
        Ok(o) => Some(o),
        Err(e) => e.output(),
    };
    if let Some(o) = output {
        let _ = out.write_all(&o.stdout);
        let _ = out.write_all(&o.stderr);
    }
}

/// `aptAutoremove` — `sudo apt-get update`, then APT-policy autoremove via
/// `clean.RunAutoremove` (always non-dry; `Run` short-circuits `--dry-run`
/// before any step executes).
fn apt_autoremove(
    runner: &dyn Runner,
    autoremove: &dyn Fn(bool) -> Result<()>,
    out: &mut dyn Write,
) -> Result<()> {
    let res = runner.run(&CommandSpec::new("sudo", ["apt-get", "update"]));
    write_captured(out, &res);
    if let Err(e) = res {
        return Err(Error::Msg(e.to_string()).with_context("apt-get update".to_string()));
    }
    autoremove(false)
}

/// `journalVacuum` — `sudo journalctl --vacuum-size=500M`, error unwrapped.
fn journal_vacuum(runner: &dyn Runner, out: &mut dyn Write) -> Result<()> {
    let res = runner.run(&CommandSpec::new(
        "sudo",
        ["journalctl", "--vacuum-size=500M"],
    ));
    write_captured(out, &res);
    res.map(|_| ()).map_err(|e| Error::Msg(e.to_string()))
}

/// `updateCaches` — runs each cache refresh independently and joins failures
/// (`errors.Join`): a failed command does not stop the rest.
fn update_caches(runner: &dyn Runner, out: &mut dyn Write) -> Result<()> {
    let mut errs = Vec::new();
    for args in [
        &["sudo", "update-mime-database", "/usr/share/mime"][..],
        &["fc-cache", "-f"][..],
    ] {
        let res = runner.run(&CommandSpec::new(args[0], &args[1..]));
        write_captured(out, &res);
        if let Err(e) = res {
            errs.push(format!("{}: {}", args.join(" "), e));
        }
    }
    if errs.is_empty() {
        Ok(())
    } else {
        Err(Error::Msg(errs.join("\n")))
    }
}

#[cfg(test)]
mod tests {
    //! Port of `internal/optimize/step_test.go`.
    //!
    //! Two Go tests guard Bubbletea internals that do not exist in this port
    //! and are intentionally not carried over:
    //! - `TestOptimizeStepRunsWithTerminalRelease` asserts an interactive
    //!   step dispatches via `tea.Exec` (so bubbletea releases the terminal
    //!   before sudo reads its password). There is no raw-mode input reader
    //!   here, so nothing races sudo's prompt — the bug class cannot occur.
    //! - `TestOptimizeModelStopsAfterActiveStepAndSkipsRemaining` drives the
    //!   `ctrl+c` `stopRequested` path through `tea.KeyMsg`. The sequential
    //!   port has no key pump (and Go's non-TTY `WithInput(nil)` path can't
    //!   receive keys either), so it is deferred to the TUI phase.
    use super::*;
    use crate::runner::FakeRunner;
    use std::fs;
    use std::sync::Mutex;

    fn tempdir(tag: &str) -> PathBuf {
        let d = std::env::temp_dir().join(format!(
            "mu-test-{}-{}-{}",
            tag,
            std::process::id(),
            std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap()
                .as_nanos()
        ));
        fs::create_dir_all(&d).unwrap();
        d
    }

    fn write_config(root: &Path, body: &str) {
        let dir = root.join("mu");
        fs::create_dir_all(&dir).unwrap();
        fs::write(dir.join("config.toml"), body).unwrap();
    }

    /// Deps wired to a temp `config_home`/`data_home` with the oplog disabled
    /// (no env mutation, no shared log-file state between parallel tests).
    fn test_deps(root: &Path, runner: Arc<FakeRunner>) -> Deps {
        Deps {
            runner,
            autoremove: Arc::new(|_| Ok(())),
            confirm: Arc::new(|_| panic!("confirm must not be reached")),
            config_home: root.to_path_buf(),
            data_home: root.join("data"),
            no_oplog: true,
        }
    }

    fn spec(argv: &[&str]) -> CommandSpec {
        CommandSpec::new(argv[0], &argv[1..])
    }

    // Port of TestStepIDs.
    #[test]
    fn step_ids_match_go_order() {
        assert_eq!(step_ids(), vec!["apt", "journal", "caches"]);
    }

    // Port of TestOptimizeModelRecordsFailedAndSuccessfulSteps — the model's
    // Update/stepErrors behavior is the sequential executor here.
    #[test]
    fn executor_records_failed_and_successful_steps() {
        let steps = vec![
            Step {
                id: "bad",
                desc: "bad",
                run: Box::new(|_| msg("blocked")),
            },
            Step {
                id: "good",
                desc: "good",
                run: Box::new(|_| Ok(())),
            },
        ];
        let mut out = Vec::new();
        let results = execute_steps(&steps, &[], &mut out);
        assert_eq!(results.len(), 2);
        assert_eq!(results[0].status, StepStatus::Failed);
        assert_eq!(results[1].status, StepStatus::Success);
        let text = String::from_utf8_lossy(&out);
        assert!(
            text.contains("failed: bad: blocked"),
            "failed line missing: {text:?}"
        );
        let err = step_errors(&results).expect_err("expected aggregate error");
        assert!(err.to_string().contains("blocked"), "got {err}");
        // Go wraps each failure as "optimize step {id}: {err}".
        assert_eq!(err.to_string(), "optimize step bad: blocked");
    }

    // Port of TestOptimizeModelRecordsSkippedStep — a skipped step never runs.
    #[test]
    fn executor_records_skipped_step() {
        let steps = vec![Step {
            id: "apt",
            desc: "apt",
            run: Box::new(|_| panic!("skipped step must not run")),
        }];
        let mut out = Vec::new();
        let results = execute_steps(&steps, &["apt".to_string()], &mut out);
        assert_eq!(results[0].status, StepStatus::Skipped);
    }

    // Port of TestUpdateCachesAggregatesFailuresAndContinues.
    #[test]
    fn update_caches_aggregates_failures_and_continues() {
        let runner = Arc::new(FakeRunner::new());
        runner.set_handler(|spec| {
            if spec.program == "sudo" {
                Err(RunError::Spawn(std::io::Error::other("mime failed")))
            } else {
                Ok(Output::default())
            }
        });
        let mut out = Vec::new();
        let err = update_caches(runner.as_ref(), &mut out).expect_err("expected cache error");
        assert!(err.to_string().contains("mime failed"), "got {err}");
        let want = [
            spec(&["sudo", "update-mime-database", "/usr/share/mime"]),
            spec(&["fc-cache", "-f"]),
        ];
        let calls = runner.invocations();
        assert_eq!(calls, want, "independent step did not continue");
    }

    // Port of TestOptimizeRejectsUnknownSkip.
    #[test]
    fn run_rejects_unknown_skip() {
        let root = tempdir("opt-unknown-skip");
        let deps = test_deps(&root, Arc::new(FakeRunner::new()));
        let opts = Options {
            dry_run: true,
            skip: vec!["unknown".to_string()],
            ..Options::default()
        };
        let mut out = Vec::new();
        let err = run_with(&opts, &deps, &mut out).expect_err("expected unknown skip error");
        assert!(
            err.to_string().contains("unknown optimize skip ID"),
            "got {err}"
        );
        fs::remove_dir_all(&root).ok();
    }

    // Port of TestRunStepFailsClosedOnMalformedConfig.
    #[test]
    fn run_step_fails_closed_on_malformed_config() {
        let root = tempdir("opt-malformed");
        write_config(&root, "broken = [");
        let deps = test_deps(&root, Arc::new(FakeRunner::new()));
        let mut out = Vec::new();
        let err = run_step_with("caches", &Options::default(), &deps, &mut out)
            .expect_err("expected fail-closed config error");
        assert!(
            err.to_string().contains("invalid mu configuration"),
            "got {err}"
        );
        fs::remove_dir_all(&root).ok();
    }

    // Port of TestRunAllSkippedCompletesAndRecordsSkippedStates.
    #[test]
    fn run_all_skipped_completes() {
        let root = tempdir("opt-all-skip");
        let runner = Arc::new(FakeRunner::new());
        let deps = test_deps(&root, runner.clone());
        let opts = Options {
            auto_yes: true,
            skip: vec![
                "apt".to_string(),
                "journal".to_string(),
                "caches".to_string(),
            ],
            ..Options::default()
        };
        let mut out = Vec::new();
        let summary = run_with(&opts, &deps, &mut out).expect("all-skipped run");
        assert_eq!(
            summary,
            "Optimization complete — 0 succeeded, 0 failed, 3 skipped"
        );
        assert!(
            runner.invocations().is_empty(),
            "skipped steps must not run: {:?}",
            runner.invocations()
        );
        fs::remove_dir_all(&root).ok();
    }

    // Port of TestRunReturnsNonzeroAfterIndependentStepFailure.
    #[test]
    fn run_returns_error_after_independent_step_failure() {
        let root = tempdir("opt-stepfail");
        let runner = Arc::new(FakeRunner::new());
        runner.set_handler(|_| {
            Err(RunError::Spawn(std::io::Error::other(
                "cache refresh failed",
            )))
        });
        let deps = test_deps(&root, runner.clone());
        let opts = Options {
            auto_yes: true,
            skip: vec!["apt".to_string(), "journal".to_string()],
            ..Options::default()
        };
        let mut out = Vec::new();
        let err = run_with(&opts, &deps, &mut out).expect_err("expected optimize failure");
        assert!(
            err.to_string().contains("cache refresh failed"),
            "got {err}"
        );
        // Go's error is prefixed "optimize step {id}: " and joins both
        // subcommand failures; the run continues past the first one.
        assert_eq!(
            runner.invocations().len(),
            2,
            "caches step should run both commands"
        );
        let text = String::from_utf8_lossy(&out);
        assert!(
            text.contains("failed: caches:"),
            "missing failed line: {text:?}"
        );
        fs::remove_dir_all(&root).ok();
    }

    // Port of TestAptAndJournalStepsUseInjectedRunners.
    #[test]
    fn apt_and_journal_steps_use_injected_runners() {
        let runner = Arc::new(FakeRunner::new());
        runner.set_handler(|_| {
            Ok(Output {
                stdout: b"ok\n".to_vec(),
                ..Output::default()
            })
        });
        let calls = Mutex::new(Vec::new());
        let autoremove = |_: bool| {
            calls.lock().unwrap().push(vec!["autoremove".to_string()]);
            Ok(())
        };
        let mut out = Vec::new();
        apt_autoremove(runner.as_ref(), &autoremove, &mut out).expect("apt step");
        journal_vacuum(runner.as_ref(), &mut out).expect("journal step");

        let want = [
            spec(&["sudo", "apt-get", "update"]),
            spec(&["sudo", "journalctl", "--vacuum-size=500M"]),
        ];
        assert_eq!(runner.invocations().as_slice(), want.as_slice());
        assert_eq!(
            calls.lock().unwrap().as_slice(),
            &[vec!["autoremove".to_string()]]
        );
        // Go's step writes captured stdout/stderr into out.
        assert_eq!(out, b"ok\nok\n");
    }

    // Port of TestRunStep_unknown.
    #[test]
    fn run_step_unknown() {
        let root = tempdir("opt-step-unknown");
        let deps = test_deps(&root, Arc::new(FakeRunner::new()));
        let mut out = Vec::new();
        let err = run_step_with("nope", &Options::default(), &deps, &mut out)
            .expect_err("expected error for unknown step");
        assert!(
            err.to_string().contains("unknown optimize step"),
            "got {err}"
        );
        fs::remove_dir_all(&root).ok();
    }

    // Port of TestRunStep_skipsConfiguredStep.
    #[test]
    fn run_step_skips_configured_step() {
        let root = tempdir("opt-cfg-skip");
        write_config(&root, "[optimize_skip]\nsteps = [\"apt\"]\n");
        let runner = Arc::new(FakeRunner::new());
        let deps = test_deps(&root, runner.clone());
        let mut out = Vec::new();
        let skipped = run_step_with("apt", &Options::default(), &deps, &mut out).expect("RunStep");
        assert!(skipped, "expected configured apt step to be skipped");
        assert!(
            runner.invocations().is_empty(),
            "skipped step must not spawn commands"
        );
        fs::remove_dir_all(&root).ok();
    }

    // `Run`'s decline path: `!AutoYes && !ui.Confirm(...)` → "Aborted.".
    #[test]
    fn run_aborts_when_confirmation_declined() {
        let root = tempdir("opt-abort");
        let runner = Arc::new(FakeRunner::new());
        let mut deps = test_deps(&root, runner.clone());
        deps.confirm = Arc::new(|_| false);
        let opts = Options::default(); // auto_yes = false
        let mut out = Vec::new();
        let summary = run_with(&opts, &deps, &mut out).expect("aborted run");
        assert_eq!(summary, "Aborted.");
        assert!(
            runner.invocations().is_empty(),
            "declined run must not execute steps"
        );
        let text = String::from_utf8_lossy(&out);
        assert!(text.contains("  Aborted.\n"), "got {text:?}");
        fs::remove_dir_all(&root).ok();
    }

    // `--dry-run` prints the plan and returns before confirm/execute,
    // matching tests/golden/optimize-dry-run.txt.
    #[test]
    fn run_dry_run_prints_plan_and_executes_nothing() {
        let root = tempdir("opt-dry");
        let runner = Arc::new(FakeRunner::new());
        let deps = test_deps(&root, runner.clone());
        let opts = Options {
            dry_run: true,
            skip: vec!["journal".to_string()],
            ..Options::default()
        };
        let mut out = Vec::new();
        let summary = run_with(&opts, &deps, &mut out).expect("dry run");
        assert_eq!(summary, "Dry run — nothing executed.");
        assert_eq!(
            String::from_utf8_lossy(&out),
            "\n  Optimize plan\n  [run]   apt-get update && apt-get autoremove --purge\n  [skip]  journalctl --vacuum-size=500M\n  [run]   update icon/mime/font caches\n  Dry run — nothing executed.\n"
        );
        assert!(runner.invocations().is_empty());
        fs::remove_dir_all(&root).ok();
    }
}
