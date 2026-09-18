# Test Matrix

| Story | Contract | Unit | Integration | E2E | Platform | Status | Evidence |
| --- | --- | --- | --- | --- | --- | --- | --- |
| MU-001 | Safety-first destructive paths and truthful outcomes | yes | yes | no | no | in_progress | unit/race/vet/static/vuln/coverage, build twice, unprivileged run, smoke, ownership, and direct compile pass on 2026-07-14; VM proof pending |
| MU-002 | Root-based status and additive scan errors | yes | yes | no | no | implemented | `cargo test status:: audit::` |
| MU-003 | Pinned fresh-clone Harness bootstrap | yes | yes | no | no | implemented | fresh snapshot bootstrap/init/import/matrix/audit pass; checksum mismatch rejected |

## Automated Gates

```bash
cargo test
cargo clippy --all-targets -- -D warnings
cargo fmt --check
make build
make smoke
```

## Platform Gate

Ubuntu 22.04 and 24.04 disposable VMs must prove real APT preview/execution, Snap failure handling, journal cleanup, same- and cross-filesystem trash, privilege cancellation boundaries, and interrupted-operation recovery before MU-001 can be marked implemented or released.

Local ownership proof passed: two consecutive builds, unprivileged `make run`, smoke, and final `tinhpt:tinhpt` ownership for every `bin/` artifact.
