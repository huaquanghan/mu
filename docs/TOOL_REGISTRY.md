# Tool Registry

Harness CLI is the only required Harness tool. It is downloaded by `make harness-bootstrap`, not built from source in this repo.

Optional tools are durable registry entries. Before using one:

```bash
scripts/bin/harness-cli query tools --capability <capability> --status present
```

- No registered provider: capability is inactive, clean skip.
- Registered but missing provider: degraded proof, record the gap.
- Present provider: use it and record relevant evidence.

Useful capability names for `mu` are `coverage`, `security-scan`, `documentation-lookup`, and `platform-verification`.

Core local commands are always available through the pinned CLI: `init`, `migrate`, `import brownfield`, `intake`, `story`, `decision`, `trace`, `audit`, `propose`, `tool`, and `query`.
