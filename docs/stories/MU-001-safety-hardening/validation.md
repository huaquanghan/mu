# Validation

## Automated Gates

| Gate | Result | Evidence |
| --- | --- | --- |
| Unit | pass | `go test ./... -count=1` |
| Race | pass | `go test ./... -race -count=1` |
| Vet | pass | `go vet ./...` |
| Coverage | pass | utils 81.0%, clean 85.0%, uninstall 80.2%, optimize 86.3% or higher |
| Staticcheck | pass | `staticcheck ./...` |
| Vulnerability | pass | no reachable vulnerabilities with Go 1.25.8 and `x/sys` 0.44.0 |
| Build | pass | `make build`, 4.2 MB binary |
| Smoke | pass | corrected audit exit handling and JSON pipeline |
| Skill validation | pass | `quick_validate.py .codex/skills/harness-intake-griller` |
| Harness bootstrap | pass | fresh snapshot bootstrap/init/import/matrix/audit passed on Linux x64; corrupt checksum was rejected |

## Required Platform Scenarios

Run on disposable Ubuntu 22.04 and 24.04 VMs:

1. APT preview candidate parity and successful autoremove.
2. APT failure, lock contention, and user cancellation without remnant loss.
3. Snap disabled revision success and partial failure.
4. Same-name APT/Snap uninstall isolation and shared-remnant retention.
5. Journal vacuum success and failure.
6. Same-filesystem and cross-filesystem trash, collision, metadata failure, and rollback failure recovery.
7. Malformed configuration and dangerous XDG root fail-closed behavior.

All scenarios are pending. Release remains blocked.
