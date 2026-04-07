# Minцифры Compliance Plan (core-scm)

## Scope

- Repository: `gitjet/core-scm`
- Policy: strict permissive-only
- Allowed licenses: `MIT`, `BSD-2-Clause`, `BSD-3-Clause`, `Apache-2.0`, `ISC`
- Disallowed: `MPL`, `LGPL`, `GPL`, `AGPL`, `CC0`, `Public Domain`, unknown/proprietary (until approved)

## Current Priority Backlog

### P0 - Blockers (must fix first)

- [x] Replace `github.com/hashicorp/golang-lru/v2` with internal permissive cache module.
- [x] Remove `github.com/hashicorp/go-version`.
- [x] Remove `github.com/hashicorp/go-multierror` chain.
- [x] Remove `github.com/hashicorp/go-retryablehttp` chain.
- [x] Remove `github.com/go-sql-driver/mysql`.
- [x] Remove `github.com/xi2/xz`.
- [x] Remove `github.com/zeebo/blake3`.
- [x] Remove `@resvg/resvg-wasm` from JS toolchain.
- [x] Remove `eslint-plugin-sonarjs` from JS linting.
- [x] Remove `stylelint-value-no-unknown-custom-properties`.
- [x] Replace/remove `htmx.org` (0BSD).
- [x] Replace/remove `idiomorph` (0BSD).

### P1 - Supply chain and reproducibility

- [x] Replace unstable `replace github.com/jaytaylor/html2text => github.com/Necoro/html2text` with verified source.
- [x] Add checksum/signature verification for downloaded binaries in Docker build (`geesefs`, etc.).
- [x] Pin container base images by digest where feasible.
- [x] Pin remote automation actions by commit SHA (N/A: repository automation workflows removed).

### P2 - Automation gate and documentation

- [x] Add strict license gate script and execution automation.
- [x] Add denylist and allowlist in versioned config.
- [x] Add third-party notices and evidence links.
- [x] Add explicit exceptions register (temporary, dated, owner-assigned).

## Execution Order

1. P0 Go dependency cleanup.
2. P0 JS dependency cleanup.
3. Re-run `go mod tidy` and lockfile regeneration.
4. Rebuild/test affected components.
5. P1 supply-chain hardening.
6. P2 automation gates and compliance docs.

## Validation Checklist

- [x] `go mod why -m` for each blocked Go module returns: `(main module does not need module ...)`.
- [x] `go.mod` has no blocked modules.
- [x] JS manifests/lockfiles have no blocked packages.
- [ ] Docker/automation provenance checks pass.
- [x] Compliance changelog updated for each completed task.
