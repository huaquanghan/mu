# Context Rules

Read the smallest set that proves the requested change.

## Always

- `AGENTS.md`
- `README.md`
- `docs/HARNESS.md`
- `docs/FEATURE_INTAKE.md`
- `docs/ARCHITECTURE.md`
- `docs/product/README.md`
- `scripts/bin/harness-cli query matrix`

If the binary is missing, run `make harness-bootstrap` first.

## By Surface

| Surface | Read |
| --- | --- |
| deletion, cache, XDG, whitelist, trash | `src/trash.rs`, `src/clean/`, `src/xdg.rs`, `src/whitelist.rs`, safety story validation |
| APT, Snap, uninstall, privilege | `src/clean/`, `src/uninstall/`, `src/optimize.rs` |
| status or audit report | `src/status/`, `src/audit/`, JSON tests |
| CLI flags or exit codes | `src/cli.rs`, affected `src/` module, smoke target |
| Harness bootstrap or durable records | `scripts/README.md`, decision 0005, schema files, CLI help |
| release | CI workflow, Makefile, dependency files, VM evidence |

High-risk work must read `docs/stories/MU-001-safety-hardening/` and keep its validation evidence current. Before final trace, re-read validation output, matrix, story status, and `git status --short`.
