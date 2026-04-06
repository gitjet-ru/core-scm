# Dependency Scope Inventory

This document splits third-party components into required compliance scopes for strict permissive-only policy (MIT/BSD/Apache-2.0/ISC).

## Runtime-Prod (mandatory)

- Go modules used by server runtime (`go.mod` + transitive runtime graph).
- Frontend runtime dependencies from `package.json` (`dependencies` block).
- Container/runtime base image for deployment.

Policy:
- Allow only MIT/BSD/Apache-2.0/ISC.
- Block GPL/AGPL/LGPL/MPL, proprietary EULA, and unknown.

## Build-Dev (mandatory for release pipeline)

- Go/Node toolchain used to build release artifacts.
- JS dev dependencies (`package.json` -> `devDependencies`).
- Build scripts under `tools/` and lints/types/tests used as release gates.

Policy:
- Same allowlist as runtime (strict mode).
- Unknown/public-domain/CC0 only via written legal approval.

## CI-Services (non-runtime, still controlled)

- GitHub Actions runners and actions.
- Any service containers used in CI jobs.

Current state:
- No DB service images in active workflows.
- Only `ubuntu-latest` + `actions/checkout` + `actions/setup-node` used by license gate workflow.

Policy:
- No proprietary or copyleft service images in release-critical CI.
- If non-compliant checks are needed, isolate them in non-release workflows.
