# Validation Report

## Automated Proof

| Gate | Result | Evidence |
| --- | --- | --- |
| Unit | pending | `cargo test` |
| Race | pending | `cargo clippy --all-targets -- -D warnings` |
| Vet | pending | `cargo fmt --check` |
| Coverage | pending | `make coverage` |
| Static | pending | `staticcheck ./...` |
| Vulnerability | pending | `govulncheck ./...` |
| Build | pending | `make build` |
| Smoke | pending | `make smoke` |

## Platform Proof

| Ubuntu | Scenario | Result | Evidence |
| --- | --- | --- | --- |
| 22.04 | destructive-path suite | pending | |
| 24.04 | destructive-path suite | pending | |

## Release Verdict

Blocked until every required gate passes.
