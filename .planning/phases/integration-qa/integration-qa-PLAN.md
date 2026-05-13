# Plan: integration-qa

## Goal
Validate all 9 acceptance criteria, verify binary constraints, and produce distribution-ready docs.

## Inputs
- All prior phases complete and building
- `Makefile` with `build` target
- `scripts/install.sh`
- `configs/default-whitelist.toml`

---

## Wave 1 — Independent: acceptance criteria verification + binary checks

### Task 1: verify all 9 acceptance criteria
- steps:
  1. `mu --help` — assert all 5 commands appear in output
  2. `mu clean --dry-run` — assert per-category sizes shown, no files modified (check mtime of `~/.cache` before and after)
  3. `mu clean` — run with YES confirmation; assert disk usage decreases; check `~/.local/share/mu/operations.log` has entries
  4. `mu status` — open dashboard, observe live updates, press `q`, assert clean exit (no panic, no leftover alt-screen)
  5. `mu status | cat` — assert JSON object output, immediate exit
  6. `mu uninstall` — install `sl` test package first (`sudo apt install sl`); run `mu uninstall`; select `sl`; confirm; verify `which sl` fails after
  7. `mu uninstall --dry-run` — select `sl`; confirm; verify `sl` is still installed
  8. `mu optimize --dry-run` — assert step list shown, nothing executed
  9. `go build ./...` — clean build with no errors
- expected outputs:
  - Mental checklist (or a `qa-results.md` in `.kit/reports/`) of pass/fail per criterion
- verify:
  - All 9 pass; document any failures as issues for immediate fix before Phase 6 is done

### Task 2: verify binary constraints
- steps:
  1. Run `make clean && make build`
  2. `ls -lh ./bin/mu` — assert size < 25 MB
  3. Time `mu clean --dry-run` — assert scan completes in < 30 seconds (full clean with deletion in < 90s)
  4. `go test ./... -count=1` — assert all unit tests pass
- expected outputs:
  - Binary `./bin/mu` < 25 MB
  - All tests passing
- verify:
  - `ls -lh ./bin/mu | awk '{print $5}'` shows < 25M

---

## Wave 2 — Depends on Wave 1 (fixes applied): docs

### Task 3: write `README.md`
- steps:
  1. Sections: Project title + one-liner, Install (curl pipe + build from source), Quick Start (5 command examples with output), Commands reference table, Safety Philosophy (3 bullet points), Compatibility, Contributing
  2. Include the `mu clean --dry-run` example output from PRD Section 7
  3. Note actual minimum Go version (1.24.2 due to Bubbles v1.0.0) — not 1.21 as originally planned
- expected outputs:
  - `README.md` in project root
- verify:
  - `cat README.md | wc -l` > 80 lines; renders correctly on GitHub (check headings, code blocks)

### Task 4: write `SECURITY.md`
- steps:
  1. Sections: Safe Usage (dry-run, confirmation), Protected Paths (list from whitelist), Trash vs Delete (explain gio trash), Responsible Disclosure (email or GitHub Issues link), Known Limitations
- expected outputs:
  - `SECURITY.md` in project root
- verify:
  - File exists, < 150 lines, covers the 4 sections

### Task 5: update SPEC constraint for Go version
- steps:
  1. Edit `.planning/SPEC.md` Constraints section: change "Go version: 1.21+" to "Go version: 1.24.2+ (required by github.com/charmbracelet/bubbles v1.0.0; GOTOOLCHAIN=auto will auto-download on older systems)"
- expected outputs:
  - Updated `.planning/SPEC.md`
- verify:
  - Grep for "1.21" in SPEC.md returns no results

---

## Risks / Watch-fors
- Binary size: if > 25 MB, ensure `-ldflags "-s -w"` is in the build command; also try `upx --best ./bin/mu` as a last resort (adds a step to Makefile)
- `mu status` alt-screen: if the dashboard doesn't restore terminal properly after `q`, add `tea.WithAltScreen()` and verify `tea.Quit` path restores the screen
- Go 1.24.2 minimum: update README clearly so users on Ubuntu 22.04 (ships Go 1.18) know they need a newer toolchain (snap install go or golang.org download)
