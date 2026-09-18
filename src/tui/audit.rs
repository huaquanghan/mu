//! Audit wizard driver — the ratatui runtime for
//! [`crate::audit::model`]'s phase machine, the direct port of Go's
//! `runWizard`/`tea.NewProgram` half (scan → findings → confirm → apply →
//! rescore).
//!
//! The loop owns the alt-screen terminal and interprets [`Cmd`]s the way
//! bubbletea runs `tea.Cmd`s — on background threads delivering [`Msg`]s
//! through a channel:
//! - `Scan`/`Rescore` run `collect_in` + `build_report` off-thread.
//! - `Apply` runs `apply_in` off-thread (Go ran it as a cmd too). Go passed
//!   `os.Stdout` so apply progress bled into the alt-screen; here output is
//!   captured into a shared buffer the caller prints after the wizard
//!   exits — same transcript, no frame corruption.
//! - `before` (the pre-apply report Go threads through `appliedMsg` →
//!   `rescoredMsg`) is held by the driver between the Apply and Rescore
//!   dispatches.
//!
//! Keys map to the `msg.String()` names the ported `handle_key` switches
//! on. Go's audit views have no spinner, so there is no tick feed.

use std::io::{IsTerminal, Write};
use std::sync::mpsc;
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::Duration;

use crossterm::event::{self, Event, KeyCode, KeyModifiers};
use crossterm::execute;
use crossterm::terminal::{EnterAlternateScreen, enable_raw_mode};
use ratatui::Terminal;
use ratatui::backend::CrosstermBackend;
use ratatui::widgets::Paragraph;

use super::{AltScreenGuard, RawModeGuard, is_ctrl_c};
use crate::audit::apply::apply_in;
use crate::audit::model::{AuditModel, Cmd, Msg, new_audit_model};
use crate::audit::{ApplyResult, Deps, Options, build_report, collect};
use crate::error::{Result, msg};
use crate::oplog;

/// `runWizard` — the interactive audit workflow. Returns the apply results
/// plus the captured apply transcript; the caller prints the transcript
/// after the alt-screen is torn down and maps `count_apply_errors` to the
/// exit code (`fmt.Errorf("%d audit action(s) failed", n)` in Go).
pub fn run_audit_wizard(opts: &Options) -> Result<(Vec<ApplyResult>, Vec<u8>)> {
    if !std::io::stdout().is_terminal() {
        return msg("could not open a new TTY: open /dev/tty: no such device or address");
    }

    enable_raw_mode()?;
    let _raw_guard = RawModeGuard;
    let mut stdout = std::io::stdout();
    execute!(stdout, EnterAlternateScreen)?;
    let _alt_guard = AltScreenGuard;
    let backend = CrosstermBackend::new(stdout);
    let mut terminal = Terminal::new(backend)?;

    let mut m = new_audit_model(opts.clone());
    let (tx, rx) = mpsc::channel::<Msg>();
    let mut ctx = WizardCtx {
        pending_before: None,
        apply_out: Arc::new(Mutex::new(Vec::new())),
    };
    let mut quit = false;

    // `Init` — the scan cmd.
    exec_cmd(m.init(), &mut m, &mut ctx, &tx, opts);

    while !quit {
        terminal.draw(|f| f.render_widget(Paragraph::new(m.view_lines()), f.area()))?;

        // Drain finished cmds before waiting on input.
        while let Ok(msg) = rx.try_recv() {
            exec_cmd(m.update(msg), &mut m, &mut ctx, &tx, opts);
            quit |= m.done;
        }
        if quit {
            break;
        }

        if event::poll(Duration::from_millis(100))? {
            match event::read()? {
                Event::Key(key) => {
                    if let Some(name) = map_key(&key) {
                        exec_cmd(m.update(Msg::Key(name)), &mut m, &mut ctx, &tx, opts);
                        quit |= m.done;
                    }
                }
                Event::Resize(w, h) => {
                    m.update(Msg::WindowSize {
                        width: w as usize,
                        height: h as usize,
                    });
                }
                _ => {}
            }
        }
    }

    let results = std::mem::take(&mut m.results);
    let transcript = ctx.apply_out.lock().map(|b| b.clone()).unwrap_or_default();
    Ok((results, transcript))
}

/// Driver-held context Go carries implicitly through the cmd closures.
struct WizardCtx {
    /// `msg.before` — the pre-apply report handed from `appliedMsg` into
    /// `rescoredMsg`.
    pending_before: Option<crate::audit::Report>,
    /// Apply progress — Go's `os.Stdout`, captured for post-exit printing.
    apply_out: Arc<Mutex<Vec<u8>>>,
}

/// `io.Writer` over the shared apply transcript.
struct SharedOut(Arc<Mutex<Vec<u8>>>);

impl Write for SharedOut {
    fn write(&mut self, buf: &[u8]) -> std::io::Result<usize> {
        let mut guard = self
            .0
            .lock()
            .map_err(|_| std::io::Error::other("apply log poisoned"))?;
        guard.extend_from_slice(buf);
        Ok(buf.len())
    }
    fn flush(&mut self) -> std::io::Result<()> {
        Ok(())
    }
}

/// Run one `tea.Cmd` equivalent — the background half the model can't run
/// itself. `m` is only read for the inputs the cmd captures.
fn exec_cmd(
    cmd: Cmd,
    m: &mut AuditModel,
    ctx: &mut WizardCtx,
    tx: &mpsc::Sender<Msg>,
    opts: &Options,
) {
    match cmd {
        Cmd::None => {}
        Cmd::Quit => m.done = true,
        Cmd::Scan | Cmd::Rescore => {
            let tx = tx.clone();
            let include = opts.include.clone();
            let before = if cmd == Cmd::Rescore {
                ctx.pending_before.take().unwrap_or_default()
            } else {
                crate::audit::Report::default()
            };
            thread::spawn(move || {
                // Go's cmd discards the progress callback (`Collect(nil)`).
                // catch_unwind: a panicking worker would otherwise never
                // send its msg and the wizard hangs (Go's goroutine panic
                // crashes the process; surfacing a scan error is gentler).
                let work = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
                    let snap = collect(None);
                    let rep = build_report(&snap, &include);
                    (snap, rep)
                }));
                let (snap, rep) = work.unwrap_or_else(|_| {
                    let mut snap = crate::audit::Snapshot::default();
                    snap.scan_errors
                        .push("internal error: scan worker panicked".to_string());
                    let rep = build_report(&snap, &include);
                    (snap, rep)
                });
                let msg = if cmd == Cmd::Scan {
                    Msg::Scanned { snap, rep }
                } else {
                    Msg::Rescored { before, after: rep }
                };
                let _ = tx.send(msg);
            });
        }
        Cmd::Apply => {
            let tx = tx.clone();
            let selected = m.selected_findings();
            let before = m.report.clone();
            ctx.pending_before = Some(before.clone());
            let dry = opts.dry_run;
            let debug = opts.debug;
            let out = SharedOut(ctx.apply_out.clone());
            thread::spawn(move || {
                // Go: InitLogger inside the cmd + defer CloseLogger.
                if std::env::var_os("MU_NO_OPLOG").as_deref() != Some(std::ffi::OsStr::new("1"))
                    && let Err(e) = oplog::init_logger_at(&Deps::real().clean.data_home)
                    && debug
                {
                    eprintln!("warn: log: {e}");
                }
                let mut out = std::io::BufWriter::new(out);
                // catch_unwind: a panicking apply worker would otherwise
                // never send Msg::Applied and the wizard hangs at
                // "Applying fixes…" with ctrl+c intentionally swallowed.
                let results = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
                    apply_in(&selected, dry, debug, &Deps::real(), &mut out)
                }))
                .unwrap_or_else(|_| {
                    vec![ApplyResult {
                        finding_id: "internal".to_string(),
                        action: "apply".to_string(),
                        err: Some(crate::error::Error::Msg(
                            "apply worker panicked".to_string(),
                        )),
                        skipped: false,
                    }]
                });
                let _ = out.flush();
                oplog::close_logger();
                let _ = tx.send(Msg::Applied { results, before });
            });
        }
    }
}

/// Crossterm key → the `tea.KeyMsg` `msg.String()` names `handle_key`
/// switches on (`"ctrl+c"`, `"q"`, `"up"`, `"k"`, `" "`, `"a"`, `"enter"`,
/// `"left"`, `"h"`, …). Unhandled keys return None (Go's default arm).
fn map_key(key: &event::KeyEvent) -> Option<String> {
    if is_ctrl_c(key) {
        return Some("ctrl+c".to_string());
    }
    // Modified keys map to tea names like "ctrl+q"/"alt+x" — no model arm
    // matches them, so they are ignored like Go's default case.
    if key
        .modifiers
        .intersects(KeyModifiers::CONTROL | KeyModifiers::ALT)
    {
        return None;
    }
    match key.code {
        KeyCode::Enter => Some("enter".to_string()),
        KeyCode::Up => Some("up".to_string()),
        KeyCode::Down => Some("down".to_string()),
        KeyCode::Left => Some("left".to_string()),
        KeyCode::Right => Some("right".to_string()),
        KeyCode::Tab => Some("tab".to_string()),
        KeyCode::BackTab => Some("shift+tab".to_string()),
        KeyCode::Char(' ') => Some(" ".to_string()),
        KeyCode::Char(c) if !c.is_control() => Some(c.to_string()),
        _ => None,
    }
}
