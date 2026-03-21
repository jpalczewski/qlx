# Go Quality Tooling, CI & Changelog Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add pre-commit hooks, CI pipeline, and automated changelog/versioning to QLX.

**Architecture:** lefthook for local git hooks (fast lint + conventional commit validation), GitHub Actions for full CI (lint, test, build), release-please for automated changelog and SemVer releases via PR workflow.

**Tech Stack:** lefthook, golangci-lint, GitHub Actions, release-please

**Spec:** `docs/superpowers/specs/2026-03-21-go-quality-workflow-design.md`

---

### Task 1: golangci-lint configuration

**Files:**
- Create: `.golangci.yml`

- [ ] **Step 1: Create `.golangci.yml`**

```yaml
version: "2"

run:
  build-tags: []

linters:
  enable:
    - govet
    - staticcheck
    - errcheck
    - unused
    - gosec
    - gocritic
    - ineffassign
    - misspell
    - gocyclo
  settings:
    gocyclo:
      min-complexity: 15

issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - errcheck
    - path: (ble|bluetooth)\.go
      linters:
        - unused
```

Note: `build-tags: []` ensures default build (no `ble` tag) — BLE files will be excluded from compilation on CI. The `unused` linter is excluded from BLE/bluetooth files to avoid false positives from build-tag-gated code.

- [ ] **Step 2: Install golangci-lint locally (if not installed)**

Run: `brew install golangci-lint`

- [ ] **Step 3: Run full lint to verify config works**

Run: `golangci-lint run ./...`
Expected: Completes without config errors. May report lint findings — that's OK, we'll fix them later if needed.

- [ ] **Step 4: Run the pre-commit subset to verify it works**

Run: `golangci-lint run --enable-only govet,staticcheck,errcheck ./...`
Expected: Completes without config errors.

- [ ] **Step 5: Commit**

```bash
git add .golangci.yml
git commit -m "build: add golangci-lint configuration"
```

---

### Task 2: lefthook setup

**Files:**
- Create: `lefthook.yml`

- [ ] **Step 1: Install lefthook (if not installed)**

Run: `brew install lefthook`

- [ ] **Step 2: Create `lefthook.yml`**

```yaml
pre-commit:
  parallel: true
  commands:
    gofmt:
      glob: "*.go"
      run: gofmt -l {staged_files} && test -z "$(gofmt -l {staged_files})"
    govet:
      glob: "*.go"
      run: go vet ./...
    golangci-lint:
      glob: "*.go"
      run: golangci-lint run --enable-only govet,staticcheck,errcheck ./...

commit-msg:
  commands:
    conventional-commit:
      run: 'grep -qE "^(feat|fix|refactor|docs|test|chore|build|ci|perf)(\(.+\))?: .+" "$1"'
```

- [ ] **Step 3: Install hooks into .git/hooks**

Run: `lefthook install`
Expected: Output confirms hooks installed.

- [ ] **Step 4: Test pre-commit hook by staging a Go file**

Run: `git add cmd/qlx/main.go && git commit -m "test: verify lefthook pre-commit"`
Expected: Pre-commit hooks execute (gofmt, govet, golangci-lint run on staged .go file). Commit succeeds.

- [ ] **Step 5: Test commit-msg hook with invalid message**

Run: `git commit --allow-empty -m "bad message"`
Expected: FAIL — rejected by conventional commit regex.

- [ ] **Step 6: Test commit-msg hook with valid message**

Run: `git commit --allow-empty -m "test: verify conventional commit hook"`
Expected: PASS — commit created.

- [ ] **Step 7: Clean up test commits**

Run: `git reset HEAD~1` (remove the test commit; the invalid commit-msg test didn't create a commit)

- [ ] **Step 8: Commit lefthook config**

```bash
git add lefthook.yml
git commit -m "build: add lefthook pre-commit and commit-msg hooks"
```

---

### Task 3: Makefile additions

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Add lint, lint-fix, and install-hooks targets to Makefile**

Add after the existing `deps` target. **IMPORTANT: Makefile recipes require a hard tab character for indentation, not spaces. Ensure your editor uses tabs.**

```makefile
lint:
	golangci-lint run ./...

lint-fix:
	golangci-lint run --fix ./...

install-hooks:
	lefthook install
```

Also update `.PHONY` line to include: `lint lint-fix install-hooks`

- [ ] **Step 2: Verify targets work**

Run: `make lint && make install-hooks`
Expected: Both complete successfully.

- [ ] **Step 3: Commit**

```bash
git add Makefile
git commit -m "build: add lint, lint-fix, and install-hooks Makefile targets"
```

---

### Task 4: CI workflow — GitHub Actions

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Create directory**

Run: `mkdir -p .github/workflows`

- [ ] **Step 2: Create `.github/workflows/ci.yml`**

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  lint:
    name: Lint
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.25"
      - uses: golangci/golangci-lint-action@v6
        with:
          version: latest

  test:
    name: Test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.25"
      - name: Run tests
        env:
          CGO_ENABLED: "1"
        run: go test ./... -race -coverprofile=coverage.out -v
      - uses: actions/upload-artifact@v4
        with:
          name: coverage
          path: coverage.out

  build:
    name: Build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.25"
      - name: Build
        run: go build ./cmd/qlx/
```

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add GitHub Actions workflow for lint, test, and build"
```

---

### Task 5: Release workflow — release-please

**Files:**
- Create: `.github/workflows/release.yml`

- [ ] **Step 1: Create `.github/workflows/release.yml`**

```yaml
name: Release

on:
  push:
    branches: [main]

permissions:
  contents: write
  pull-requests: write

jobs:
  release:
    name: Release
    runs-on: ubuntu-latest
    steps:
      - uses: googleapis/release-please-action@v4
        with:
          release-type: go
          token: ${{ secrets.GITHUB_TOKEN }}
```

- [ ] **Step 2: Create `.release-please-manifest.json` for initial version**

```json
{
  ".": "0.1.0"
}
```

- [ ] **Step 3: Create `release-please-config.json`**

```json
{
  "packages": {
    ".": {
      "release-type": "go",
      "changelog-sections": [
        { "type": "feat", "section": "Features" },
        { "type": "fix", "section": "Bug Fixes" },
        { "type": "perf", "section": "Performance" },
        { "type": "refactor", "section": "Miscellaneous" },
        { "type": "docs", "section": "Documentation", "hidden": true },
        { "type": "chore", "section": "Miscellaneous", "hidden": true },
        { "type": "test", "section": "Miscellaneous", "hidden": true },
        { "type": "build", "section": "Miscellaneous", "hidden": true },
        { "type": "ci", "section": "Miscellaneous", "hidden": true }
      ]
    }
  }
}
```

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/release.yml .release-please-manifest.json release-please-config.json
git commit -m "ci: add release-please for automated changelog and versioning"
```

---

### Task 6: Fix existing lint issues (if any)

**Files:**
- Modify: any files flagged by `golangci-lint run`

- [ ] **Step 1: Run full lint**

Run: `golangci-lint run ./...`

- [ ] **Step 2: Fix reported issues**

Fix only real issues. If a finding is a false positive (e.g., build tag related), add a `//nolint` comment with justification.

- [ ] **Step 3: Run lint again to verify clean**

Run: `golangci-lint run ./...`
Expected: No issues reported.

- [ ] **Step 4: Run tests to confirm nothing broke**

Run: `go test ./... -v`
Expected: All tests pass.

- [ ] **Step 5: Commit fixes**

```bash
git add -A
git commit -m "fix: resolve golangci-lint findings"
```

---

### Task 7: Branch protection setup (manual)

This task requires manual GitHub UI configuration after the CI workflow is pushed and runs successfully.

- [ ] **Step 1: Push changes to GitHub and create a PR to verify CI runs**

- [ ] **Step 2: Configure branch protection on `main`**

Go to GitHub → Settings → Branches → Add rule for `main`:
- ✅ Require a pull request before merging
- ✅ Require status checks to pass: `Lint`, `Test`, `Build`
- ✅ Require branches to be up to date before merging
