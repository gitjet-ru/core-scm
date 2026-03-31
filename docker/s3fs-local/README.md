# Gitea + geesefs (local image)

Build target `gitea-s3fs` extends the default image with `geesefs` and mounts an S3-compatible bucket on **`/data/git`** before Gitea starts (bare repositories live under `/data/git/repositories`).

## Build

```bash
docker build --target gitea-s3fs -t core-scm:s3fs .
```

## Required environment variables

| Variable | Description |
|----------|-------------|
| `S3_BUCKET` | Bucket name |
| `AWS_ACCESS_KEY_ID` | Access key |
| `AWS_SECRET_ACCESS_KEY` | Secret key |

`S3_BUCKET` unset -> entrypoint falls back to normal Gitea startup (no fuse mount).

## Optional

| Variable | Description |
|----------|-------------|
| `AWS_SESSION_TOKEN` | STS session token (written into passwd file) |
| `S3_ENDPOINT` | Non-AWS API URL, e.g. `https://s3.gis-1.storage.selcloud.ru` or `http://minio:9000` |
| `S3_REGION` | SigV4 region (e.g. Selectel: `gis-1`) |
| `S3_USE_PATH_STYLE` | `1` (default) adds `use_path_request_style`; try `0` for Selectel/AWS-style virtual-hosted |
| `S3_NO_CHECK_CERT` | Set to `1` for self-signed TLS to custom S3 |
| `DEBUG_GEESEFS` | Set to `1` to enable `--debug_s3 --debug_fuse` |
| `S3FS_UID` / `S3FS_GID` | Ownership of objects (default `1000` / `1000`, `git` user) |
| `GEESEFS_EXTRA_OPTS` | Extra `geesefs` arguments (space-separated) |

## Docker Compose

Секреты в репозитории не хранятся: скопируйте шаблон и заполните значения.

```bash
cp .env.s3.example .env.s3
# отредактируйте .env.s3

docker compose -f docker-compose.yml -f docker-compose.s3fs-local.yml up -d --build gitea
```

Файл [`.env.s3.example`](../../.env.s3.example) → локальный `.env.s3` подключается через `env_file` в [`docker-compose.s3fs-local.yml`](../../docker-compose.s3fs-local.yml). Файл `.env.s3` в `.gitignore`.

The merge file sets `privileged: true` so FUSE works inside the container. For production Kubernetes, prefer a sidecar or CSI; see product docs.

## Caveats

- Use an **empty** bucket or accept that fuse mount will show remote state; a pre-filled local Docker volume at `/data/git` can conflict with a first mount.
- FUSE filesystems have POSIX limitations; large binaries and LFS are better handled via native S3 storage in Gitea.
