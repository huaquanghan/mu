# Trace Specification

Record a trace after normal or high-risk work:

```bash
scripts/bin/harness-cli trace \
  --summary "<outcome>" \
  --story MU-001 \
  --agent codex \
  --outcome success \
  --actions "<commands and edits>" \
  --read "<important files>" \
  --changed "<changed files>" \
  --decisions "<safety decisions>" \
  --errors "<remaining blockers>" \
  --friction "<harness gaps or none>"
```

High-risk traces must name destructive boundaries, automated proof, VM proof status, release blockers, and any skipped external capability. Do not record `success` when required proof failed; use `partial` or `blocked` and explain why.
