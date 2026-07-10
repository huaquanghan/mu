# MU-001 Safety-First Hardening

Lane: `high-risk`

Status: `in_progress`, release blocked

## Current Behavior

The prior implementation built kernel purge lists from package names, coupled same-name APT and Snap selections, removed remnants after failed package removals, swallowed cleanup failures, accepted dangerous XDG roots, used a non-transactional trash fallback, and treated unavailable or pseudo-filesystem metrics as healthy input.

Harness content also described source, workflows, installers, and proof that do not exist in `mu`.

## Target Behavior

- APT policy owns the complete autoremove candidate set.
- Package removal and remnant cleanup are source-qualified and transactional per package.
- Destructive path validation and malformed configuration fail closed in dry-run and real execution.
- Trash metadata and movement are collision-safe, filesystem-aware, atomic, and recoverable.
- Clean and optimize failures remain visible and return nonzero after independent work finishes.
- Audit/status contracts expose scan errors and preserve report exit codes without library `os.Exit`.
- Automated gates run on Ubuntu 22.04 and 24.04; destructive VM proof remains a release gate.
- Fresh clones bootstrap a pinned, checksummed Harness CLI.

## Non-Goals

- No new end-user command or flag.
- No live APT, Snap, journal, or destructive filesystem validation on the development machine.
- No release, commit, or push.
