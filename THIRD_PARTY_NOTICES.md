# Third-Party Notices (core-scm)

This file captures third-party components that are vendored/forked in-repository for license compliance and supply-chain control.

## Vendored Components and Evidence

- `third_party/certmagic` (source: `github.com/caddyserver/certmagic`, license: Apache-2.0)
- `third_party/go-rpmutils` (source: `github.com/sassoftware/go-rpmutils`, license: Apache-2.0)
- `third_party/gitea-chi-cache` (source: `gitea.com/go-chi/cache`, license: MIT)
- `third_party/gitea-chi-session` (source: `gitea.com/go-chi/session`, license: MIT)
- `third_party/redsync` (source: `github.com/go-redsync/redsync`, license: BSD-3-Clause)
- `third_party/sevenzip` (source: `github.com/bodgit/sevenzip`, license: MIT)
- `third_party/xorm` (source: `xorm.io/xorm`, license: BSD-3-Clause)
- `third_party/html2text` (source: `github.com/jaytaylor/html2text`, license: MIT)

## Verification Notes

- Forks are used to remove blocked dependencies and stabilize provenance under repository control.
- Effective dependency graph and blocked-module checks are enforced via `tools/license-gate.mjs` and `compliance/license-policy.json`.
