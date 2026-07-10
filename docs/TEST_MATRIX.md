# Test Matrix

| Story | Contract | Unit | Integration | E2E | Platform | Status | Evidence |
| --- | --- | --- | --- | --- | --- | --- | --- |
| MU-001 | Safety-first destructive paths and truthful outcomes | yes | yes | no | no | in_progress | unit/race/vet/static/vuln/coverage/build/smoke pass; VM proof pending |
| MU-002 | Root-based status and additive scan errors | yes | yes | no | no | implemented | `go test ./internal/status ./internal/audit -count=1` |
| MU-003 | Pinned fresh-clone Harness bootstrap | yes | yes | no | no | implemented | fresh snapshot bootstrap/init/import/matrix/audit pass; checksum mismatch rejected |

## Automated Gates

```bash
go test ./... -count=1
go test ./... -race -count=1
go vet ./...
make coverage
staticcheck ./...
govulncheck ./...
make build
make smoke
```

`internal/utils`, `clean`, `uninstall`, and `optimize` must each remain at or above 80% statement coverage.

## Platform Gate

Ubuntu 22.04 and 24.04 disposable VMs must prove real APT preview/execution, Snap failure handling, journal cleanup, same- and cross-filesystem trash, privilege cancellation boundaries, and interrupted-operation recovery before MU-001 can be marked implemented or released.
