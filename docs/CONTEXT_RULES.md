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
| deletion, cache, XDG, whitelist, trash | `internal/utils/`, `internal/clean/`, safety story validation |
| APT, Snap, uninstall, privilege | `internal/clean/`, `internal/uninstall/`, `internal/optimize/` |
| status or audit report | `internal/status/`, `internal/audit/`, JSON tests |
| CLI flags or exit codes | `cmd/mu/cli/`, affected internal package, smoke target |
| Harness bootstrap or durable records | `scripts/README.md`, decision 0005, schema files, CLI help |
| release | CI workflow, Makefile, dependency files, VM evidence |

High-risk work must read `docs/stories/MU-001-safety-hardening/` and keep its validation evidence current. Before final trace, re-read validation output, matrix, story status, and `git status --short`.
