//! Clean flow driver — the ratatui runtime for [`crate::clean::flow`]'s
//! state machine, the direct port of Go's `runFlow`/`tea.NewProgram` half.
//!
//! The loop owns the alt-screen terminal and interprets [`Cmd`]s in order.
//! `Scan(i)`/`Run(i)` run synchronously on the main thread (the model's
//! `scan_cmd`/`run_cmd` borrow `self`, and the `Box<dyn Fn>` closures aren't
//! `Send`), with a frame drawn after each command so the "Scanning system
//! (n of m)" progression and per-row run status stay live like Go's async
//! cmds. Queued input is drained non-blocking between commands so
//! q/ctrl+c land at the next target boundary — Go's cmds can't preempt a
//! running target either. `RunDispatch::ExecTerminal` targets (real-run
//! sudo) suspend the terminal — disable raw mode + leave alt-screen — so
//! sudo's password prompt owns the TTY, matching `tea.Exec`/
//! `ui.ExecTerminal`.
//!
//! Spinner animation is driver-fed: the model's returned `Cmd::SpinnerTick`
//! is dropped (it only signals "keep ticking"), and `Msg::SpinnerTick` is
//! sent on a 100ms cadence — on poll timeout while idle, or by wall-clock
//! while a command chain is running.

use std::collections::VecDeque;
use std::io::IsTerminal;
use std::time::{Duration, Instant};

use crossterm::event::{self, Event, KeyCode, KeyModifiers};
use crossterm::execute;
use crossterm::terminal::{
    EnterAlternateScreen, LeaveAlternateScreen, disable_raw_mode, enable_raw_mode,
};
use ratatui::Terminal;
use ratatui::backend::CrosstermBackend;
use ratatui::text::Text;
use ratatui::widgets::Paragraph;

use super::{AltScreenGuard, RawModeGuard, is_ctrl_c};
use crate::clean::flow::{Cmd, FlowModel, Key, Msg, RunDispatch};
use crate::clean::{CleanTarget, Deps, Options};
use crate::error::{Error, Result};
use crate::oplog;

// bubbles `spinner.Dot` (ui.NewSpinner) ticks at 10fps → 100ms.
const TICK: Duration = Duration::from_millis(100);

/// `runFlow` — the interactive clean workflow. Returns the summary string
/// ("Freed X", "Aborted.", …) or the run error, exactly like Go's
/// `(m.summaryOut, m.runErr)` pair.
pub fn run_clean_flow(opts: &Options, targets: Vec<CleanTarget>, deps: &Deps) -> Result<String> {
    if !std::io::stdout().is_terminal() {
        return Err(Error::Msg(
            "could not open a new TTY: open /dev/tty: no such device or address".to_string(),
        ));
    }

    // `runFlow` opens the ops log before starting the program and closes it
    // on return — same MU_NO_OPLOG + debug-warn semantics as `execute`.
    if std::env::var_os("MU_NO_OPLOG").as_deref() != Some(std::ffi::OsStr::new("1"))
        && let Err(e) = oplog::init_logger_at(&deps.data_home)
        && opts.debug
    {
        eprintln!("warn: could not open log: {e}");
    }
    let _log = CloseLoggerGuard;

    enable_raw_mode()?;
    let _raw_guard = RawModeGuard;
    let mut stdout = std::io::stdout();
    execute!(stdout, EnterAlternateScreen)?;
    let _alt_guard = AltScreenGuard;
    let backend = CrosstermBackend::new(stdout);
    let mut terminal = Terminal::new(backend)?;

    let mut m = FlowModel::new(opts.clone(), targets);
    let mut cmds: VecDeque<Cmd> = m.init().into_iter().collect();
    let mut quit = false;
    let mut last_tick = Instant::now();

    while !quit {
        if let Some(cmd) = cmds.pop_front() {
            match cmd {
                // Driver-fed — the model's returned tick is a keep-alive
                // signal, not a queue item; feeding it back would starve
                // input handling in a self-perpetuating loop.
                Cmd::SpinnerTick => {}
                Cmd::Scan(i) => {
                    let msg = m.scan_cmd(i);
                    cmds.extend(m.update(msg));
                }
                Cmd::Run(i) => {
                    let msg = if m.run_dispatch(i) == RunDispatch::ExecTerminal {
                        // tea.Exec equivalent: release the terminal so sudo
                        // can prompt on the TTY, then restore and continue.
                        suspend_terminal()?;
                        let msg = m.run_cmd(i);
                        resume_terminal(&mut terminal)?;
                        msg
                    } else {
                        m.run_cmd(i)
                    };
                    cmds.extend(m.update(msg));
                }
                Cmd::Quit => quit = true,
            }
            if quit {
                break;
            }
            if last_tick.elapsed() >= TICK {
                cmds.extend(m.update(Msg::SpinnerTick));
                last_tick = Instant::now();
            }
            // Drain queued input without stalling the command chain — a
            // queued q/ctrl+c takes effect at the next target boundary.
            while event::poll(Duration::ZERO)? {
                match event::read()? {
                    Event::Key(key) => {
                        if let Some(k) = map_key(&key) {
                            cmds.extend(m.update(Msg::Key(k)));
                        }
                    }
                    Event::Resize(w, _h) => {
                        cmds.extend(m.update(Msg::WindowSize { width: w as usize }));
                    }
                    _ => {}
                }
            }
        }

        terminal.draw(|f| f.render_widget(Paragraph::new(Text::from(m.view_lines())), f.area()))?;

        if cmds.is_empty() && !quit && !event::poll(TICK)? {
            cmds.extend(m.update(Msg::SpinnerTick));
            last_tick = Instant::now();
        }
        if cmds.is_empty() && !quit && event::poll(Duration::ZERO)? {
            match event::read()? {
                Event::Key(key) => {
                    if let Some(k) = map_key(&key) {
                        cmds.extend(m.update(Msg::Key(k)));
                    }
                }
                Event::Resize(w, _h) => {
                    cmds.extend(m.update(Msg::WindowSize { width: w as usize }));
                }
                _ => {}
            }
        }
    }

    match m.run_err {
        Some(e) => Err(e),
        None => Ok(m.summary_out),
    }
}

/// Crossterm key → the `tea.KeyMsg` strings `handleKey` switches on.
fn map_key(key: &event::KeyEvent) -> Option<Key> {
    if is_ctrl_c(key) {
        return Some(Key::CtrlC);
    }
    // Modified keys map to tea names like "ctrl+q"/"alt+x" — no arm
    // matches them, so they are ignored like Go's default case.
    if key
        .modifiers
        .intersects(KeyModifiers::CONTROL | KeyModifiers::ALT)
    {
        return None;
    }
    match key.code {
        KeyCode::Char('q') | KeyCode::Esc => Some(Key::Quit),
        KeyCode::Enter | KeyCode::Char(' ') => Some(Key::Select),
        KeyCode::Left | KeyCode::Char('h') | KeyCode::Right | KeyCode::Char('l') | KeyCode::Tab => {
            Some(Key::Flip)
        }
        _ => None,
    }
}

/// `tea.Exec` suspend half — raw mode off + leave alt-screen so the child
/// process (sudo) owns the terminal.
fn suspend_terminal() -> Result<()> {
    disable_raw_mode()?;
    execute!(std::io::stdout(), LeaveAlternateScreen)?;
    Ok(())
}

/// `tea.Exec` resume half — re-enter alt-screen + raw mode and force a full
/// redraw (the re-entered alt buffer starts blank).
fn resume_terminal(terminal: &mut Terminal<CrosstermBackend<std::io::Stdout>>) -> Result<()> {
    enable_raw_mode()?;
    execute!(std::io::stdout(), EnterAlternateScreen)?;
    terminal.clear()?;
    Ok(())
}

/// `defer utils.CloseLogger()` — closes on drop at every return path.
struct CloseLoggerGuard;

impl Drop for CloseLoggerGuard {
    fn drop(&mut self) {
        oplog::close_logger();
    }
}
