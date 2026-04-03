# Single-Instance Local Smoke Checklist

Use this checklist after deployment with:

- `GIT_STORAGE_BACKEND=local`
- `GIT_STORAGE_READ_BACKEND=local`
- PostgreSQL enabled

## Web smoke

1. Create repository via UI.
2. Push at least one branch from local git client.
3. Open `src` and browse directories/files.
4. Open compare page (`/compare/...`) and verify diff/tree rendering.
5. Create PR and open PR diff page.
6. Create/read wiki page.
7. Download archives (`zipball`/`tarball`/`archive`).

## API smoke

Run against a test repository:

1. `GET /repos/{owner}/{repo}/raw/{ref}/{path}`
2. `GET /repos/{owner}/{repo}/media/{ref}/{path}`
3. `GET /repos/{owner}/{repo}/contents/{path}?ref=...`
4. `GET /repos/{owner}/{repo}/branches`
5. `GET /repos/{owner}/{repo}/branches/{branch}`
6. `GET /repos/{owner}/{repo}/commits`
7. `GET /repos/{owner}/{repo}/git/refs/*`
8. `GET /repos/{owner}/{repo}/git/commits/{sha}.diff`

Expectations:

- no panics in server logs
- no dependency on `GIT_STORAGE_ENDPOINT`
- responses are consistent after restart

## Restart/redeploy smoke

1. Restart `core-scm`.
2. Re-run key API and web checks.
3. Verify repositories and LFS data persist.

## Failure signals

- panic containing `no GitRepo, forgot to call the middleware?`
- API 500 on branch/commit/raw/media paths for existing refs
- missing repos after restart (volume mount issue)
