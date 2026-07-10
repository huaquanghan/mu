# Validation

## Proof Strategy

Explain what must pass before the story is done.

For destructive paths, include dry-run parity, boundary escape, malformed configuration, partial failure, accurate logging, rollback, and recovery-location cases.

## Test Plan

| Layer | Cases |
| --- | --- |
| Unit | |
| Integration | |
| E2E | |
| Platform | |
| Performance | |
| Logs/Audit | |

## Fixtures

List deterministic users, accounts, records, provider responses, or other
fixtures needed for repeatable proof.

## Commands

```bash
go test ./... -count=1
go test ./... -race -count=1
go vet ./...
make coverage
staticcheck ./...
govulncheck ./...
make build
make smoke
```

## Acceptance Evidence

Add results after verification.

Keep release blocked while any required Ubuntu VM scenario is pending.
