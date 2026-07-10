# Architecture

`mu` is a Go 1.25.8 CLI for Ubuntu. `cmd/mu/main.go` enters Cobra commands under `cmd/mu/cli/`; no-argument use opens the Bubble Tea menu.

## Packages

| Package | Responsibility |
| --- | --- |
| `internal/audit` | Read-only scanners, findings, report exit codes, and selected remediation orchestration |
| `internal/clean` | Cleanup targets, APT-policy autoremove, Snap revisions, caches, journal, and Docker build cache |
| `internal/uninstall` | APT/Snap discovery, source-qualified selection, package removal, and owned-remnant cleanup |
| `internal/optimize` | Independent maintenance steps with success, failed, and skipped states |
| `internal/status` | `/proc` and mountinfo metrics, root-disk health, JSON and TUI output |
| `internal/utils` | XDG paths, whitelist configuration, cleanup boundaries, trash, sizes, and operation log |
| `internal/command` | Context-aware injectable external command runner |

## Safety Boundaries

- APT decides autoremove eligibility through `apt-get -s autoremove --purge`; real execution is `sudo apt-get autoremove --purge -y`.
- APT and Snap package identities are `{source,name}`.
- Remnants are removed only after that package succeeds and no remaining package owns the app identity or path.
- Destructive paths validate absolute cleanup roots and in-root non-symlink candidates before dry-run or real execution.
- Relative XDG variables are ignored. Malformed user configuration blocks destructive commands.
- User files use `gio trash` or a FreeDesktop-compatible transactional fallback.
- Active APT transactions do not receive short generic timeouts. Read-only command scans use cancellable contexts.
- Operation outcomes are logged only after completion as `success`, `failure`, `dry-run`, or `skipped`.

## Compatibility

Existing command names and flags remain stable. JSON changes are additive. Partial failures and invalid configuration now return nonzero.
