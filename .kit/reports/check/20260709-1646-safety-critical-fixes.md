# CHECK REPORT

Run ID: check-20260709-1646-safety-critical-fixes
Scope: full
Artifact Alignment: skipped
Review Verdict: APPROVE with requests
Phase: none (session plan: safety-critical audit fixes)
Spec: .planning/SPEC.md (MVP context only; not phase harness)
Plan: session plan (safety-critical fixes) | no `.kit/planning/`
Workflow State: none
Cook Run: none
Created At: 2026-07-09 09:46 UTC

## Gate Evidence
- secrets: `git diff HEAD | grep secrets pattern` → none
- tests: `go test ./... -count=1` → pass
- race: `go test ./... -count=1 -race` → pass
- types/vet: `go vet ./...` → pass
- lint: `staticcheck` → not installed (skipped)
- build: `make build` → pass, 3.9M
- smoke: `make smoke` → pass

## Scope
- depth: standard (324+ insertions, 11 files including untracked tests; safety-sink review elevated)
- label: **on target** — all changes map to approved safety-critical plan (whitelist, cache_skip, docker OptIn, install checksum, denylist)

## Artifact Alignment
- status: skipped
- notes:
  - No `.kit/planning/` or `.kit/workflow-state.yml` harness
  - Session-approved plan (interview deep check) is the authority
  - Diff matches plan outcomes 1–5; untracked tests must be included on commit
  - `.planning/SPEC.md` MVP still valid; docker opt-in now matches README

## Findings

### Critical
- none

### Major
- **Release pipeline gap:** `install.sh` now requires `checksums.txt`, but repo has no `.goreleaser.yml` / `.github/workflows` producing that asset. Until the next release publishes it, curl-install fails closed for everyone. Request: document release checklist or add checksum generation before tagging.
- **Untracked tests:** `internal/clean/scan_docker_test.go`, `internal/clean/targets_cache_test.go` are untracked. Commit must include them or proof trail is incomplete.

### Minor / Suggestions
- **Whitelist process cache:** `getWhitelist()` loads once per process; config edits mid-run are ignored. Acceptable for short CLI lifetime; document if long-lived daemon ever appears.
- **LoadWhitelist errors swallowed** in `userCacheTarget` (`wl, _ := ...`): corrupt user TOML falls back to defaults+error ignored — safe but silent; consider debug log when `--debug`.
- **Nested `cache_skip` first-segment rule:** pattern `mozilla/...` skips entire top-level `mozilla` under Execute (by design). Nested-only reclaim is not supported without walking — intentional conservatism.
- **Pre-existing:** SECURITY.md still documents `Type 'YES'`; UI is button default-NO. Not introduced by this diff.
- **Pre-existing:** symlink targets not resolved before protection check in `SafeDelete`.

## Pattern-Fix Completeness
- SafeDelete now uses `IsWhitelisted` (not IsProtected-only). Grep: all user deletes go through SafeDelete (`targets.go`, `scan_browser.go`, `remove.go`). System ops (apt/journal/snap/docker) remain package-manager paths — correct.
- Docker OptIn mirrors browser-cache OptIn pattern — complete.
- cache_skip: applied in user-cache Scan+Execute; no other cache wipe path.

## Autofix
- safe_auto applied: `CLAUDE.md` safety invariants updated (whitelist gate, cache_skip, docker OptIn, install checksums, correct embed path).
- gated_auto: none
- manual requests: release checksum asset pipeline; stage untracked tests

## Next Action
- Stage all changed + untracked files; commit via `git` skill
- Before release: ensure `checksums.txt` ships with binary
- ready for commit; not blocked for merge after untracked tests staged

## Sign-Off

```
scope:              on target
depth:              standard (safety-elevated)
artifact_alignment: skipped: no .kit/planning harness; session plan aligned
gate:               ✅ pass (test, race, vet, build, smoke); staticcheck skipped
review:             APPROVE with requests
blockers:           0 critical, 2 major (release checksums asset; untracked tests)
autofix:            1 safe applied (CLAUDE.md), 0 awaiting confirmation
verification:       go test ./... && go test -race ./... && go vet ./... && make build && make smoke → pass
```
