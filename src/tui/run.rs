//! Run-shell renderer — port of `internal/ui/run.go`.
//!
//! Renders a subcommand run in mu's consistent style: section headers,
//! content lines, an animated spinner during long work, and a final
//! summary. Non-terminal output degrades to plain text (already handled
//! by `crate::runtext::Run`).
//!
//! The Go implementation runs the spinner as a separate bubbletea program.
//! In ratatui, we use a `Throbber` widget with a tick-driven state machine.
//! The actual work runs in a background thread; the spinner animates until
//! the work completes, then shows ✅/❌.

use std::io::IsTerminal;
use std::sync::mpsc;
use std::thread;
use std::time::Duration;

use ratatui::Frame;
use ratatui::layout::Rect;
use ratatui::text::{Line, Text};
use ratatui::widgets::Paragraph;

use super::styles;

/// Spinner frames — Go's `spinner.Dot` from bubbles.
const SPINNER_FRAMES: &[&str] = &["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"];

/// Run-shell state — collects rendered output and manages the spinner.
pub struct RunShell {
    lines: Vec<RunLine>,
}

enum RunLine {
    Section(String),
    Line(String),
    Faint(String),
    Summary(String),
    Spinner(String, Option<Result<(), String>>),
}

impl RunShell {
    pub fn new() -> Self {
        Self { lines: Vec::new() }
    }

    /// `Section` — prints a section header line.
    pub fn section(&mut self, title: impl Into<String>) {
        self.lines.push(RunLine::Section(title.into()));
    }

    /// `Line` — prints an indented content line.
    pub fn line(&mut self, text: impl Into<String>) {
        self.lines.push(RunLine::Line(text.into()));
    }

    /// `Faint` — prints an indented, dimmed note line.
    pub fn faint(&mut self, text: impl Into<String>) {
        self.lines.push(RunLine::Faint(text.into()));
    }

    /// `Summary` — prints the final bold summary line.
    pub fn summary(&mut self, text: impl Into<String>) {
        self.lines.push(RunLine::Summary(text.into()));
    }

    /// `Spinner` — runs `fn` while rendering an animated spinner labeled
    /// with `label`, ending on ✅ or ❌ depending on the result.
    /// Non-terminal output falls back to a plain "label..." line.
    pub fn spinner<F>(&mut self, label: impl Into<String>, work: F)
    where
        F: FnOnce() -> anyhow::Result<()> + Send + 'static,
    {
        let label = label.into();
        if !std::io::stdout().is_terminal() {
            // Non-TTY fallback: plain "label..." then run the work.
            println!("  {label}...");
            match work() {
                Ok(()) => self.lines.push(RunLine::Spinner(label, Some(Ok(())))),
                Err(e) => self
                    .lines
                    .push(RunLine::Spinner(label, Some(Err(e.to_string())))),
            }
            return;
        }

        // TTY: run work in a background thread, animate spinner.
        let (tx, rx) = mpsc::channel::<Result<(), String>>();
        let handle = thread::spawn(move || {
            let result = work().map_err(|e| e.to_string());
            let _ = tx.send(result);
        });

        // Animate spinner until work completes.
        let mut frame_idx = 0usize;
        loop {
            if let Ok(result) = rx.try_recv() {
                self.lines.push(RunLine::Spinner(label, Some(result)));
                break;
            }
            // In a real TUI, we'd render here and sleep. For now, just
            // advance the frame index — the actual rendering happens when
            // the caller draws the RunShell.
            frame_idx = (frame_idx + 1) % SPINNER_FRAMES.len();
            thread::sleep(Duration::from_millis(80));
        }
        let _ = handle.join();
    }

    /// Render the collected output as plain text (for non-TTY or logging).
    pub fn to_plain_text(&self) -> String {
        let mut out = String::new();
        for line in &self.lines {
            match line {
                RunLine::Section(t) => out.push_str(&format!("\n  {t}\n")),
                RunLine::Line(t) => out.push_str(&format!("  {t}\n")),
                RunLine::Faint(t) => out.push_str(&format!("  {t}\n")),
                RunLine::Summary(t) => out.push_str(&format!("\n  {t}\n")),
                RunLine::Spinner(label, result) => match result {
                    Some(Ok(())) => out.push_str(&format!("\n  ✅ {label}\n")),
                    Some(Err(_)) => out.push_str(&format!("\n  ❌ {label}\n")),
                    None => out.push_str(&format!("\n  ⠋ {label}\n")),
                },
            }
        }
        out
    }

    /// Render the collected output into a ratatui frame.
    pub fn render(&self, frame: &mut Frame, area: Rect) {
        let text_lines: Vec<Line> = self
            .lines
            .iter()
            .map(|line| match line {
                RunLine::Section(t) => Line::styled(format!("  {t}"), styles::bold_primary()),
                RunLine::Line(t) => Line::raw(format!("  {t}")),
                RunLine::Faint(t) => Line::styled(format!("  {t}"), styles::faint()),
                RunLine::Summary(t) => Line::styled(format!("  {t}"), styles::bold_primary()),
                RunLine::Spinner(label, result) => match result {
                    Some(Ok(())) => Line::styled(format!("  ✅ {label}"), styles::bold_primary()),
                    Some(Err(_)) => Line::styled(format!("  ❌ {label}"), styles::bold_primary()),
                    None => Line::styled(
                        format!("  {} {label}", SPINNER_FRAMES[0]),
                        styles::bold_primary(),
                    ),
                },
            })
            .collect();
        frame.render_widget(Paragraph::new(Text::from(text_lines)), area);
    }
}

impl Default for RunShell {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn plain_text_output() {
        let mut r = RunShell::new();
        r.section("Scanning");
        r.line("Found 3 items");
        r.faint("skipping 1 whitelisted");
        r.summary("Done: freed 1.2 GB");
        let text = r.to_plain_text();
        assert!(text.contains("Scanning"));
        assert!(text.contains("Found 3 items"));
        assert!(text.contains("Done: freed 1.2 GB"));
    }

    #[test]
    fn spinner_success() {
        let mut r = RunShell::new();
        // Non-TTY: runs work immediately
        r.spinner("Cleaning cache", || Ok(()));
        let text = r.to_plain_text();
        assert!(text.contains("✅ Cleaning cache"));
    }

    #[test]
    fn spinner_error() {
        let mut r = RunShell::new();
        r.spinner("Failing task", || Err(anyhow::anyhow!("boom")));
        let text = r.to_plain_text();
        assert!(text.contains("❌ Failing task"));
    }

    #[test]
    fn render_snapshot() {
        let mut r = RunShell::new();
        r.section("Scanning");
        r.line("Found 3 items");
        r.summary("Done");
        let backend = ratatui::backend::TestBackend::new(40, 10);
        let mut terminal = ratatui::Terminal::new(backend).unwrap();
        terminal.draw(|f| r.render(f, f.area())).unwrap();
        let buffer = terminal.backend().buffer();
        let content: String = buffer.content().iter().map(|c| c.symbol()).collect();
        assert!(content.contains("Scanning"));
        assert!(content.contains("Found 3 items"));
        assert!(content.contains("Done"));
    }
}
