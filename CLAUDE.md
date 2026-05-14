# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
make build          # compile → ./bin/mu (CGO_ENABLED=0, stripped)
make install-local  # install to ~/.local/bin/mu (no sudo)
make test           # go test ./... -count=1
make test-verbose   # go test ./... -v
make test-race      # go test ./... -race
make smoke          # build + run --help, clean --dry-run, optimize --dry-run, status
make clean          # remove ./bin/
```

Run a single test:
```bash
go test ./internal/utils/... -run TestIsProtected -v
go test ./internal/clean/... -run TestKernelScan -v
```

Run directly without installing:
```bash
go run ./cmd/mu
go run ./cmd/mu clean --dry-run
go run ./cmd/mu uninstall
```

## Architecture

**Entry point:** `cmd/mu/main.go` → `cli.Execute()` (Cobra root). No-arg invocation launches `runTUI()`.

**Two execution paths:**

1. **TUI path** (`mu` with no args): `cmd/mu/cli/tui.go:runTUI()` runs a Bubbletea `mainMenuModel` in a `for` loop. After each subcommand exits (including on user abort with `q`), the loop returns to the main menu. Only `Quit` menu item or an error breaks the loop.

2. **CLI path** (`mu clean`, `mu status`, etc.): Cobra subcommands in `cmd/mu/cli/` call thin `runX()` wrappers that delegate directly to `internal/` packages.

**`internal/` packages:**

| Package | Responsibility |
|---------|---------------|
| `clean` | Scan/execute clean targets. `CleanTarget` struct has `Scan() int64` and `Execute(dryRun bool) error`. `AllTargets()` returns targets in display order; `OptIn: true` targets are excluded unless their ID is in `--include`. |
| `uninstall` | Bubbletea TUI with 3 phases: `phaseSearch` (type-to-filter package list), `phaseConfirm` (YES/NO button), `phaseDone`. Packages discovered via `dpkg-query` + `snap list`. |
| `optimize` | Plain `fmt.Print` flow → `ui.Confirm()` → Bubbletea spinner for execution. Steps: `apt`, `journal`, `caches`. |
| `status` | `/proc`-based live dashboard (Bubbletea ticker). JSON output when stdout is not a TTY. `proc.go` reads CPU/RAM/disk/net; `health.go` computes 0-100 score. |
| `ui` | Shared `Confirm(prompt string) bool` — interactive YES/NO button prompt (default NO). Used by clean and optimize. |
| `utils` | `SafeDelete` (via `gio trash` or XDG trash fallback), `IsProtected`, `LoadWhitelist`, logger with 10 MB rotation, XDG path helpers. |

**Safety invariants (never bypass):**
- All user-file deletion goes through `utils.SafeDelete` — moves to trash, never `rm`.
- `utils.IsProtected(path)` is checked before any deletion; hardcoded prefixes in `utils/paths.go`.
- `LoadWhitelist()` merges `configs/default-whitelist.toml` + `~/.config/mu/config.toml` (user wins on conflicts).
- Every destructive path must have a `dryRun bool` branch that logs without acting.

**TUI conventions (Bubbletea + Lipgloss):**
- All `View()` functions start with `"\n\n"` for top padding.
- Every layout ends with 2 blank lines then a faint hint line (`"  ←/→ navigate  •  q to quit"`).
- Primary color: `#0097A7` (cyan). Inactive/secondary: `#374151` bg / `#9CA3AF` fg.
- `tea.WithAltScreen()` used for all full-screen TUI programs (main menu, uninstall, status).

**Module path:** `github.com/huaquanghan/mu` — used in all import paths even when developing locally.
