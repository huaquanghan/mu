# Execution Plan

## Phases

1. Safety boundaries and command runner: implemented.
2. APT policy and uninstall transactionality: implemented.
3. Truthful clean, optimize, audit, and status outcomes: implemented.
4. Focused tests and 80% destructive-package coverage: implemented locally, gate verification required.
5. Ubuntu 22.04 and 24.04 CI: workflow added, remote run pending.
6. Disposable VM destructive scenarios: pending, release blocker.
7. Pinned Harness bootstrap and `mu`-specific state: implemented and fresh-snapshot verified on Linux x64.

## Stop Conditions

- Do not run live destructive commands on the development host.
- Do not release while VM proof or CI is pending or failed.
- Do not weaken a safety validation or test to make smoke pass.
- Do not change existing command names, remove JSON fields, or merge APT/Snap identities.
