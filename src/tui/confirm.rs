//! YES/NO confirm dialog — port of `internal/ui/confirm.go`.
//!
//! Defaults to NO (cursor=1) to prevent accidental destructive actions.
//! Keys: ←/→/h/l/tab toggle, Enter confirms, q/ctrl+c quits.

use crossterm::event::KeyModifiers;
use ratatui::Frame;
use ratatui::layout::Rect;
use ratatui::text::{Line, Span, Text};
use ratatui::widgets::Paragraph;

use super::styles;

/// Confirm dialog state. `cursor` 0 = YES, 1 = NO. Defaults to NO (1).
#[derive(Clone, Debug)]
pub struct Confirm {
    pub prompt: String,
    pub cursor: usize,        // 0=YES 1=NO
    pub result: Option<bool>, // None = still running
}

impl Confirm {
    /// Create a new confirm dialog defaulting to NO.
    pub fn new(prompt: impl Into<String>) -> Self {
        Self {
            prompt: prompt.into(),
            cursor: 1, // default to NO
            result: None,
        }
    }

    /// Handle a key. Returns true if the dialog is done (quit or confirmed).
    pub fn handle_key(&mut self, key: crossterm::event::KeyCode) -> bool {
        use crossterm::event::KeyCode;
        match key {
            KeyCode::Left
            | KeyCode::Char('h')
            | KeyCode::Tab
            | KeyCode::Right
            | KeyCode::Char('l') => {
                self.cursor = 1 - self.cursor;
                false
            }
            KeyCode::Enter => {
                self.result = Some(self.cursor == 0);
                true
            }
            KeyCode::Char('q') => {
                self.result = Some(false);
                true
            }
            _ => false,
        }
    }

    /// Handle a key event (with modifiers). Returns true if done.
    /// Go: ctrl+c and q quit. Esc is NOT a quit key in confirm (M6 fix).
    pub fn handle_key_event(&mut self, key: crossterm::event::KeyEvent) -> bool {
        if super::is_ctrl_c(&key) {
            self.result = Some(false);
            return true;
        }
        // Modified keys map to tea names like "ctrl+h"/"alt+x" — no arm
        // matches them, so they are ignored like Go's default case.
        if key
            .modifiers
            .intersects(KeyModifiers::CONTROL | KeyModifiers::ALT)
        {
            return false;
        }
        self.handle_key(key.code)
    }

    /// Render the confirm dialog into the frame area.
    pub fn render(&self, frame: &mut Frame, area: Rect) {
        let yes_style = if self.cursor == 0 {
            styles::button_on()
        } else {
            styles::button_off()
        };
        let no_style = if self.cursor == 1 {
            styles::button_on()
        } else {
            styles::button_off()
        };

        // Build the view — mirrors Go's confirmModel.View():
        //   "\n\n  {prompt}\n\n  YES  NO\n\n\n  ←/→ navigate  •  Enter: confirm  •  q: quit\n"
        let nav = "←/→ navigate  •  Enter: confirm  •  q: quit";

        let mut lines: Vec<Line> = Vec::new();
        lines.push(Line::raw(""));
        lines.push(Line::raw(""));
        lines.push(Line::raw(format!("  {}", self.prompt)));
        lines.push(Line::raw(""));
        // Render buttons as styled spans (like Go's StyleButtonOn/Off with
        // Padding(0,2) → "  YES  " / "  NO  ").
        lines.push(Line::from(vec![
            Span::raw("  "),
            Span::styled(" YES ", yes_style),
            Span::raw("  "),
            Span::styled(" NO ", no_style),
        ]));
        lines.push(Line::raw(""));
        lines.push(Line::raw(""));
        lines.push(Line::raw(format!("  {nav}")));
        lines.push(Line::raw(""));

        frame.render_widget(Paragraph::new(Text::from(lines)), area);
    }

    /// Get the result if the dialog has concluded.
    pub fn result(&self) -> Option<bool> {
        self.result
    }
}

/// Render styled YES/NO button strings — port of Go's `RenderButtons`.
/// Returns (yes, no) styled for the given cursor position.
pub fn render_buttons(cursor: usize) -> (&'static str, &'static str) {
    // Returns the button labels; styling is applied by the caller via
    // styles::button_on() / styles::button_off().
    let _ = cursor;
    ("YES", "NO")
}

#[cfg(test)]
mod tests {
    use super::*;
    use crossterm::event::KeyCode;
    use ratatui::Terminal;
    use ratatui::backend::TestBackend;

    #[test]
    fn defaults_to_no() {
        let c = Confirm::new("Remove everything?");
        assert_eq!(c.cursor, 1);
        assert_eq!(c.result, None);
    }

    #[test]
    fn enter_yes() {
        let mut c = Confirm::new("test");
        c.cursor = 0;
        assert!(c.handle_key(KeyCode::Enter));
        assert_eq!(c.result, Some(true));
    }

    #[test]
    fn enter_no() {
        let mut c = Confirm::new("test");
        c.cursor = 1;
        assert!(c.handle_key(KeyCode::Enter));
        assert_eq!(c.result, Some(false));
    }

    #[test]
    fn toggle_left_right() {
        let mut c = Confirm::new("test");
        assert_eq!(c.cursor, 1);
        c.handle_key(KeyCode::Left);
        assert_eq!(c.cursor, 0);
        c.handle_key(KeyCode::Right);
        assert_eq!(c.cursor, 1);
        c.handle_key(KeyCode::Tab);
        assert_eq!(c.cursor, 0);
    }

    #[test]
    fn quit_returns_false() {
        let mut c = Confirm::new("test");
        assert!(c.handle_key(KeyCode::Char('q')));
        assert_eq!(c.result, Some(false));
    }

    #[test]
    fn render_snapshot() {
        let mut c = Confirm::new("Remove everything?");
        c.cursor = 0; // YES active
        let backend = TestBackend::new(60, 10);
        let mut terminal = Terminal::new(backend).unwrap();
        terminal.draw(|f| c.render(f, f.area())).unwrap();
        let buffer = terminal.backend().buffer();
        let content: String = buffer.content().iter().map(|c| c.symbol()).collect();
        assert!(content.contains("Remove everything?"));
        assert!(content.contains("YES"));
        assert!(content.contains("NO"));
    }
}
