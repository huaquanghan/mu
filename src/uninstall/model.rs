//! `model.go` — the uninstall package-selection model. The bubbletea
//! presentation is deferred to the tui-port wave; what lives here is the
//! headless half the Go suite drives through `Update`: the `phase` state
//! machine (search → confirm → done), the search query/backspace/cursor/
//! scroll logic, the `selected` set keyed by `pkg.Key()`, `filterItems`,
//! `listHeight`, and `selectedPackages()`.
//!
//! `view()` is ported as the plain-text rendering Go produces without a TTY
//! (lipgloss emits zero ANSI there, so the text below is what the Go tests
//! assert on); the styled/animated TUI view is deferred with the runtime.

use std::collections::HashMap;

use crate::error::Error;
use crate::size;

use super::Deps;
use super::Options;
use super::discover::{self, Package};
use super::remnants;

/// `phase` — the model's current screen.
#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub(crate) enum Phase {
    /// `phaseSearch`
    Search,
    /// `phaseConfirm`
    Confirm,
    /// `phaseDone`
    Done,
}

/// `spinnerFrames`.
const SPINNER_FRAMES: &[&str] = &["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"];

/// `pkgItem` — a package row plus its selection flag.
#[derive(Clone, Debug)]
pub(crate) struct PkgItem {
    pub pkg: Package,
    pub selected: bool,
}

/// `uninstallModel` — fields the views don't need (none were dropped here:
/// everything `Update`/`selectedPackages` read is present).
pub(crate) struct UninstallModel {
    pub phase: Phase,
    pub spinner_idx: usize,
    /// `allItems` — the full loaded list.
    pub all_items: Vec<PkgItem>,
    /// `items` — the filtered view.
    pub items: Vec<PkgItem>,
    pub query: String,
    pub loaded: bool,
    pub discovery_err: Option<Error>,
    pub cursor: usize,
    /// `selected` — `pkg.Key()` → selected flag (keys survive filtering).
    pub selected: HashMap<String, bool>,
    #[allow(dead_code)] // carried for parity; only the view/run flow reads it
    pub opts: Options,
    pub window_width: usize,
    pub window_h: usize,
    pub scroll_off: usize,
    /// `confirmCursor` — 0=YES 1=NO.
    pub confirm_cursor: usize,
}

/// The `tea.KeyMsg` values `Update` switches on. `Runes` carries
/// `tea.KeyMsg{Type: KeyRunes}` runes verbatim — in search phase they append
/// to the query (`q`/`j`/`k` are text there), while in confirm phase a `"q"`
/// rune quits, matching `msg.String()` dispatch.
#[derive(Clone, PartialEq, Eq, Debug)]
pub(crate) enum Key {
    CtrlC,
    Esc,
    Backspace,
    Up,
    Down,
    Left,
    Right,
    Tab,
    Space,
    Enter,
    /// `tea.KeyMsg{Type: KeyRunes}` — printable text typed by the user.
    Runes(Vec<char>),
    /// Any other key — matches no `case` in Go's switch and carries no runes.
    Other,
}

impl Key {
    /// `msg.String()` — the name Go's `Update` switches on (`" "` for space,
    /// the runes themselves for `KeyRunes`).
    fn name(&self) -> String {
        match self {
            Self::CtrlC => "ctrl+c".to_string(),
            Self::Esc => "esc".to_string(),
            Self::Backspace => "backspace".to_string(),
            Self::Up => "up".to_string(),
            Self::Down => "down".to_string(),
            Self::Left => "left".to_string(),
            Self::Right => "right".to_string(),
            Self::Tab => "tab".to_string(),
            Self::Space => " ".to_string(),
            Self::Enter => "enter".to_string(),
            Self::Runes(runes) => runes.iter().collect(),
            Self::Other => String::new(),
        }
    }
}

/// `tea.Msg` — the messages `Update` switches on. `Noop` stands in for the
/// message types Go's `Update` ignores (and for non-`KeyMsg` reaching the
/// phase switch, which Go `break`s out of).
pub(crate) enum Msg {
    /// `tea.WindowSizeMsg`.
    WindowSize { width: usize, height: usize },
    /// `loadedMsg`.
    Loaded {
        pkgs: Vec<Package>,
        err: Option<Error>,
    },
    /// `tickMsg`.
    Tick,
    /// `tea.KeyMsg`.
    Key(Key),
    /// Anything `Update` doesn't handle → nil cmd.
    Noop,
}

/// `tea.Cmd` — what `update` asks the runtime to do next. `Load` runs
/// [`UninstallModel::load`]; `Tick` re-arms the spinner tick.
#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub(crate) enum Cmd {
    /// `tickCmd`.
    Tick,
    /// `Init`'s discover command (the `tea.Batch` closure).
    Load,
    /// `tea.Quit`.
    Quit,
}

/// `newModel`.
pub(crate) fn new_model(opts: Options) -> UninstallModel {
    UninstallModel {
        phase: Phase::Search,
        spinner_idx: 0,
        all_items: Vec::new(),
        items: Vec::new(),
        query: String::new(),
        loaded: false,
        discovery_err: None,
        cursor: 0,
        selected: HashMap::new(),
        opts,
        window_width: 80,
        window_h: 24,
        scroll_off: 0,
        confirm_cursor: 0,
    }
}

impl UninstallModel {
    /// `Init` — `tea.Batch(tickCmd(), discoverCmd)`.
    pub(crate) fn init(&self) -> Vec<Cmd> {
        vec![Cmd::Tick, Cmd::Load]
    }

    /// The `Init` discover closure: `Discover()`, then `FindRemnants` +
    /// `RemnantSize` per package — the work the Go command goroutine does
    /// before delivering `loadedMsg`.
    pub(crate) fn load(&self, deps: &Deps) -> Msg {
        let (mut pkgs, err) = discover::discover(deps.runner.as_ref());
        for p in &mut pkgs {
            p.remnants_found = remnants::find_remnants_in(deps, &p.name);
            p.remnants_kb = remnants::remnant_size(&p.remnants_found) / 1024;
        }
        Msg::Loaded { pkgs, err }
    }

    /// `Update`.
    pub(crate) fn update(&mut self, msg: Msg) -> Vec<Cmd> {
        let key = match msg {
            Msg::WindowSize { width, height } => {
                self.window_h = height;
                self.window_width = width;
                return Vec::new();
            }
            Msg::Loaded { pkgs, err } => {
                self.all_items = pkgs
                    .into_iter()
                    .map(|p| PkgItem {
                        selected: self.selected.get(&p.key()).copied().unwrap_or(false),
                        pkg: p,
                    })
                    .collect();
                self.loaded = true;
                self.discovery_err = err;
                self.items = filter_items(&self.all_items, &self.query);
                self.cursor = 0;
                self.scroll_off = 0;
                return Vec::new();
            }
            Msg::Tick => {
                if !self.loaded {
                    self.spinner_idx = (self.spinner_idx + 1) % SPINNER_FRAMES.len();
                    return vec![Cmd::Tick];
                }
                return Vec::new();
            }
            Msg::Noop => return Vec::new(),
            Msg::Key(k) => k,
        };

        match self.phase {
            Phase::Search => self.search_key(key),
            Phase::Confirm => self.confirm_key(key),
            Phase::Done => Vec::new(),
        }
    }

    /// `Update`'s `phaseSearch` arm — the switch on `msg.String()` plus the
    /// printable-runes default.
    fn search_key(&mut self, key: Key) -> Vec<Cmd> {
        match key.name().as_str() {
            "ctrl+c" | "esc" => vec![Cmd::Quit],
            "backspace" => {
                if !self.query.is_empty() {
                    // `utf8.DecodeLastRuneInString` — char-boundary pop.
                    self.query.pop();
                    self.refilter();
                }
                Vec::new()
            }
            "up" => {
                if self.cursor > 0 {
                    self.cursor -= 1;
                    if self.cursor < self.scroll_off {
                        self.scroll_off = self.cursor;
                    }
                }
                Vec::new()
            }
            "down" => {
                if self.cursor + 1 < self.items.len() {
                    self.cursor += 1;
                    let vis_h = self.list_height();
                    if self.cursor >= self.scroll_off + vis_h {
                        self.scroll_off = self.cursor - vis_h + 1;
                    }
                }
                Vec::new()
            }
            " " => {
                if self.cursor < self.items.len() {
                    let key = self.items[self.cursor].pkg.key();
                    let sel = !self.selected.get(&key).copied().unwrap_or(false);
                    self.selected.insert(key.clone(), sel);
                    if let Some(it) = self.all_items.iter_mut().find(|it| it.pkg.key() == key) {
                        it.selected = sel;
                    }
                    self.items = filter_items(&self.all_items, &self.query);
                }
                Vec::new()
            }
            "enter" => {
                if self.n_selected() == 0 {
                    return Vec::new();
                }
                self.phase = Phase::Confirm;
                self.confirm_cursor = 1; // default to NO
                Vec::new()
            }
            _ => {
                // Default arm: append `unicode.IsPrint`-filtered runes.
                if let Key::Runes(runes) = key {
                    let printable: String = runes.into_iter().filter(|r| is_print(*r)).collect();
                    if !printable.is_empty() {
                        self.query.push_str(&printable);
                        self.refilter();
                    }
                }
                Vec::new()
            }
        }
    }

    /// `Update`'s `phaseConfirm` arm.
    fn confirm_key(&mut self, key: Key) -> Vec<Cmd> {
        match key.name().as_str() {
            "ctrl+c" | "q" => vec![Cmd::Quit],
            "left" | "h" | "right" | "l" | "tab" => {
                self.confirm_cursor = 1 - self.confirm_cursor;
                Vec::new()
            }
            "enter" => {
                if self.confirm_cursor == 0 {
                    self.phase = Phase::Done;
                    return vec![Cmd::Quit];
                }
                // NO: return to search phase so the user can revise.
                self.phase = Phase::Search;
                Vec::new()
            }
            _ => Vec::new(),
        }
    }

    /// Re-filter and reset position — the shared backspace/text-input tail.
    fn refilter(&mut self) {
        self.items = filter_items(&self.all_items, &self.query);
        self.cursor = 0;
        self.scroll_off = 0;
    }

    /// Count of `selected` entries that are true — Go counts the map values
    /// (a key set false by an unselect still sits in the map).
    fn n_selected(&self) -> usize {
        self.selected.values().filter(|v| **v).count()
    }

    /// `listHeight` — `windowH - 7` (header + hint + blank + search + blank +
    /// footer + scroll), minimum 3.
    pub(crate) fn list_height(&self) -> usize {
        self.window_h.saturating_sub(7).max(3)
    }

    /// `selectedPackages` — the selected packages in `allItems` order.
    pub(crate) fn selected_packages(&self) -> Vec<Package> {
        self.all_items
            .iter()
            .filter(|it| self.selected.get(&it.pkg.key()).copied().unwrap_or(false))
            .map(|it| it.pkg.clone())
            .collect()
    }

    /// `View` — the plain-text form lipgloss renders without a TTY (no ANSI).
    /// The styled TUI view is deferred; the text content is byte-faithful so
    /// the ported tests assert the same strings the Go suite does.
    pub(crate) fn view(&self) -> String {
        match self.phase {
            Phase::Search => self.search_view(),
            Phase::Confirm => self.confirm_view(),
            Phase::Done => "\n\n  Confirmed. Removing packages...\n\n".to_string(),
        }
    }

    fn search_view(&self) -> String {
        let mut sb = String::new();
        sb.push_str(&format!(
            "\n\n  mu uninstall  ({} selected)\n\n",
            self.n_selected()
        ));
        let spin = if self.loaded {
            String::new()
        } else {
            format!("  {}", SPINNER_FRAMES[self.spinner_idx])
        };
        sb.push_str(&format!("  Search: {}█{spin}\n\n", self.query));
        if let Some(e) = &self.discovery_err {
            sb.push_str(&format!("  Discovery warning: {e}\n\n"));
        }

        if self.loaded && self.all_items.is_empty() && self.discovery_err.is_some() {
            sb.push_str("  No package source could be loaded.\n");
        } else if self.query.is_empty() {
            sb.push_str("  Type to search installed packages...\n");
        } else if self.items.is_empty() {
            sb.push_str("  No packages found.\n");
        } else {
            let vis_h = self.list_height();
            let end = (self.scroll_off + vis_h).min(self.items.len());
            for i in self.scroll_off..end {
                let it = &self.items[i];
                let total_kb = it.pkg.installed_kb + it.pkg.remnants_kb;
                let size = size::human_size(total_kb * 1024);
                let check = if self.selected.get(&it.pkg.key()).copied().unwrap_or(false) {
                    "✓ "
                } else {
                    "  "
                };
                let mut line = format!(
                    "{}{} [{}] {}  {}",
                    check, it.pkg.name, it.pkg.source, size, it.pkg.version
                );
                if i == self.cursor {
                    // The cursor row's highlight styles render plain on a
                    // pipe: `"  " + TrimLeft(line, " ")`.
                    line = format!("  {}", line.trim_start());
                }
                sb.push_str(&format!("  {line}\n"));
            }
            if self.items.len() > vis_h {
                sb.push_str(&format!(
                    "  [{}-{} of {}]\n",
                    self.scroll_off + 1,
                    end,
                    self.items.len()
                ));
            }
        }

        // `Padding(0, 2)` keeps its whitespace without a TTY.
        sb.push_str("\n\n\n  Type: search  •  ↑/↓: navigate  •  Space: select  •  Esc: quit  \n");
        sb
    }

    fn confirm_view(&self) -> String {
        let mut sb = String::from("\n\n  Will remove:\n");
        for (key, sel) in &self.selected {
            if !sel {
                continue;
            }
            if let Some(it) = self.all_items.iter().find(|it| it.pkg.key() == *key) {
                sb.push_str(&format!("    • {} ({})\n", it.pkg.name, it.pkg.source));
                for r in &it.pkg.remnants_found {
                    sb.push_str(&format!("      - {r}\n"));
                }
            }
        }
        // `ui.RenderButtons` pads both buttons to "  YES  " / "  NO  "; the
        // active/inactive styling is invisible without a TTY, so the text is
        // identical at either cursor position.
        sb.push_str("\n    YES      NO  \n");
        sb.push_str("\n\n\n  ←/→ navigate  •  Enter: confirm  •  q: quit  \n");
        sb
    }
}

/// `filterItems` — case-insensitive substring match on the package name;
/// an empty query yields nothing (the view shows the search prompt instead).
pub(crate) fn filter_items(all: &[PkgItem], query: &str) -> Vec<PkgItem> {
    if query.is_empty() {
        return Vec::new();
    }
    let q = query.to_lowercase();
    all.iter()
        .filter(|it| it.pkg.name.to_lowercase().contains(&q))
        .cloned()
        .collect()
}

/// `unicode.IsPrint` approximation: letters, marks, numbers, punctuation,
/// symbols, and ASCII space are printable — i.e. everything but control,
/// format, surrogate, unassigned, and non-ASCII whitespace (Zs) codepoints.
/// Rust exposes only `is_control`/`is_whitespace` without unicode tables, so
/// format/unassigned rare chars pass where Go would reject them; typed-query
/// input in tests is ASCII.
fn is_print(c: char) -> bool {
    if c == ' ' {
        return true;
    }
    !c.is_control() && !c.is_whitespace()
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::error::Error;

    fn pkg(name: &str, source: &str, version: &str, installed_kb: i64) -> Package {
        Package {
            name: name.to_string(),
            source: source.to_string(),
            version: version.to_string(),
            installed_kb,
            ..Default::default()
        }
    }

    // model_coverage_test.go: TestUninstallModelSearchConfirmAndRenderFlow
    #[test]
    fn model_search_confirm_and_render_flow() {
        let mut m = new_model(Options::default());
        assert!(
            m.view().contains("Type to search"),
            "initial view={:?}",
            m.view()
        );
        let cmds = m.update(Msg::WindowSize {
            width: 100,
            height: 10,
        });
        assert!(
            cmds.is_empty() && m.window_width == 100,
            "window size not applied"
        );
        let cmds = m.update(Msg::Tick);
        assert_eq!(cmds, vec![Cmd::Tick], "loading spinner did not advance");
        assert_eq!(m.spinner_idx, 1);

        m.query = "app".to_string();
        m.update(Msg::Loaded {
            pkgs: vec![
                pkg("app", "apt", "1", 10),
                pkg("app", "snap", "2", 20),
                pkg("other", "apt", "", 0),
            ],
            err: None,
        });
        let view = m.view();
        assert!(
            view.contains("[apt]") && view.contains("[snap]"),
            "search results missing: {view:?}"
        );

        m.update(Msg::Key(Key::Space));
        m.update(Msg::Key(Key::Down));
        m.update(Msg::Key(Key::Space));
        m.update(Msg::Key(Key::Enter));
        assert_eq!(m.phase, Phase::Confirm, "confirm phase={:?}", m.phase);
        assert!(
            m.view().contains("Will remove"),
            "confirm view={:?}",
            m.view()
        );

        m.update(Msg::Key(Key::Left));
        m.update(Msg::Key(Key::Enter));
        assert_eq!(m.phase, Phase::Done, "done phase={:?}", m.phase);
        assert!(m.view().contains("Confirmed"), "done view={:?}", m.view());
        assert_eq!(m.selected_packages().len(), 2);
    }

    // model_coverage_test.go: TestUninstallModelEmptyNoMatchAndNavigationBranches
    #[test]
    fn model_empty_no_match_and_navigation_branches() {
        let mut m = new_model(Options::default());
        assert!(!m.init().is_empty(), "expected init command");
        m.window_h = 20;
        assert_eq!(m.list_height(), 13);
        m.window_h = 5;
        assert_eq!(m.list_height(), 3);
        m.loaded = true;
        m.update(Msg::Key(Key::Enter));
        assert_eq!(
            m.phase,
            Phase::Search,
            "empty selection should remain in search"
        );
        m.update(Msg::Key(Key::Runes(vec!['x'])));
        assert!(
            m.view().contains("No packages found"),
            "no-match view={:?}",
            m.view()
        );
        m.update(Msg::Key(Key::Backspace));
        assert_eq!(m.query, "", "backspace did not clear query");
        m.phase = Phase::Confirm;
        m.confirm_cursor = 0;
        m.update(Msg::Key(Key::Right));
        m.update(Msg::Key(Key::Enter));
        assert_eq!(m.phase, Phase::Search, "NO should return to search");
        m.phase = Phase::Confirm;
        let cmds = m.update(Msg::Key(Key::Runes(vec!['q'])));
        assert_eq!(m.phase, Phase::Confirm);
        assert_eq!(cmds, vec![Cmd::Quit], "q should quit from confirm");
    }

    // remove_test.go: TestSameNameAPTAndSnapSelectionIsIsolated — the model
    // coverage lives in remove_test.go in the oracle suite.
    #[test]
    fn same_name_apt_and_snap_selection_is_isolated() {
        let mut m = new_model(Options::default());
        m.query = "shared".to_string();
        m.update(Msg::Loaded {
            pkgs: vec![pkg("shared", "apt", "", 0), pkg("shared", "snap", "", 0)],
            err: None,
        });
        m.update(Msg::Key(Key::Space));
        assert!(
            m.selected.get("apt:shared").copied().unwrap_or(false)
                && !m.selected.get("snap:shared").copied().unwrap_or(false),
            "selection coupled by name: {:?}",
            m.selected
        );
        m.update(Msg::Key(Key::Down));
        m.update(Msg::Key(Key::Space));
        assert!(
            m.selected.get("apt:shared").copied().unwrap_or(false)
                && m.selected.get("snap:shared").copied().unwrap_or(false),
            "expected independent selections: {:?}",
            m.selected
        );
    }

    // model_coverage_test.go: TestUninstallSearchTreatsQJKAsTextAndShowsDiscoveryWarning
    #[test]
    fn search_treats_qjk_as_text_and_shows_discovery_warning() {
        let mut m = new_model(Options::default());
        m.update(Msg::Loaded {
            pkgs: Vec::new(),
            err: Some(Error::Msg("APT discovery: unavailable".to_string())),
        });
        let view = m.view();
        assert!(
            view.contains("Discovery warning")
                && view.contains("No package source could be loaded"),
            "missing discovery failure state: {view:?}"
        );
        for key in ['q', 'j', 'k'] {
            let cmds = m.update(Msg::Key(Key::Runes(vec![key])));
            assert!(cmds.is_empty(), "key {key:?} unexpectedly quit");
        }
        assert_eq!(m.query, "qjk");
        let cmds = m.update(Msg::Key(Key::Esc));
        assert_eq!(cmds, vec![Cmd::Quit], "escape should quit search");
    }
}
