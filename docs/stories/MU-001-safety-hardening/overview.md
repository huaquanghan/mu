# MU-001 Safety-First Hardening

Type: `bug`

Lane: `high-risk`

Status: `in_progress`, release blocked

## Current Behavior

The prior implementation built kernel purge lists from package names, coupled same-name APT and Snap selections, removed remnants after failed package removals, swallowed cleanup failures, accepted dangerous XDG roots, used a non-transactional trash fallback, and treated unavailable or pseudo-filesystem metrics as healthy input.

The 2026-07-14 review also found protected-path ancestor deletion, stale whitelist caching, unsafe existing trash directories, missing sudo boundaries, incorrect Docker decoding, coupled APT/Snap discovery failures, stale CPU rendering, false root-disk pressure, and terminal overflow.

Harness content also described source, workflows, installers, and proof that do not exist in `mu`.

## Target Behavior

- APT policy owns the complete autoremove candidate set.
- Package removal and remnant cleanup are source-qualified and transactional per package.
- Destructive path validation and malformed configuration fail closed in dry-run and real execution.
- Trash metadata and movement are collision-safe, filesystem-aware, atomic, and recoverable.
- Existing filesystem trash directories are accepted only when type, symlink, sticky-bit, owner, and exact mode checks pass.
- Journal and system MIME mutations cross sudo only at their command boundaries; font cache refresh remains unprivileged.
- APT and Snap discovery return successful partial results with visible warnings.
- Clean and optimize failures remain visible and return nonzero after independent work finishes.
- Audit/status contracts expose scan errors and preserve report exit codes without library `os.Exit`.
- Status CPU recovery requires two consecutive samples, and every status line fits the terminal display width.
- Automated gates run on Ubuntu 22.04 and 24.04; destructive VM proof remains a release gate.
- Fresh clones bootstrap a pinned, checksummed Harness CLI.

## Non-Goals

- No new end-user command or flag.
- No live APT, Snap, journal, or destructive filesystem validation on the development machine.
- No release, commit, or push.
