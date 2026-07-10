# Feature Intake

Classify each request before implementation.

## Input Types

| Type | Use |
| --- | --- |
| change | User-visible CLI behavior or compatibility change |
| bug | Existing behavior is unsafe or incorrect |
| maintenance | Dependency, CI, architecture, or operational work |
| harness | Project workflow, story, matrix, bootstrap, or trace work |

## Lanes

| Lane | Criteria | Required proof |
| --- | --- | --- |
| tiny | Docs or copy only, no behavior claim | focused review or syntax check |
| normal | Bounded non-destructive Go or reporting change | unit tests, vet, build |
| high-risk | Filesystem deletion/trash, APT/Snap, `sudo`, configuration protection, release, installer, dependency security, or interruption behavior | failure-path unit tests, race/vet/security/build/smoke, plus affected Ubuntu VM scenarios |

## High-Risk Intake

Record:

- current unsafe or ambiguous behavior;
- exact command, path, package, and privilege boundaries;
- dry-run parity;
- partial-failure and recovery behavior;
- logging and exit-code behavior;
- compatibility constraints;
- automated and VM proof;
- stop conditions and release blockers.

Use `.codex/skills/harness-intake-griller/SKILL.md` and the high-risk story templates. Invalid safety configuration must fail closed. Required proof may not be weakened without user approval.
