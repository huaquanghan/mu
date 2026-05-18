# Cook Run: Remaining Quick Fixes

**Mode:** simple
**Date:** 2026-05-18
**Source:** /think session — R1, R2, R4 from remaining improvements

## Tasks

### R1 — clean.execute() error propagation
- **File:** `internal/clean/clean.go`
- **Problem:** `execute()` swallows all target errors via `continue`, returns `nil` even on total failure
- **Fix:** Track whether any error occurred, return a combined error
- **Verify:** `make build && make test`
- **Status:** DONE — added `failed` counter, returns `fmt.Errorf` when any target fails

### R2 — snapInstalledKB perf regression
- **File:** `internal/uninstall/discover.go`
- **Problem:** `snapInstalledKB()` runs `du -sk` sequentially per snap in the loading goroutine, adding N blocking subprocess calls
- **Fix:** Extracted `populateSnapSizes()` — fans out goroutines per snap, joins via `sync.WaitGroup`
- **Verify:** `make build && make test-race` — all pass, no data races
- **Status:** DONE

### R4 — stale configs/default-whitelist.toml
- **File:** `configs/default-whitelist.toml`
- **Problem:** File is now a stale duplicate of the embedded `internal/utils/default-whitelist.toml`
- **Fix:** Trashed the duplicate. `internal/utils/default-whitelist.toml` is the single source of truth.
- **Verify:** `make build && make smoke` — all pass
- **Status:** DONE

## Verification
- `make build` — ✅ clean compile, 3.9M binary
- `make test` — ✅ all pass
- `make test-race` — ✅ no data races
- `make smoke` — ✅ all checks pass
