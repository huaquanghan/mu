# Architecture

`mu` is a Rust CLI for Ubuntu (ported 1:1 from the Go original, removed at the parity-gate cutover). `src/main.rs` enters `cli::run()` — a manual Cobra-compatible argv parser in `src/cli.rs`; no-argument use on a TTY opens the ratatui menu.

## Modules

| Module | Responsibility |
| --- | --- |
| `src/audit` | Read-only scanners, findings, report exit codes, and selected remediation orchestration |
| `src/clean` | Cleanup targets, APT-policy autoremove, Snap revisions, caches, journal, and Docker build cache |
| `src/uninstall` | APT/Snap discovery, source-qualified selection, package removal, and owned-remnant cleanup |
| `src/optimize.rs` | Independent maintenance steps with success, failed, and skipped states |
| `src/status` | `/proc` and mountinfo metrics, root-disk health, JSON and TUI output |
| `src/{paths,whitelist,config,xdg,trash,size,oplog,error}.rs` | XDG paths, whitelist configuration, cleanup boundaries, trash, sizes, and operation log |
| `src/runner.rs` | Injectable external command runner with timeout kill + fake for tests |
| `src/tui` | ratatui widgets: main menu, YES/NO confirm, uninstall search, status dashboard, run shell |

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
