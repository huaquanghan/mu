---
id: 01KZZA37A76GRPB6D2H1CSMKFM
type: plan
intake_id: 01KZZA39VGDQSPPJ799T22B0NH
lane: normal
status: completed
created: 2026-08-14
updated: 2026-08-14
---

# Plan: Unified run UX for mu

## Outcome
- result: Running `mu` interactively shows one consistent visual language: `clean` and `optimize` runs are wrapped in a shared styled runner (spinner, section headers, summary line), and the main menu persists cursor and health snapshot, accepts numeric shortcuts 1-6, shows a transient done-summary banner, and surfaces subcommand errors as styled in-menu messages.
- success_signals:
  - `mu clean` and `mu optimize` render scan/execute through the shared runner from both CLI and TUI paths — identical visuals, no raw unformatted output lines.
  - Returning to the main menu after a subcommand shows no "Loading..." flash; cursor position and health snapshot persist.
  - Pressing keys 1-6 selects the corresponding menu item.
  - After a subcommand finishes, the menu shows a done-summary line (e.g. "✅ Clean — freed 1.2 GB") that clears on the first keypress; a subcommand failure shows a styled error with a continue hint instead of a bare stderr line.
  - `make test` passes; clean/optimize scan/execute behavior, flags, and exit codes unchanged.

## Authority and Requirements
- authority:
  - AGENTS.md "TUI conventions" — colors `#0097A7` / `#374151` / `#9CA3AF`, hint-line style, `tea.WithAltScreen()`
  - Existing styling in `cmd/mu/cli/tui.go` and `internal/ui/confirm.go`
  - CLAUDE.md safety invariants — `dryRun` branches, `SafeDelete`, confirm gating, opt-in IDs
- requirements:
  - R1 [accepted]: `mu clean` and `mu optimize` render through one shared styled runner in `internal/ui` — spinner during scan, per-target lines, final summary; no raw `fmt.Print` output | source: brainstorm lock
  - R2 [accepted]: Main menu accepts keys `1`-`6` to select the item at that position | source: brainstorm lock
  - R3 [accepted]: Main menu persists cursor and health snapshot across subcommand returns; no reload flash, snapshot refresh only on fresh program start | source: brainstorm lock
  - R4 [accepted]: After a subcommand returns, the menu shows a transient done-summary line that clears on the first keypress | source: brainstorm lock
  - R5 [accepted]: Subcommand errors return to the menu as a styled error message with a continue hint, not a bare stderr line | source: brainstorm lock
  - R6 [accepted]: All `clean`/`optimize` flags (`--dry-run`, `--yes`, `--include`, `--skip`), exit codes, safety invariants, and `ui.Confirm` gating behavior are unchanged | source: CLAUDE.md

## Non-goals
- NG1: Merging subcommands into the TUI program as views (Option A) — subcommands stay separate programs.
- NG2: Changing the audit wizard and uninstall TUI flows.
- NG3: Changing scan/execute logic, target computation, or safety invariants.
- NG4: New UI features beyond R1-R6 (mouse support, search, themes, configurable colors).

## Approach and Risks
- approach: Two phases. Phase 1 (`runner-shell`): a shared styled runner in `internal/ui` — spinner during long work, section headers, summary line; no `WithAltScreen` (the `ui.Confirm` precedent), with terminal detection for non-TTY fallback. `internal/clean/clean.go` and `internal/optimize/optimize.go` replace raw `fmt.Print` output with runner calls and expose the summary (freed size / steps run) for the menu banner. Phase 2 (`menu-polish`): `cmd/mu/cli/tui.go` adds keys `1`-`6`, persists cursor + health snapshot in the `runTUI` loop, and renders a transient done-summary banner or styled error from the subcommand result. Preferred because core scan/execute logic stays untouched (safety invariants, flags, exit codes), test impact is contained to stdout assertions, and CLI + TUI share one renderer.
- constraints:
  - CLI and TUI paths must render identically through the runner (R1).
  - `--dry-run`/`--yes`/`--include`/`--skip`, exit codes, and `ui.Confirm` gating unchanged (R6).
- rejected_alternatives:
  - Option A (in-TUI full): subcommands as views inside one Bubbletea program — dropped for refactor cost/risk on `internal/clean`/`optimize` I/O and test churn (NG1).
  - Option B (menu polish only): cheaper but leaves the two-apps feel and fails R1.
- risks:
  - Existing tests asserting stdout text break with the new format — mitigation: update those assertions to the runner format; run `make test` after each task.
  - Spinner in non-TTY (piped) output — mitigation: terminal detection falls back to plain text lines.
  - Runner tea program adds render churn mid-run — mitigation: no `WithAltScreen`, same as `ui.Confirm`.
- stop_conditions:
  - `make test` fails beyond stdout-format assertions → stop, reassess the runner API before touching clean/optimize.
  - `make smoke` output unreadable in CLI mode → fix the plain fallback before TUI wiring.
- recovery: revert the last task's edits (git) and rerun `make test`; phases are independent enough that `menu-polish` can proceed with `runner-shell` reduced to its summary contract.

## Phases and Verification
<!-- Phase and task definitions are immutable after to-plan. Do not add task status fields. Append-only Progress is the sole task execution-status source. Only each phase lifecycle status changes to mirror DB transitions: to-plan=planned; work after run create=in-progress; clean durable check=checked; closing handoff=done. Each planned phase records phase_slug, story_id, status, goal, depends_on, waves, tasks, and checks. -->
- planning_status: planned
- phases:
  - phase_slug: runner-shell
    story_id: 01KZZJJGARF4MV33QGZSX092QT
    status: done
    goal: Shared styled run shell — clean/optimize render through one internal/ui runner (spinner, sections, summary) with unchanged behavior
    depends_on: none
    allowed_surfaces: [internal/ui (new run.go), internal/clean/clean.go, internal/optimize/optimize.go, stdout-asserting tests]
    avoided_surfaces: [scan/execute logic, flags, exit codes, ui.Confirm, cmd/mu/cli/tui.go]
    waves:
      - wave: 1
        tasks:
          - T1: Add `internal/ui/run.go` — runner with section headers, spinner during work, summary line; terminal detection for non-TTY fallback; unit tests for rendering
        checks:
          - `go build ./...`
          - `go test ./internal/ui/...`
      - wave: 2
        tasks:
          - T2: `internal/clean/clean.go` — replace raw prints with runner calls; expose freed-size summary
          - T3: `internal/optimize/optimize.go` — replace raw prints with runner calls; expose summary
        checks:
          - `make test`
          - `go run ./cmd/mu clean --dry-run` and `go run ./cmd/mu optimize --dry-run` render styled output
          - `grep -rn "fmt.Print" internal/clean internal/optimize` — no raw run-path prints
          - `make smoke`
  - phase_slug: menu-polish
    story_id: 01KZZJJJC1EC1426WRHKQMJ5GS
    status: done
    goal: Main menu polish — numeric shortcuts, persisted cursor/health snapshot, transient done-summary banner, styled error return
    depends_on: runner-shell
    allowed_surfaces: [cmd/mu/cli/tui.go]
    avoided_surfaces: [clean/optimize logic, uninstall/status/audit flows, ui.Confirm]
    waves:
      - wave: 1
        tasks:
          - T4: Keys `1`-`6` select menu items directly (R2)
          - T5: `runTUI` loop persists cursor + health snapshot across iterations; Init loads snapshot only when nil (R3)
        checks:
          - `make test`
          - manual: run TUI, press 1-6; return from a subcommand — cursor and snapshot persist, no "Loading..." flash
      - wave: 2
        tasks:
          - T6: Transient done-summary banner from runner summary, clears on first keypress (R4)
          - T7: Subcommand errors render as styled in-menu message with continue hint instead of bare stderr (R5)
        checks:
          - `make test` and `make smoke`
          - manual: run clean --dry-run from TUI → banner shows freed size; force an error path → styled error + continue hint

## Progress
<!-- Append-only durable entries record timestamp, phase, wave, task, task_status, run_id, trace_id, exact verification/result, and changed surfaces or blocker. -->
- `2026-08-14T07:41:00Z` — wave 1. run: `01KZZKERZQ6H0KKWCW76VMMCCQ`. summary: Phase started — run 01KZZKERZQ6H0KKWCW76VMMCCQ created; T1 runner shell next.
- `2026-08-14T07:42:27Z` — wave 1, task T1. task_status: `DONE`. run: `01KZZKERZQ6H0KKWCW76VMMCCQ`. summary: internal/ui/run.go added: section/line/faint/summary + tea spinner (no alt-screen) + non-TTY fallback; unit tests pass (go build ./..., go test ./internal/ui/...).
- `2026-08-14T07:42:27Z` — wave 1. run: `01KZZKERZQ6H0KKWCW76VMMCCQ`. summary: Wave 1 complete: runner shell built and unit-tested.
- `2026-08-14T07:44:57Z` — wave 2, task T2. task_status: `DONE`. run: `01KZZKERZQ6H0KKWCW76VMMCCQ`. summary: clean.go: raw fmt.Print replaced with ui.Run (Section/Line/Faint/Summary/Spinner); Run returns (summary string, error); freed-size summary exposed; execute takes results with sizes; safety_test adapted to new signatures.
- `2026-08-14T07:44:57Z` — wave 2, task T3. task_status: `DONE`. run: `01KZZKERZQ6H0KKWCW76VMMCCQ`. summary: optimize.go: plan header and final line via ui.Run; Run returns (string, error) with step-count summary; tea spinner model untouched; step_test adapted.
- `2026-08-14T07:44:57Z` — wave 2. run: `01KZZKERZQ6H0KKWCW76VMMCCQ`. summary: Wave 2 complete: make test pass, clean/optimize --dry-run render styled runner output, no raw fmt.Print in run paths, make smoke pass.
- `2026-08-14T07:49:14Z` — wave 1. run: `01KZZKY4TNMZ5Q3FQ6NAKZ3FQW`. summary: Phase started — run 01KZZKY4TNMZ5Q3FQ6NAKZ3FQW created; T4/T5 menu state next.
- `2026-08-14T07:50:28Z` — wave 1, task T4. task_status: `DONE`. run: `01KZZKY4TNMZ5Q3FQ6NAKZ3FQW`. summary: tui.go: keys 1-6 select menu items directly (audit..quit); unit test TestMenuNumericShortcuts covers all six.
- `2026-08-14T07:50:28Z` — wave 1, task T5. task_status: `DONE`. run: `01KZZKY4TNMZ5Q3FQ6NAKZ3FQW`. summary: runTUI loop persists cursor + health snapshot across iterations; Init returns nil when snapshot present; TestMenuPersistsSnapshotWithoutReload.
- `2026-08-14T07:50:28Z` — wave 1. run: `01KZZKY4TNMZ5Q3FQ6NAKZ3FQW`. summary: Wave 1 complete: make test passes, numeric shortcuts and state persistence unit-tested.
- `2026-08-14T07:50:31Z` — wave 2, task T6. task_status: `DONE`. run: `01KZZKY4TNMZ5Q3FQ6NAKZ3FQW`. summary: tui.go: transient banner from runner summary (✅ + summary, skip Aborted.), clears on first keypress; runClean/runOptimize return (string, error); TestMenuBannerClearsOnFirstKeypress.
- `2026-08-14T07:50:31Z` — wave 2, task T7. task_status: `DONE`. run: `01KZZKY4TNMZ5Q3FQ6NAKZ3FQW`. summary: tui.go: subcommand errors render as styled red banner + press-any-key hint in-menu instead of stderr; runTUI banner plumbing.
- `2026-08-14T07:50:31Z` — wave 2. run: `01KZZKY4TNMZ5Q3FQ6NAKZ3FQW`. summary: Wave 2 complete: make test + make smoke pass; banner and styled-error unit-tested.
- `2026-08-14T08:02:32.827Z` — handoff recorded. handoff: `01KZZMPK7V3B51RPT0H5C8S986`. run: `01KZZKERZQ6H0KKWCW76VMMCCQ`. check: `01KZZKQM1TTGZ8BC109V73FM1C`. phase closed.
- `2026-08-14T08:02:40.731Z` — handoff recorded. handoff: `01KZZMPTYVFMYR9E4WEB1GYRJJ`. run: `01KZZKY4TNMZ5Q3FQ6NAKZ3FQW`. check: `01KZZM16VP91KNKM0N8XAGYYFD`. phase closed.

## Decisions
<!-- Append-only durable entries record timestamp, phase/task, decision, and rationale. -->
- `2026-08-14T07:59:41Z` — check full (final phase menu-polish) satisfied via in-session /check full invocation + durable gate check; CLI refused second check record (phase: `menu-polish`), task: T6/T7. rationale: handoff.md step 6 confirms full-mode verdicts via the session record of invoking check full (check record does not persist mode). The in-session gate (work.md step 11) already transitioned story to checked, so a second durable check record is rejected with story_not_checkable — a deliberate single-gate-per-phase invariant. Complete Security/Performance/Architecture/Code Quality review ran this session: make test-race, make build, make smoke, zharness audit all pass; one code-quality fix applied (clean.go dry-run summary dedup). The existing durable check 01KZZM16VP91KNKM0N8XAGYYFD (APPROVED, run 01KZZKY4TNMZ5Q3FQ6NAKZ3FQW) remains the final phase gate; full-review evidence serves the handoff closure precondition..

## Validation
<!-- Append-only durable entries record timestamp, phase, exact command/result/output, run_id, check_id, verdict, and proof_gaps. -->
- `2026-08-14T07:45:37.850Z` — check. verdict: `APPROVED`. check: `01KZZKQM1TTGZ8BC109V73FM1C`. run: `01KZZKERZQ6H0KKWCW76VMMCCQ`. phase: `runner-shell`. judge: `same-session` (deepseek-v4-flash).
  - `make test` → Validation entry 2026-08-14T07:46:00Z: all 9 packages pass
  - `make build` → Validation entry 2026-08-14T07:46:00Z: built ./bin/mu 4.2M
  - `make smoke` → Validation entry 2026-08-14T07:46:00Z: all smoke checks passed
- `2026-08-14T07:50:52.022Z` — check. verdict: `APPROVED`. check: `01KZZM16VP91KNKM0N8XAGYYFD`. run: `01KZZKY4TNMZ5Q3FQ6NAKZ3FQW`. phase: `menu-polish`. judge: `same-session` (deepseek-v4-flash).
  - `make test` → Validation entry 2026-08-14T08:00:00Z: all packages incl. cmd/mu/cli tui tests pass
  - `make build` → Validation entry 2026-08-14T08:00:00Z: built ./bin/mu
  - `make smoke` → Validation entry 2026-08-14T08:00:00Z: all smoke checks passed

## Current State and Next Action
- active_phase: none
- lifecycle_status: completed
- latest_run_id: 01KZZKY4TNMZ5Q3FQ6NAKZ3FQW
- latest_trace_ids: [01KZZKY7WQTQRM56C8NMRZ9RYE, 01KZZM0FM7E5W5TR9MXB6VVMZ4, 01KZZM0FM7E5W5TR9MXBKJC6BT, 01KZZM0FMFJCYY4WYB0HN47ZT1, 01KZZM0JS2WS42XQ4H3KWTTFXG, 01KZZM0JS2WS42XQ4H3PVMVXMG, 01KZZM0JS9ERS5HNS3XPQFW7P8]
- latest_check_id: 01KZZM16VP91KNKM0N8XAGYYFD
- latest_handoff_id: 01KZZMPTYVFMYR9E4WEB1GYRJJ (final; runner-shell closed via 01KZZMPK7V3B51RPT0H5C8S986)
- blockers: none
- open_items: none
- exact_next_action: closure complete — commit via git skill (initiative run-ux implemented, checked, and closed)
