# QLX E2E Tests

End-to-end tests for QLX using [Playwright](https://playwright.dev/).

## Prerequisites

- Node.js 20+
- Go 1.22+ (to build the QLX binary)

## Setup

```bash
cd e2e
npm install
npx playwright install chromium
```

## Running Tests

```bash
# All tests (headless)
npm test

# With browser visible
npm run test:headed

# Interactive UI mode
npm run test:ui

# Single test file
npx playwright test tests/inventory.spec.ts

# From project root
make test-e2e
```

## How It Works

Tests use a custom Playwright fixture (`fixtures/app.ts`) that:
1. Builds the Go binary (`qlx-e2e-test`) once
2. Starts a fresh QLX instance per worker on a random port
3. Each instance gets a clean temporary data directory
4. Server is killed and data cleaned up after tests

## Test Files

| File | Coverage |
|------|----------|
| `inventory.spec.ts` | Container/item CRUD, navigation, breadcrumbs |
| `templates.spec.ts` | Template CRUD, designer, tag filtering, edit |
| `printers.spec.ts` | Printer CRUD, BLE scan (real + mocked) |
| `print.spec.ts` | Single item print, batch print from container |
| `export.spec.ts` | JSON and CSV export verification |

## Later Steps

These are planned improvements not yet implemented:

- [ ] **Edit container name inline** -- inline editing of container names
- [ ] **Edit item name and description** -- inline editing of item fields
- [ ] **Drag & drop tests** -- container/item reordering via drag
- [ ] **Asset upload tests** -- image upload in template designer
- [ ] **SSE printer events** -- test real-time printer status updates
- [ ] **Visual regression** -- screenshot comparison for UI consistency
- [ ] **CI/CD pipeline** -- GitHub Actions workflow for automated E2E on PR
- [ ] **Multi-browser** -- add Firefox and WebKit to `playwright.config.ts`
- [ ] **Mobile viewports** -- test responsive layout on phone/tablet sizes
- [ ] **Performance testing** -- measure page load times, HTMX response times
- [ ] **Accessibility (a11y)** -- automated accessibility audits with axe-core
- [ ] **Template designer interaction** -- canvas element manipulation, drag on canvas
- [ ] **Move operations** -- move items between containers, move containers
