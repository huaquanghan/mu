# mu — Mole Ubuntu

A 4 MB binary that cleans, uninstalls, optimizes, and monitors your Ubuntu system. Everything destructive shows you what it's about to do before it does it. Files go to trash, not `/dev/null`.

---

## Install

**One-liner:**
```bash
curl -fsSL https://raw.githubusercontent.com/huaquanghan/mu/main/scripts/install.sh | bash
```

**From source** (requires Go 1.24.2+):
```bash
git clone https://github.com/huaquanghan/mu
cd mu
make build          # → ./bin/mu
make install-local  # → ~/.local/bin/mu (no sudo)
```

> Ubuntu 22.04 ships Go 1.18. Install a newer toolchain: `sudo snap install go --classic`

---

## Usage

```
mu                              # interactive TUI menu
mu clean --dry-run              # preview what would be cleaned
mu clean                        # clean with YES/NO confirmation
mu clean --include=browser-cache,docker
mu uninstall                    # search and remove packages + remnants
mu optimize --dry-run           # preview maintenance steps
mu optimize --skip=apt          # skip specific steps
mu status                       # live CPU/RAM/disk/network dashboard
mu status | jq '.health'        # JSON when piped
```

### `mu` (TUI menu)

Opens an interactive menu with a live system snapshot (CPU, RAM, disk). Select any command with arrow keys or `j/k`. Pressing `q` from any sub-screen returns to the menu.

### `mu clean`

Scans and frees disk space across:

| Category | Default | Flag to enable |
|----------|:-------:|---------------|
| User cache (`~/.cache`) | ✓ | |
| Thumbnail cache | ✓ | |
| APT package cache | ✓ | |
| Journal logs | ✓ | |
| Snap disabled revisions | ✓ | |
| Old kernel packages | ✓ | |
| Browser caches (Chrome/Firefox/VSCode) | | `--include=browser-cache` |
| Docker build cache | | `--include=docker` |

### `mu uninstall`

Type to search installed packages (APT + Snap). Space to select multiple. Shows installed size plus leftover config/cache dirs before you confirm. Confirmation requires selecting YES from a button prompt — the default is NO.

### `mu optimize`

Runs: `apt update && apt autoremove`, `journalctl --vacuum-size=500M`, and icon/font/MIME cache refresh. Steps can be skipped per-run (`--skip=apt`) or permanently via config.

### `mu status`

Live-updating dashboard reading directly from `/proc`. Shows CPU%, RAM, disk per mount, network I/O rates, and a health score (0–100 weighted: CPU 30%, RAM 30%, disk 30%, swap 10%). Outputs JSON when stdout is not a TTY.

---

## Safety

- **Trash, not delete.** User-owned files go through `gio trash` (falls back to XDG trash spec). `rm` is never used on user data.
- **Protected paths.** `/`, `/boot`, `/etc`, `/usr`, `/lib`, `/bin`, `/sbin`, `/proc`, `/sys`, `/dev`, `/run` are blocked in code. The running kernel is never touched.
- **Dry-run everywhere.** Every destructive command has `--dry-run`. No surprises.
- **Audit log.** All operations log to `~/.local/share/mu/operations.log` (10 MB, 1 rotation). Disable: `MU_NO_OPLOG=1`.

---

## Configuration

`~/.config/mu/config.toml` — overrides defaults from `configs/default-whitelist.toml`:

```toml
[protected_paths]
system = ["/data", "/mnt/backup"]

[cache_skip]
dirs = ["my-app/important-cache"]

[optimize_skip]
steps = ["apt"]   # always skip apt in optimize
```

---

## Requirements

- Ubuntu 22.04 / 24.04 LTS (primary). Debian 12+, Pop!\_OS, Mint: compatible, not officially tested.
- Runtime: `gio` (from `glib2`), `dpkg-query`, `journalctl`. `snap` and `docker` are optional.
- Build: Go 1.24.2+.

---

## Contributing

`make test` must pass. `make build` must stay under 25 MB. All destructive paths need a `--dry-run` branch. See `SECURITY.md` for reporting vulnerabilities.
