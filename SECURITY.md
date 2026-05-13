# Security

## Safe Usage

### Dry-run before every destructive action

Every command that modifies the system supports `--dry-run`:

```bash
mu clean --dry-run      # shows sizes, touches nothing
mu optimize --dry-run   # shows plan, executes nothing
mu uninstall --dry-run  # shows what would be removed
```

`mu status` is read-only and never modifies the system.

### Confirmation prompt

Commands that delete files require explicit confirmation:

```
Type 'YES' to continue:
```

Any input other than `YES` (case-sensitive) aborts immediately.

---

## Protected Paths

The following paths are blocked at the code level and cannot be deleted or modified by `mu` under any circumstances:

```
/          /boot      /etc       /usr
/lib       /lib64     /bin       /sbin
/proc      /sys       /dev       /run
```

Additionally:
- The **currently running kernel** (detected via `uname -r`) is always excluded from kernel cleanup.
- `/var/lib/<package>` paths found during remnant detection are **displayed only** — `mu uninstall` does not attempt to delete them (root-only paths; user is shown the path for manual review).

You can add your own protected paths in `~/.config/mu/config.toml`:

```toml
[protected_paths]
system = ["/data", "/mnt/backup"]
```

---

## Trash, Not Delete

For user-owned files (under `~/.cache`, `~/.config`, `~/.local`), `mu` uses `gio trash` when available, or moves files to `~/.local/share/Trash/files/` following the FreeDesktop Trash specification. Files are **never permanently deleted** with `rm -rf` from user home directories.

For system caches (`/var/cache/apt/archives`), `mu` calls `apt clean` or `journalctl --vacuum-*` — the system package manager's own safe cleanup commands. `mu` never calls `rm -rf /var/...` directly.

---

## Audit Log

Every operation `mu` performs is recorded in:

```
~/.local/share/mu/operations.log
```

Log format: `2006-01-02T15:04:05Z07:00 [op] path`

The log rotates at 10 MB (one `.log.1` backup kept). Set `MU_NO_OPLOG=1` to disable logging entirely.

---

## Known Limitations

- `mu uninstall` uses `sudo apt purge` and `sudo snap remove` — these require sudo privileges and will prompt for a password.
- `mu optimize` apt steps use `sudo -n` (non-interactive); if sudo credentials are not cached, the apt steps are skipped silently (non-fatal).
- `mu clean` with browser cache (`--include=browser-cache`) may trash files from a running browser session. Close the browser first.
- The running kernel exclusion relies on `uname -r` output matching `dpkg-query` package names. Unusual kernel package naming (e.g., from third-party repos) may not be detected correctly.

---

## Responsible Disclosure

Found a security issue? Please open an issue at:

**https://github.com/huaquanghan/mu/issues**

For sensitive issues (credential leaks, privilege escalation), use the GitHub private security advisory feature or email the maintainer directly before public disclosure.
