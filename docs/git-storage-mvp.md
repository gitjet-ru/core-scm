# git-storage MVP (bare repos only)

This MVP integrates an external `git-storage` service over gRPC for **Git bare repository** operations only.

## Scope

- Included:
  - Repository lifecycle for bare repos: init/delete/rename.
  - Default branch HEAD operations: set/get.
  - Smart HTTP operations: `upload-pack`, `receive-pack`, `upload-archive` via remote service.
- Not included:
  - LFS, attachments, avatars, packages, actions artifacts.
  - Hook event bus / distributed lock manager.

## Feature Flags

Use environment variables in `gitea` container:

- `GIT_STORAGE_BACKEND=local|remote|shadow`
- `GIT_STORAGE_ENDPOINT=host:port` (default for compose overlay: `git-storage:9093`)
- `GIT_STORAGE_TIMEOUTS=30s`
- `GIT_STORAGE_LOCAL_CACHE_ROOT=/tmp/core-scm-git-storage-cache` (optional local mirror cache root)
- `GIT_STORAGE_LOCAL_CACHE_TTL=3s` (optional cache reuse window; `0s` disables reuse)

Behavior:

- `local`: current in-process local filesystem behavior.
- `remote`: bare-repo operations use external gRPC service.
- `shadow`: same routing as `remote` for this MVP (reserved for rollout policy/telemetry extension).

## Local Bring-up

Run with overlay:

```bash
docker compose -f docker-compose.yml -f docker-compose.git-storage.yml up -d --build
```

Switch backend:

```bash
GIT_STORAGE_BACKEND=remote docker compose -f docker-compose.yml -f docker-compose.git-storage.yml up -d gitea
```

Rollback:

```bash
GIT_STORAGE_BACKEND=local docker compose -f docker-compose.yml -f docker-compose.git-storage.yml up -d gitea
```

## Remote Mirror Cache Behavior

- `core-scm` keeps a local mirror cache for read-heavy and custom I/O flows.
- Cache entries are reused for `GIT_STORAGE_LOCAL_CACHE_TTL`, then refreshed from `git-storage`.
- Any mutating remote operation invalidates corresponding local mirror cache path.

## WriteRepoFile Contract

`WriteRepoFileRequest` supports optional `file_mode`:

- `file_mode = 0`: backend default mode `0644`.
- `file_mode > 0`: exact mode is used (for example `0777` for generated hooks).

This is required so remote hook generation keeps executable bits.

## Smoke Checklist (remote mode)

- Start stack:
  - `GIT_STORAGE_BACKEND=remote docker compose -f docker-compose.yml -f docker-compose.git-storage.yml up -d --build`
- Gitrepo remote smokes:
  - `GIT_STORAGE_SMOKE_ENDPOINT=127.0.0.1:19096 go test ./modules/gitrepo -run TestCreateDelegateHooksRemoteSmoke -count=1`
  - `GIT_STORAGE_SMOKE_ENDPOINT=127.0.0.1:19096 go test ./modules/gitrepo -run TestRemotePRWikiReleaseFlowsSmoke -count=1`
- Fallback:
  - `GIT_STORAGE_BACKEND=local docker compose -f docker-compose.yml -f docker-compose.git-storage.yml up -d gitea`
