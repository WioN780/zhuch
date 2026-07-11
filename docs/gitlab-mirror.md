# GitLab mirror

The repo lives on GitHub (source of truth: CI, releases, GHCR images, Railway
deploy). A GitLab mirror exists only to run the tests-only pipeline in
`.gitlab-ci.yml` — go tests, python tests, and the Go↔Python arena parity
gate. No CI/CD variables or secrets are needed on the GitLab side.

## Setting up the mirror

Pick one:

**Option A — GitLab pull mirror (easiest, needs GitLab Premium on gitlab.com)**

1. Create an empty project on GitLab (e.g. `zhuch`).
2. GitLab project → Settings → Repository → Mirroring repositories.
3. Direction: *Pull*. Git repository URL: `https://github.com/<owner>/zhuch.git`.
   For a private GitHub repo, use `https://<github-username>@github.com/...`
   plus a GitHub fine-grained PAT (repo contents: read) as the password.
4. Save. GitLab pulls roughly every 30 minutes; the pipeline runs on each
   mirrored update.

**Option B — push from your machine (free)**

GitHub has no built-in push-mirror setting, so just add GitLab as a second
push URL on your local remote:

```sh
git remote set-url --add --push origin https://github.com/<owner>/zhuch.git
git remote set-url --add --push origin https://gitlab.com/<owner>/zhuch.git
```

Every `git push` then updates both. (First run the command with the GitHub
URL, as shown — adding a push URL replaces the implicit default.)

## What the pipeline runs

| Job | Stage | Does |
|---|---|---|
| `go-test` | test | gofmt check, `go vet`, `go test -race` in `backend/` |
| `python-test` | test | `pip install -e .[dev]`, `pytest` in `training/` (parity auto-skips) |
| `build-arena` | test | builds the static `arena` binary, passes it as an artifact |
| `parity` | integration | starts arena on :8081, runs `tests/test_parity.py` against `POST /forward`, smoke-tests `POST /eval` |

Pytest results show up in GitLab's Tests tab (junit artifacts); the arena log
is kept as a job artifact on failure.
