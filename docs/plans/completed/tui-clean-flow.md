---
id: 01KZZV1PNH1D6BFHF2RQBY02NK
type: plan
intake_id: 01KZZV1RX7MXAGGJGJRDCPDGX3
lane: normal
status: completed
created: 2026-08-14
updated: 2026-08-14
---

# Plan: TUI clean-flow redesign (post-hoc lock)

## Outcome
- result: The interactive `mu clean` flow renders as one alt-screen Bubble Tea program with five fully-redrawn screens — scanning (single spinner line with per-target progress), summary (aligned table, right-aligned sizes, total), confirm (one self-contained YES/NO view, default NO), running (per-item spinner → colored ✓/✗ on real completion events, "N of M" counter, past-tense verbs), done (freed space, elapsed time, failures with reasons, exit/rerun hint). Non-TTY invocation falls back to plain line-by-line output with no prompts; `--yes`/`--dry-run` skip the confirm screen; ctrl+c cancels cooperatively with full terminal restore; narrow terminals truncate instead of wrapping. Scan/execute business logic, flags, exit codes, and safety invariants are untouched.
- success_signals:
  - `mu clean` on a terminal shows one continuous flow on the alt screen — every screen fully replaces the previous one, no leftover frames from a previous view.
  - Each item's spinner ticks to ✓/✗ only when the real `Execute` completes (proven by unit tests driving `itemDoneMsg` through the model).
  - Ctrl-C mid-run finishes the current item, marks the rest skipped, shows a "Cancelled" done screen, and restores the terminal (alt screen off, cursor visible).
  - `mu clean | cat` prints plain non-animated lines; `mu clean < /dev/null` explains `--yes`/`--dry-run` instead of silently aborting.
  - `make test`, `make test-race`, `make build`, `make smoke` pass; `--dry-run`/`--yes`/`--include` behavior and exit codes unchanged.

## Authority and Requirements
- authority:
  - CLAUDE.md "TUI conventions" — colors `#0097A7` / `#374151` / `#9CA3AF`, hint-line style, `tea.WithAltScreen()`
  - Existing `internal/ui` spinner/confirm patterns (bubbles spinner, `RenderButtons`)
  - clig.dev CLI UX principles + Charm Bubble Tea/Lip Gloss conventions (user request, session 2026-08-14)
  - CLAUDE.md safety invariants — `dryRun` branches, `SafeDelete`, confirm gating, opt-in IDs
- requirements:
  - R1 [accepted]: One state machine (5 screens) in `internal/clean/flow.go`, rendered by a single alt-screen Bubble Tea program; never append a new view on top of an old one | source: user request
  - R2 [accepted]: Per-item spinner progress is driven by real completion events (sequential `tea.Cmd` chain), never a fake timer | source: user request
  - R3 [accepted]: Confirm is one self-contained YES/NO view defaulting to NO; `--yes` and `--dry-run` skip it | source: user decision (session)
  - R4 [accepted]: Done screen shows freed space, elapsed time, failures with brief reasons, and an exit/rerun hint | source: user request
  - R5 [accepted]: Ctrl-C cancels cooperatively — finish the in-flight item, skip the rest, restore terminal state | source: user decision (session)
  - R6 [accepted]: Non-TTY/piped invocation falls back to plain non-animated line output and never prompts | source: user request
  - R7 [accepted]: Styling lives in one shared lipgloss definition (`internal/ui/styles.go`); no ad-hoc ANSI codes in views | source: user request
  - R8 [accepted]: Layout adapts to narrow terminals — data-driven column widths, truncation instead of hard-coded widths | source: user request
  - R9 [accepted]: Scan/execute business logic, flags (`--dry-run`, `--yes`, `--include`), exit codes, and safety invariants unchanged | source: CLAUDE.md

## Non-goals
- NG1: Threading a cancelable context through `CleanTarget.Execute` (true abort of in-flight `sudo` commands) — deferred; `Execute` keeps `func(dryRun bool) error`.
- NG2: Applying the flow to optimize/uninstall/status — this initiative is clean-only.
- NG3: Per-category checkboxes in the confirm view — YES/NO chosen by user decision.
- NG4: An overall progress bar — execution is sequential, so an "N of M" counter suffices.
- NG5: Changing `ui.Confirm` / `ui.Run` behavior for optimize (only mechanical style dedup is allowed).

## Approach and Risks
- approach: One alt-screen Bubble Tea program owns five states. Scans and executes run as sequential `tea.Cmd` chains — the next item starts only when the previous one returns a real completion message, so progress ticks are event-driven by construction. Column widths derive from data (`HumanSize` strings) and `tea.WindowSizeMsg`, truncated via `ansi.Truncate` for narrow terminals. `interactive()` (isatty on stdin+stdout) gates the TTY flow vs. `runPlain`, which keeps the previous line-by-line renderer. All palette/buttons/spinner/marks centralized in `internal/ui/styles.go`. Preferred because scan/execute logic stays untouched, completion is event-driven, and alt-screen exit restores the terminal automatically.
- constraints:
  - `Execute` is sequential and returns no freed bytes — freed is the scan-size estimate (same as before).
  - `CleanTarget.Execute` has no context — cancellation is cooperative at item boundaries (R5, NG1).
  - Dry-run never prompted historically; it now walks the flow but skips only the confirm screen.
- rejected_alternatives:
  - Option A (single goroutine + `p.Send` channel): more complex quit/cancel lifecycle and leak-prone — dropped for the cmd-chain pattern (R2).
  - Option B (checkbox confirm): richer but adds a slower default path; YES/NO chosen by user (NG3).
  - Option C (thread ctx through `Execute`): true abort but touches business logic — deferred (NG1).
- risks:
  - Ctrl-C during scan cannot kill an in-flight `WalkDir` (no context) — mitigation: exit is immediate; worst case one bounded read-only goroutine finishes its walk then blocks on a dead channel; it never touches user data.
  - `make smoke` runs `clean --dry-run` interactively in a human terminal and would hang at the flow's Enter prompts — mitigation: smoke pipes the dry-run so it takes the non-TTY path.
  - Size strings vary in width (`0 B` vs `2.9 GB`) — mitigation: size column width computed from data, right-aligned.
  - Freed totals are estimates (Execute returns no bytes) — mitigation: same semantics as pre-refactor; stated in the done screen as freed.
- stop_conditions:
  - `make test` fails outside the new flow tests → stop and reassess before touching `clean.go`.
  - A pty run of `clean --dry-run` shows leftover frames or a broken terminal restore → fix the renderer before proceeding.
- recovery: `git revert` the failing wave's edits, re-run `make test` + the pty scenario; phases are ordered so `shared-styles` can land independently of `clean-flow-tui`.

## Phases and Verification
<!-- Phase and task definitions are immutable after to-plan. Do not add task status fields. Append-only Progress is the sole task execution-status source. Only each phase lifecycle status changes to mirror DB transitions: to-plan=planned; work after run create=in-progress; clean durable check=checked; closing handoff=done. Each planned phase records phase_slug, story_id, status, goal, depends_on, waves, tasks, and checks. -->
- planning_status: planned
- phases:
  - phase_slug: shared-styles
    story_id: 01KZZV1VH3CZGCHWVKYCH5SZBQ
    status: done
    goal: Shared lipgloss style module — internal/ui/styles.go palette/buttons/spinner/marks with mechanical dedup across ui/confirm.go, ui/run.go, cmd/mu/cli/tui.go and unchanged rendering
    depends_on: none
    allowed_surfaces: [internal/ui/styles.go (new), internal/ui/confirm.go, internal/ui/run.go, cmd/mu/cli/tui.go]
    avoided_surfaces: [internal/clean, internal/optimize, scan/execute logic, rendered output of any screen]
    waves:
      - wave: 1
        tasks:
          - T1: Add `internal/ui/styles.go` — palette consts (ColorPrimary/Danger/Success/Inactive/Muted), StyleBoldPrimary/StyleFaint/StyleButtonOn/StyleButtonOff, MarkSuccess/MarkError, NewSpinner
          - T2: Mechanical dedup — `ui/confirm.go` RenderButtons and `ui/run.go` spinner use the shared styles; `cmd/mu/cli/tui.go` swaps hex literals for palette consts
        checks:
          - `go build ./...`
          - `go test ./internal/ui/... ./cmd/mu/... -count=1`
          - `grep -rn '#0097A7\|#EF4444' cmd/mu/cli/tui.go internal/ui/` — no raw hex literals left
  - phase_slug: clean-flow-tui
    story_id: 01KZZV1W2J20QVFK29KV0KK4BH
    status: done
    goal: Single alt-screen 5-state Bubble Tea clean flow (scanning, summary, confirm, running, done) with per-target real completion events, cooperative ctrl+c, non-TTY fallback, narrow-width-safe layout
    depends_on: shared-styles
    allowed_surfaces: [internal/clean/flow.go (new), internal/clean/flow_test.go (new), internal/clean/clean.go, Makefile]
    avoided_surfaces: [internal/clean/targets.go, internal/clean/scan_*.go, internal/utils, internal/ui/confirm.go behavior, optimize/uninstall/status flows]
    waves:
      - wave: 1
        tasks:
          - T3: `internal/clean/flow.go` — flowModel with 5 states, sequential scan/run `tea.Cmd` chains (scanTargetMsg/itemDoneMsg), YES/NO confirm (RenderButtons), N-of-M counter, done screen with time/failures, ctrl+c cooperative cancel, WindowSizeMsg-aware truncation
          - T4: `internal/clean/clean.go` — `Run` splits on `interactive()`: TTY → `runFlow` (alt screen), else `runPlain` (previous line renderer, no prompts; non-TTY without `--yes` prints a hint + "Aborted.")
        checks:
          - `go build ./...` and `go vet ./...`
          - `go test ./internal/clean/... -run TestFlow -count=1 -v` — 8 model tests (happy/decline/dry-run/--yes/ctrl-c/failed/scan-error/narrow)
          - `go test -race ./... -count=1`
      - wave: 2
        tasks:
          - T5: `internal/clean/flow_test.go` — model-level tests driving Update() directly (R1-R5, R8)
          - T6: Makefile smoke pipes `clean --dry-run` (non-TTY path); pty scenario verification: happy path, decline, ctrl+c, narrow 30-col
        checks:
          - `make test` and `make build` and `make smoke`
          - pty runs via `script -qec`: happy `clean --dry-run` (Scan complete + ✓ Cleaned rows + Done frame), decline `clean` with NO (0 items cleaned), ctrl+c (`[?1049l` + `[?25h` restore sequences, exit 0), `stty cols 30` dry-run (exit 0, no wrap)
          - `./bin/mu clean | cat` and `./bin/mu clean < /dev/null` — plain output, hint for `--yes`

## Progress
<!-- Append-only durable entries record timestamp, phase, wave, task, task_status, run_id, trace_id, exact verification/result, and changed surfaces or blocker. -->
- `2026-08-14T10:03:56Z` — wave 1. run: `01KZZVMP63H11X5SV3GTSQE0KM`. summary: Phase started — run 01KZZVMP63H11X5SV3GTSQE0KM created (post-hoc lock: implementation already in working tree); T1/T2 verification next.
- `2026-08-14T10:04:08Z` — wave 1, task T1. task_status: `DONE`. run: `01KZZVMP63H11X5SV3GTSQE0KM`. summary: internal/ui/styles.go added: palette consts, StyleBoldPrimary/StyleFaint/StyleButtonOn/StyleButtonOff, MarkSuccess/MarkError, NewSpinner.
- `2026-08-14T10:04:08Z` — wave 1, task T2. task_status: `DONE`. run: `01KZZVMP63H11X5SV3GTSQE0KM`. summary: confirm.go RenderButtons + run.go spinner use shared styles; tui.go hex literals swapped for palette consts.
- `2026-08-14T10:04:08Z` — wave 1. run: `01KZZVMP63H11X5SV3GTSQE0KM`. summary: Wave 1 complete: go build ./... OK; go test ./internal/ui/... ./cmd/mu/... pass; hex literals only in styles.go palette (no raw hex in views).
- `2026-08-14T10:04:16Z` — wave 1. run: `01KZZVN8GFFHVBRWKKSJXBTEQ3`. summary: Phase started — run 01KZZVN8GFFHVBRWKKSJXBTEQ3 created (post-hoc lock: implementation already in working tree); T3/T4 verification next.
- `2026-08-14T10:04:35Z` — wave 1, task T3. task_status: `DONE`. run: `01KZZVN8GFFHVBRWKKSJXBTEQ3`. summary: flow.go: flowModel 5 states, sequential scan/run tea.Cmd chains (scanTargetMsg/itemDoneMsg), YES/NO confirm, N-of-M counter, done screen time/failures, cooperative ctrl+c, WindowSizeMsg truncation.
- `2026-08-14T10:04:35Z` — wave 1, task T4. task_status: `DONE`. run: `01KZZVN8GFFHVBRWKKSJXBTEQ3`. summary: clean.go Run split: interactive() -> runFlow (alt screen), else runPlain (no prompts, non-TTY hint + Aborted.); flags --yes/--dry-run honored.
- `2026-08-14T10:04:36Z` — wave 1. run: `01KZZVN8GFFHVBRWKKSJXBTEQ3`. summary: Wave 1 complete: go build + go vet OK; 8/8 TestFlow model tests pass; go test -race ./... 9/9 packages ok.
- `2026-08-14T10:04:37Z` — wave 2, task T5. task_status: `DONE`. run: `01KZZVN8GFFHVBRWKKSJXBTEQ3`. summary: flow_test.go: 8 model tests driving Update() directly — happy/decline/dry-run/--yes/ctrl-c/failed/scan-error/narrow-width.
- `2026-08-14T10:04:37Z` — wave 2, task T6. task_status: `DONE`. run: `01KZZVN8GFFHVBRWKKSJXBTEQ3`. summary: Makefile smoke pipes clean --dry-run (non-TTY path); pty scenarios verified: happy dry-run (Scan complete + Cleaned rows + Done frame), decline (0 items cleaned), ctrl+c ([?1049l [?25h restore, exit 0), stty cols 30 dry-run (exit 0); make test + make smoke pass.
- `2026-08-14T10:04:38Z` — wave 2. run: `01KZZVN8GFFHVBRWKKSJXBTEQ3`. summary: Wave 2 complete: make test + make build + make smoke all pass; pty evidence captured (happy/decline/ctrl-c/narrow/non-tty).
- `2026-08-14T10:08:16.928Z` — handoff recorded. handoff: `01KZZVWTH0ARC7T3KXK62C8NWD`. run: `01KZZVMP63H11X5SV3GTSQE0KM`. check: `01KZZVRV0VX1ZGSX6P8VNDJJW7`. phase closed.
- `2026-08-14T10:08:19.413Z` — handoff recorded. handoff: `01KZZVWWYNCD1SCS2YGHFJ9YK9`. run: `01KZZVN8GFFHVBRWKKSJXBTEQ3`. check: `01KZZVRZFKKX3XDG1PS66FYTZR`. phase closed.

## Validation
<!-- Append-only durable check entries: verdict, check id, run id, judge, proof commands. -->
- `2026-08-14T10:06:06.363Z` — check. verdict: `APPROVED`. check: `01KZZVRV0VX1ZGSX6P8VNDJJW7`. run: `01KZZVMP63H11X5SV3GTSQE0KM`. phase: `shared-styles`. judge: `same-session` (deepseek-v4-flash).
  - `go build ./...` → Validation entry 2026-08-14T10:06:06Z: build passes
  - `go test ./internal/ui/... ./cmd/mu/... -count=1` → Validation entry 2026-08-14T10:06:06Z: ui + cli tests pass
- `2026-08-14T10:06:10.931Z` — check. verdict: `APPROVED`. check: `01KZZVRZFKKX3XDG1PS66FYTZR`. run: `01KZZVN8GFFHVBRWKKSJXBTEQ3`. phase: `clean-flow-tui`. judge: `same-session` (deepseek-v4-flash).
  - `go build ./...` → Validation entry 2026-08-14T10:06:06Z: build passes
  - `go vet ./...` → Validation entry 2026-08-14T10:06:06Z: vet passes
  - `go test ./... -count=1` → Validation entry 2026-08-14T10:06:06Z: all 9 packages pass
  - `make smoke` → Validation entry 2026-08-14T10:06:06Z: all smoke checks passed

## Current State and Next Action
- active_phase: none
- lifecycle_status: done
- latest_run_id: 01KZZVN8GFFHVBRWKKSJXBTEQ3
- latest_trace_ids: [01KZZVNFW5MYCHVHMVAHGWZFY6, 01KZZVP2F7192VCJPJJNCXBY4Y, 01KZZVP2F7192VCJPJJQKFWSAP, 01KZZVP32612HJ35EWHM7E14HJ, 01KZZVP4AQWNJ60JV8J55J2H6G, 01KZZVP4AQWNJ60JV8J78E8S86, 01KZZVP4YEBED46GZVHNSFH34D]
- latest_check_id: 01KZZVRZFKKX3XDG1PS66FYTZR
- latest_handoff_id: 01KZZVWWYNCD1SCS2YGHFJ9YK9
- blockers: none
- open_items: none — initiative closed: T1-T6 DONE, both phases done (APPROVED), committed on main
- exact_next_action: none — initiative complete; plan moved to docs/plans/completed/tui-clean-flow.md
