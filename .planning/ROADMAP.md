# ROADMAP: mu — Deep Clean & Optimize Tool for Ubuntu

## Planning Basis
- source spec: `.planning/SPEC.md`
- planning mode: `full`
- scaffold state: project structure, go.mod, Makefile, install.sh, all Cobra commands, utils package, and basic `mu clean --dry-run` are already working. `mu uninstall` and `mu status` are stubs.

---

## Phase 1: safety-core
**Goal:** Lock the shared safety primitives all commands depend on — safe deletion via `gio trash`, log rotation, and whitelist TOML loading. Nothing destructive ships without this layer.

**Deliverables:**
- `internal/utils/trash.go` — `SafeDelete(path, dryRun)` using `gio trash`; falls back to system-temp move if `gio` absent
- Log rotation in `internal/utils/logger.go` (10 MB cap, rename old log)
- `internal/utils/whitelist.go` — loads `configs/default-whitelist.toml` + user overrides from `~/.config/mu/config.toml`
- Unit tests for path protection and whitelist enforcement

**Dependencies:**
- Scaffold complete (✅ from brainstorm session)
- `configs/default-whitelist.toml` exists (✅)

**Risks / Watch-fors:**
- `gio` may not be present on Ubuntu server minimal — test the fallback path
- TOML parser: use stdlib-compatible library already in go.mod; avoid adding a new dep

---

## Phase 2: clean-full
**Goal:** Implement all `mu clean` scan categories, wire progress bars, and respect `--include` opt-in for browser caches. This is the highest-user-value command and the acceptance criteria centrepiece.

**Deliverables:**
- Complete scanners: snap disabled revisions, old kernels/headers, browser caches (Chrome, Firefox, VSCode), Docker build cache
- `--include=browser-cache` flag (opt-in categories, off by default)
- Bubbles progress bar during deletion phase
- `SafeDelete` wired for all user-owned paths; `sudo rm` only for `/var/cache/apt` and old kernels
- Unit tests for each scanner function
- `mu clean --dry-run` output matches PRD example format

**Dependencies:**
- Phase 1 (SafeDelete, whitelist)

**Risks / Watch-fors:**
- Old kernel detection: protect running kernel via `uname -r` — easy to get wrong
- Docker daemon check: use socket existence (`/var/run/docker.sock`), not `docker` binary alone
- Snap revision removal requires `sudo snap remove --revision`; must handle gracefully when snap not installed

---

## Phase 3: status-dashboard
**Goal:** Build the live `/proc`-based system dashboard with Bubbletea ticker, health score, and JSON piped output.

**Deliverables:**
- `internal/status/proc.go` — readers for CPU (via `/proc/stat`), RAM (`/proc/meminfo`), disk (`/proc/diskstats` + `syscall.Statfs`), network (`/proc/net/dev`)
- `internal/status/health.go` — health score algorithm (0-100, weighted: CPU 30%, RAM 30%, disk 30%, temp 10%)
- `internal/status/model.go` — Bubbletea model with 1-second tick; `q`/`Ctrl+C` exits cleanly
- JSON output when `!isatty(os.Stdout)`
- Unit tests for /proc parsers with fixture files

**Dependencies:**
- Phase 1 (logger)
- No dependency on Phase 2

**Risks / Watch-fors:**
- CPU % requires two `/proc/stat` reads with a delta — do not read once and divide
- Network bytes must be per-second rate, not cumulative total
- isatty check: use `github.com/mattn/go-isatty` (already in go.mod via Bubbletea transitive dep)

---

## Phase 4: uninstall-tui
**Goal:** Build the interactive `mu uninstall` TUI: package discovery (APT + Snap), Bubbles multi-select list, remnant scanning, and safe removal.

**Deliverables:**
- `internal/uninstall/discover.go` — parses `dpkg -l` and `snap list`; produces `[]Package{Name, Version, Size, Source}`
- `internal/uninstall/remnants.go` — maps package name → known remnant dirs (`~/.config/<app>`, `~/.local/share/<app>`, `~/.cache/<app>`)
- `internal/uninstall/model.go` — Bubbles list multi-select TUI; shows package name + total size (installed + remnants)
- `internal/uninstall/remove.go` — executes `apt purge` (with sudo prompt) + `SafeDelete` for each remnant dir
- Unit tests for dpkg parser, snap parser, remnant scanner

**Dependencies:**
- Phase 1 (SafeDelete, logger)
- Phase 2 not required, but snap graceful-skip pattern already established there

**Risks / Watch-fors:**
- `dpkg -l` output format is whitespace-delimited with package size not included — size comes from `dpkg-query --show --showformat='${Installed-Size}\t${Package}\n'`
- Snap graceful skip: check `which snap` or `snap list 2>/dev/null` exit code
- `apt purge` requires sudo; prompt user clearly, never cache credentials

---

## Phase 5: optimize-polish
**Goal:** Polish `mu optimize` with progress bars and integrated whitelist; polish the main TUI menu with a live health snapshot from the status package.

**Deliverables:**
- Bubbles progress bar or spinner in `mu optimize` during each step
- Whitelist integration: read `[optimize_skip]` from TOML config, merge with `--skip` flag
- Main TUI menu (`cmd/mu/cli/tui.go`) updated: show CPU %, RAM %, disk usage line from `internal/status/proc.go`
- Updated `mu optimize --dry-run` output matches spec format

**Dependencies:**
- Phase 1 (whitelist TOML loader)
- Phase 3 (status proc.go for health snapshot in TUI menu)

**Risks / Watch-fors:**
- TUI menu health snapshot must be a one-shot read (not a ticker) — avoid starting the full status dashboard inside the menu

---

## Phase 6: integration-qa
**Goal:** Validate all acceptance criteria on a clean Ubuntu 24.04 environment, verify binary constraints, and produce distribution-ready artifacts.

**Deliverables:**
- All 9 acceptance criteria checked and passing
- Binary size verified: `ls -lh ./bin/mu` shows < 25 MB
- `make build` from fresh clone (Go 1.21 only) passes
- `mu clean` timed: < 90 seconds on a 512 GB SSD
- `SECURITY.md` documenting safe usage, protected paths, and responsible disclosure
- `README.md` with install, usage, and philosophy sections
- `scripts/install.sh` tested: curl-pipe install works end-to-end

**Dependencies:**
- All prior phases complete

**Risks / Watch-fors:**
- Binary size may exceed 25 MB if debug symbols not stripped — ensure `-ldflags "-s -w"` in Makefile
- Fresh-clone Go version: go.mod currently declares `go 1.24.2` due to Bubbles v1.0.0 — update SPEC constraint and README accordingly (actual minimum is 1.24.2, not 1.21)
- `gio` absent on CI/server: document in README as optional dep
