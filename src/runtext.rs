//! Port of `internal/ui/run.go` — [`Run`] renders a subcommand run in mu's
//! consistent style: section headers, content lines, a spinner during long
//! work, and a final summary.
//!
//! This wave renders plain text unconditionally: lipgloss emits zero ANSI
//! bytes without a TTY, so `section`/`summary` (`StyleBoldPrimary`) and
//! `faint` (`StyleFaint`) are byte-identical to the Go non-terminal output.
//! Deferred to the tui-port wave: the TTY-styled variants and the animated
//! bubbletea spinner (✅/❌ end frames) — `spinner` keeps the plain
//! `label...` fallback even on a terminal for now (Go's `Spinner` checks
//! `os.Stdout`, not the writer, when it picks the animated path).

use std::fmt::Arguments;
use std::io::Write;

use crate::error::Result;

/// `ui.Run` — a run renderer writing to `w` (`os.Stdout` for CLI runs).
pub(crate) struct Run<'a> {
    w: &'a mut dyn Write,
}

impl<'a> Run<'a> {
    /// `NewRun`.
    pub(crate) fn new(w: &'a mut dyn Write) -> Self {
        Self { w }
    }

    /// `Section` — a blank line, then the title line. Bold + primary color
    /// on a TTY in Go; plain text here.
    #[allow(dead_code)] // part of the ui.Run surface; clean uses the rest
    pub(crate) fn section(&mut self, title: &str) -> Result<()> {
        writeln!(self.w, "\n  {title}")?;
        Ok(())
    }

    /// `Line` — an indented content line: `"  " + Sprintf(format, args...)`.
    pub(crate) fn line(&mut self, args: Arguments<'_>) -> Result<()> {
        writeln!(self.w, "  {args}")?;
        Ok(())
    }

    /// `Faint` — an indented, dimmed note line; plain without a TTY.
    pub(crate) fn faint(&mut self, args: Arguments<'_>) -> Result<()> {
        self.line(args)
    }

    /// `Summary` — a blank line, then the (bold-on-TTY) summary line.
    pub(crate) fn summary(&mut self, text: &str) -> Result<()> {
        writeln!(self.w, "\n  {text}")?;
        Ok(())
    }

    /// `Spinner` — Go animates `label` via bubbletea on a terminal (ending
    /// on ✅/❌ per `fn`'s error) and falls back to a plain `"  label..."`
    /// line on a pipe. The animation is deferred, so this is always the
    /// plain fallback: print the line, run `f`, return its result. The
    /// label-line write error is dropped exactly like Go's `fmt.Fprintln` —
    /// the work still runs.
    pub(crate) fn spinner<T>(&mut self, label: &str, f: impl FnOnce() -> Result<T>) -> Result<T> {
        let _ = writeln!(self.w, "  {label}...");
        f()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::error::Error;

    // run_test.go: TestRunPlainRendering — non-terminal output degrades to
    // plain text: sections, lines, summary, and the spinner fallback all
    // render labels without a live terminal. Asserted byte-exact (stronger
    // than the Go contains-checks) since the non-TTY bytes are the contract.
    #[test]
    fn run_plain_rendering() {
        let mut buf = Vec::new();
        {
            let mut r = Run::new(&mut buf);
            r.section("Scanning system").unwrap();
            r.line(format_args!("{:<40} {}", "User cache", "1.2 GB"))
                .unwrap();
            r.faint(format_args!("This is a DRY RUN")).unwrap();
            r.summary("Potential space to free: 1.2 GB").unwrap();
            r.spinner("Cleaning User cache", || Ok(())).unwrap();
        }
        let out = String::from_utf8(buf).unwrap();
        let want = format!(
            "\n  Scanning system\n  {:<40} 1.2 GB\n  This is a DRY RUN\n\n  Potential space to free: 1.2 GB\n  Cleaning User cache...\n",
            "User cache"
        );
        assert_eq!(out, want);
        // No ANSI bytes without a TTY.
        assert!(!out.contains('\x1b'), "unexpected ANSI in {out:?}");
    }

    // run_test.go: TestRunSpinnerRunsFn.
    #[test]
    fn run_spinner_runs_fn() {
        let mut buf = Vec::new();
        let mut ran = false;
        {
            let mut r = Run::new(&mut buf);
            r.spinner("Working", || -> Result<()> {
                ran = true;
                Ok(())
            })
            .expect("spinner returned unexpected error");
        }
        assert!(ran, "spinner did not run the work function");
        assert_eq!(String::from_utf8(buf).unwrap(), "  Working...\n");
    }

    // run_test.go: TestRunSpinnerPropagatesError.
    #[test]
    fn run_spinner_propagates_error() {
        let mut buf = Vec::new();
        let got = {
            let mut r = Run::new(&mut buf);
            r.spinner("Working", || -> Result<()> {
                Err(Error::Msg("boom".to_string()))
            })
        };
        assert_eq!(got.unwrap_err().to_string(), "boom");
    }
}
