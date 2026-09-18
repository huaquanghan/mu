//! Wizard state machine — the non-driver half of `internal/audit/model.go`.
//!
//! The Bubble Tea program itself (`tea.NewProgram`, `Init`, key input, the
//! `tea.Exec`-style apply dispatch, and all lipgloss styling) is deferred to
//! the tui-port wave; `crate::audit::run` currently stubs the wizard with
//! `mu: audit TUI not implemented`. What is ported here, because
//! `options_test.go` exercises it, is the model's transition logic: the
//! phase machine (scan → findings → confirm → apply → rescore), selection
//! accounting, the YES/NO confirm gate (default NO), and the
//! ctrl-c-during-apply guard. Go's `tea.Cmd` returns become [`Cmd`] intents
//! a future driver executes; `view()` renders the same text unstyled (a
//! non-TTY lipgloss render is plain anyway).

use std::collections::HashMap;

use ratatui::text::{Line, Span};

use super::Options;
use super::apply::ApplyResult;
use super::findings::{Finding, Report};
use super::scan::Snapshot;
use crate::size;
use crate::tui::styles;

/// `phase` — the wizard's current screen.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub(crate) enum Phase {
    Scan,
    Findings,
    Confirm,
    Apply,
    Rescore,
}

/// `Cmd` — the side effects Go returns as `tea.Cmd`. The deferred TUI
/// driver interprets these; `handle_key`/`update` only produce the intent.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub(crate) enum Cmd {
    /// Go's nil cmd.
    None,
    /// `tea.Quit`.
    Quit,
    /// `startScan` — `Collect(nil)` + `BuildReport(snap, opts.Include)`,
    /// answered with [`Msg::Scanned`].
    Scan,
    /// The phaseConfirm YES cmd — `InitLogger` (warn only in debug), then
    /// `Apply(selectedFindings(), opts.DryRun, opts.Debug, os.Stdout)`,
    /// answered with [`Msg::Applied`].
    Apply,
    /// The appliedMsg cmd — re-`Collect(nil)` + `BuildReport`, answered
    /// with [`Msg::Rescored`].
    Rescore,
}

/// `tea.Msg` — the messages `Update` switches on.
#[derive(Debug)]
pub(crate) enum Msg {
    /// `tea.WindowSizeMsg`.
    WindowSize { width: usize, height: usize },
    /// `scannedMsg`.
    Scanned { snap: Snapshot, rep: Report },
    /// `appliedMsg`.
    Applied {
        results: Vec<ApplyResult>,
        before: Report,
    },
    /// `rescoredMsg`.
    Rescored { before: Report, after: Report },
    /// `tea.KeyMsg` — `msg.String()` value.
    Key(String),
}

/// `auditModel` — the wizard's state.
#[derive(Debug)]
pub(crate) struct AuditModel {
    pub phase: Phase,
    pub opts: Options,
    pub snap: Snapshot,
    pub report: Report,
    pub after: Report,
    pub findings: Vec<Finding>,
    /// Cursor over `findings` — always lands on a selectable row via
    /// `first`/`next`/`prev_selectable`.
    pub cursor: usize,
    /// `map[int]bool` — selected indices into `findings`.
    pub selected: HashMap<usize, bool>,
    /// 0=YES 1=NO (default NO).
    pub confirm: usize,
    pub applying: bool,
    pub results: Vec<ApplyResult>,
    pub width: usize,
    pub height: usize,
    pub done: bool,
    pub status: String,
}

/// `newAuditModel`.
pub(crate) fn new_audit_model(opts: Options) -> AuditModel {
    AuditModel {
        phase: Phase::Scan,
        opts,
        snap: Snapshot::default(),
        report: Report::default(),
        after: Report::default(),
        findings: Vec::new(),
        cursor: 0,
        selected: HashMap::new(),
        confirm: 1, // default NO
        applying: false,
        results: Vec::new(),
        width: 80,
        height: 24,
        done: false,
        status: String::new(),
    }
}

impl AuditModel {
    /// `Init` — the first command the program runs.
    pub(crate) fn init(&self) -> Cmd {
        Cmd::Scan
    }

    /// `Update` — message handling for every phase.
    pub(crate) fn update(&mut self, msg: Msg) -> Cmd {
        match msg {
            Msg::WindowSize { width, height } => {
                self.width = width;
                self.height = height;
                Cmd::None
            }
            Msg::Scanned { snap, rep } => {
                self.snap = snap;
                self.report = rep;
                self.findings = self.report.findings.clone();
                self.selected = HashMap::new();
                for (i, f) in self.findings.iter().enumerate() {
                    if f.selectable && f.default_selected {
                        self.selected.insert(i, true);
                    }
                }
                self.cursor = first_selectable(&self.findings, 0);
                self.phase = Phase::Findings;
                Cmd::None
            }
            Msg::Applied { results, before } => {
                self.results = results;
                self.applying = false;
                self.phase = Phase::Rescore;
                let _ = before; // carried into Msg::Rescored by the driver
                Cmd::Rescore
            }
            Msg::Rescored { before, after } => {
                self.after = after;
                self.report = before;
                Cmd::None
            }
            Msg::Key(key) => self.handle_key(&key),
        }
    }

    /// `handleKey` — per-phase key handling (`msg.String()` values).
    pub(crate) fn handle_key(&mut self, key: &str) -> Cmd {
        match self.phase {
            Phase::Scan => {
                if key == "ctrl+c" || key == "q" {
                    self.done = true;
                    return Cmd::Quit;
                }
            }
            Phase::Findings => match key {
                "ctrl+c" | "q" => {
                    self.done = true;
                    return Cmd::Quit;
                }
                "up" | "k" => self.cursor = prev_selectable(&self.findings, self.cursor),
                "down" | "j" => self.cursor = next_selectable(&self.findings, self.cursor),
                " " => {
                    if self.cursor < self.findings.len() && self.findings[self.cursor].selectable {
                        let on = self.selected.get(&self.cursor).copied().unwrap_or(false);
                        self.selected.insert(self.cursor, !on);
                    }
                }
                "a" => {
                    // toggle all safe (non-opt-in selectable)
                    let mut all_on = true;
                    for (i, f) in self.findings.iter().enumerate() {
                        if f.selectable
                            && !f.opt_in
                            && !self.selected.get(&i).copied().unwrap_or(false)
                        {
                            all_on = false;
                            break;
                        }
                    }
                    for (i, f) in self.findings.iter().enumerate() {
                        if f.selectable && !f.opt_in {
                            self.selected.insert(i, !all_on);
                        }
                    }
                }
                "enter" => {
                    if self.count_selected() == 0 {
                        self.status = "Select at least one finding, or q to quit.".to_string();
                        return Cmd::None;
                    }
                    self.status = String::new();
                    self.phase = Phase::Confirm;
                    self.confirm = 1;
                }
                _ => {}
            },
            Phase::Confirm => match key {
                "ctrl+c" | "q" => self.phase = Phase::Findings,
                "left" | "h" | "right" | "l" | "tab" => self.confirm = 1 - self.confirm,
                "enter" => {
                    if self.confirm != 0 {
                        self.phase = Phase::Findings;
                        return Cmd::None;
                    }
                    // YES — Go's returned cmd opens the ops log, runs
                    // `Apply(selected, dry, debug, os.Stdout)`, and answers
                    // `appliedMsg{results, before}`.
                    self.phase = Phase::Apply;
                    self.applying = true;
                    return Cmd::Apply;
                }
                _ => {}
            },
            Phase::Apply => {
                if key == "ctrl+c" {
                    self.status = "Active maintenance cannot be interrupted safely; \
                                   waiting for completion."
                        .to_string();
                    return Cmd::None;
                }
            }
            Phase::Rescore => {
                if key == "enter" || key == "q" || key == "ctrl+c" {
                    self.done = true;
                    return Cmd::Quit;
                }
            }
        }
        Cmd::None
    }

    /// `countSelected` — selected rows that are selectable.
    pub(crate) fn count_selected(&self) -> usize {
        self.selected
            .iter()
            .filter(|(i, on)| **on && **i < self.findings.len() && self.findings[**i].selectable)
            .count()
    }

    /// `selectedBytes`.
    pub(crate) fn selected_bytes(&self) -> i64 {
        self.selected
            .iter()
            .filter(|(i, on)| **on && **i < self.findings.len())
            .map(|(i, _)| self.findings[*i].bytes)
            .sum()
    }

    /// `selectedFindings`.
    pub(crate) fn selected_findings(&self) -> Vec<Finding> {
        self.findings
            .iter()
            .enumerate()
            .filter(|(i, f)| self.selected.get(i).copied().unwrap_or(false) && f.selectable)
            .map(|(_, f)| f.clone())
            .collect()
    }

    /// `View` — the same text Go renders, unstyled (lipgloss emits plain
    /// bytes without a TTY; the styled TTY render is tui-port work).
    pub(crate) fn view(&self) -> String {
        match self.phase {
            Phase::Scan => "\n\n  mu audit\n\n  🔍 Scanning system…\n\n\n  q to quit\n".to_string(),
            Phase::Findings => self.view_findings(),
            Phase::Confirm => self.view_confirm(),
            Phase::Apply => {
                let mut notice = "  Please wait.".to_string();
                if !self.status.is_empty() {
                    notice.push_str("\n  ");
                    notice.push_str(&self.status);
                }
                format!(
                    "\n\n  Applying fixes…\n\n{notice}\n\n\n  active package operations are allowed to finish\n"
                )
            }
            Phase::Rescore => self.view_rescore(),
        }
    }

    /// `viewFindings`.
    fn view_findings(&self) -> String {
        let mut b = String::new();
        b.push_str("\n\n  mu audit — findings\n");
        b.push_str(&format!(
            "  Health {}/100  •  Disk / {:.0}% free  •  Reclaimable {}\n\n",
            self.report.health,
            self.report.disk_free_pct_root,
            size::human_size(self.report.reclaimable_bytes)
        ));

        if self.findings.is_empty() {
            b.push_str("  ✅ No issues found.\n\n\n");
            b.push_str("  q to quit\n");
            return b;
        }

        for (i, f) in self.findings.iter().enumerate() {
            let mark = if f.selectable {
                if self.selected.get(&i).copied().unwrap_or(false) {
                    "✓"
                } else {
                    " "
                }
            } else {
                "·"
            };
            let mut line = format!("{} [{}] {}", mark, f.severity.as_str(), f.title);
            if f.bytes > 0 {
                line.push_str("  ");
                line.push_str(&size::human_size(f.bytes));
            }
            if f.opt_in {
                line.push_str(" (opt-in)");
            }
            if !f.selectable {
                line.push_str(" (info)");
            }
            if i == self.cursor {
                b.push_str(&format!("  ▶ {line}\n"));
            } else {
                b.push_str(&format!("    {line}\n"));
            }
        }

        b.push('\n');
        b.push_str(&format!(
            "  Selected: {}  •  ~{}\n",
            self.count_selected(),
            size::human_size(self.selected_bytes())
        ));
        if !self.status.is_empty() {
            b.push_str("  ");
            b.push_str(&self.status);
            b.push('\n');
        }
        b.push_str("\n\n");
        b.push_str("  j/k move  •  space toggle  •  a safe-all  •  enter continue  •  q quit\n");
        b
    }

    /// `viewConfirm` — byte-exact text for tests; the TUI renders
    /// `view_confirm_lines` instead (styled YES/NO armed state).
    fn view_confirm(&self) -> String {
        let mut b = String::new();
        b.push_str("\n\n  Confirm apply\n\n");
        if self.opts.dry_run {
            b.push_str("  Mode: DRY RUN (no changes)\n\n");
        }
        b.push_str("  Will apply:\n");
        for f in self.selected_findings() {
            b.push_str(&format!("    • {} ({})\n", f.title, f.action));
        }
        b.push_str("\n  Sudo may be required for apt/snap/kernel/journal actions.\n");
        b.push_str("  User files go to trash when cleaned.\n\n");
        let (yes, no) = ("YES", "NO");
        b.push_str(&format!("  {yes}  {no}\n\n\n"));
        b.push_str("  ←/→  •  Enter  •  q back\n");
        b
    }

    /// `viewConfirm` as styled lines — `ui.RenderButtons` highlights the
    /// armed button (`self.confirm`: 0 = YES, 1 = NO). The string `view`
    /// stays byte-exact for tests; the TUI driver renders this so the
    /// armed destructive choice is visible.
    pub(crate) fn view_confirm_lines(&self) -> Vec<Line<'static>> {
        let (yes_style, no_style) = if self.confirm == 0 {
            (styles::button_on(), styles::button_off())
        } else {
            (styles::button_off(), styles::button_on())
        };
        let mut lines = vec![
            Line::raw(""),
            Line::raw(""),
            Line::styled("  Confirm apply", styles::bold_primary()),
            Line::raw(""),
        ];
        if self.opts.dry_run {
            lines.push(Line::raw("  Mode: DRY RUN (no changes)"));
            lines.push(Line::raw(""));
        }
        lines.push(Line::raw("  Will apply:"));
        for f in self.selected_findings() {
            lines.push(Line::raw(format!("    • {} ({})", f.title, f.action)));
        }
        lines.push(Line::raw(""));
        lines.push(Line::raw(
            "  Sudo may be required for apt/snap/kernel/journal actions.",
        ));
        lines.push(Line::raw("  User files go to trash when cleaned."));
        lines.push(Line::raw(""));
        lines.push(Line::from(vec![
            Span::raw("  "),
            Span::styled(" YES ", yes_style),
            Span::raw("  "),
            Span::styled(" NO ", no_style),
        ]));
        lines.push(Line::raw(""));
        lines.push(Line::raw(""));
        lines.push(Line::styled("  ←/→  •  Enter  •  q back", styles::faint()));
        lines
    }

    /// Phase-aware styled view for the TUI driver — only `Confirm` needs
    /// Span-level styling (armed button); every other phase renders the
    /// byte-exact `view()` text unstyled.
    pub(crate) fn view_lines(&self) -> Vec<Line<'static>> {
        match self.phase {
            Phase::Confirm => self.view_confirm_lines(),
            _ => self
                .view()
                .lines()
                .map(|l| Line::raw(l.to_string()))
                .collect(),
        }
    }

    /// `viewRescore`.
    fn view_rescore(&self) -> String {
        let mut b = String::new();
        b.push_str("\n\n  Audit complete\n\n");
        let failed = super::apply::count_apply_errors(&self.results);
        if failed > 0 {
            b.push_str(&format!("  ⚠️  {failed} action(s) had errors.\n\n"));
        } else if self.opts.dry_run {
            b.push_str("  Dry run finished — nothing modified.\n\n");
        } else {
            b.push_str("  ✅ Actions finished.\n\n");
        }
        b.push_str(&format!(
            "  Health:      {} → {}\n",
            self.report.health, self.after.health
        ));
        b.push_str(&format!(
            "  Disk free:   {:.0}% → {:.0}%\n",
            self.report.disk_free_pct_root, self.after.disk_free_pct_root
        ));
        b.push_str(&format!(
            "  Reclaimable: {} → {}\n",
            size::human_size(self.report.reclaimable_bytes),
            size::human_size(self.after.reclaimable_bytes)
        ));
        b.push_str("\n\n");
        b.push_str("  Enter or q to exit\n");
        b
    }
}

/// `firstSelectable` — first selectable index at/after `from`, else the
/// first selectable overall, else 0.
fn first_selectable(fs: &[Finding], from: usize) -> usize {
    for (i, f) in fs.iter().enumerate().skip(from) {
        if f.selectable {
            return i;
        }
    }
    for (i, f) in fs.iter().enumerate() {
        if f.selectable {
            return i;
        }
    }
    0
}

/// `nextSelectable` — next selectable index after `cur`, else `cur`.
fn next_selectable(fs: &[Finding], cur: usize) -> usize {
    for (i, f) in fs.iter().enumerate().skip(cur + 1) {
        if f.selectable {
            return i;
        }
    }
    cur
}

/// `prevSelectable` — previous selectable index before `cur`, else `cur`.
fn prev_selectable(fs: &[Finding], cur: usize) -> usize {
    for i in (0..cur).rev() {
        if fs[i].selectable {
            return i;
        }
    }
    cur
}

#[cfg(test)]
mod tests {
    use super::*;

    // options_test.go: TestAuditApplyCannotReportSuccessByInterruptingActiveMaintenance
    #[test]
    fn apply_cannot_report_success_by_interrupting_active_maintenance() {
        let mut m = new_audit_model(Options::default());
        m.phase = Phase::Apply;
        let cmd = m.handle_key("ctrl+c");
        assert!(
            cmd == Cmd::None && !m.done && m.status.contains("cannot be interrupted safely"),
            "unexpected interrupt state: done={} status={:?}",
            m.done,
            m.status
        );
        assert!(
            m.view().contains("waiting for completion"),
            "active-operation notice missing: {:?}",
            m.view()
        );
    }

    // The armed YES/NO must be visible on the TTY — a plain-text render
    // would let h/l/tab silently arm YES on a destructive confirm.
    #[test]
    fn confirm_lines_expose_armed_button() {
        let mut m = new_audit_model(Options::default());
        m.phase = Phase::Confirm;

        let mut style_of = |confirm: usize| {
            m.confirm = confirm;
            m.view_confirm_lines()
                .iter()
                .flat_map(|l| l.spans.iter())
                .find(|s| s.content.trim() == "YES")
                .map(|s| s.style)
        };
        assert_ne!(
            style_of(0),
            style_of(1),
            "YES button must render differently when armed"
        );
    }
}
