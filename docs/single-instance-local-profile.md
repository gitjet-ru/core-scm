# Single-Instance Local Profile (no S3)

This profile is for running `core-scm` as a single instance with:

- PostgreSQL enabled
- local filesystem repositories/LFS
- remote git-storage disabled

It is intended to work the same way on bare metal and Kubernetes.

## Required runtime flags

Set these environment variables for `core-scm`:

- `GIT_STORAGE_BACKEND=local`
- `GIT_STORAGE_READ_BACKEND=local`

For this profile, do not set `GIT_STORAGE_ENDPOINT`.

## Required persistent paths

From the default container template (`docker/root/etc/templates/app.ini`):

- repositories: `/data/git/repositories`
- LFS: `/data/git/lfs`
- app data: `/data/gitea`

If you customize paths in `app.ini`, mount those exact custom paths.

## Minimal app.ini expectations

- `[database]` points to PostgreSQL (`DB_TYPE=postgres`)
- `[repository] ROOT` points to mounted repo path
- `[lfs] PATH` points to mounted LFS path
- `LFS_START_SERVER=true` if LFS API is required

## Bare metal checklist

1. Start PostgreSQL.
2. Mount/create persistent directories for repositories and LFS.
3. Start `core-scm` with:
   - `GIT_STORAGE_BACKEND=local`
   - `GIT_STORAGE_READ_BACKEND=local`
4. Verify startup and DB migrations.

## Kubernetes checklist

1. Deploy PostgreSQL (stateful, persistent volume).
2. Mount persistent volumes to:
   - `/data/git/repositories`
   - `/data/git/lfs`
   - `/data/gitea`
3. Set env:
   - `GIT_STORAGE_BACKEND=local`
   - `GIT_STORAGE_READ_BACKEND=local`
4. Do not inject `GIT_STORAGE_ENDPOINT`.

## Notes

- This profile is local-first by design.
- Remote storage experiments are documented separately and are not part of this rollout profile.
