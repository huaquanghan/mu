# Context: safety-core

## Goal
Deliver the shared safety primitives that every destructive command depends on: safe file deletion via `gio trash`, operations log rotation, and whitelist TOML loading. No command in mu permanently deletes user data without going through this layer.

## Spec Hooks
- Req 1: every destructive command must support `--dry-run`
- Req 2: protected path whitelist enforced at runtime
- Req 3: operations logged to `~/.local/share/mu/operations.log`; suppressed via `MU_NO_OPLOG=1`
- Req 4: use `gio trash` or safe move; never `rm -rf` for user-owned files
- Constraint: no new runtime dependencies; `gio` is part of `glib2` (standard on Ubuntu desktop)

## Locked Decisions
- `SafeDelete(path string, dryRun bool) error` is the single entry point for all user-file removal; system cache removal (`/var/cache/apt`) goes through `sudo rm` separately and is logged the same way
- TOML parsing: use `github.com/BurntSushi/toml` if not already in go.mod, otherwise use a stdlib-compatible approach. Check go.mod first before adding a dep.
- Log rotation trigger: when `operations.log` exceeds 10 MB, rename to `operations.log.1` (overwrite any existing `.1`); keep only 2 files total
- Whitelist merge order: built-in defaults (from `configs/default-whitelist.toml`) → user overrides (`~/.config/mu/config.toml`). User file wins on conflicts.
- If `gio` binary absent: fall back to moving the target to `~/.local/share/Trash/files/` manually following the FreeDesktop Trash spec (create a `.trashinfo` file)

## Assumptions
- Ubuntu desktop installs have `gio` available; server minimal may not — fallback path must work silently
- `~/.config/mu/config.toml` may not exist — treat as empty (no error)
- `configs/default-whitelist.toml` is embedded at build time or read from the binary's install prefix; for MVP, read from the relative path at runtime

## Canonical Refs
- `.planning/SPEC.md` — Requirements 1-5, Constraints section
- `.planning/ROADMAP.md` — Phase 1 deliverables
- `internal/utils/logger.go` — existing logger (needs rotation added)
- `internal/utils/paths.go` — existing path protection (IsProtected already implemented)
- `configs/default-whitelist.toml` — existing whitelist config

## Rejected Options
- **`rm -rf` for user files**: rejected per spec (irreversible, safety-first philosophy)
- **Embed TOML in binary**: adds build complexity; for MVP, runtime path read is simpler and easier to let users override
- **Unlimited log file**: rejected — 10 MB cap keeps the log useful without consuming disk

## Deferred Ideas
- User-editable `~/.config/mu/config.toml` with full theme and behavior settings (v0.2)
- Log shipping or structured JSON log format (v0.2)
- `gio trash` with undo support via `mu restore` (v0.2)
