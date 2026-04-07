# Container Base Image Pinning (core-scm)

This project uses internal, digest-pinned base images. External registry references are not used in project Dockerfiles by default.
Base images are built from local source artifacts (`artifacts/*`) without `FROM docker.io/library/*` in base-image Dockerfiles.

## Internal Naming

Set corporate registry namespace once per environment:

```bash
export REGISTRY_BASE=registry.example.local/gitjet/base
```

Lock file:

- `compliance/base-images.lock`

## Build and Publish Flow

0. Place local source artifacts and set checksums:

- `artifacts/rootfs/alpine-minirootfs-3.23.0-x86_64.tar.gz`
- `artifacts/rootfs/alpine-minirootfs-3.19.0-x86_64.tar.gz`
- `artifacts/go/go1.26.0.linux-amd64.tar.gz`

Update `compliance/base-image-artifacts.sha256` with real hashes.

Verify artifact integrity:

```bash
make base-images-verify-sources
```

1. Build internal base images:

```bash
make base-images-build REGISTRY_BASE="$REGISTRY_BASE"
```

2. Push to corporate registry:

```bash
make base-images-push REGISTRY_BASE="$REGISTRY_BASE"
```

3. Resolve and persist immutable digests:

```bash
make base-images-lock-sync REGISTRY_BASE="$REGISTRY_BASE"
```

4. Generate SBOM evidence:

```bash
make base-images-sbom REGISTRY_BASE="$REGISTRY_BASE"
```

5. Sign digest-pinned refs (optional but recommended):

```bash
make base-images-sign REGISTRY_BASE="$REGISTRY_BASE"
```

## Build Args Used by Project Dockerfiles

- `GOLANG_BASE_REF`
- `ALPINE_BASE_REF`

For etcd image:

- `ALPINE_BASE_REF` (3.19 variant)

## Validation

- `grep '^FROM ' Dockerfile Dockerfile.rootless docker/etcd/Dockerfile` points only to `*_BASE_REF`.
- `compliance/base-images.lock` contains digest-pinned refs (`@sha256:`) after sync.
- `compliance/sbom/*.spdx.json` exists after SBOM generation.
- `docker/base-images/*.Dockerfile` contain no `FROM docker.io/library/*`.
