# Context: uninstall-tui

## Goal
Build the interactive `mu uninstall` TUI: discover installed packages from APT and Snap, show a Bubbles multi-select list with size, confirm remnant dirs, then execute `apt purge` + safe remnant cleanup.

## Spec Hooks
- Req 8: interactive multi-select list; show dirs to be removed; execute `apt purge` + remnant cleanup
- Req 1: `--dry-run` shows plan without executing
- Req 3-4: log operations; use `SafeDelete` for remnant dirs
- Req 13: Bubbles list component for multi-select

## Locked Decisions
- **Package size**: `dpkg-query --show --showformat='${Installed-Size}\t${Package}\n'` gives size in KB for APT packages. `dpkg -l` alone does not include size.
- **Snap size**: `snap info <name>` is slow for all packages; instead use `df` output on snap mount points at `/snap/<name>/current` or estimate from `snap list` revision info. For MVP, show snap packages with size "N/A" if unavailable within 500ms.
- **Remnant discovery**: check existence of `~/.config/<name>`, `~/.local/share/<name>`, `~/.cache/<name>`, `/var/lib/<name>`. Show only paths that exist. Do not recurse into paths to avoid slow scanning — just stat + top-level size.
- **Bubbles list**: use `github.com/charmbracelet/bubbles/list` with a custom `ItemDelegate` that renders checkbox state. Multi-select via Space key; Enter confirms selection.
- **Confirmation flow**: after selection → show "Will remove:" list → second confirmation prompt → execute
- **Execution**: `apt purge` requires sudo; use `exec.Command("sudo", "apt", "purge", "-y", pkgs...)`. Snap removal: `exec.Command("sudo", "snap", "remove", name)`. Remnant cleanup: `SafeDelete` per path.
- **Snap graceful skip**: if `exec.LookPath("snap")` returns error, omit snap section entirely with a status line "snap not found, skipping"

## Assumptions
- Package names in APT are lowercase and stable; browser cache folders often differ (e.g., `google-chrome` vs `chromium`) — use exact known mappings for MVP
- Remnant dir name == package name is correct for most packages; known exceptions (e.g., `vscode` → `Code`) are handled in a static mapping table
- `apt purge` on a package that's already removed is a no-op (safe to call)

## Canonical Refs
- `.planning/SPEC.md` — Req 8, 13
- `.planning/ROADMAP.md` — Phase 4
- `internal/uninstall/uninstall.go` — existing stub
- `internal/utils/trash.go` — (Phase 1 output)
- `github.com/charmbracelet/bubbles/list` — Bubbles list component docs

## Rejected Options
- **`aptitude` or `apt-get`**: use `apt` (the modern high-level CLI); `aptitude` not always installed
- **Full systemd unit scanning in MVP**: deferred — complex, rarely needed; basic dotfile cleanup covers 90% of cases
- **Flatpak removal**: out of scope for MVP per spec

## Deferred Ideas
- Systemd user unit detection and cleanup (v0.2)
- Desktop entry removal from `/usr/share/applications` (v0.2)
- Flatpak uninstall (v0.2)
- "Last used" timestamp per package via `atime` or `apt-log` (v0.2)
