# Product Contract

`mu` safely cleans, uninstalls, optimizes, audits, and reports Ubuntu system health.

## Commands

- `mu clean`: scans default cleanup targets, previews candidates, confirms, then executes. Browser and Docker targets remain opt-in.
- `mu uninstall`: selects installed packages by APT or Snap source and removes owned remnants only after confirmed package success.
- `mu optimize`: runs independent APT, journal, and cache refresh steps and returns nonzero after any failure.
- `mu audit`: reports or interactively applies canonical clean/optimize actions. Report exit codes are 0, 1, and 2.
- `mu status`: reports CPU, memory, root disk, and network metrics; unavailable metrics appear in `scan_errors`.

## Safety Contract

- APT policy is the only source of autoremove eligibility.
- User cleanup roots and candidates are validated before dry-run and real execution.
- Invalid user protection configuration blocks destructive execution.
- User files move to a valid trash location with recovery information on failed rollback.
- APT and Snap packages with the same name are independently selectable.
- Partial failures are visible, logged accurately, and return nonzero.
- JSON fields are not removed or renamed; warnings and scan errors are additive.

## Supported Platform

Ubuntu 22.04 and 24.04 are the release targets. Automated CI is necessary but not sufficient for destructive-path release proof; disposable VM scenarios remain mandatory.
