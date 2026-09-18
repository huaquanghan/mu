# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
make build          # cargo build --release + copy → ./bin/mu
make checksums      # build + write bin/checksums.txt (required release asset for install.sh)
make install-local  # install to ~/.local/bin/mu (no sudo)
make test           # cargo test
make test-verbose   # cargo test -- --nocapture
make smoke          # build + run --help, clean --dry-run, optimize --dry-run, status
make lint           # cargo clippy --all-targets -- -D warnings
make fmt            # cargo fmt --check
make clean          # remove ./bin/ + cargo clean
```

Run a single test:
```bash
cargo test paths::tests         # module filter
cargo test font_cache -- --nocapture
```

Run directly without installing:
```bash
cargo run --release --
cargo run --release -- clean --dry-run
./target/release/mu uninstall
```

Parity oracle (pre-cutover verification):
```bash
scripts/capture-golden.sh   # capture golden outputs → tests/golden/
scripts/parity-diff.sh      # diff all 12 golden commands Go-vs-Rust
```

## Architecture

**Entry point:** `src/main.rs` → `cli::run()`. `src/cli.rs` is a hand-written argv parser (no clap) that reproduces Cobra's help text, error strings, and exit codes byte-exact against `tests/golden/help-*.txt`. No-arg invocation on a TTY launches `tui::run_tui()`; on a non-TTY it exits 1 with Go's TTY error text. `build.rs` emits `MU_VERSION` from `git describe --tags --always --dirty` (equivalent of the old Go `-X` ldflag).

**Two execution paths:**

1. **TUI path** (`mu` with no args): `src/tui/mod.rs:run_tui()` runs the ratatui `MainMenu` loop in an alt-screen. After each subcommand exits (including on user abort), the loop returns to the main menu. `RawModeGuard`/`AltScreenGuard` RAII structs restore the terminal on every exit path.

2. **CLI path** (`mu clean`, `mu status`, etc.): `cli.rs` parses flags into per-command `Options` and dispatches to `clean::run`, `optimize::run`, `audit::run`, `uninstall::run`, `status::run` — each returns an `i32` exit code mirroring Go's error→cobra-exit mapping.

**`src/` modules:**

| Module | Responsibility |
|--------|---------------|
| `clean` | `CleanTarget` closure-struct (`scan`, `preview`, `execute(dry_run)`), 9 targets incl. opt-in browser/docker, `flow.rs` bubbletea-model-as-state-machine, `run_plain`/`run_tui` |
| `uninstall` | APT/Snap discovery (`dpkg-query`/`snap` argv verbatim), `source:name` model, remnant scan + `SafeDelete`-only removal, whitelist fail-closed |
| `optimize` | Confirmed steps (apt update, autoremove --purge, journal vacuum, cache refresh) with success/failed/skipped states, continue-after-failure, aggregate nonzero exit |
| `status` | `/proc` + mountinfo metrics (`proc.rs`), health score (`health.rs`), `Dashboard` tick state machine, serde types with Go-exact JSON names (`model.rs`) |
| `audit` | collect→findings→report pipeline, Go field-order JSON schema with `omitempty` + `serialize_go_f64` + `escape_html_json`, exit codes 0/1/2 |
| `tui` | ratatui+crossterm widgets: `menu.rs` (main menu), `confirm.rs` (YES/NO default-NO), `uninstall.rs` (search/multi-select), `status.rs` (live dashboard), `run.rs` (RunShell spinner), `styles.rs` (palette) |
| `trash` | `SafeDelete` — whitelist → lstat → `gio trash` → FreeDesktop fallback (renameat2 RENAME_NOREPLACE, .trashinfo, rollback) |
| `paths`, `whitelist`, `config`, `xdg` | protected prefixes, `ValidateCleanupRoot`/`Candidate` (symlink + home-ancestor), fail-closed TOML config + glob matching, XDG resolution (relative env ignored) |
| `runner` | `Runner` trait, `ProcessRunner` (timeout via SIGKILL), `FakeRunner` for tests |
| `oplog`, `size`, `error`, `runtext` | 10 MB rotating operations log (`MU_NO_OPLOG=1` kills), DirSize/HumanSize, error types, plain-text run renderer |

Leaf modules carry `#[allow(dead_code)]` in `main.rs` — the port keeps Go-faithful API surfaces that are not all consumed by the CLI.

**Safety invariants (never bypass):**
- All user-file deletion goes through `trash::SafeDelete` — moves to trash, never `rm`.
- `SafeDelete` refuses paths via whitelist checks (hardcoded prefixes in `paths.rs` **plus** user `protected_paths` from config).
- `config` merges embedded defaults (`src/default-whitelist.toml`) and user config. Malformed user config blocks destructive commands.
- APT policy is the source of autoremove candidates. Never reintroduce `dpkg-query` plus kernel-name filtering.
- Cleanup roots and candidates must pass `ValidateCleanupRoot`/`ValidateCleanupCandidate` in dry-run and real execution.
- `mu clean` user-cache must honor `cache_skip` (scan size + execute); never wipe denylisted tool caches.
- Docker build-cache target is **OptIn** (`--include=docker`); browser-cache remains OptIn.
- `scripts/install.sh` must verify release `checksums.txt` (SHA-256) before install — fail closed.
- Every destructive path must have a `dry_run` branch that logs without acting.

**TUI conventions (ratatui):**
- All views start with `"\n\n"` top padding and end with blank lines + a faint hint line (`"  ←/→ navigate  •  q to quit"`).
- Primary color: `#0097A7` (cyan). Inactive/secondary: `#374151` bg / `#9CA3AF` fg. Danger: `#EF4444`. Success: `#22C55E`.
- Alt-screen used for all full-screen TUI (main menu, uninstall, status, confirm overlays).
- `TestBackend` snapshot tests verify render structure; live TTY comparison is manual (CI is non-TTY).

<!-- ZHARNESS:BEGIN -->
@AGENTS.md
<!-- ZHARNESS:END -->
