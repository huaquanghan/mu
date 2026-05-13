# Plan: clean-full

## Goal
Implement all `mu clean` scan categories, wire progress bars, enforce opt-in for browser caches, and use `SafeDelete` for user-owned files.

## Inputs
- `internal/clean/clean.go` — existing partial (user cache, APT cache, thumbnails, journal scan work)
- `internal/utils/trash.go` — Phase 1 output (required)
- `internal/utils/whitelist.go` — Phase 1 output (required)
- `mu-prd.md` Section 7 — exact dry-run output format

---

## Wave 1 — Independent: add `--include` flag + refactor target registry

### Task 1: refactor `CleanTarget` struct and registry
- steps:
  1. Redefine `CleanTarget` in `internal/clean/targets.go` (new file):
     ```go
     type CleanTarget struct {
         ID           string
         Label        string
         scan         func() (int64, error)
         execute      func(dryRun bool) error
         RequiresSudo bool
         OptIn        bool
     }
     ```
  2. Register all targets in `allTargets() []CleanTarget` — start with existing ones (UserCache, Thumbnails, AptCache, JournalLogs)
  3. Add `--include` string-slice flag to `cleanCmd` in `cmd/mu/cli/clean.go`
  4. Filter targets in `clean.Run()`: skip `OptIn` targets unless their `ID` is in `opts.Include`
- expected outputs:
  - `internal/clean/targets.go`
  - Updated `internal/clean/clean.go` and `cmd/mu/cli/clean.go`
- verify:
  - `go build ./...` passes
  - `mu clean --dry-run` still shows existing categories

---

## Wave 2 — Parallel: implement remaining scan categories

### Task 2: snap disabled revisions scanner
- steps:
  1. Check `exec.LookPath("snap")` — if not found, return `(0, nil)` (graceful skip)
  2. Run `snap list --all`, parse lines with "disabled" in the 6th field
  3. For each disabled revision: `snap remove --revision <rev> <name>` (sudo)
  4. Add as `CleanTarget{ID: "snap", RequiresSudo: true}`
- expected outputs:
  - `internal/clean/scanners/snap.go`
- verify:
  - On a machine with snap: `mu clean --dry-run` shows snap category with size estimate
  - On a machine without snap: `mu clean --dry-run` omits snap category silently

### Task 3: old kernel/headers scanner
- steps:
  1. Run `uname -r` to get running kernel version
  2. Run `dpkg-query --show --showformat='${Package} ${Installed-Size}\n' 'linux-image-*' 'linux-headers-*'`
  3. Filter: exclude packages matching running kernel; keep at most one non-running backup
  4. Removal: `sudo apt purge -y <packages>`
  5. Add as `CleanTarget{ID: "kernels", RequiresSudo: true}`
- expected outputs:
  - `internal/clean/scanners/kernels.go`
- verify:
  - Dry-run output lists old kernels (if any) but never the running kernel package

### Task 4: browser cache scanner (opt-in)
- steps:
  1. Define known browser cache paths:
     - Chrome: `~/.config/google-chrome/Default/Cache`
     - Firefox: `~/.mozilla/firefox/*/cache2` (glob)
     - VSCode: `~/.config/Code/CachedData`, `~/.config/Code/CachedExtensions`
  2. Use `filepath.Glob` for Firefox wildcard path
  3. Sum sizes, mark `OptIn: true`
  4. Deletion: `SafeDelete` each path
- expected outputs:
  - `internal/clean/scanners/browser.go`
- verify:
  - `mu clean --dry-run` omits browser cache
  - `mu clean --dry-run --include=browser-cache` shows browser cache with size

### Task 5: Docker build cache scanner
- steps:
  1. Check `/var/run/docker.sock` existence — if absent, skip entirely
  2. If present: run `docker system df --format json` to get build cache size
  3. Deletion: `docker system prune -f --volumes` with a separate "Type YES to also prune Docker volumes:" prompt
  4. Add as `CleanTarget{ID: "docker"}`
- expected outputs:
  - `internal/clean/scanners/docker.go`
- verify:
  - On machine without Docker socket: category absent from output
  - On machine with Docker: size shown correctly

---

## Wave 3 — Depends on Wave 2: wire progress bar + final integration

### Task 6: wire Bubbles progress bar into deletion phase
- steps:
  1. In `clean.execute()`, switch from simple `fmt.Println` loop to a Bubbletea `tea.Program` model
  2. Model state: current target index, progress float (0.0–1.0 per target), done bool
  3. Use `github.com/charmbracelet/bubbles/progress` for each target; advance to 1.0 when target complete
  4. After all targets: exit Bubbletea, print final size delta summary
- expected outputs:
  - Updated `internal/clean/clean.go` with Bubbletea progress model
- verify:
  - `mu clean` (with YES confirmation) shows animated progress bar per category
  - Final output shows "Before: X GB | After: Y GB | Freed: Z GB"

### Task 7: unit tests for scanners
- steps:
  1. `internal/clean/scanners/kernels_test.go`: mock `dpkg-query` output (string fixture), assert correct packages selected for removal, assert running kernel never included
  2. `internal/clean/scanners/browser_test.go`: create temp dirs matching browser cache paths, assert sizes computed correctly
  3. `internal/clean/scanners/snap_test.go`: mock `snap list --all` output, assert disabled revisions parsed
- expected outputs:
  - Three `_test.go` files
- verify:
  - `go test ./internal/clean/...` passes with 0 failures

---

## Risks / Watch-fors
- Browser cache glob for Firefox returns empty if no profile exists — handle nil gracefully
- Running kernel exclusion: package name format is `linux-image-<version>-generic`; match against `uname -r` output which is just `<version>-generic` — prepend `linux-image-` for comparison
- Docker prune is irreversible for volumes — the extra confirmation prompt is mandatory
