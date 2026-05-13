# Plan: status-dashboard

## Goal
Build the live `/proc`-based dashboard: CPU %, RAM, disk, network I/O, health score, Bubbletea ticker model, and JSON piped output.

## Inputs
- `internal/status/status.go` — existing stub
- `github.com/mattn/go-isatty` — already in go.mod (transitive)

---

## Wave 1 — Independent: /proc readers + health score

### Task 1: implement `internal/status/proc.go`
- steps:
  1. `ReadCPU() (idle, total uint64, error)`: parse `/proc/stat` first line, return raw ticks
  2. `CPUPercent(prev, curr CPUSample) float64`: `(1 - delta_idle/delta_total) * 100`
  3. `ReadMemory() (MemStats, error)`: parse `/proc/meminfo` — extract `MemTotal`, `MemAvailable`, `SwapTotal`, `SwapFree`
  4. `ReadDisk() ([]DiskStat, error)`: iterate `/proc/mounts`, call `syscall.Statfs` on each mount; skip `tmpfs`, `proc`, `sysfs`, `devtmpfs`, `cgroup`, `debugfs`
  5. `ReadNetwork() (map[string]NetStat, error)`: parse `/proc/net/dev`, skip `lo`; store `{RxBytes, TxBytes}` per interface
- expected outputs:
  - `internal/status/proc.go` with 5 exported functions and 4 exported structs
- verify:
  - `go build ./internal/status/...` passes
  - `go test ./internal/status/...` with fixture files (see Task 3) passes

### Task 2: implement `internal/status/health.go`
- steps:
  1. `HealthScore(cpu float64, mem MemStats, disks []DiskStat) int`: compute 0-100
  2. CPU score: `30 * (1 - cpu/100)`
  3. RAM score: `30 * (MemAvailable / MemTotal)`
  4. Disk score: `30 * (min free% across non-tmpfs mounts / 100)`
  5. Swap score: `10 * (SwapFree / SwapTotal)`; if no swap: full 10 points
  6. Sum and clamp to [0, 100]
- expected outputs:
  - `internal/status/health.go`
- verify:
  - Unit test: `HealthScore(0, full_mem, empty_disks)` returns 100; `HealthScore(100, 0, 0)` returns 0

---

## Wave 2 — Depends on Wave 1: Bubbletea model + JSON output

### Task 3: implement `internal/status/model.go`
- steps:
  1. Define `Model` struct: `prevCPU CPUSample`, `cpu float64`, `mem MemStats`, `disks []DiskStat`, `nets map[string]NetStat`, `prevNets map[string]NetStat`, `netRates map[string]NetRate`, `health int`
  2. `Init()`: return `tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })`
  3. `Update()`: on `tickMsg`, re-read all `/proc` sources; compute CPU delta and net rates; next tick via recursive `tea.Tick`; on `tea.KeyMsg` `q`/`Q`/`ctrl+c`: return `tea.Quit`
  4. `View()`: render with Lipgloss — title bar, CPU bar, RAM bar, disk table, network table, health score badge
  5. In `internal/status/status.go` `Run()`:
     - If `!isatty.IsTerminal(os.Stdout.Fd())`: collect one snapshot, `json.NewEncoder(os.Stdout).Encode(snapshot)`, return
     - Else: `tea.NewProgram(model, tea.WithAltScreen()).Run()`
- expected outputs:
  - `internal/status/model.go`
  - Updated `internal/status/status.go`
- verify:
  - `mu status` opens a live dashboard; press `q` exits cleanly
  - `mu status | cat` outputs a single JSON object and exits

### Task 4: unit tests for /proc parsers
- steps:
  1. Create `testdata/proc_stat.txt`, `testdata/proc_meminfo.txt`, `testdata/proc_net_dev.txt` with realistic fixture content
  2. `internal/status/proc_test.go`: parse fixtures, assert expected field values
  3. `internal/status/health_test.go`: unit test health score computation with known inputs
- expected outputs:
  - `internal/status/testdata/` with 3 fixture files
  - `internal/status/proc_test.go`, `internal/status/health_test.go`
- verify:
  - `go test ./internal/status/...` passes with 0 failures

---

## Risks / Watch-fors
- CPU % requires two reads with a 1-second gap: the first `Update()` tick will have `prevCPU` == zero state; handle gracefully by showing 0% on first tick
- Disk: some mounts appear multiple times in `/proc/mounts` (bind mounts) — deduplicate by `Fsid` from `Statfs` before computing scores
- Network rates: first tick has no `prevNets` — show 0 B/s on first tick
- JSON output: snapshot struct must be exported and serializable; no unexported fields
