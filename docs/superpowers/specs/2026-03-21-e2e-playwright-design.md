# E2E Playwright Tests — Design Spec

## Overview

Add scripted Playwright E2E test suite for QLX in a self-contained `e2e/` directory. Tests run against a real QLX server instance with a fresh data directory per test suite. Node.js Playwright (TypeScript).

## Structure

```
e2e/
  package.json
  playwright.config.ts
  tsconfig.json
  README.md                 # setup instructions + later steps
  tests/
    inventory.spec.ts
    templates.spec.ts
    printers.spec.ts
    print.spec.ts
    export.spec.ts
  fixtures/
    app.ts                  # custom fixture: builds & starts QLX, fresh data dir
```

## Key Decisions

### 1. Isolated `e2e/` directory
Separate Node.js project — doesn't pollute Go root with `node_modules` or `package.json`. Own `tsconfig.json` and `playwright.config.ts`.

### 2. Custom app fixture manages server lifecycle
A custom Playwright fixture (`fixtures/app.ts`) handles the full lifecycle:
- Builds the Go binary once (shared across workers)
- Per-worker: creates a temp data dir, starts QLX on a random port
- Waits for server readiness before tests
- Kills server and cleans up data dir after tests

We do NOT use Playwright's `webServer` config option — it doesn't support per-worker fresh data directories.

### 3. Fresh data per test suite
Each test file gets a clean temporary data directory. No shared state between test suites. Within a suite, tests run serially and can build on each other (create → read → update → delete).

### 4. BLE scan testing
Test the BLE scan UI flow — button click triggers scan, results populate the printer form. On machines without BLE (CI), the scan endpoint returns an error which the UI should handle gracefully. Test both paths: scan success (when available) and scan failure/empty.

### 5. HTMX-aware waiting
Tests wait for HTMX responses using `page.waitForResponse()` combined with DOM selector assertions, since HTMX swaps content without full page navigation.

### 6. No printer hardware required
Print flow tests verify the UI submission path (select printer, select template, click print). Actual printing requires hardware — tests assert the request was made and handle the expected error response.

## Test Scenarios

### `inventory.spec.ts`
- Create root container via form
- Navigate into container
- Create sub-container
- Create item in container
- Navigate to item detail
- Navigate via breadcrumbs
- Attempt delete non-empty container → button not visible
- Delete item
- Delete empty container

### `templates.spec.ts`
- List templates (empty state)
- Create new template via designer (name, tags, dimensions, add text element)
- Save template
- Template appears in list
- Filter templates by tag click
- Edit existing template
- Delete template

### `printers.spec.ts`
- List printers (empty state)
- Add printer via form (manual entry)
- Printer appears in list
- BLE scan button triggers scan request
- BLE scan results populate form (or graceful error on no-BLE)
- Delete printer

### `print.spec.ts`
- Navigate to item → select printer → select template → click print
- Batch print: navigate to container → click "print all" → verify request sequence
- Handle print error gracefully (no actual printer connected)

### `export.spec.ts`
- Create test data (containers + items)
- Export JSON → verify structure and content
- Export CSV → verify headers and rows

## Later Steps (not in this implementation)

Documented in `e2e/README.md`:
- Edit container name inline
- Edit item name and description
- Drag & drop tests for container/item reordering
- Asset upload tests in template designer
- SSE printer events testing
- Visual regression tests (screenshot comparison)
- CI/CD pipeline integration (GitHub Actions)
- Parallel test execution across browsers
- Mobile viewport testing
- Performance/load testing
- Accessibility (a11y) audit tests

## Makefile Integration

```makefile
test-e2e:
	cd e2e && npx playwright test

test-e2e-ui:
	cd e2e && npx playwright test --ui
```
