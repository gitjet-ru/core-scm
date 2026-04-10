# GitJet Base Images

Central repository for internal base images used by GitJet projects:

- `alpine:3.23`
- `alpine:3.19`
- `go:1.26.1`
- `python:3.12-alpine`

## Local source artifacts

Place artifacts before build:

- `artifacts/rootfs/alpine-minirootfs-3.23.0-amd64.tar.gz`
- `artifacts/rootfs/alpine-minirootfs-3.19.0-amd64.tar.gz`
- `artifacts/go/go1.26.1.linux-amd64.tar.gz`
- `artifacts/rootfs/alpine-minirootfs-3.23.0-arm64.tar.gz`
- `artifacts/rootfs/alpine-minirootfs-3.19.0-arm64.tar.gz`
- `artifacts/go/go1.26.1.linux-arm64.tar.gz`

Checksum file: `compliance/base-image-artifacts.sha256`

## Build / publish flow

```bash
make base-images-verify-sources
make base-images-build REGISTRY_BASE=registry.gitjet.ru
make base-images-push REGISTRY_BASE=registry.gitjet.ru
make base-images-lock-sync REGISTRY_BASE=registry.gitjet.ru
```

Optional:

```bash
make base-images-sbom REGISTRY_BASE=registry.gitjet.ru
make base-images-sign REGISTRY_BASE=registry.gitjet.ru
```

## Consumer usage

### core-scm

Use refs from `compliance/base-images.lock`:

- `GOLANG_1_26_ALPINE_3_23_REF`
- `ALPINE_3_23_REF`
- `ALPINE_3_19_REF` (etcd)

### gitjet-docs

Use:

- `PYTHON_3_12_ALPINE_REF` as `PYTHON_BASE_REF`
