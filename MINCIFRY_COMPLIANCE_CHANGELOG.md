# Minцифры Compliance Changelog (core-scm)

This file is the execution log for `MINCIFRY_COMPLIANCE_PLAN.md`.

## Rules

- Log every completed remediation item with date and short evidence.
- Keep entries in reverse chronological order.
- Reference affected files and validation command outputs.

## Entries

### 2026-04-07 - Independent internal base images (alpine/golang)

- Removed upstream dependency from base-image source Dockerfiles:
  - `docker/base-images/alpine-3.23.Dockerfile` switched to `FROM scratch` + local rootfs artifact
  - `docker/base-images/alpine-3.19.Dockerfile` switched to `FROM scratch` + local rootfs artifact
  - `docker/base-images/golang-1.26-alpine3.23.Dockerfile` switched to `FROM scratch` + local alpine rootfs + local Go tarball
- Added artifact integrity controls:
  - added `compliance/base-image-artifacts.sha256`
  - updated `tools/base-images.mjs` with `verify-artifacts` and pre-build checksum validation
  - added Makefile target `base-images-verify-sources`
- Added local artifact storage rule:
  - `.gitignore`: `/artifacts/`
- Implemented internal base-image source layout:
  - added `docker/base-images/alpine-3.23.Dockerfile`
  - added `docker/base-images/alpine-3.19.Dockerfile`
  - added `docker/base-images/golang-1.26-alpine3.23.Dockerfile`
- Added lock and automation for build/push/digest/SBOM/sign:
  - added `compliance/base-images.lock`
  - added `tools/base-images.mjs` (`build`, `push`, `sbom`, `sign`)
  - added `tools/sync-base-images-lock.mjs` (resolve pushed tags to immutable digests)
  - added Makefile targets: `base-images-build`, `base-images-push`, `base-images-lock-sync`, `base-images-sbom`, `base-images-sign`
- Switched project Dockerfiles to internal digest refs by default:
  - `Dockerfile`: `GOLANG_BASE_REF`, `ALPINE_BASE_REF`
  - `Dockerfile.rootless`: `GOLANG_BASE_REF`, `ALPINE_BASE_REF`
  - `docker/etcd/Dockerfile`: `ALPINE_BASE_REF` (3.19 lock entry)
- Updated runbook:
  - expanded `compliance/CONTAINER_BASE_IMAGES.md` with end-to-end internal build/publish/verify flow
- Validation evidence:
  - `node tools/license-gate.mjs` returns `OK`
  - `grep '^FROM '` in the three Dockerfiles confirms only `*_BASE_REF` usage

### 2026-04-06 - P1 base image pinning preparation

- Prepared Docker builds for digest pinning via explicit build args:
  - `Dockerfile`: added `GOLANG_BASE_IMAGE` and `ALPINE_BASE_IMAGE` args; all external `FROM` now reference these args
  - `Dockerfile.rootless`: added `GOLANG_BASE_IMAGE` and `ALPINE_BASE_IMAGE` args; all external `FROM` now reference these args
  - `docker/etcd/Dockerfile`: added `ALPINE_BASE_IMAGE` arg and switched `FROM` to arg reference
- Added runbook for digest-pinned invocation:
  - `compliance/CONTAINER_BASE_IMAGES.md`
- Validation evidence:
  - Dockerfiles accept digest-qualified image refs via `--build-arg ...=<image>@sha256:<digest>`
  - Automatic digest resolution from Docker Hub was attempted but failed in current environment due network TLS handshake timeout

### 2026-04-06 - P2 automation gate and compliance docs

- Added strict local license gate automation (without repository workflows):
  - added `tools/license-gate.mjs`
  - added `Makefile` target `compliance-license-gate`
  - added npm script `compliance:license-gate` in `package.json`
- Added versioned policy config with allow/deny lists:
  - added `compliance/license-policy.json`
- Added compliance documentation artifacts:
  - added `THIRD_PARTY_NOTICES.md` with vendored component evidence links
  - added `compliance/EXCEPTIONS_REGISTER.md` with owner/date-based exception format
- Validation evidence:
  - `node tools/license-gate.mjs` returns `OK` on current dependency manifests

### 2026-04-06 - Automation workflows removed

- Removed all automation workflow definitions from main repository scope:
  - deleted all files under `.github/workflows/`
  - deleted workflow-helper configs used only by repository automation: `.github/actionlint.yaml`, `.github/labeler.yml`, `.github/dependabot.yml`
- Removed vendored workflow configs from `third_party` to satisfy "no repository automation configs":
  - deleted `third_party/**/.github/workflows/*`
  - deleted `third_party/**/.gitea/workflows/*`
  - deleted `third_party/redsync/.gitlab-ci.yml`
- Validation evidence:
  - no files remain matching `**/.github/workflows/**`, `**/.gitea/workflows/**`, `**/.gitlab-ci.yml`

### 2026-04-06 - P1 supply-chain hardening (part 1)

- Replaced unverified external fork reference for `html2text` with repository-local verified source:
  - added `third_party/html2text` (vendorized from previously used Necoro fork)
  - pinned root module replace to `replace github.com/jaytaylor/html2text => ./third_party/html2text`
  - adjusted module path in local fork to keep dependency path compatibility
- Added binary checksum verification in Docker builds:
  - `Dockerfile`: `geesefs` download now verifies SHA256 using GitHub release API asset `digest` metadata before install
  - `docker/etcd/Dockerfile`: etcd tarball now fetched from official storage with `SHA256SUMS` verification before extraction
- Validation evidence:
  - `go mod tidy` + `go build ./...` pass after html2text source switch
  - checksum verification commands are embedded in Docker build stages (`sha256sum -c`)

### 2026-04-06 - P0 closure (remaining blockers)

- Closed remaining P0 JS blockers:
  - removed `htmx.org` and `idiomorph` from `package.json` and `pnpm-lock.yaml`
  - replaced runtime dependency with compatibility stub in `web_src/js/htmx.ts`
  - removed eager `htmx.org` import in `web_src/js/index.ts`
  - replaced `htmx` global type import with local interface in `web_src/js/globals.d.ts`
- Closed remaining P0 Go blockers without HashiCorp module paths in root module graph:
  - added `third_party/redsync` and removed `hashicorp/go-multierror` usage in `mutex.go`
  - added `third_party/sevenzip` and removed `hashicorp/golang-lru/v2` usage in `internal/aes7z/key.go`
  - updated root `go.mod` replaces to local forks (`redsync`, `sevenzip`) and re-tidied
- Validation evidence:
  - `go mod why -m` for blocked modules now returns `(main module does not need module ...)`
  - `go.mod`/`go.sum` have no matches for blocked Go modules (`hashicorp/*`, mysql/sqlite, xi2/xz, blake3, gitlab client)
  - `package.json`/`pnpm-lock.yaml` have no matches for `htmx.org`, `idiomorph`, `@resvg/resvg-wasm`, `eslint-plugin-sonarjs`, `stylelint-value-no-unknown-custom-properties`
  - `go build ./...` passes

### 2026-04-06 - P0 Go/JS remediation wave 1

- Replaced direct HashiCorp version parser usage with `github.com/Masterminds/semver/v3` across package, migration, update checker, and git-version code paths.
- Removed migration downloaders that pulled non-permissive transitive dependencies:
  - deleted `services/migrations/gitlab.go`
  - deleted `services/migrations/gitlab_test.go`
  - deleted `services/migrations/gitea_downloader.go`
  - deleted `services/migrations/gitea_downloader_test.go`
- Introduced internal permissive cache module:
  - added `modules/lru/lru.go`
  - added `modules/lru/twoqueue.go`
  - switched cache consumers from `hashicorp/golang-lru/v2` to internal `modules/lru`.
- Added local permissive replacements in `go.mod` for high-risk transitive chains:
  - `third_party/certmagic` (replaced BLAKE3 with SHA-256)
  - `third_party/go-rpmutils` (replaced `xi2/xz` with `ulikunitz/xz`)
  - `third_party/gitea-chi-cache` / `third_party/gitea-chi-session` (removed MySQL adapters and ledis/nodb adapters)
  - `third_party/xorm` (reduced module requirements to runtime set)
  - `third_party/hashicorp-golang-lru-v2` and `third_party/hashicorp-go-multierror` replace targets
- JS toolchain cleanup:
  - replaced `@resvg/resvg-wasm` usage in `tools/generate-images.ts` with Playwright-based SVG→PNG rendering
  - removed `eslint-plugin-sonarjs` rules from `eslint.config.ts`
  - removed `stylelint-value-no-unknown-custom-properties` plugin from `stylelint.config.js`
  - removed these packages from `package.json`, regenerated `pnpm-lock.yaml`
- Validation evidence:
  - `go build ./...` passes
  - `go.sum` has no matches for `go-sql-driver/mysql`, `mattn/go-sqlite3`, `xi2/xz`, `zeebo/blake3`, `hashicorp/go-version`, `gitlab.com/gitlab-org/api`
  - `package.json` and `pnpm-lock.yaml` have no matches for `@resvg/resvg-wasm`, `eslint-plugin-sonarjs`, `stylelint-value-no-unknown-custom-properties`

### 2026-04-06 - Plan initialized

- Initialized plan file: `MINCIFRY_COMPLIANCE_PLAN.md`.
- Initialized compliance changelog: `MINCIFRY_COMPLIANCE_CHANGELOG.md`.
- No remediation tasks marked as completed yet.

## Task Status Snapshot

- P0 completed: `12`
- P1 completed: `3`
- P2 completed: `4`
