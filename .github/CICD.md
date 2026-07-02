# Nimbus CI/CD Pipeline

This document explains the continuous-integration pipeline that guards the Nimbus
codebase, why each part exists, and how to work with it day to day.

The pipeline is defined in [`.github/workflows/main.yml`](workflows/main.yml) and
runs on GitHub Actions. Its job is simple to state: **no change reaches `main`
unless it builds, passes tests, lints clean, and is free of known vulnerabilities
and leaked secrets.**

---

## At a glance

```
                   push to main  ┐
                                 ├──►  ┌─────────────────────────────────────────┐
   pull request → main  ─────────┘     │              CI Pipeline                │
                                       │                                         │
                                       │  lint (client) ─┐                       │
                                       │  lint (server) ─┤                       │
                                       │  test (client) ─┼─► build ─┐            │
                                       │  test (server) ─┘          │            │
                                       │  govulncheck (client)      ├─► (all     │
                                       │  govulncheck (server)      │    green?) │
                                       │  gitleaks (secret scan)  ──┘            │
                                       │                                         │
                                       │  notify-n8n-on-failure (push only)      │
                                       └─────────────────────────────────────────┘
```

Every job runs in parallel except `build`, which waits for `lint` and `test` to
pass (no point building code that fails those). Jobs are green/red independently
(`fail-fast: false`) so one failure doesn't hide the others.

---

## Triggers

```yaml
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
```

- **`pull_request` → `main`** is the important one: it runs the full pipeline on
  every PR so failures are caught **before** merge. Combined with branch
  protection (below), a red pipeline blocks the merge button.
- **`push` → `main`** re-runs everything after a merge as a safety net, and is the
  only event that fires the failure notification.

A `concurrency` group cancels superseded runs on the same ref, so pushing twice
in quick succession doesn't queue two full runs.

```yaml
concurrency:
  group: ci-${{ github.ref }}
  cancel-in-progress: true
```

---

## Shared configuration

```yaml
env:
  GO_VERSION: "1.26.4"   # keep in sync with go.mod in both modules
permissions:
  contents: read         # least privilege; jobs only read the repo
```

`GO_VERSION` is defined once and referenced by every job. **It must match the
`go` directive in both `client/go.mod` and `server/go.mod`.** This is not
cosmetic — see the govulncheck and lint notes below for why the exact patch
version matters.

The repo has **two independent Go modules** (`client/` and `server/`), so most
jobs use a matrix to run the same steps against each:

```yaml
strategy:
  fail-fast: false
  matrix:
    module: [client, server]
```

---

## The jobs

### 1. `lint` — formatting + static analysis

Runs per module. Two steps:

1. **`gofmt` check** — fails if any file isn't gofmt-clean. This is a formatting
   gate, kept separate so the error message is obvious ("these files are not
   gofmt-clean") rather than buried in linter output.
2. **`golangci-lint`** (v2) — the real static analysis: `errcheck`, `staticcheck`,
   `govet`, `ineffassign`, `unused`, `bodyclose`, `unconvert`, `misspell`, plus
   `gofmt`/`goimports` as formatters. Configured by the repo-root
   [`.golangci.yml`](../.golangci.yml).

> **Why `install-mode: goinstall`?**
> The prebuilt golangci-lint binaries are compiled with an older Go toolchain. A
> binary built with Go 1.24 **refuses** to lint a module whose `go.mod` targets a
> newer language version (`"the Go language version (go1.24) ... is lower than the
> targeted Go version (1.26.4)"`). Setting `install-mode: goinstall` makes the
> action compile golangci-lint from source with the job's Go 1.26.4 toolchain,
> keeping the linter's build version in sync with the code it lints. This costs a
> little build time but removes a whole class of version-skew failures.

### 2. `test` — unit tests, with the race detector where it matters

Runs per module, after `go vet`. The test strategy is deliberately split because
of a real constraint: the server's auth tests hash passwords with **bcrypt cost
14**, which is intentionally slow. The full server suite takes ~110s on its own;
under the race detector (`-race`, ~5–10× overhead) it blows past the default
10-minute test timeout.

So:

| What | Command | Why |
| --- | --- | --- |
| Client (all) | `go test -race -coverprofile=...` | Fast; race detector on everything |
| Server — `middleware/...` | `go test -race` | The rate limiter has real goroutines; race detection matters here |
| Server — full suite | `go test -timeout 8m -coverprofile=...` (no `-race`) | bcrypt-heavy handler tests have no concurrency to detect; skipping `-race` keeps CI fast, with an 8-minute timeout as a guard |

`JWT_SECRET` is injected for the server tests (its package `init` requires it). CI
uses the `JWT_SECRET` repo secret if present, otherwise a throwaway default —
fine for tests, never for production.

### 3. `build` — compile both binaries

```yaml
needs: [lint, test]
```

Gated behind lint and test. Builds `client → nimbus` and `server → api-server`.
If this fails after lint/test passed, it usually means a cross-package or
build-tag issue the per-package steps didn't surface.

### 4. `govulncheck` — known-vulnerability scan

Runs per module. Installs and runs [`govulncheck`](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck),
which reports **only vulnerabilities your code actually reaches** (call-graph
aware), including ones in the Go standard library.

> **Why the Go version matters here.** govulncheck analyzes the stdlib of
> whatever toolchain runs it. When this project was pinned to Go 1.26.3, this job
> correctly flagged `GO-2026-5037` (crypto/x509) and `GO-2026-5039`
> (net/textproto), both fixed in Go 1.26.4. Bumping `GO_VERSION` (and both
> `go.mod` files) to 1.26.4 resolved them. **If this job goes red, first check
> whether the fix is simply a newer Go patch release.**

### 5. `gitleaks` — secret scanning

Checks out full history (`fetch-depth: 0`) and scans for committed credentials as
a backstop to the gitignored `.env*` files. Free for public/personal repos; an
**organization** repo would additionally require a `GITLEAKS_LICENSE` secret.

### 6. `notify-n8n-on-failure` — alerting

```yaml
needs: [lint, test, build, govulncheck, gitleaks]
if: failure() && github.event_name == 'push'
```

Fires only when something fails **on a push to `main`** (not on PRs — PR failures
are visible in the PR itself). Posts run metadata to an n8n webhook using the
`N8N_WEBHOOK_URL` / `N8N_WEBHOOK_SECRET` secrets.

---

## Branch protection

CI checks only *gate* merges if `main` requires them. This repo's `main` branch is
protected with:

- **Required status checks** (must pass before merge):
  `Lint (client)`, `Lint (server)`, `Test (client)`, `Test (server)`, `Build`,
  `govulncheck (client)`, `govulncheck (server)`, `Secret scan`
- **Strict mode** — a branch must be up to date with `main` before merging.
- **Pull requests required** — no direct pushes to `main`.
- **Force-pushes and branch deletion disabled.**
- Required approving reviews: **0** (solo maintainer; bump to 1 when collaborators
  are added). `enforce_admins` is **off** so the maintainer can override in an
  emergency.

> The required-check **names must exactly match** the job names GitHub reports
> (matrix jobs expand to `Job (module)`). If you rename a job in the workflow,
> update the branch-protection required checks too, or merges will hang on a
> check that never reports.

---

## Required repository secrets

| Secret | Used by | Required? |
| --- | --- | --- |
| `JWT_SECRET` | `test` (server) | Optional — falls back to a test default |
| `N8N_WEBHOOK_URL` | `notify-n8n-on-failure` | Only for failure alerts |
| `N8N_WEBHOOK_SECRET` | `notify-n8n-on-failure` | Only for failure alerts |
| `GITLEAKS_LICENSE` | `gitleaks` | Only if the repo lives under an org |

---

## Working with the pipeline

### Reproduce every gate locally before pushing

```bash
# From the repo root. Run inside each module (client/ and server/).

# 1. Formatting
gofmt -l .                      # any output = not clean

# 2. Lint (build the linter with your local toolchain to match CI)
GOTOOLCHAIN=go1.26.4 go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.1.6
golangci-lint run --config=../.golangci.yml ./...

# 3. Tests (mirror the CI split)
go test -race ./...                                   # client
JWT_SECRET=dev go test -race ./middleware/...         # server, concurrent code
JWT_SECRET=dev go test -timeout 8m ./...              # server, full suite

# 4. Build
go build ./...

# 5. Vulnerabilities
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

### The standard change flow

1. Branch off `main` with a descriptive name (`feature/…`, `fix/…`, `chore/…`).
2. Push and open a PR against `main`.
3. The pipeline runs; the merge button stays disabled until all 8 checks are green.
4. Merge, then delete the branch. Repeat one branch per task.

### When a check goes red

- **Lint / gofmt** — run the local commands above; fix or (rarely) adjust
  `.golangci.yml` with a documented rationale.
- **Test** — reproduce locally with the matching command; the server timeout
  means a genuinely slow test can trip the 8-minute guard.
- **Build** — usually a compile error the per-package test step didn't hit.
- **govulncheck** — check for a newer Go patch release first; the fix is often a
  version bump, not a code change.
- **Secret scan** — never "fix" by force-pushing over history without rotating the
  leaked credential first.

---

## Keeping this in sync

When you change the workflow, remember the three things that must move together:

1. **Go version** — `GO_VERSION` in the workflow **and** the `go` directive in
   both `go.mod` files.
2. **Job names** — if renamed, update the branch-protection required checks.
3. **This document** — update it so the pipeline stays legible to the next person
   (including future you).
