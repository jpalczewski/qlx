# Go Code Quality, CI Workflow & Changelog Management

## Overview

Pre-commit hooks, CI pipeline, and automated changelog/versioning for QLX — a solo Go project with PR-based workflow on GitHub.

## 1. Pre-commit Hooks (lefthook)

**Tool:** [lefthook](https://github.com/evilmartians/lefthook) — Go-native git hooks manager, zero external dependencies.

### Pre-commit hook (<3s)
- `gofmt -l` — check formatting (fail on unformatted files, no auto-fix)
- `go vet ./...` — built-in static analysis
- `golangci-lint run --fast` — minimal fast linter subset: `govet`, `staticcheck`, `errcheck`, `unused`

### Commit-msg hook
- Validate conventional commit format via regex
- Pattern: `^(feat|fix|refactor|docs|test|chore|build|ci|perf)(\(.+\))?: .+`
- Allowed types: `feat`, `fix`, `refactor`, `docs`, `test`, `chore`, `build`, `ci`, `perf`

## 2. CI — GitHub Actions

**File:** `.github/workflows/ci.yml`
**Trigger:** PR to `main` + push to `main`

### Jobs

**`lint`**
- Uses `golangci/golangci-lint-action`
- Full linter set (superset of local): adds `gosec`, `gocritic`, `ineffassign`, `misspell`, `gocyclo`
- Blocks merge on failure

**`test`**
- `go test ./... -race -coverprofile=coverage.out`
- Upload coverage as artifact (no external service)

**`build`**
- `go build ./cmd/qlx/` (dev build, no CGO)
- Does NOT cross-compile mac/mips — requires CGO/specific toolchains

### Branch Protection on `main`
- Require PR
- Require status checks: `lint`, `test`, `build`

## 3. golangci-lint Configuration

**File:** `.golangci.yml`

| Linter | Purpose | Where |
|---|---|---|
| `govet` | static analysis from Go | pre-commit + CI |
| `staticcheck` | advanced vet | pre-commit + CI |
| `errcheck` | uncaught errors | pre-commit + CI |
| `unused` | dead code | pre-commit + CI |
| `gosec` | security issues | CI only |
| `gocritic` | code style, performance | CI only |
| `ineffassign` | inefficient assignments | CI only |
| `misspell` | typos in comments/strings | CI only |
| `gocyclo` | cyclomatic complexity (limit ~15) | CI only |

**Exclusions:**
- `*_test.go` — relaxed rules (no `errcheck`)
- Default build tags (no `ble`) — BLE code requires CGO/CoreBluetooth, not available on CI

**Local vs CI:** lefthook runs `golangci-lint run --fast`, CI runs full `golangci-lint run`.

## 4. Changelog + Versioning (release-please)

**Tool:** [release-please](https://github.com/googleapis/release-please) as GitHub Action.
**File:** `.github/workflows/release.yml` — separate from CI, triggered on push to `main`.

### Flow
1. PRs merged to `main` with conventional commits
2. release-please auto-creates/updates a **Release PR** with accumulated changes
3. Release PR contains updated `CHANGELOG.md` + version bump
4. Merge Release PR when ready to release
5. release-please creates git tag (`v0.1.0`) + GitHub Release

### Configuration
- `release-type: go`
- Version tracked in git tags (no extra version file)
- Changelog grouped as: Features, Bug Fixes, Miscellaneous
- Initial release: `v0.1.0`

### CHANGELOG.md format
```
## [0.2.0](https://github.com/.../compare/v0.1.0...v0.2.0) (2026-03-21)

### Features
* **niimbot:** add 50x20mm label barcode to offline db (ea7f375)

### Bug Fixes
* **ui:** expose showToast globally (0b7b7f0)
```

## 5. Makefile Additions

```makefile
lint:
	golangci-lint run

lint-fix:
	golangci-lint run --fix

install-hooks:
	lefthook install
```

## 6. New Files Summary

| File | Purpose |
|---|---|
| `lefthook.yml` | Git hooks configuration |
| `.golangci.yml` | Linter configuration |
| `.github/workflows/ci.yml` | CI pipeline (lint, test, build) |
| `.github/workflows/release.yml` | Automated changelog + releases |
