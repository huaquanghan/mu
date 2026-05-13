# Context: integration-qa

## Goal
Validate all 9 spec acceptance criteria, verify binary constraints (size < 25 MB, build < 60s), produce distribution-ready docs (README, SECURITY.md), and confirm `scripts/install.sh` works end-to-end.

## Spec Hooks
- All acceptance criteria from SPEC.md
- Req 15: binary < 25 MB stripped
- Req 19: `make build` < 60 seconds
- Constraint: Ubuntu 22.04 / 24.04 LTS primary targets

## Locked Decisions
- **README**: sections — Install, Quick Start, Commands (with dry-run examples), Safety Philosophy, Building from Source. No API docs or plugin system.
- **SECURITY.md**: lists protected paths, explains `--dry-run`, explains `gio trash`, gives responsible disclosure email.
- **Binary size**: `CGO_ENABLED=0 go build -ldflags "-s -w"` is already in Makefile. Verify with `ls -lh` after build.
- **Go version note**: go.mod currently declares `go 1.24.2` (forced by Bubbles v1.0.0). Update README and SPEC Constraints to reflect this (minimum is 1.24.2, not 1.21). The GOTOOLCHAIN=auto feature will auto-download 1.24.2 on machines with older Go.
- **Test environment**: run acceptance checks manually on this machine (Ubuntu); document results. A clean VM test is strongly recommended but not automated in MVP.

## Assumptions
- CI/automated testing is out of scope for MVP (GitHub Actions deferred to post-MVP per spec)
- `scripts/install.sh` can only be fully tested after a real GitHub release exists; for MVP, test the script logic locally with a mock binary
- `gio` is present on this developer machine for trash testing

## Canonical Refs
- `.planning/SPEC.md` — Acceptance Criteria section
- `.planning/ROADMAP.md` — Phase 6
- `Makefile` — build targets
- `scripts/install.sh` — install script

## Rejected Options
- **Automated CI in MVP**: out of scope per spec (GitHub Actions deferred to post-MVP)
- **Docker-based test environment**: adds complexity; manual VM check is sufficient for MVP

## Deferred Ideas
- GitHub Actions release workflow with GoReleaser (post-MVP per spec)
- `brew install mu` or `apt install mu` (post-v1.0)
- Automated integration test suite on clean Ubuntu 24.04 VM (post-MVP)
