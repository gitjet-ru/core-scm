#!/usr/bin/env bash
# Run Playwright e2e tests against an already-deployed instance (no local gitea-e2e server).
# Required: GITEA_TEST_E2E_URL (e.g. https://staging.example.com)
# Typically also set: GITEA_TEST_E2E_USER, GITEA_TEST_E2E_PASSWORD, GITEA_TEST_E2E_DOMAIN
set -euo pipefail

if [ -z "${GITEA_TEST_E2E_URL:-}" ]; then
	echo "error: GITEA_TEST_E2E_URL must be set (base URL of the deployed instance)" >&2
	echo "example: GITEA_TEST_E2E_URL=https://git.example.com make test-e2e-deployed" >&2
	exit 1
fi

pnpm exec playwright test "$@"
