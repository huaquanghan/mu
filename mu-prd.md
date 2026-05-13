# PRD: mu — Deep Clean & Optimize Tool for Ubuntu
**Version:** 1.0  
**Date:** May 13, 2026  
**Author:** Grok Executive Orchestrator (delegated to Atlas, Forge, Pulse, Sentinel, Vector, Operator)  
**Status:** Ready for Coding Agent  
**Goal:** Build a production-grade, safe, open-source CLI tool for Ubuntu (and Debian-based distros) that mirrors the philosophy and UX of the original Mole macOS tool, but fully adapted to Linux ecosystem.

---

## 1. Executive Summary

`mu` (Mole Ubuntu) is a single-binary CLI tool that performs **deep system cleaning**, **smart app uninstallation**, **performance optimization**, **disk analysis**, and **real-time system monitoring** on Ubuntu.

It combines the power of `CleanMyMac` + `AppCleaner` + `DaisyDisk` + `htop` + `ncdu` into **one Go + Bash binary**, exactly like the original Mole.

**Core Philosophy (inherited from Mole):**
- Safety-first: Every destructive action has `--dry-run`, confirmation, whitelist, and operation logging.
- Beautiful interactive CLI with arrow/Vim navigation.
- All-in-one: No need for multiple tools.
- Free & open source forever.

**Target Release:** v0.1.0 MVP in 4-6 weeks (coding agent timeline).

---

## 2. Problem Statement

Ubuntu users lack a **unified, safe, modern CLI tool** for:
- Reclaiming gigabytes from cache, logs, snap, apt, journald, old kernels, etc.
- Completely removing apps + all their config, cache, and service remnants.
- Optimizing system (vacuum journal, autoremove, refresh caches).
- Visual disk usage explorer.
- Live hardware health dashboard.

Existing tools are fragmented (`bleachbit`, `stacer`, `ubuntu-cleaner`, manual `rm -rf ~/.cache`), risky, or GUI-only.

---

## 3. Goals & Success Metrics

**Primary Goal:** Reclaim **at least 10-30GB** on a typical Ubuntu desktop with one `mu clean` command, safely.

**Success Metrics (MVP):**
- `mu clean` frees ≥15GB average on test machines (22.04/24.04)
- 100% of destructive actions support `--dry-run`
- Interactive menu works in all major terminals (GNOME Terminal, Kitty, Alacritty, Warp, iTerm2 on WSL)
- Zero data loss in 100+ test runs (with dry-run + confirmation)
- Installation time < 30 seconds via curl | bash
- Binary size < 25MB

---

## 4. Target Users

- Power users & developers on Ubuntu 22.04 / 24.04 LTS (desktop + server)
- People coming from macOS who miss Mole
- System administrators who want safe automation
- Anyone tired of manual `sudo apt clean && journalctl --vacuum-time=2weeks`

---

## 5. Feature Requirements (Detailed)

### 5.1 Core Commands (MVP Scope)

| Command          | Description                                      | Priority | Acceptance Criteria |
|------------------|--------------------------------------------------|----------|---------------------|
| `mu`             | Interactive main menu                            | P0       | Beautiful TUI, Vim/Arrow keys, categories |
| `mu clean`       | Deep cleanup (caches, logs, apt, snap, journal, old kernels, thumbnails, etc.) | P0 | Shows size before/after, --dry-run, progress bar |
| `mu uninstall`   | Smart app removal + full remnant cleanup         | P0 | Select multiple apps, shows size, cleans ~/.config, ~/.local, /var/lib, systemd units |
| `mu optimize`    | System refresh (apt update, autoremove, journal vacuum, icon cache, etc.) | P0 | Whitelist support, --dry-run |
| `mu status`      | Live dashboard (CPU, RAM, Disk, Network, Health score) | P0 | Real-time, JSON output when piped |
| `mu analyze`     | Interactive disk usage explorer (ncdu-like)      | P1 | Navigate folders, show large files, delete to trash |
| `mu purge`       | Clean project build artifacts (node_modules, target, dist, .cache, etc.) | P1 | Configurable paths via `--paths` |
| `mu installer`   | Find & remove large .deb/.AppImage/installers in Downloads | P2 | Optional in MVP |

### 5.2 Safety & Security Requirements (Non-negotiable)

- **Every destructive command** must support `--dry-run` (default preview only)
- `--debug` flag for verbose logging
- Protected paths whitelist (user can add exceptions)
- Explicit confirmation for high-risk actions (e.g. "Type 'YES' to continue")
- All file operations logged to `~/.local/share/mu/operations.log`
- Environment variable `MU_NO_OPLOG=1` to disable logging
- Path validation: never touch `/`, `/boot`, `/etc` without explicit flag
- Use `gio trash` or safe move-to-trash when possible instead of direct `rm`

### 5.3 Additional Features (Post-MVP v0.2+)

- `mu touchid` equivalent → `mu sudo` (configure passwordless sudo with fingerprint if available)
- Shell completion (`mu completion bash/zsh/fish`)
- `mu update` self-update
- `mu remove` uninstall itself cleanly
- JSON output for all commands (`--json`)
- Theme support (light/dark) via config file `~/.config/mu/config.toml`

---

## 6. Technical Architecture

**Tech Stack (exactly as requested):**
- **Primary:** Go 1.21+ ( Cobra for CLI, Bubbletea + Lipgloss for beautiful TUI, Charmbracelet libraries)
- **Secondary:** Bash (install.sh, quick launcher scripts, heavy system calls where Go is overkill)
- **Build:** Makefile + GoReleaser (for GitHub releases)
- **Dependencies:** Minimal. Recommend `fd` (optional, faster file search), `ncdu` (optional for analyze fallback)

**Project Structure (Mole-style):**
```
mu/
├── cmd/mu/main.go                 # Entry point + root command
├── internal/
│   ├── clean/                     # Clean logic + size calculation
│   ├── uninstall/                 # App discovery + remnant scanner
│   ├── optimize/                  # Optimization units
│   ├── status/                    # /proc + sysinfo collector
│   ├── analyze/                   # Disk walker + TUI
│   ├── ui/                        # Shared Bubbletea components
│   └── utils/                     # Path helpers, size humanizer, logger
├── scripts/
│   ├── install.sh
│   └── setup-completion.sh
├── configs/
│   └── default-whitelist.toml
├── go.mod
├── Makefile
├── README.md
├── SECURITY.md
└── .github/workflows/release.yml
```

**Key Design Decisions:**
- Use **XDG Base Directory Specification** strictly (`$XDG_CACHE_HOME`, `$XDG_CONFIG_HOME`, etc.)
- Go for all business logic + TUI
- Bash only for installation and one-off privileged operations (e.g. `journalctl --vacuum`)
- Single static binary (CGO_ENABLED=0 where possible)

---

## 7. Command Specification (Exact UX)

### Example: `mu clean --dry-run`

```
$ mu clean --dry-run

🔍 Scanning system...

User Cache ( ~/.cache )                    12.4 GB
APT Cache ( /var/cache/apt )                3.8 GB
Snap Cache                                2.1 GB
Journal Logs (journalctl)                   4.7 GB
Old Kernels & Headers                       1.9 GB
Thumbnails ( ~/.cache/thumbnails )          890 MB
Browser Caches (Chrome/Firefox)             2.3 GB
Docker Build Cache (if present)             5.6 GB
-------------------------------------------------
Potential space to free: 33.7 GB

⚠️  This is a DRY RUN. No files will be deleted.
Run without --dry-run to proceed.
```

### Example: `mu uninstall`

Interactive multi-select list of installed packages + size + "last used".

After selection → shows exactly which folders will be cleaned.

---

## 8. Installation

**Primary method (recommended):**
```bash
curl -fsSL https://raw.githubusercontent.com/YOURORG/mu/main/scripts/install.sh | bash
```

**Alternative:**
```bash
# Future: sudo apt install mu
```

**Build from source:**
```bash
git clone https://github.com/YOURORG/mu
cd mu && make install
```

---

## 9. Non-Functional Requirements

- **Compatibility:** Ubuntu 22.04 LTS, 24.04 LTS, Debian 12+, Pop!_OS, Linux Mint (tested)
- **Performance:** `mu clean` completes in < 90 seconds on 512GB SSD
- **Binary size:** < 25 MB (stripped)
- **Memory:** < 150 MB peak during analyze
- **Permissions:** Most operations run as user; only privileged ops use `sudo` with clear prompt
- **Logging:** Structured logs + human-readable operations.log

---

## 10. MVP Scope & Roadmap

**MVP (v0.1.0) — Must have:**
- `mu`, `mu clean`, `mu uninstall`, `mu optimize`, `mu status`
- Full `--dry-run` + confirmation system
- Interactive TUI menu
- Safe path handling + whitelist
- install.sh + Makefile + basic README

**v0.2.0:**
- `mu analyze` (full ncdu-like TUI)
- `mu purge`
- Shell completion
- Self-update

**v1.0.0:**
- `mu installer`, theming, plugins/hooks, GUI companion (optional)

---

## 11. Open Questions for Coding Agent

1. Should we use **Bubbletea** (Charm) or **tview** for TUI? (Recommendation: Bubbletea for beauty)
2. How to discover installed apps reliably? (`dpkg -l` + `snap list` + desktop files)
3. Fallback if `fd` not installed? (pure Go filepath.Walk or recommend install)
4. Should `mu uninstall` also remove Flatpak apps? (Yes in v0.2)

---

## 12. Appendix: Ubuntu Cleaning Targets (Initial List)

**High-value targets for `mu clean`:**
- `~/.cache/*` (except whitelisted)
- `/var/cache/apt/archives/*.deb`
- `journalctl --vacuum-time=30d` (configurable)
- `/var/log/journal/*`
- Old kernels: `dpkg -l | grep linux-image | awk '$3 ~ /-generic/ && $3 !~ /$(uname -r)/'`
- `~/.local/share/Trash`
- `~/.thumbnails` / `~/.cache/thumbnails`
- Snap: `snap list --all | awk '/disabled/{print $1, $3}'` → remove disabled revisions
- `~/.config/*/Cache` (Chrome, Firefox, VSCode, etc.)
- Docker: `docker system prune -a --volumes` (with confirmation)
- Flatpak unused runtimes

**For `mu uninstall <app>`:**
- `apt purge <pkg>`
- Remove: `~/.config/<app>`, `~/.local/share/<app>`, `~/.cache/<app>`, `/var/lib/<app>`, systemd user units, desktop entries, etc.

---

## 13. Final Notes for Coding Agent

This PRD is **complete and ready to implement**.

**Next step for you (Coding Agent):**
1. `go mod init github.com/YOURORG/mu`
2. Implement root command with Cobra
3. Build the TUI menu first (highest UX impact)
4. Implement `clean` with dry-run as the first working feature
5. Follow Mole's safety philosophy religiously

**Repository naming suggestion:** `mu` (short, memorable, matches `mo` from original Mole)

---

**This PRD was produced by the full team delegation (Atlas + Forge + Pulse + Sentinel + Vector + Operator).**  
No further research needed — ready for immediate development.

**Hand this file directly to your coding agent.** It contains everything required to build a production-quality tool. 

**Ready when you are.** Let me know if you want any section expanded or a companion `TASKS.md` with ticket breakdown.