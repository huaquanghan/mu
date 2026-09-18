//! Shared mu palette — direct port of `internal/ui/styles.go`.
//!
//! Every screen draws from these tokens so styling stays consistent; no
//! ad-hoc ANSI codes scattered through views. Color values match the Go
//! constants exactly.

use ratatui::style::{Color, Modifier, Style};

// --- Color constants (Go: ColorPrimary, ColorDanger, …) ---

pub const PRIMARY: Color = Color::Rgb(0x00, 0x97, 0xA7); // mu cyan
pub const DANGER: Color = Color::Rgb(0xEF, 0x44, 0x44); // failures, errors
pub const SUCCESS: Color = Color::Rgb(0x22, 0xC5, 0x5E); // completed items
pub const INACTIVE: Color = Color::Rgb(0x37, 0x41, 0x51); // inactive button bg
pub const MUTED: Color = Color::Rgb(0x9C, 0xA3, 0xAF); // inactive button fg
pub const WHITE: Color = Color::Rgb(0xFF, 0xFF, 0xFF);

// --- Style helpers (Go: StyleBoldPrimary, StyleFaint, …) ---

/// `StyleBoldPrimary` — titles and highlights.
pub fn bold_primary() -> Style {
    Style::default().add_modifier(Modifier::BOLD).fg(PRIMARY)
}

/// `StyleFaint` — secondary text and hints.
pub fn faint() -> Style {
    Style::default().add_modifier(Modifier::DIM)
}

/// `StyleButtonOn` — active button (primary bg, white fg).
pub fn button_on() -> Style {
    Style::default()
        .add_modifier(Modifier::BOLD)
        .bg(PRIMARY)
        .fg(WHITE)
}

/// `StyleButtonOff` — inactive button (inactive bg, muted fg).
pub fn button_off() -> Style {
    Style::default()
        .add_modifier(Modifier::BOLD)
        .bg(INACTIVE)
        .fg(MUTED)
}

/// `MarkSuccess` — green check prefix.
pub fn mark_success(text: &str) -> String {
    format!("✓ {text}")
}

/// `MarkError` — red cross prefix.
pub fn mark_error(text: &str) -> String {
    format!("✗ {text}")
}

// --- Status dashboard colors (Go: internal/status/view.go) ---

pub const AMBER: Color = Color::Rgb(0xF5, 0x9E, 0x0B);
pub const EMPTY: Color = Color::Rgb(0x37, 0x41, 0x51);

/// `usageColor` — stress color for fill percentage (higher = worse).
pub fn usage_color(pct: f64) -> Color {
    if pct >= 85.0 {
        DANGER
    } else if pct >= 60.0 {
        AMBER
    } else {
        SUCCESS
    }
}

/// `healthColor` — color for a 0–100 health score (higher = better).
pub fn health_color(score: i64) -> Color {
    if score < 30 {
        DANGER
    } else if score < 60 {
        AMBER
    } else {
        SUCCESS
    }
}

/// `healthLabel` — short qualitative label for a health score.
pub fn health_label(score: i64) -> &'static str {
    if score < 30 {
        "Poor"
    } else if score < 60 {
        "Fair"
    } else {
        "Good"
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn usage_color_thresholds() {
        assert_eq!(usage_color(0.0), SUCCESS);
        assert_eq!(usage_color(59.9), SUCCESS);
        assert_eq!(usage_color(60.0), AMBER);
        assert_eq!(usage_color(84.9), AMBER);
        assert_eq!(usage_color(85.0), DANGER);
        assert_eq!(usage_color(100.0), DANGER);
    }

    #[test]
    fn health_color_thresholds() {
        assert_eq!(health_color(0), DANGER);
        assert_eq!(health_color(29), DANGER);
        assert_eq!(health_color(30), AMBER);
        assert_eq!(health_color(59), AMBER);
        assert_eq!(health_color(60), SUCCESS);
        assert_eq!(health_color(100), SUCCESS);
    }

    #[test]
    fn health_label_thresholds() {
        assert_eq!(health_label(0), "Poor");
        assert_eq!(health_label(29), "Poor");
        assert_eq!(health_label(30), "Fair");
        assert_eq!(health_label(59), "Fair");
        assert_eq!(health_label(60), "Good");
        assert_eq!(health_label(100), "Good");
    }
}
