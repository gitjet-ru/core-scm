# Remote Storage Freeze Status

This document freezes the current remote-storage direction while we stabilize the single-instance local profile.

## Scope freeze

During this phase:

- no new remote-storage architecture changes are introduced
- no rollout assumptions are made on remote mode
- all critical fixes prioritize local-first stability

## What already exists

- Feature flags:
  - `GIT_STORAGE_BACKEND=local|remote|shadow`
  - `GIT_STORAGE_READ_BACKEND=local|remote|shadow`
- Remote read/write integrations across multiple modules
- Existing docs:
  - `docs/git-storage-mvp.md`
  - `docs/git-storage-cutover-checklist.md`

## Known gaps observed before freeze

- Inconsistent branching contracts between API middleware and some handlers
- Mixed assumptions around when `ctx.Repo.GitRepo` must be present
- Operational complexity for remote mode compared with local single-instance

## Next phase roadmap (after local stabilization)

1. Re-audit all endpoint-level remote/local branching with explicit compatibility matrix.
2. Decide target remote read strategy (mirror-based vs mirrorless) per endpoint group.
3. Add remote-focused smoke suite for API/web parity.
4. Resume performance work (compare/diff/tree paths) only after parity and reliability criteria are met.

## Exit criteria for this freeze

Remote-storage feature work resumes only when:

- local single-instance profile passes smoke checks on bare metal and Kubernetes
- deployment profile documentation is validated in CI/dev environments
- no critical local-mode regressions remain open
