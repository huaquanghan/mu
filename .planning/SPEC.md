# SPEC: mu — Deep Clean & Optimize Tool for Ubuntu

## Source Mode
files

## Scenario
project bootstrap

## Goal
Build `mu` (Mole Ubuntu) — a single Go binary CLI that gives Ubuntu/Debian users one tool for system cleaning, app uninstallation, optimization, and live monitoring. Targets power users who want the UX of macOS cleaner tools (Mole/CleanMyMac) but adapted to the Linux ecosystem. Safety-first: every destructive action is previewed before execution.

## Users / Actors
- **Primary:** Developers and power users on Ubuntu 22.04 / 24.04 LTS desktop
- **Secondary:** System administrators who want safe, scriptable cleanup automation
- **Adjacent:** macOS users migrating to Linux who miss Mole

---

## Requirements

### Safety (non-negotiable)
1. Every destructive command supports `--dry-run`; running without it shows a confirmation prompt with explicit "YES" requirement for high-risk actions.
2. Protected path whitelist enforced at runtime: never touch `/`, `/boot`, `/etc` without explicit override flag.
3. All file operations logged to `~/.local/share/mu/operations.log`; suppressed via `MU_NO_OPLOG=1`.
4. Use `gio trash` or move-to-trash (never `rm -rf`) for user-owned files; `sudo rm` only for system cache explicitly listed.
5. `--debug` flag produces verbose structured log output.

### Commands (MVP v0.1.0)
6. `mu` (no args): Launch interactive TUI main menu with arrow/Vim navigation, showing all sub-commands and system health snapshot.
7. `mu clean [--dry-run]`: Scan and optionally remove: user cache (`~/.cache`), APT cache (`/var/cache/apt/archives`), Snap disabled revisions, journal logs (`journalctl --vacuum-time=30d`), APT-policy autoremove candidates, thumbnails, browser caches (Chrome/Firefox/VSCode), Docker build cache (if Docker present). Show size-per-category before/after.
8. `mu uninstall [--dry-run]`: Interactive multi-select list of installed packages (from `dpkg -l` + `snap list`). After selection, show which dirs will be removed (`~/.config/<app>`, `~/.local/share/<app>`, `~/.cache/<app>`), then execute `apt purge` + remnant cleanup.
9. `mu optimize [--dry-run]`: Run: `apt update && apt autoremove`, `journalctl --vacuum-size=500M`, update icon/font/mime caches. Whitelist support for skipping specific steps.
10. `mu status`: Live dashboard showing CPU %, RAM %, disk usage, network I/O, and a computed "Health Score". Outputs structured JSON when stdout is not a TTY (piped).

### TUI & UX
11. TUI built with Bubbletea + Lipgloss (Charmbracelet stack); Elm-style architecture.
12. Progress bars for long-running operations (clean, optimize).
13. Interactive multi-select for `mu uninstall` using Bubbles list component.
14. Works in GNOME Terminal, Kitty, Alacritty, and Warp without configuration.

### Build & Distribution
15. Single static binary, `CGO_ENABLED=0`, stripped, < 25 MB.
16. Go module path: `github.com/huaquanghan/mu`.
17. `Makefile` with targets: `build`, `install`, `test`, `clean`, `release`.
18. `scripts/install.sh` for curl-pipe install.
19. `make build` produces `./bin/mu` in < 60 seconds on a modern machine.

---

## Boundaries

### In Scope (MVP v0.1.0)
- Commands: `mu`, `mu clean`, `mu uninstall`, `mu optimize`, `mu status`
- Ubuntu 22.04 / 24.04 LTS (primary), Debian 12 / Pop!_OS / Mint (best-effort)
- APT packages + Snap packages for uninstall discovery
- XDG Base Directory compliance throughout
- Project scaffold: directory structure, `go.mod`, Makefile, `install.sh`, `configs/default-whitelist.toml`

### Out of Scope (v0.2+)
- `mu analyze` (ncdu-like disk explorer)
- `mu purge` (project build artifact cleaner)
- `mu installer` (Downloads folder large-file finder)
- Flatpak app uninstall
- Shell completion (`mu completion bash/zsh/fish`)
- Self-update (`mu update`)
- Theme config file (`~/.config/mu/config.toml`)
- JSON output flag (`--json`) per command
- `mu sudo` (passwordless sudo via fingerprint)
- GitHub Actions release workflow

---

## Constraints
- **Go version:** 1.25.8+ (required by current module / Charm stack pins; `GOTOOLCHAIN=auto` will auto-download on systems with older Go installations)
- **External runtime deps:** none required; `fd` and `ncdu` are optional enhancements
- **Privileges:** Most operations run as user; APT/kernel ops require `sudo` with explicit prompt — never auto-escalate silently
- **Binary size:** < 25 MB stripped
- **Memory:** < 150 MB peak during any operation
- **Performance:** `mu clean` completes in < 90 seconds on a 512 GB SSD

---

## Acceptance Criteria
- `mu --help` shows all MVP commands with descriptions
- `mu clean --dry-run` outputs per-category size estimates with no files modified
- `mu clean` (interactive confirm) actually frees disk space and logs each operation
- `mu status` renders a live-updating dashboard and exits cleanly on `q` / `Ctrl+C`
- `mu uninstall` shows a selectable package list; selecting a package and confirming calls `apt purge` and removes known remnant dirs
- `mu optimize --dry-run` lists planned actions without executing
- All commands complete without panic on a clean Ubuntu 24.04 VM
- `make build` succeeds from a fresh `git clone` with Go 1.25.8+ installed
- Binary size `< 25 MB` verified with `ls -lh ./bin/mu`

---

## Key Decisions

| Decision | Chosen | Rejected | Reason for rejection |
|----------|--------|----------|---------------------|
| Implementation language | Go 1.25.8+ | Shell-script only | Shell can't produce a single TUI binary; lacks type safety for path operations |
| Runtime language | Go | Python | Python requires interpreter on target; Go produces a zero-dependency static binary |
| TUI framework | Bubbletea + Lipgloss | tview | tview is widget-based but limited styling; Bubbletea Elm architecture scales better; Charm ecosystem gives Bubbles components (lists, progress, spinner) for free |
| TUI framework | Bubbletea + Lipgloss | No TUI (plain Cobra only) | PRD's core differentiator is UX; a plain Cobra tool would look no different from existing fragmented tools |
| File deletion | gio trash / safe move | `rm -rf` | Direct deletion is irreversible; trash preserves user data and aligns with safety-first philosophy |
| `mu uninstall` in MVP | Yes (basic) | Defer to v0.2 | PRD defines it as P0; starting with `apt purge` + dotfile cleanup is safe and useful without full systemd scanning |

---

## Dependencies / Assumptions
- Go 1.25.8+ available in build environment
- Target machines have `apt`, `snap` (optional), `journalctl`, `gio` (from glib2)
- Docker cleanup only runs if Docker daemon is present (checked at runtime)
- APT simulated autoremove policy supplies the complete candidate set
- `sudo` available for APT and kernel operations; `mu` never stores credentials

---

## Open Questions
- What happens on systems where `snap` is removed (Ubuntu minimal / server)? — gracefully skip snap sections if `snap` binary not found.
- Should `mu clean` include browser cache by default or require opt-in? — treat as opt-in (user selects categories in TUI or passes `--include=browser-cache`).
- Operations log rotation: cap at 10 MB or 30 days? — decide during implementation; document in config.

---

## Ambiguity Report
- **Goal clarity:** Clear — safety-first cleanup tool with TUI
- **Scope clarity:** Clear — 5 MVP commands, explicit v0.2+ deferred list
- **Constraints clarity:** Clear — binary size, memory, performance numbers from PRD
- **Acceptance clarity:** Strong — each command has a concrete falsifiable check

## Deferred Ideas
- `mu analyze`: ncdu-like interactive disk explorer (v0.2)
- `mu purge`: project artifact cleaner for node_modules/target/dist (v0.2)
- Flatpak support in `mu uninstall` (v0.2)
- Shell completion and self-update (v0.2)
- JSON output flag and theming (v0.2)
- GitHub Actions release workflow with GoReleaser (post-MVP)
