# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
make build          # compile → ./bin/mu (CGO_ENABLED=0, stripped)
make checksums      # build + write bin/checksums.txt (required release asset for install.sh)
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
| `clean` | Scan/execute clean targets. `CleanTarget` has `Scan() (int64, error)`, optional `Preview()`, and `Execute(dryRun bool) error`. Opt-in IDs are validated. |
| `uninstall` | Bubbletea search/confirm TUI. Packages are keyed by `source:name`; remnants are removed only after source-specific removal succeeds and ownership is clear. |
| `optimize` | Confirmed `apt`, `journal`, and `caches` steps with explicit success, failed, and skipped states plus aggregate errors. |
| `status` | `/proc` and mountinfo dashboard. Root disk drives health; JSON adds `scan_errors` for unavailable metrics. |
| `ui` | Shared `Confirm(prompt string) bool` — interactive YES/NO button prompt (default NO). Used by clean and optimize. |
| `utils` | `SafeDelete` (via `gio trash` or XDG trash fallback), `IsProtected`/`IsWhitelisted`, `LoadWhitelist` + `cache_skip` matching, logger with 10 MB rotation, XDG path helpers. |
| `command` | Injectable `exec.CommandContext` runner used to test command arguments, output, exit status, and cancellation. |

**Safety invariants (never bypass):**
- All user-file deletion goes through `utils.SafeDelete` — moves to trash, never `rm`.
- `SafeDelete` refuses paths via `IsWhitelisted` (hardcoded prefixes in `utils/paths.go` **plus** user `protected_paths` from config).
- `LoadWhitelist()` merges embedded defaults and user config. Malformed user config blocks destructive commands.
- APT policy is the source of autoremove candidates. Never reintroduce `dpkg-query` plus kernel-name filtering.
- Cleanup roots and candidates must pass `ValidateCleanupRoot` / `ValidateCleanupCandidate` in dry-run and real execution.
- `mu clean` user-cache must honor `cache_skip` (scan size + execute); never wipe denylisted tool caches.
- Docker build-cache target is **OptIn** (`--include=docker`); browser-cache remains OptIn.
- `scripts/install.sh` must verify release `checksums.txt` (SHA-256) before install — fail closed.
- Every destructive path must have a `dryRun bool` branch that logs without acting.

**TUI conventions (Bubbletea + Lipgloss):**
- All `View()` functions start with `"\n\n"` for top padding.
- Every layout ends with 2 blank lines then a faint hint line (`"  ←/→ navigate  •  q to quit"`).
- Primary color: `#0097A7` (cyan). Inactive/secondary: `#374151` bg / `#9CA3AF` fg.
- `tea.WithAltScreen()` used for all full-screen TUI programs (main menu, uninstall, status).

**Module path:** `github.com/huaquanghan/mu` — used in all import paths even when developing locally.

<!-- ZHARNESS:BEGIN -->
@AGENTS.md
<!-- ZHARNESS:END -->
