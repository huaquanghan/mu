---
name: harness-intake-griller
description: Shape requests for the mu Ubuntu maintenance CLI into risk-classified Harness stories with explicit acceptance criteria and proof. Use before implementation when a request needs discussion, feature intake, product or Harness documentation, story shaping, or Symphony-ready execution boundaries, especially for filesystem deletion, package management, privilege, configuration safety, or release work.
---

# Harness Intake Griller

Turn intent into the smallest executable `mu` story whose destructive boundaries and proof requirements are unambiguous.

## Workflow

1. Read `README.md`, `docs/FEATURE_INTAKE.md`, `docs/product/README.md`, and the current test matrix.
2. Restate the requested behavior, non-goals, affected commands, and compatibility constraints.
3. Classify the lane:
   - `high-risk`: filesystem deletion or trash behavior, APT/Snap transactions, `sudo` or other privilege changes, configuration that protects data, release gates, installers, or dependency security work.
   - `normal`: bounded non-destructive CLI behavior or reporting changes.
   - `tiny`: documentation or copy-only changes with no behavioral claim.
4. Ask only for decisions that materially change safety or product behavior. Otherwise state the safe assumption and continue.
5. Create or update a story packet. Use `docs/templates/high-risk-story/` for high-risk work and `docs/templates/story.md` otherwise.
6. Make every acceptance criterion observable and map it to unit, integration, CLI smoke, or Ubuntu platform proof.

## Destructive Acceptance Criteria

For every destructive path, require all applicable criteria:

- Dry-run and real execution traverse the same validation boundary.
- Paths are absolute, remain within a declared root, and reject ambiguous top-level symlinks.
- Invalid protection configuration fails closed before scanning or mutation.
- Package and filesystem failures return nonzero and retain recoverable user data.
- Success is logged only after confirmed completion; failure, skipped, and dry-run are distinct.
- External commands are testable through an injected runner, including arguments, output, exit status, and cancellation.
- Unit tests cover failure and rollback behavior. Disposable Ubuntu 22.04 and 24.04 proof covers real APT, Snap, journal, trash, privilege, and interruption behavior when touched.

## Output Contract

Produce:

- lane and risk flags;
- current and target behavior;
- in-scope and out-of-scope boundaries;
- phased implementation with stop conditions;
- acceptance criteria and exact proof commands;
- platform scenarios that remain release blockers.

Do not mark a story release-ready while any required VM scenario lacks evidence.
