# Plan: uninstall-tui

## Goal
Build the interactive `mu uninstall` TUI: APT + Snap package discovery, Bubbles multi-select list, remnant scanning, and safe removal execution.

## Inputs
- `internal/uninstall/uninstall.go` — existing stub
- `internal/utils/trash.go` — Phase 1 output (required)
- `internal/utils/logger.go` — Phase 1 output (required)

---

## Wave 1 — Independent: package discovery + remnant scanner

### Task 1: implement `internal/uninstall/discover.go`
- steps:
  1. Define `Package` struct: `Name, Version, Source string, InstalledKB int64, RemnantsKB int64, RemnantsFound []string`
  2. `DiscoverAPT() ([]Package, error)`: run `dpkg-query --show --showformat='${Status}\t${Installed-Size}\t${Package}\t${Version}\n'`; filter lines where status starts with "install ok installed"; parse name, version, size in KB
  3. `DiscoverSnap() ([]Package, error)`: check `exec.LookPath("snap")` — if not found, return `(nil, nil)`. Run `snap list`, parse name and version. Mark `Source: "snap"`. Size: set to 0 (snap size detection deferred).
  4. `Discover() ([]Package, error)`: concatenate APT + Snap results, sort by name
- expected outputs:
  - `internal/uninstall/discover.go`
- verify:
  - `go build ./internal/uninstall/...` passes
  - Manual: run a small test program that calls `Discover()` and prints the first 5 packages

### Task 2: implement `internal/uninstall/remnants.go`
- steps:
  1. Define a static mapping `knownAliases map[string]string` for packages where folder name differs from package name: e.g., `"code": "Code"`, `"google-chrome-stable": "google-chrome"`
  2. `FindRemnants(name string) []string`: resolve alias, then check existence of:
     - `~/.config/<name>` and `~/.config/<alias>`
     - `~/.local/share/<name>` and `~/.local/share/<alias>`
     - `~/.cache/<name>` and `~/.cache/<alias>`
     - `/var/lib/<name>` (stat only, no size for root-owned paths in MVP)
  3. Return only paths that exist
  4. `RemnantSize(paths []string) int64`: call `utils.DirSize` on each user-owned path (skip `/var/lib`); sum results
- expected outputs:
  - `internal/uninstall/remnants.go`
- verify:
  - Unit test: create temp dirs matching remnant pattern, assert `FindRemnants` returns them

---

## Wave 2 — Depends on Wave 1: Bubbles multi-select TUI model

### Task 3: implement `internal/uninstall/model.go`
- steps:
  1. Define `item` implementing `list.Item` interface: `FilterValue() string` returns name; render shows name, version, source badge, total size (installed + remnants)
  2. Build a custom `list.DefaultDelegate` that toggles a `selected map[string]bool` on Space key
  3. `Model` struct: `list list.Model`, `selected map[string]bool`, `packages []Package`, `phase string` ("list" | "confirm" | "done")
  4. `Init()`: load packages via `Discover()` + `FindRemnants` + `RemnantSize` in a background `tea.Cmd` (use `tea.ExecProcess` pattern or a msg-based loader)
  5. `Update()`: handle list navigation, Space for multi-select, Enter to advance to "confirm" phase, `q`/`Ctrl+C` to quit
  6. Confirm phase: render "Will remove:" list of selected packages + their remnant paths; prompt "Type YES to continue"
  7. Done phase: show summary of what was removed
- expected outputs:
  - `internal/uninstall/model.go`
- verify:
  - `mu uninstall` opens a list; Space selects packages; Enter shows confirm screen; `q` exits without changes

---

## Wave 3 — Depends on Wave 2: removal execution

### Task 4: implement `internal/uninstall/remove.go`
- steps:
  1. `RemoveAPT(names []string, dryRun bool) error`:
     - If `dryRun`: log "would run: sudo apt purge -y <names>", return nil
     - Else: `exec.Command("sudo", append([]string{"apt", "purge", "-y"}, names...)...).Run()` with Stdout/Stderr → os.Stdout/Stderr; log each name
  2. `RemoveSnap(names []string, dryRun bool) error`: same pattern with `sudo snap remove <name>` per package (snap remove is one at a time)
  3. `RemoveRemnants(paths []string, dryRun bool) error`: call `utils.SafeDelete(path, dryRun)` for each path; log each
  4. Wire into `model.go` confirm phase: call Remove functions in order (APT → Snap → Remnants) after YES confirmation
- expected outputs:
  - `internal/uninstall/remove.go`
  - Updated `internal/uninstall/model.go` confirm execution
  - Updated `internal/uninstall/uninstall.go` to call `tea.NewProgram(model, tea.WithAltScreen()).Run()`
- verify:
  - `mu uninstall --dry-run`: select a package → confirm → output shows "would run" lines, no apt invoked
  - `mu uninstall`: end-to-end with a test package (e.g., `sl`) — installs `sl`, runs `mu uninstall`, selects `sl`, confirms, verifies `sl` is no longer installed

### Task 5: unit tests for discovery and remnants
- steps:
  1. `internal/uninstall/discover_test.go`: parse a fixture `dpkg-query` output string, assert package list
  2. `internal/uninstall/remnants_test.go`: create temp dirs, assert `FindRemnants` and `RemnantSize`
- expected outputs:
  - Two `_test.go` files
- verify:
  - `go test ./internal/uninstall/...` passes

---

## Risks / Watch-fors
- Loading all packages + remnant sizes may take 2-5 seconds on machines with many packages — show a spinner during load (`tea.Cmd` returns a `loadingMsg` when done)
- Remnant size for `/var/lib/<name>` paths would require sudo stat — skip size for those in MVP; show "?" instead
- `apt purge` may prompt for additional removals (dependencies) — pipe `yes` is not safe; show the raw sudo apt output to the user instead
