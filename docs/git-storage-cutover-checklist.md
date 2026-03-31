# git-storage Cutover Checklist

## 1) Pre-checks

- `git-storage` service is reachable from `core-scm` (`GIT_STORAGE_ENDPOINT`).
- `git-storage` has correct storage config (S3/local root) and write permissions.
- `core-scm` env is set:
  - `GIT_STORAGE_BACKEND=remote`
  - `GIT_STORAGE_ENDPOINT=<host:port>`
  - `GIT_STORAGE_TIMEOUTS=30s` (or project default)
  - optional mirror tuning:
    - `GIT_STORAGE_LOCAL_CACHE_ROOT=...`
    - `GIT_STORAGE_LOCAL_CACHE_TTL=3s`

## 2) Bring up in remote mode

```bash
GIT_STORAGE_BACKEND=remote docker compose -f docker-compose.yml -f docker-compose.git-storage.yml up -d --build
```

Verify containers:

```bash
docker compose -f docker-compose.yml -f docker-compose.git-storage.yml ps
```

## 3) Smoke tests (mandatory)

```bash
GIT_STORAGE_SMOKE_ENDPOINT=127.0.0.1:19096 go test ./modules/gitrepo -run 'TestCreateDelegateHooksRemoteSmoke|TestRemotePRWikiReleaseFlowsSmoke' -count=1
```

Expected:

- both tests pass;
- hooks are generated and executable in remote backend;
- PR/wiki/release-like git flows pass (branch, merge-base, archive, bundle).

## 4) Operational checks

- No sustained errors in `gitea`/`git-storage` logs.
- Push/fetch/clone from UI and API work on test repositories.
- Wiki and release archive downloads work for test repositories.

## 5) Rollback

Switch `core-scm` back to local backend:

```bash
GIT_STORAGE_BACKEND=local docker compose -f docker-compose.yml -f docker-compose.git-storage.yml up -d gitea
```

Verify:

- `gitea` is healthy and serving requests;
- basic clone/push still work in local mode.

## 6) Post-cutover monitoring (recommended)

- Monitor gRPC error rate (`RunGitCommand`, `SmartService`).
- Monitor git operation latency and timeout rate.
- Monitor mirror cache behavior (hit/miss, refresh frequency) if metrics are available.
