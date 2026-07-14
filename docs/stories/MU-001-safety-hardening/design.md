# Design

## Destructive Boundaries

`utils.ValidateCleanupRoot` rejects relative roots, `/`, protected system roots, the home directory, and ancestors of home. `ValidateCleanupCandidate` requires an absolute descendant and rejects top-level symlinks. `SafeDelete` loads protection configuration for every operation, fails closed when it is malformed, and rejects a protected path, its descendants, and ancestors that contain it in dry-run and real execution.

## Package Management

`clean.ParseAutoremoveSimulation` reads only APT `Remv` and `Purg` preview records. `clean.RunAutoremove` executes APT's policy command without a short transaction timeout. Audit uses the same canonical clean action, avoiding duplicate autoremove execution.

Uninstall selection keys are `source:name`. Every package gets a `RemovalResult`; failed removals retain remnants. A successful package cannot remove a remnant still owned by another installed source or app identity.

## Trash

Home trash is used only on the same filesystem. Other files use a valid mount-local `.Trash/<uid>` or `.Trash-<uid>`. Selection uses `Lstat`; shared `.Trash` must be a non-symlink directory with the sticky bit, while user-owned trash roots plus `files/` and `info/` must belong to the current UID with exact mode `0700`. Invalid existing directories are never repaired. The original path is percent encoded. A reserved temporary metadata name prevents collisions; metadata finalization is atomic. Failed finalization rolls the move back, or returns the final recovery path if rollback also fails.

## Privilege Boundaries

Clean journal runs `sudo journalctl --vacuum-time=30d`; optimize journal runs `sudo journalctl --vacuum-size=500M`. Dry-run does not invoke sudo. MIME refresh runs `sudo update-mime-database /usr/share/mime`, while `fc-cache -f` runs as the current user and still executes after MIME failure. `make run` builds and launches as the current user.

## Discovery and Scanning

Docker decoding uses the real `Reclaimable` JSON field. APT and Snap package discovery execute independently and join source errors while preserving partial packages. Missing Snap remains optional. Missing user cache is zero bytes, journal size forces `LC_ALL=C`, and missing root-disk metrics are reported without manufacturing a critical disk finding.

## Outcomes

External commands use `internal/command.Runner`. Clean and optimize aggregate failures. Optimize states are `success`, `failed`, and `skipped`. Operation log outcomes are written after completion.

## Reporting

Audit report failures return typed exit errors. JSON adds `warnings` and `scan_errors`. Status parses mountinfo, excludes pseudo-filesystems, scores root disk pressure, clamps reset CPU counters, and reports unavailable metrics.

The live status model clears CPU readiness after a failed sample and requires two fresh samples after recovery. Rendering shrinks bars, removes secondary details, then removes bars as width decreases; final ANSI-aware truncation guarantees every line fits the terminal display width.
