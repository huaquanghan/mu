# Plan: optimize-polish

## Goal
Add a Bubbles spinner to `mu optimize`, integrate whitelist config, and show a one-shot health snapshot in the main TUI menu.

## Inputs
- `internal/optimize/optimize.go` — existing (dry-run + step plan works)
- `internal/utils/whitelist.go` — Phase 1 output (required)
- `internal/status/proc.go` — Phase 3 output (required for menu health snapshot)
- `cmd/mu/cli/tui.go` — existing main TUI menu

---

## Wave 1 — Independent: whitelist integration

### Task 1: integrate whitelist config into `mu optimize`
- steps:
  1. In `optimize.Run()`, call `utils.LoadWhitelist()` at startup
  2. Merge `wl.OptimizeSkip` (from TOML) with `opts.Skip` (from `--skip` flag); flag takes precedence
  3. Add `[optimize_skip]` section to `configs/default-whitelist.toml`: `steps = []` (empty by default — user can add "apt", "journal", "caches")
  4. Update `internal/utils/whitelist.go` `Whitelist` struct to include `OptimizeSkip struct{ Steps []string }`
- expected outputs:
  - Updated `internal/optimize/optimize.go`
  - Updated `internal/utils/whitelist.go` struct
  - Updated `configs/default-whitelist.toml`
- verify:
  - `mu optimize --dry-run` with `optimize_skip = ["apt"]` in TOML shows apt as skipped without `--skip` flag

---

## Wave 2 — Depends on Wave 1: spinner + menu health snapshot

### Task 2: add Bubbles spinner to `mu optimize`
- steps:
  1. Import `github.com/charmbracelet/bubbles/spinner`
  2. Replace the plain `fmt.Printf("\n→ %s\n", s.desc)` loop with a Bubbletea model:
     - `optimizeModel` has: `spinner spinner.Model`, `steps []step`, `current int`, `done bool`, `err error`
     - `Init()`: return `spinner.Tick`
     - `Update()`: on `spinner.TickMsg`, advance spinner; on `stepDoneMsg`, increment `current`; run each step as a `tea.Cmd` goroutine that sends `stepDoneMsg`
     - `View()`: show completed steps with ✅, current step with spinner, pending steps
  3. After all steps: exit Bubbletea, print "✅ Optimization complete."
- expected outputs:
  - Updated `internal/optimize/optimize.go` with Bubbletea spinner model
- verify:
  - `mu optimize` (with YES confirmation) shows animated spinner per step; all steps complete; final summary printed

### Task 3: add health snapshot to main TUI menu
- steps:
  1. In `cmd/mu/cli/tui.go` `mainMenuModel`, add a `snapshot statusLine` field
  2. In `Init()`, fire a `tea.Cmd` that reads CPU, RAM, disk via one-shot calls from `internal/status/proc.go` and sends a `healthMsg`
  3. `Update()`: on `healthMsg`, store snapshot in model
  4. `View()`: render snapshot as a single line below the title: `CPU: 12%  RAM: 4.2/16 GB  Disk /: 82% free`; if snapshot not yet loaded: "Loading..."
- expected outputs:
  - Updated `cmd/mu/cli/tui.go`
- verify:
  - `mu` (no args) opens TUI; health snapshot line shows real values within 1 second

---

## Risks / Watch-fors
- Bubbletea spinner for optimize: each step blocks; run steps sequentially via `tea.ExecProcess` or a goroutine that sends a `stepDoneMsg` when the step completes — do NOT block in `Update()`
- Health snapshot in TUI menu: CPU % requires two reads 1 second apart — acceptable to show a 1-second blank "Loading..." before the value appears; do not sleep in `Init()`
