# Scripts

## Harness CLI

`mu` consumes a pinned upstream Harness binary. It does not contain Harness Rust source or upstream release workflows.

```bash
make harness-bootstrap
make harness-init
scripts/bin/harness-cli query matrix
scripts/bin/harness-cli audit
```

Pins:

- `harness-cli.version`: release tag.
- `harness-cli.sha256`: supported platform checksums.
- `harness-bootstrap.sh`: platform detection, download, checksum and version verification, atomic install.

Supported bootstrap platforms are Linux and macOS on x64 and arm64. `HARNESS_CLI_BASE_URL` may point to a controlled mirror containing the same pinned asset names; checksum verification remains mandatory.

The ignored durable database is `harness.db`. Version-controlled schema migrations are under `scripts/schema/`.
