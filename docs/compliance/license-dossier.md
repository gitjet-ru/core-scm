# License Compliance Dossier

## Policy baseline

- Strict permissive-only policy for all required scopes:
  - MIT
  - BSD (2/3-Clause)
  - Apache-2.0
  - ISC
- Explicitly blocked:
  - GPL/AGPL/LGPL/MPL
  - proprietary EULA
  - unknown / unresolved

## Implemented remediation

### Go

- Removed HashiCorp MPL chain from code usage:
  - Replaced `github.com/hashicorp/go-version` with internal wrapper `modules/semver` based on `github.com/Masterminds/semver/v3` (permissive).
- Removed external migration downloaders that pulled non-permissive dependency chains:
  - removed GitLab/Gitea migration downloader implementations from `services/migrations`.
- Removed non-postgres DB driver usage and references in application code.
- Removed `github.com/hashicorp/go-retryablehttp` and related HashiCorp MPL modules from `go.mod`/`go.sum`.

### JavaScript

- Removed blocked dependencies:
  - `@resvg/resvg-wasm`
  - `eslint-plugin-sonarjs`
- Updated image generation tool to use Playwright rendering instead of `resvg-wasm`.
- Updated ESLint config to remove SonarJS plugin/rules.

### CI / Gate

- Added mandatory CI workflow: `.github/workflows/license-gate.yml`.
- Added gate script: `tools/license-gate.mjs`.
- Gate fails on:
  - denied Go modules (`hashicorp/*`, mysql/sqlite drivers, and unresolved exceptions from prior audit),
  - forbidden licenses from `assets/go-licenses.json`,
  - unknown/non-allowlist licenses from `assets/go-licenses.json`,
  - denied JS packages (`@resvg/resvg-wasm`, `eslint-plugin-sonarjs`).

## Evidence artifacts

- Go license inventory: `assets/go-licenses.json`
- Scope inventory: `docs/compliance/dependency-scope-inventory.md`
- Gate implementation:
  - `tools/license-gate.mjs`
  - `.github/workflows/license-gate.yml`

## Residual risk / legal review points

- Temporary exceptions are tracked in `docs/compliance/license-exceptions.json`.
- Any exception requires written legal/security approval and must be removed after replacement.
