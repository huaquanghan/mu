# Context: status-dashboard

## Goal
Build the live system dashboard: read `/proc` for CPU, RAM, disk, network; compute a health score; render with Bubbletea ticker; output JSON when piped.

## Spec Hooks
- Req 10: live dashboard — CPU %, RAM %, disk, network I/O, health score; JSON when piped
- Req 11: Bubbletea + Lipgloss for rendering
- Constraint: < 150 MB memory peak; dashboard must exit cleanly on `q` / `Ctrl+C`

## Locked Decisions
- **CPU %**: read `/proc/stat` twice (t=0 and t=1s); delta of `(non-idle / total)` ticks gives usage. Single read is meaningless.
- **RAM**: parse `/proc/meminfo` — `MemTotal`, `MemAvailable`, `SwapTotal`, `SwapFree`
- **Disk**: `syscall.Statfs` on each mounted filesystem from `/proc/mounts`; skip pseudo-filesystems (`tmpfs`, `proc`, `sysfs`, `devtmpfs`, etc.)
- **Network I/O**: parse `/proc/net/dev` — store previous read, compute bytes/sec delta per interface; skip `lo`
- **Health score**: weighted average — CPU (30%), RAM (30%), disk free % (30%), swap usage (10%). Score = 100 − weighted_pressure. No temperature sensor in MVP (hardware access too varied).
- **JSON output**: check `isatty(os.Stdout)` using `github.com/mattn/go-isatty` (already in go.mod). If not TTY, `json.NewEncoder(os.Stdout).Encode(snapshot)` and exit immediately (no ticker).
- **Tick rate**: 1-second ticker via `tea.Tick(time.Second, ...)`
- **Exit**: `q`, `Q`, `Ctrl+C` all call `tea.Quit`

## Assumptions
- `/proc` is available (Linux-only; macOS/Windows are explicitly out of scope)
- Only the first CPU aggregate line in `/proc/stat` is used (not per-core in MVP)
- Network: skip interfaces with zero bytes (inactive VPNs, etc.)
- Health score thresholds: CPU > 80% = pressure, RAM > 80% = pressure, disk < 10% free = pressure

## Canonical Refs
- `.planning/SPEC.md` — Req 10, Constraint (< 150 MB memory)
- `.planning/ROADMAP.md` — Phase 3
- `internal/status/status.go` — existing stub
- `github.com/mattn/go-isatty` — already in go.mod (transitive via Bubbletea)

## Rejected Options
- **Using `top`/`htop` subprocess**: subprocess output parsing is fragile; direct `/proc` reads are reliable and fast
- **cgo / C bindings for sysinfo**: breaks `CGO_ENABLED=0` static binary requirement
- **Per-core CPU display in MVP**: deferred — adds UI complexity; aggregate is sufficient for health score
- **Temperature via `lm-sensors`**: hardware sensor access varies wildly by machine; deferred to v0.2

## Deferred Ideas
- Per-core CPU bars (v0.2)
- GPU usage if NVIDIA present (v0.2)
- Temperature via `/sys/class/thermal` (v0.2)
- `--interval=N` flag for refresh rate (v0.2)
