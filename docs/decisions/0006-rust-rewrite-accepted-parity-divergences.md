# 0006 — Rust rewrite cutover and accepted parity divergences

## Status

Accepted 2026-09-18

## Context

`mu` was rewritten from Go (cobra/bubbletea) to Rust (ratatui/crossterm) as a
drop-in replacement: same commands, flags, exit codes, JSON schemas, TUI
screens, keybindings, and safety invariants. The Go tree (`cmd/`, `internal/`,
`go.mod`, `go.sum`) was deleted in the same commit that shipped the Rust
implementation — there is no commit where both coexist. Parity was verified
against a Go oracle worktree and frozen golden fixtures under `tests/golden/`,
including ~180 argv cases compared byte-for-byte.

Some divergences were found during independent review and deliberately not
fixed, because matching Go would require `unsafe`, new dependencies, or
disproportionate complexity for unreachable or low-value edges.

## Decision

Ship the rewrite with the following accepted divergences rather than hold the
cutover for them:

- **SIGPIPE**: Go exits 141 when a downstream pipe closes; Rust reports a write
  error and exits 1. Matching 141 needs `unsafe` libc signal handling.
- **Completion script bodies**: `mu completion <shell>` prints functional
  scripts that are not byte-identical to cobra's generated scripts. Help text
  and error paths are byte-identical.
- **Side-by-side TTY screen diff**: each TUI flow is pty-verified individually,
  but no automated visual diff against the Go TUI exists (CI is non-TTY).
- **Buffered stdout scanner cap**: Go's 64 KiB `bufio` scanner cap is not
  reproduced; unreachable for `/proc` inputs.
- **Error-text phrasing**: a few io-error strings differ from Go's wording;
  exit codes and error classes match.
- **cwd-deleted dirfd edge**: `File::open(".")` fails if the working directory
  was deleted; Go's fd-relative behavior is not reproduced.
- **`TrashRecovery` error chaining**: no `#[source]` chain on recovery errors.
- **`RunShell` animated widget**: ported but unwired; `clean`/`optimize` use
  `run_plain` like their Go `tea.Exec` counterparts.
- **Menu health collection**: synchronous (~1s block) vs Go's async `tea.Cmd`.
- **Confirm button padding**: approximates Go's `Padding(0,2)`.
- **Menu dispatch error text**: generic where Go was specific; the real error
  still reaches stderr.

Rejected alternative: fixing each divergence before cutover — rejected because
the gate cost (unsafe code, new deps, TTY automation) exceeded the parity value
of edges users cannot reach or cannot observe.

Rollback point for the Go implementation: tag `v0.2.0` / commit `3b4bbe3`
(tree state immediately before the cutover commit).

## Consequences

Future work that needs any listed divergence must implement it deliberately;
the gap is recorded here, not discoverable from tests. The Go oracle is no
longer in-tree — byte-parity work requires checking out `3b4bbe3` or the
frozen `tests/golden/` fixtures. Completion-script and TTY-diff parity can be
revisited without re-porting, since both are isolated surfaces.

## Authority

- Cutover commit `c2cb17f` (`feat: rewrite mu in Rust, retire Go
  implementation`).
- Plan run log `docs/plans/completed/rust-rewrite.md` (Validation entries
  FULL/FULL2/FULL3, independent judges).
- `tests/golden/` fixtures; `tests/cli_integration.rs`; `src/cli.rs` cobra
  emulation comments.
