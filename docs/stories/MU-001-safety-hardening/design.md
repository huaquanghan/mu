# Design

## Destructive Boundaries

`utils.ValidateCleanupRoot` rejects relative roots, `/`, protected system roots, the home directory, and ancestors of home. `ValidateCleanupCandidate` requires an absolute descendant and rejects top-level symlinks. `SafeDelete` fails when user configuration is malformed.

## Package Management

`clean.ParseAutoremoveSimulation` reads only APT `Remv` and `Purg` preview records. `clean.RunAutoremove` executes APT's policy command without a short transaction timeout. Audit uses the same canonical clean action, avoiding duplicate autoremove execution.

Uninstall selection keys are `source:name`. Every package gets a `RemovalResult`; failed removals retain remnants. A successful package cannot remove a remnant still owned by another installed source or app identity.

## Trash

Home trash is used only on the same filesystem. Other files use a valid mount-local `.Trash/<uid>` or `.Trash-<uid>`. The original path is percent encoded. A reserved temporary metadata name prevents collisions; metadata finalization is atomic. Failed finalization rolls the move back, or returns the final recovery path if rollback also fails.

## Outcomes

External commands use `internal/command.Runner`. Clean and optimize aggregate failures. Optimize states are `success`, `failed`, and `skipped`. Operation log outcomes are written after completion.

## Reporting

Audit report failures return typed exit errors. JSON adds `warnings` and `scan_errors`. Status parses mountinfo, excludes pseudo-filesystems, scores root disk pressure, clamps reset CPU counters, and reports unavailable metrics.
