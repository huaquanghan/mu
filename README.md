# mu — Mole Ubuntu

A single-binary CLI for Ubuntu that safely cleans, optimizes, and monitors your system. Think CleanMyMac, but for Ubuntu power users.

Every destructive action shows what it would do before it does it.

---

## Install

**One-liner (requires Go 1.24.2+):**

```bash
curl -fsSL https://raw.githubusercontent.com/huaquanghan/mu/main/scripts/install.sh | bash
```

**Build from source:**

```bash
git clone https://github.com/huaquanghan/mu
cd mu
make build          # produces ./bin/mu
sudo make install   # installs to /usr/local/bin/mu
```

> **Note:** Go 1.24.2 or later is required (forced by `github.com/charmbracelet/bubbles v1.0.0`). On Ubuntu 22.04 which ships Go 1.18, install a newer toolchain via `sudo snap install go --classic` or from [go.dev/dl](https://go.dev/dl/).

---

## Quick Start

```
# Open interactive TUI menu (shows live CPU/RAM/disk snapshot)
mu

# Preview what would be cleaned — nothing is deleted
mu clean --dry-run

# Clean with confirmation prompt
mu clean

# Include browser and Docker caches (opt-in)
mu clean --include=browser-cache,docker

# Live system dashboard (CPU, RAM, disk, network, health score)
mu status

# JSON output for scripting
mu status | jq '.health'

# Preview uninstall with remnant detection
mu uninstall --dry-run

# Preview maintenance tasks
mu optimize --dry-run
```

**Example `mu clean --dry-run` output:**

```
🔍 Scanning system...

  User Cache (~/.cache)                    2.1 GB
  Thumbnail Cache                          48.2 MB
  APT Package Cache                        312.8 MB
  Journal Logs                             180.4 MB
  Snap Disabled Revisions                  620.0 MB
  Old Kernel Packages                      93.2 MB
  Docker Build Cache                       0 B
  --------------------------------------------------
  Potential space to free: 3.4 GB

⚠️  This is a DRY RUN. No files will be deleted.
Run without --dry-run to proceed.
```

---

## Commands

| Command | Description |
|---------|-------------|
| `mu` | Interactive TUI menu with system health snapshot |
| `mu clean [--dry-run] [--include=...]` | Scan and clean caches, logs, old kernels, snaps |
| `mu uninstall [--dry-run]` | Remove apps and all config/cache remnants via TUI |
| `mu optimize [--dry-run] [--skip=...]` | apt autoremove, journal vacuum, cache refresh |
| `mu status` | Live Bubbletea dashboard; JSON when piped |

### `mu clean` categories

| Category | Default | Opt-in |
|----------|---------|--------|
| User cache (`~/.cache`) | ✓ | |
| Thumbnails | ✓ | |
| APT package cache | ✓ | |
| Journal logs | ✓ | |
| Snap disabled revisions | ✓ | |
| Old kernel packages | ✓ | |
| Browser caches (Chrome/Firefox/VSCode) | | `--include=browser-cache` |
| Docker build cache | | `--include=docker` |

### `mu optimize` steps

| Step ID | Action | Skip with |
|---------|--------|-----------|
| `apt` | `apt update && apt autoremove --purge` | `--skip=apt` |
| `journal` | `journalctl --vacuum-size=500M` | `--skip=journal` |
| `caches` | `update-mime-database`, `fc-cache -f` | `--skip=caches` |

Persistent skip list: add `steps = ["apt"]` under `[optimize_skip]` in `~/.config/mu/config.toml`.

---

## Safety Philosophy

- **Dry-run first.** Every destructive command supports `--dry-run`. No surprises.
- **Trash, not delete.** User-owned files go to `~/.local/share/Trash` via `gio trash`, never `rm -rf`.
- **Protected paths.** `/`, `/boot`, `/etc`, `/usr`, `/lib`, `/bin`, `/sbin`, `/proc`, `/sys`, `/dev`, `/run` are blocked at the code level. The running kernel is never touched.
- **Full audit log.** Every operation is logged to `~/.local/share/mu/operations.log` (10 MB, 1 rotation). Disable with `MU_NO_OPLOG=1`.

---

## Configuration

User config file: `~/.config/mu/config.toml`

```toml
[protected_paths]
system = ["/data", "/mnt/backup"]   # additional protected paths

[cache_skip]
dirs = ["my-app/important-cache"]   # globs relative to ~/.cache to skip

[optimize_skip]
steps = ["apt"]                     # always skip apt in optimize
```

---

## Compatibility

- Ubuntu 22.04 LTS, 24.04 LTS (primary targets)
- Debian 12+ (compatible, not officially tested)
- Go 1.24.2+ required to build from source
- Requires: `gio` (from `glib2`), `dpkg-query`, `journalctl`; `snap` and `docker` optional

---

## Contributing

1. Fork, create a branch, open a PR
2. `make test` must pass; `make build` must produce a binary under 25 MB
3. All destructive code paths must have a `--dry-run` test
4. Report security issues at the GitHub Issues page (see `SECURITY.md`)
