# Implementation Notes

/work --notes — 2026-07-09. Spec: session plan “Safety-critical fixes (mu audit)” (interview deep check).

Running log of decisions not fully specified in the plan, deviations, and tradeoffs.

---

## 2026-07-09T10:11Z preflight / mode

**Decision**: Execute as **simple** work (no `.kit/planning/`); authority is the approved session plan + prior implementation already in the tree.
**Spec gap**: Plan lived under `.grok/sessions/.../plan.md`, not harness phase files.
**Tradeoff**: Skipped full-mode phase waves; treated prior code as Wave 0 complete and only closed residual check items.
**Risk**: Future `/work full` needs formal phase artifacts if multi-phase work resumes.

---

## 2026-07-09T prior session — SafeDelete whitelist API

**Decision**: Keep `SafeDelete(path, dryRun)` signature; load whitelist via process-wide `sync.Once` cache (`getWhitelist`). Expose `ResetWhitelistCacheForTest` for tests only.
**Spec gap**: Plan allowed this; did not specify cache invalidation or concurrent test safety beyond race suite.
**Tradeoff**: Config changes mid-process are ignored (fine for CLI lifetime). Thread-safe for concurrent `SafeDelete`; test reset is not intended for production concurrent use.
**Risk**: Long-running embed of `mu` as a library would need explicit reload API.

---

## 2026-07-09T prior session — cache_skip matching rules

**Decision**:
1. Plain patterns (`go-build`): match exact path and all descendants.
2. Glob patterns: segment-wise `filepath.Match` when path depth equals pattern depth; also full-path Match + ancestor Match.
3. Execute uses **top-level first-segment skip** (`ShouldSkipCacheTopLevel`): any pattern whose first segment is `mozilla` skips the whole top-level `mozilla` dir under `~/.cache` (no partial walk/delete).

**Spec gap**: Plan listed three rules; exact interaction of nested globs vs whole-dir skip was ambiguous.
**Tradeoff**: Nested-only reclaim (delete `cache2` but keep `startupCache`) is **not** implemented for top-level Execute. Safer and simpler; frees less space under browser-related cache trees.
**Risk**: Users who only list nested globs without realizing first-segment skip may think more is cleaned than is.

---

## 2026-07-09T prior session — default denylist expansion

**Decision**: Expanded `cache_skip` with common tool caches (`go-build`, `pip`, `npm`, …) **and** whole top-level `mozilla` / `google-chrome` under `~/.cache`.
**Spec gap**: Plan listed candidates; did not rank which are mandatory vs optional.
**Tradeoff**: Less reclaimable space by default; fewer accidental wipes of language package caches.
**Risk**: Desktop “cleanup marketing numbers” may look smaller than PRD’s 10–30GB claim on some machines.

---

## 2026-07-09T prior session — Docker OptIn refactor

**Decision**: Extract `newDockerTarget()` so OptIn is unit-testable without a live Docker socket; `dockerTarget()` still returns nil when socket absent.
**Spec gap**: Plan only said set `OptIn: true`.
**Tradeoff**: One extra constructor; clearer tests.
**Risk**: None material.

---

## 2026-07-09T prior session — install.sh checksums

**Decision**: Fail-closed download of `checksums.txt`; parse GNU sha256sum lines (`hash  mu` or `hash *mu`); compare to local `sha256sum`. No cosign/minisign.
**Spec gap**: Exact failure messages and multi-line checksum file handling.
**Tradeoff**: Existing releases without `checksums.txt` break curl-install until assets are published (accepted fail-closed).
**Risk**: Operational until `make checksums` (or CI) is used at release time.

---

## 2026-07-09T10:11Z — close check major: release checksum helper

**Decision**: Add `make checksums` → writes `bin/checksums.txt` after build; README points maintainers there. No full GoReleaser config (out of plan scope).
**Spec gap**: Plan optional “Makefile one-liner”; check full flagged missing release pipeline as 🟠 Major.
**Tradeoff**: Manual attach on GitHub releases still required; not automated CI.
**Risk**: Human forgets to upload `checksums.txt` → install fails (safe) but support burden.

---

## 2026-07-09T — intentionally NOT done (plan out of scope / deferred)

| Item | Why |
|------|-----|
| optimize always prints “complete” | reliability UX, not path safety |
| RemoveSnap log-after-error | audit accuracy |
| SECURITY.md Type YES vs button | docs-only drift |
| `protect_running_kernel` runtime use | kernels already gated via `uname -r` |
| Symlink resolve before IsWhitelisted | pre-existing; larger design |
| Age-based cache clean | rejected in interview |
| Allowlist-only ~/.cache | rejected in interview |

---

## 2026-07-09T10:11Z — verification evidence (this run)

- `go test ./...` pass; `-race` pass; `go vet` pass
- `make build` → 3.9M; `make checksums` → `bin/checksums.txt` (`584dafb9…  mu`)
- `make smoke` pass
- Default `clean --dry-run`: no Docker line
- `clean --dry-run --include=docker`: Docker line present

---

## Files touched (implementation surface)

| Path | Role |
|------|------|
| `internal/utils/trash.go` | SafeDelete → IsWhitelisted |
| `internal/utils/whitelist.go` | cache, MatchCacheSkip, ShouldSkipCacheTopLevel |
| `internal/utils/default-whitelist.toml` | expanded denylist |
| `internal/utils/*_test.go` | unit coverage |
| `internal/clean/targets.go` | apply cache_skip scan/execute |
| `internal/clean/scan_docker.go` | OptIn + newDockerTarget |
| `internal/clean/scan_docker_test.go` | OptIn test |
| `internal/clean/targets_cache_test.go` | denylist integration |
| `scripts/install.sh` | checksum verify |
| `Makefile` | `checksums` target |
| `README.md` | install + checksum note |
| `CLAUDE.md` | safety invariants sync (check autofix) |
