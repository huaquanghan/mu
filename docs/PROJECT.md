# PROJECT — identity

## What is this project?
- `mu` (Mole Ubuntu): a single static binary CLI that cleans, uninstalls,
  optimizes, and monitors Ubuntu/Debian systems with safety-first defaults
  (trash-not-delete, `--dry-run` everywhere, protected paths, audit log).
  Implemented in Rust (~16.5k LOC, manual cobra-compatible argv parser +
  ratatui/crossterm TUI) — ported 1:1 from the Go original (Cobra + Bubble
  Tea), which was removed in the parity-gate cutover after golden diffs
  passed.

## Who is it for?
- Ubuntu 22.04/24.04 power users and developers, ex-macOS Mole users, and
  sysadmins who want one safe cleanup/monitoring tool instead of fragmented
  manual commands.

## Non-goals
- No GUI; TUI only. No non-Debian distros. No destructive deletion without
  trash/dry-run/confirmation. No new features during the Rust port — parity
  only (PRD v0.2 scope like `analyze`/`purge` stays deferred).

## What are the gate commands?
- run from: repository root (`/home/tinhpt/Personal/mu`)
- tests: `make test` (cargo test)
- types: `cargo clippy --all-targets -- -D warnings`
- lint: `make lint` (cargo clippy)
- build: `make build` → `bin/mu` (cargo build --release + copy)
- format: `make fmt` (cargo fmt --check)
- parity: `scripts/parity-diff.sh` (Go-vs-Rust golden diff — pre-cutover only)

## Architecture in one breath
- runtime shape: single static binary (cargo release build); CLI subcommands
  plus an interactive alt-screen TUI menu; destructive ops shell out to
  `gio trash`, `apt-get`, `journalctl`, `snap`, `docker`
- where state lives: `~/.config/mu/config.toml` (whitelist/skips),
  `~/.local/share/mu/operations.log` (10 MB, 1 rotation, `MU_NO_OPLOG` kills),
  live metrics read from `/proc` + mountinfo; no daemon, no database
- entrypoints: `src/main.rs` → `cli::run()` (manual cobra-compatible argv
  parser); no-arg TTY → `tui::run_tui()` ratatui menu

## What are we working on right now?
- plan: docs/plans/active/rust-rewrite.md (active)
