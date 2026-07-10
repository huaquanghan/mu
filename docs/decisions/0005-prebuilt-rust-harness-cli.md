# 0005 Pinned Prebuilt Harness CLI

Date: 2026-07-10

## Status

Accepted

## Context

`mu` uses Harness durable records but contains no Rust/Cargo Harness source. Committing binaries is undesirable, while an unpinned ignored binary makes fresh clones unusable and unverifiable.

## Decision

- Pin upstream `harness-cli-v0.1.11` in `scripts/harness-cli.version`.
- Commit SHA-256 values for Linux and macOS x64/arm64 in `scripts/harness-cli.sha256`.
- Download only the matching platform asset from the pinned release.
- Verify checksum and `--version` before atomically installing `scripts/bin/harness-cli`.
- Fail closed for unsupported platforms, missing tools, missing pins, and mismatches.
- Keep the binary and `harness.db` ignored.

## Consequences

Fresh clones need network access and `curl` or `wget`. The repo does not claim to build, release, or own upstream Harness CLI internals. Updating the pin requires independently verifying every supported checksum and rerunning fresh-clone proof.
