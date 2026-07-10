# Harness

Harness is the project-local operating layer for the existing `mu` Go CLI. It records intake, stories, proof, decisions, and traces. It does not contain or build Harness CLI source.

## Bootstrap

The CLI is a pinned upstream binary and is ignored by Git.

```bash
make harness-bootstrap
make harness-init
scripts/bin/harness-cli query matrix
```

`make harness-init` also imports the committed matrix, decisions, and backlog into durable state. `scripts/harness-bootstrap.sh` supports Linux and macOS on x64 and arm64. It downloads `harness-cli-v0.1.11`, verifies the committed platform checksum and reported version, then installs `scripts/bin/harness-cli`. Unsupported platforms, missing pins, and checksum or version mismatches fail closed.

Durable state lives in ignored `harness.db`. Version-controlled migrations live in `scripts/schema/`.

## Task Loop

1. Classify the request with `docs/FEATURE_INTAKE.md`.
2. Record intake and locate the affected product contract and story.
3. Read `scripts/bin/harness-cli query matrix`.
4. For an external capability, query `query tools --capability <name> --status present`; no provider is a clean skip.
5. Implement only within the selected lane and run the story proof.
6. Update the matrix and story evidence.
7. Record a trace using `docs/TRACE_SPEC.md`.

Filesystem deletion, APT/Snap work, privilege changes, safety configuration, installers, dependencies, and release changes are `high-risk`.

## Durable Commands

```bash
scripts/bin/harness-cli init
scripts/bin/harness-cli import brownfield
scripts/bin/harness-cli intake --type maintenance --summary "..." --lane high-risk
scripts/bin/harness-cli story add --id MU-001 --title "..." --lane high-risk --verify "go test ./... -count=1"
scripts/bin/harness-cli story update --id MU-001 --status in_progress
scripts/bin/harness-cli story verify MU-001
scripts/bin/harness-cli story verify-all
scripts/bin/harness-cli query matrix
scripts/bin/harness-cli audit
scripts/bin/harness-cli propose
scripts/bin/harness-cli trace --summary "..." --outcome success
```

## Source Hierarchy

1. User request and accepted story packet.
2. `docs/product/README.md` for current user-visible behavior.
3. Go code and executable tests.
4. `docs/TEST_MATRIX.md` and durable story records for proof status.
5. Decisions for distribution or architecture rationale.

No release claim is valid until automated gates pass and required destructive scenarios have passed on disposable Ubuntu 22.04 and 24.04 VMs.
