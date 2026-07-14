# Validation

## Automated Gates

| Gate | Result | Evidence |
| --- | --- | --- |
| Unit | pass | `go test ./... -count=1` |
| Race | pass | `go test ./... -race -count=1` |
| Vet | pass | `go vet ./...` |
| Coverage | pass | utils 81.1%, clean 85.4%, uninstall 82.8%, optimize 89.6% |
| Staticcheck | pass | `/home/tinhpt/go/bin/staticcheck ./...` |
| Vulnerability | pass | `/home/tinhpt/go/bin/govulncheck ./...`: 0 reachable vulnerabilities |
| Direct compile | pass | `CGO_ENABLED=0 go build -o /tmp/mu-codex-proof ./cmd/mu` |
| Make build | pass | two consecutive `make build` runs; 4.2 MB binary; `bin/` remains user-owned |
| Make run | pass | launched unprivileged TUI and exited with `q`, exit 0 |
| Smoke | pass | `make smoke`; all dry-run, audit, status JSON, and size checks passed |
| Harness | pass | `story verify MU-001`, `story verify-all`, and `audit` with entropy 0/100 |
| Skill validation | pass | `quick_validate.py .codex/skills/harness-intake-griller` |
| Harness bootstrap | pass | fresh snapshot bootstrap/init/import/matrix/audit passed on Linux x64; corrupt checksum was rejected |

Focused package gates also pass:

- `go test ./internal/utils ./internal/clean -count=1`
- `go test ./internal/optimize ./internal/uninstall -count=1`
- `go test ./internal/audit ./internal/status -count=1`

## Required Platform Scenarios

Run on disposable Ubuntu 22.04 and 24.04 VMs:

1. Protected descendant, protected ancestor, and configuration reload inside one TUI process.
2. Same-filesystem and cross-filesystem trash with valid directories, symlinks, foreign owners, unsafe modes, collisions, metadata failures, and rollback recovery.
3. Non-root clean and optimize journal success, sudo rejection, and cancellation.
4. System MIME refresh through sudo and font refresh as the current user.
5. Missing `~/.cache` without blocking APT scan or dry-run.
6. Journal size scan under a non-English locale.
7. `make run` without sudo and user-owned artifacts after consecutive builds.
8. Docker `Reclaimable` output on a VM with Docker; Docker absent is a clean skip.
9. APT preview candidate parity, successful autoremove, failure, lock contention, and cancellation without remnant loss.
10. Snap disabled revision success and partial failure.
11. Same-name APT/Snap uninstall isolation and shared-remnant retention.
12. Malformed configuration and dangerous XDG root fail-closed behavior.

All scenarios are pending because no `platform-verification` provider is equipped. Release remains blocked.
