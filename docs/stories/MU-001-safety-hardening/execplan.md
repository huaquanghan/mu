# Execution Plan

## Phases

1. Safety boundaries and command runner: implemented.
2. APT policy and uninstall transactionality: implemented.
3. Truthful clean, optimize, audit, and status outcomes: implemented.
4. Review-finding regression tests and 80% destructive-package coverage: unit, race, vet, coverage, static analysis, vulnerability, and direct compile gates pass locally.
5. Ubuntu 22.04 and 24.04 CI: workflow added, remote run pending.
6. Disposable VM destructive scenarios: pending, release blocker.
7. Pinned Harness bootstrap and `mu`-specific state: implemented and fresh-snapshot verified on Linux x64.

## Local Ownership Proof

The owner corrected the existing `root:root` artifacts with `sudo chown -R "$(id -u):$(id -g)" bin`. Two consecutive `make build` runs passed, `make run` opened and exited cleanly as the current user, and `make smoke` passed. Final ownership is `tinhpt:tinhpt` for both `bin/` and `bin/mu`.

## Stop Conditions

- Do not run live destructive commands on the development host.
- Do not release while VM proof or CI is pending or failed.
- Do not weaken a safety validation or test to make smoke pass.
- Do not change existing command names, remove JSON fields, or merge APT/Snap identities.
- Keep MU-001 `in_progress` while Ubuntu 22.04/24.04 platform proof is incomplete.
