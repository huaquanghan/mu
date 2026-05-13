# Plan: safety-core

## Goal
Deliver `SafeDelete`, log rotation, and whitelist TOML loading — the shared safety layer all commands depend on.

## Inputs
- `internal/utils/logger.go` — existing (needs rotation)
- `internal/utils/paths.go` — existing (`IsProtected` already done)
- `configs/default-whitelist.toml` — existing

---

## Wave 1 — Independent: trash utility + whitelist loader

### Task 1: implement `internal/utils/trash.go`
- steps:
  1. Define `SafeDelete(path string, dryRun bool) error`
  2. Check `IsProtected(path)` — return error immediately if true
  3. If `dryRun`: log the would-be action and return nil
  4. Check `exec.LookPath("gio")` — if found: `exec.Command("gio", "trash", path).Run()`
  5. Fallback: move file to `~/.local/share/Trash/files/<basename>` + write `~/.local/share/Trash/info/<basename>.trashinfo`
  6. On success: call `utils.LogOp("trash", path)`
- expected outputs:
  - `internal/utils/trash.go` with exported `SafeDelete`
- verify:
  - `go build ./internal/utils/...` compiles without errors
  - Manual: create a temp file, call `SafeDelete` with `dryRun=false`, confirm file appears in `~/.local/share/Trash/`

### Task 2: implement `internal/utils/whitelist.go`
- steps:
  1. Add `github.com/BurntSushi/toml` to go.mod if not present (`go get github.com/BurntSushi/toml`)
  2. Define `Whitelist` struct matching `configs/default-whitelist.toml` shape: `ProtectedPaths.System []string`, `ProtectedPaths.ProtectRunningKernel bool`, `CacheSkip.Dirs []string`
  3. `LoadWhitelist() (*Whitelist, error)`: load `configs/default-whitelist.toml` first, then merge `~/.config/mu/config.toml` if present (user file keys override)
  4. `IsWhitelisted(path string, wl *Whitelist) bool`: check against `wl.ProtectedPaths.System` + `IsProtected`
- expected outputs:
  - `internal/utils/whitelist.go`
- verify:
  - `go build ./internal/utils/...` passes
  - Unit test: load default whitelist, assert `/etc` is protected; assert a random temp path is not

---

## Wave 2 — Depends on Wave 1 being compilable

### Task 3: add log rotation to `internal/utils/logger.go`
- steps:
  1. Read `InitLogger`: after opening the file, check `fi.Size() > 10*1024*1024`
  2. If over limit: close, rename `operations.log` → `operations.log.1` (overwrite any existing `.1`), reopen fresh `operations.log`
- expected outputs:
  - Updated `internal/utils/logger.go`
- verify:
  - `go build ./internal/utils/...` passes
  - Manual: write > 10 MB to the log (or lower threshold temporarily), confirm rotation occurs

### Task 4: unit tests for safety primitives
- steps:
  1. Create `internal/utils/trash_test.go`: test `SafeDelete` with `dryRun=true` (no file moved), `dryRun=false` (file moved to trash)
  2. Create `internal/utils/whitelist_test.go`: test `IsWhitelisted` for protected and unprotected paths
  3. Create `internal/utils/paths_test.go`: test `IsProtected` for `/`, `/boot`, `/etc`, and a safe path
- expected outputs:
  - Three `_test.go` files
- verify:
  - `go test ./internal/utils/...` passes with 0 failures

---

## Risks / Watch-fors
- If `gio` uses a different trash path format on some distros, the fallback `.trashinfo` file is the safety net — test both paths
- `configs/default-whitelist.toml` path resolution: in a fresh clone, the file is relative to the binary or repo root — use `os.Executable()` + `filepath.Dir` to locate it; document the assumption
