# CI/CD Pipeline — Nimbus

<p>
  <img src="https://img.shields.io/badge/CI-GitHub%20Actions-2088FF?logo=github-actions&logoColor=white" alt="GitHub Actions">
  <img src="https://img.shields.io/badge/gates-8%20required%20checks-success" alt="8 required checks">
  <img src="https://img.shields.io/badge/quality-lint%20%7C%20race%20%7C%20CVE%20%7C%20secrets-blue" alt="quality gates">
</p>

Every change to Nimbus passes through an automated pipeline before it can reach
`main`. The rule is simple:

> **No code merges unless it builds, passes tests, lints clean, and is free of
> known vulnerabilities and leaked secrets.**

This is enforced by [GitHub Actions](main.yml) **and** branch protection — a red
pipeline disables the merge button, so `main` stays releasable at all times.

For the full deep-dive (every step, secret, and design decision), see
**[../CICD.md](../CICD.md)**. This page is the quick tour.

---

## The pipeline at a glance

```
   pull request → main          push → main
          │                          │
          └────────────┬─────────────┘
                       ▼
   ┌───────────────────────────────────────────────┐
   │  lint (client)   ──┐                           │
   │  lint (server)   ──┤                           │
   │  test (client)   ──┼──►  build  ──┐            │
   │  test (server)   ──┘               │            │
   │  govulncheck (client) ─────────────┤  all green │
   │  govulncheck (server) ─────────────┤  → mergeable
   │  gitleaks (secret scan) ───────────┘            │
   │                                                 │
   │  notify-n8n-on-failure  (push to main only)     │
   └───────────────────────────────────────────────┘
```

Jobs run in parallel (`fail-fast: false`, so one failure never hides another).
`build` waits for `lint` + `test` — no point compiling code that already failed
those. The two Go modules (`client/`, `server/`) are covered by a matrix, so each
gate runs against both.

---

## What each gate does — and why

| Gate | Tooling | Why it's here |
| --- | --- | --- |
| **Lint** | `gofmt` + `golangci-lint` v2 | Catches unhandled errors, unused code, unclosed HTTP bodies, and enforces one consistent format. Built from source (`goinstall`) so the linter's Go version matches the code's. |
| **Test** | `go test` (+ `-race`) | Unit tests for handlers, auth, and file ops. The race detector runs on the client and the concurrent rate-limiter package; the bcrypt-heavy server suite runs under an 8-minute timeout instead (no concurrency there to detect). |
| **Build** | `go build` | Confirms both binaries actually compile — a backstop for cross-package issues the tests miss. |
| **govulncheck** | `golang.org/x/vuln` | Call-graph-aware scan for known CVEs, **including the Go standard library**. It has already caught real stdlib CVEs on this repo and forced a Go patch bump. |
| **Secret scan** | `gitleaks` | Backstop against committed credentials, scanning full history on every run. |
| **Failure alert** | n8n webhook | Pings an external workflow when `main` breaks (push events only). |

---

## Triggers & concurrency

```yaml
on:
  push:        { branches: [main] }   # safety net after merge; fires alerts
  pull_request: { branches: [main] }  # the gate — runs before merge

concurrency:                          # a newer push cancels the stale run
  group: ci-${{ github.ref }}
  cancel-in-progress: true
```

---

## Branch protection on `main`

The pipeline only *gates* merges because `main` requires it:

- ✅ **8 required checks** must pass: `Lint`, `Test`, `Build`, `govulncheck`
  (× client/server), and `Secret scan`.
- 🔀 **Pull request required** — no direct pushes.
- 🔒 **Strict mode** — branches must be current with `main` before merging.
- ⛔ **Force-push and deletion disabled.**

---

## Reproduce it locally before you push

Run these inside each module (`client/` and `server/`) to mirror CI exactly:

```bash
gofmt -l .                                            # 1. formatting (any output = fail)
golangci-lint run --config=../.golangci.yml ./...     # 2. lint
go test -race ./...                                    #    (client) tests
JWT_SECRET=dev go test -timeout 8m ./...              #    (server) full suite
go build ./...                                         # 4. build
govulncheck ./...                                      # 5. vulnerabilities
```

---

## Standard change flow

```bash
git checkout main && git pull
git checkout -b feature/my-change      # one branch per task
# ...work...
git push -u origin feature/my-change   # open a PR; the 8 checks run
# merge once green, then delete the branch
```

---

## Keeping the pipeline healthy

Three things must always move together:

1. **Go version** — `GO_VERSION` in [`main.yml`](main.yml) **and** the `go`
   directive in both `go.mod` files.
2. **Job names** — if you rename a job, update the branch-protection required
   checks, or merges will hang on a check that never reports.
3. **The docs** — update [../CICD.md](../CICD.md) so the pipeline stays legible.

_Full reference: [../CICD.md](../CICD.md)._
