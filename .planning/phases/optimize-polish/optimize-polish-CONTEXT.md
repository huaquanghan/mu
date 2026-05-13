# Context: optimize-polish

## Goal
Polish `mu optimize` with a Bubbles spinner/progress indicator and config-driven whitelist; update the main TUI menu to show a live health snapshot using the status package.

## Spec Hooks
- Req 9: `mu optimize --dry-run`; whitelist support for skipping steps
- Req 6: main TUI menu shows system health snapshot
- Req 11-12: Bubbletea + progress bars for long-running operations

## Locked Decisions
- **Spinner not progress bar**: `mu optimize` steps are not size-trackable; use `github.com/charmbracelet/bubbles/spinner` (dots style) with step label. Progress bar is for `mu clean` (bytes-trackable).
- **Whitelist config**: add `[optimize_skip]` section to `configs/default-whitelist.toml`; `internal/utils/whitelist.go` (Phase 1) loads it. `--skip` CLI flag values are merged on top.
- **TUI menu health snapshot**: call `internal/status/proc.go` functions once at menu startup (one-shot, no ticker). Display as a single info line: `CPU: 12%  RAM: 4.2/16GB  Disk: 82% free`. If proc read fails, show "---".
- **Menu layout**: the existing Bubbletea menu in `cmd/mu/cli/tui.go` gets a header line for the health snapshot; rest of menu is unchanged.

## Assumptions
- Steps complete in seconds (apt, journal, caches) — a spinner is sufficient feedback
- Health snapshot in the menu is acceptable to be 1-2 seconds stale (read once at init)
- `mu optimize` with sudo steps will show a sudo password prompt in the terminal (below the spinner) — this is acceptable for MVP

## Canonical Refs
- `.planning/SPEC.md` — Req 6, 9, 11, 12
- `.planning/ROADMAP.md` — Phase 5
- `internal/optimize/optimize.go` — existing implementation
- `cmd/mu/cli/tui.go` — existing main menu model
- `internal/status/proc.go` — (Phase 3 output, required here)
- `internal/utils/whitelist.go` — (Phase 1 output, required here)

## Rejected Options
- **Progress bar for optimize**: steps have no measurable size; spinner is honest about unknown duration
- **Full status dashboard inside TUI menu**: too heavy; one-shot proc read is sufficient

## Deferred Ideas
- `[optimize_skip]` user config fully documented in README (v0.2)
- `mu optimize --only=journal` for individual step execution (v0.2)
