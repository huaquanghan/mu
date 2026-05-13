# Context: clean-full

## Goal
Complete the `mu clean` command: implement all scan categories, wire the Bubbles progress bar, and enforce `--include` opt-in for browser caches. This is the primary user-value command and must match the PRD dry-run output format exactly.

## Spec Hooks
- Req 7: scan categories — user cache, APT cache, snap disabled revisions, journal logs, old kernels/headers, thumbnails, browser caches (opt-in), Docker build cache (if Docker present)
- Req 11-12: Bubbletea TUI, progress bars for long-running operations
- Req 4: `gio trash` for user-owned files; `sudo rm` only for system-owned paths
- Open question resolved: browser cache is opt-in via `--include=browser-cache`

## Locked Decisions
- Scan categories are defined as a slice of `CleanTarget` structs with: `id`, `label`, `paths []string`, `requiresSudo bool`, `optIn bool`
- Browser cache paths (Chrome, Firefox, VSCode) are `optIn: true` — excluded from default scan unless `--include=browser-cache` passed
- Docker cleanup: check `/var/run/docker.sock` existence, not `which docker`; run `docker system prune -f --volumes` only with explicit user YES confirmation (separate from the main YES prompt)
- Old kernel detection: run `dpkg -l 'linux-image-*' 'linux-headers-*'` and exclude the package matching `uname -r`; also exclude the latest non-running kernel (keep one backup)
- Snap disabled revisions: `snap list --all | awk '/disabled/{print $1, $3}'` → `snap remove --revision`; gracefully skip entire snap section if `snap` not found in `$PATH`
- Progress bar: use `github.com/charmbracelet/bubbles/progress` component; one bar per category during deletion, updated via `tea.Cmd`
- Size reporting: scan sizes before deletion, report before/after delta at end

## Assumptions
- `/var/cache/apt/archives` cleanup uses `sudo apt clean` (not manual rm) to let apt manage its own cache integrity
- Journal vacuum uses `journalctl --vacuum-time=30d` (not size-based) for clean
- `~/.cache/thumbnails` is a subset of `~/.cache` — exclude from the parent user-cache scan to avoid double-counting; scan it separately as its own category
- Browser cache paths: `~/.config/google-chrome/Default/Cache`, `~/.mozilla/firefox/*/cache2`, `~/.config/Code/CachedData`

## Canonical Refs
- `.planning/SPEC.md` — Req 7, PRD Appendix Section 12 (cleaning targets)
- `.planning/ROADMAP.md` — Phase 2
- `internal/clean/clean.go` — existing partial implementation (scan + basic dry-run)
- `internal/utils/trash.go` — (Phase 1 output, required here)
- `mu-prd.md` — Section 7 (exact dry-run output format)

## Rejected Options
- **Browser cache on by default**: rejected — users may have active browser sessions; opt-in is safer
- **Single `rm -rf ~/.cache`**: rejected — too broad, would destroy active app data (e.g., npm, pip caches)
- **Docker: only prune images**: rejected — PRD says `docker system prune -a --volumes`; respect full intent

## Deferred Ideas
- Flatpak unused runtime cleanup (v0.2)
- User-configurable vacuum days (currently hardcoded to 30) via config.toml (v0.2)
- `mu clean --only=apt,journal` category filter (v0.2)
