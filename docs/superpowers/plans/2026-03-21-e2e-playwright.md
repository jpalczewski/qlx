# E2E Playwright Tests — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a self-contained Playwright E2E test suite in `e2e/` that tests all QLX UI flows against a real server instance.

**Architecture:** Isolated Node.js project in `e2e/` with custom Playwright fixtures that build the Go binary and start a fresh QLX server per test suite. Tests use HTMX-aware waiting patterns (waitForResponse + DOM assertions). BLE scan tested via API mocking route interception.

**Tech Stack:** Playwright Test (TypeScript), Node.js 20+, Go (build target)

---

### Task 1: Project scaffold — package.json, tsconfig, playwright config

**Files:**
- Create: `e2e/package.json`
- Create: `e2e/tsconfig.json`
- Create: `e2e/playwright.config.ts`
- Create: `e2e/.gitignore`

- [ ] **Step 1: Create `e2e/package.json`**

```json
{
  "name": "qlx-e2e",
  "private": true,
  "scripts": {
    "test": "playwright test",
    "test:ui": "playwright test --ui",
    "test:headed": "playwright test --headed"
  },
  "devDependencies": {
    "@playwright/test": "^1.50.0"
  }
}
```

- [ ] **Step 2: Create `e2e/tsconfig.json`**

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "module": "commonjs",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true,
    "resolveJsonModule": true,
    "outDir": "dist",
    "rootDir": "."
  },
  "include": ["tests/**/*.ts", "fixtures/**/*.ts", "playwright.config.ts"]
}
```

- [ ] **Step 3: Create `e2e/playwright.config.ts`**

```typescript
import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  fullyParallel: false, // tests within suite are serial (CRUD flows)
  retries: 0,
  reporter: [['html', { open: 'never' }], ['list']],
  use: {
    // No baseURL here — custom fixture provides app.baseURL per worker
    trace: 'on-first-retry',
  },
  projects: [
    {
      name: 'chromium',
      use: { browserName: 'chromium' },
    },
  ],
});
```

Note: We do NOT use Playwright's `webServer` option — the custom fixture in Task 2 handles server lifecycle for fresh data dirs.

- [ ] **Step 4: Create `e2e/.gitignore`**

```
node_modules/
dist/
test-results/
playwright-report/
blob-report/
```

- [ ] **Step 5: Install dependencies**

Run: `cd e2e && npm install`
Expected: `node_modules/` created, `package-lock.json` generated

- [ ] **Step 6: Install Playwright browsers**

Run: `cd e2e && npx playwright install chromium`
Expected: Chromium browser downloaded

- [ ] **Step 7: Commit**

```bash
git add e2e/package.json e2e/package-lock.json e2e/tsconfig.json e2e/playwright.config.ts e2e/.gitignore
git commit -m "feat(e2e): scaffold playwright project with config"
```

---

### Task 2: Custom app fixture — build, start server, fresh data dir

**Files:**
- Create: `e2e/fixtures/app.ts`

- [ ] **Step 1: Create `e2e/fixtures/app.ts`**

This fixture:
1. Builds the Go binary once (shared across all tests)
2. Per-worker: creates a temp data dir, starts QLX on a unique port, tears down after

```typescript
import { test as base, expect } from '@playwright/test';
import { execFileSync, spawn, ChildProcess } from 'child_process';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import * as net from 'net';

const PROJECT_ROOT = path.resolve(__dirname, '../..');
const BINARY_PATH = path.join(PROJECT_ROOT, 'qlx-e2e-test');

async function getAvailablePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const srv = net.createServer();
    srv.listen(0, () => {
      const port = (srv.address() as net.AddressInfo).port;
      srv.close(() => resolve(port));
    });
    srv.on('error', reject);
  });
}

function buildBinary() {
  if (!fs.existsSync(BINARY_PATH)) {
    execFileSync('go', ['build', '-o', BINARY_PATH, './cmd/qlx/'], {
      cwd: PROJECT_ROOT,
      stdio: 'inherit',
    });
  }
}

type AppFixtures = {
  app: { baseURL: string; port: number; dataDir: string };
};

export const test = base.extend<AppFixtures>({
  app: [async ({}, use) => {
    buildBinary();

    const port = await getAvailablePort();
    const dataDir = fs.mkdtempSync(path.join(os.tmpdir(), 'qlx-e2e-'));

    const proc: ChildProcess = spawn(BINARY_PATH, [
      '--port', String(port),
      '--host', '127.0.0.1',
      '--data', dataDir,
    ], {
      cwd: PROJECT_ROOT,
      stdio: 'pipe',
    });

    // Wait for server to be ready
    const startTime = Date.now();
    const timeout = 10_000;
    let ready = false;
    while (Date.now() - startTime < timeout) {
      try {
        const res = await fetch(`http://127.0.0.1:${port}/ui`);
        if (res.ok) { ready = true; break; }
      } catch {
        // server not ready yet
      }
      await new Promise(r => setTimeout(r, 100));
    }
    if (!ready) throw new Error(`QLX server failed to start on port ${port}`);

    await use({ baseURL: `http://127.0.0.1:${port}`, port, dataDir });

    // Teardown
    proc.kill('SIGTERM');
    await new Promise<void>((resolve) => {
      proc.on('exit', () => resolve());
      setTimeout(() => { proc.kill('SIGKILL'); resolve(); }, 3000);
    });
    fs.rmSync(dataDir, { recursive: true, force: true });
  }, { scope: 'worker' }],
});

export { expect };
```

- [ ] **Step 2: Verify fixture compiles**

Run: `cd e2e && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add e2e/fixtures/app.ts
git commit -m "feat(e2e): add custom app fixture with fresh data dir per worker"
```

---

### Task 3: Inventory E2E tests

**Files:**
- Create: `e2e/tests/inventory.spec.ts`

- [ ] **Step 1: Write inventory test file**

```typescript
import { test, expect } from '../fixtures/app';

test.describe('Inventory management', () => {
  test.describe.configure({ mode: 'serial' });

  let containerName: string;
  let subContainerName: string;
  let itemName: string;

  test('create root container', async ({ page, app }) => {
    containerName = `Test Container ${Date.now()}`;
    await page.goto(`${app.baseURL}/ui`);
    await expect(page.locator('h1')).toContainText('Kontenery');

    // Open "Dodaj kontener" form
    await page.click('summary:has-text("Dodaj kontener")');
    await page.fill('input[name="name"]', containerName);
    await page.fill('textarea[name="description"]', 'E2E test container');

    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/ui/actions/containers') && r.request().method() === 'POST'
    );
    await page.click('button:has-text("Utwórz")');
    await responsePromise;

    await expect(page.locator('.container-list')).toContainText(containerName);
  });

  test('navigate into container', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui`);
    await page.click(`.container-item:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);
  });

  test('create sub-container', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui`);
    await page.click(`.container-item:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);

    subContainerName = `Sub ${Date.now()}`;
    await page.click('summary:has-text("Dodaj kontener")');
    await page.fill('input[name="name"]', subContainerName);

    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/ui/actions/containers') && r.request().method() === 'POST'
    );
    await page.click('button:has-text("Utwórz")');
    await responsePromise;

    await expect(page.locator('.container-list')).toContainText(subContainerName);
  });

  test('create item in container', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui`);
    await page.click(`.container-item:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);

    itemName = `Item ${Date.now()}`;
    await page.click('summary:has-text("Dodaj przedmiot")');
    await page.fill('input[name="name"]', itemName);
    await page.fill('textarea[name="description"]', 'E2E test item');

    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/ui/actions/items') && r.request().method() === 'POST'
    );
    await page.click('form:has(input[name="container_id"]) button:has-text("Utwórz")');
    await responsePromise;

    await expect(page.locator('.item-list')).toContainText(itemName);
  });

  test('navigate to item detail', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui`);
    await page.click(`.container-item:has-text("${containerName}")`);
    await page.click(`.item-item:has-text("${itemName}")`);
    await expect(page.locator('h1')).toContainText(itemName);
    await expect(page.locator('.description')).toContainText('E2E test item');
  });

  test('navigate via breadcrumbs', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui`);
    await page.click(`.container-item:has-text("${containerName}")`);
    await page.click(`.item-item:has-text("${itemName}")`);

    // Click breadcrumb back to container
    await page.click(`.breadcrumb a:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);
  });

  test('attempt delete non-empty container shows error', async ({ page, app }) => {
    // Container has sub-container and item, so delete should fail
    await page.goto(`${app.baseURL}/ui`);
    await page.click(`.container-item:has-text("${containerName}")`);

    // The delete button should not be visible when container has children
    await expect(page.locator('button:has-text("Usuń kontener")')).not.toBeVisible();
  });

  test('delete item', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui`);
    await page.click(`.container-item:has-text("${containerName}")`);
    await page.click(`.item-item:has-text("${itemName}")`);

    page.on('dialog', dialog => dialog.accept());
    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/ui/actions/items/') && r.request().method() === 'DELETE'
    );
    await page.click('button:has-text("Usuń")');
    await responsePromise;

    // Should redirect back to container, item should be gone
    await expect(page.locator('.item-list')).not.toContainText(itemName);
  });

  test('delete empty sub-container', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui`);
    await page.click(`.container-item:has-text("${containerName}")`);
    await page.click(`.container-item:has-text("${subContainerName}")`);

    page.on('dialog', dialog => dialog.accept());
    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/ui/actions/containers/') && r.request().method() === 'DELETE'
    );
    await page.click('button:has-text("Usuń kontener")');
    await responsePromise;
  });
});
```

- [ ] **Step 2: Run the test**

Run: `cd e2e && npx playwright test tests/inventory.spec.ts --headed`
Expected: All tests pass, containers and items created/deleted in the UI

- [ ] **Step 3: Commit**

```bash
git add e2e/tests/inventory.spec.ts
git commit -m "feat(e2e): add inventory management tests"
```

---

### Task 4: Printers & BLE scan E2E tests

**Files:**
- Create: `e2e/tests/printers.spec.ts`

- [ ] **Step 1: Write printers test file**

```typescript
import { test, expect } from '../fixtures/app';

test.describe('Printer management', () => {
  test.describe.configure({ mode: 'serial' });

  const printerName = `Test Printer ${Date.now()}`;

  test('shows empty printer list', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui/printers`);
    await expect(page.locator('h1')).toContainText('Drukarki');
    await expect(page.locator('.empty')).toContainText('Brak skonfigurowanych drukarek');
  });

  test('add printer via form', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui/printers`);

    await page.click('summary:has-text("Dodaj drukarkę")');
    await page.fill('#name', printerName);
    await page.selectOption('#encoder', 'niimbot');
    await page.selectOption('#transport', 'ble');
    await page.fill('#address', 'AA:BB:CC:DD:EE:FF');

    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/ui/actions/printers') && r.request().method() === 'POST'
    );
    await page.click('button:has-text("Dodaj")');
    await responsePromise;

    await expect(page.locator('.printer-card .name')).toContainText(printerName);
  });

  test('BLE scan button triggers request', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui/printers`);

    // Intercept BLE scan API — may not be available (no ble build tag)
    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/api/bluetooth/scan')
    );

    await page.click('button:has-text("Skanuj Bluetooth")');
    await expect(page.locator('#ble-results')).toContainText('Skanowanie');

    const response = await responsePromise;
    // On non-BLE builds, scan returns 404 or error — test that UI handles it gracefully
    if (!response.ok()) {
      await expect(page.locator('#ble-results')).toContainText('Błąd');
    }
  });

  test('BLE scan with mocked results populates form', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui/printers`);

    // Mock the BLE scan endpoint to return devices
    await page.route('**/api/bluetooth/scan', async route => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([
          { name: 'Niimbot B1', address: '11:22:33:44:55:66', rssi: -55 },
        ]),
      });
    });

    await page.click('button:has-text("Skanuj Bluetooth")');
    await expect(page.locator('#ble-results')).toContainText('Niimbot B1');

    // Click scan result to fill form
    await page.click('#ble-results .container-item:has-text("Niimbot B1")');

    // Verify form was filled
    await expect(page.locator('#address')).toHaveValue('11:22:33:44:55:66');
    await expect(page.locator('#transport')).toHaveValue('ble');
  });

  test('delete printer', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui/printers`);

    page.on('dialog', dialog => dialog.accept());
    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/ui/actions/printers/') && r.request().method() === 'DELETE'
    );
    await page.click(`.printer-card:has-text("${printerName}") button:has-text("Usuń")`);
    await responsePromise;

    await expect(page.locator('.empty')).toContainText('Brak skonfigurowanych drukarek');
  });
});
```

- [ ] **Step 2: Run the test**

Run: `cd e2e && npx playwright test tests/printers.spec.ts --headed`
Expected: All tests pass. BLE scan mock test verifies form auto-fill works.

- [ ] **Step 3: Commit**

```bash
git add e2e/tests/printers.spec.ts
git commit -m "feat(e2e): add printer management and BLE scan tests"
```

---

### Task 5: Templates E2E tests

**Files:**
- Create: `e2e/tests/templates.spec.ts`

- [ ] **Step 1: Write templates test file**

```typescript
import { test, expect } from '../fixtures/app';

test.describe('Template management', () => {
  test.describe.configure({ mode: 'serial' });

  const templateName = `Test Template ${Date.now()}`;
  const tagName = 'e2e-test';

  test('shows empty template list', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui/templates`);
    await expect(page.locator('h1')).toContainText('Szablony');
    await expect(page.locator('.empty')).toContainText('Brak szablonów');
  });

  test('navigate to template designer', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui/templates`);
    await page.click('a:has-text("Nowy szablon")');

    // Template designer page should load with canvas
    await expect(page.locator('#template-name, input[name="name"]')).toBeVisible();
  });

  test('create template via designer', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui/templates/new`);

    // Fill template metadata
    await page.fill('#template-name', templateName);
    await page.fill('#template-tags', tagName);

    // Wait for Fabric.js canvas to initialize
    await page.waitForFunction(() => typeof (window as any).fabric !== 'undefined');

    // Add a text element via toolbar
    await page.click('#tool-text, button:has-text("Tekst"), [data-tool="text"]');

    // Save template
    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/ui/actions/templates') && r.request().method() === 'POST'
    );
    await page.click('#save-btn, button:has-text("Zapisz")');
    const response = await responsePromise;
    expect(response.ok()).toBeTruthy();
  });

  test('template appears in list', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui/templates`);
    await expect(page.locator('.template-card .name')).toContainText(templateName);
  });

  test('filter templates by tag', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui/templates`);

    // Click tag to filter
    const tagLink = page.locator(`.tag:has-text("${tagName}")`).first();
    if (await tagLink.isVisible()) {
      await tagLink.click();
      await expect(page.locator('.template-card .name')).toContainText(templateName);
    }
  });

  test('edit existing template', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui/templates`);

    // Click edit on the template
    await page.click(`.template-card:has-text("${templateName}") a:has-text("Edytuj")`);

    // Should load designer with existing template data
    await expect(page.locator('#template-name')).toHaveValue(templateName);

    // Change the name
    const updatedName = templateName + ' Edited';
    await page.fill('#template-name', updatedName);

    // Save
    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/ui/actions/templates/') && r.request().method() === 'PUT'
    );
    await page.click('#save-btn, button:has-text("Zapisz")');
    const response = await responsePromise;
    expect(response.ok()).toBeTruthy();

    // Verify in list
    await page.goto(`${app.baseURL}/ui/templates`);
    await expect(page.locator('.template-card .name')).toContainText('Edited');
  });

  test('delete template', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui/templates`);

    page.on('dialog', dialog => dialog.accept());
    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/ui/actions/templates/') && r.request().method() === 'DELETE'
    );
    await page.click(`.template-card button:has-text("Usuń")`);
    await responsePromise;

    await expect(page.locator('.empty')).toContainText('Brak szablonów');
  });
});
```

- [ ] **Step 2: Run the test**

Run: `cd e2e && npx playwright test tests/templates.spec.ts --headed`
Expected: All tests pass

- [ ] **Step 3: Commit**

```bash
git add e2e/tests/templates.spec.ts
git commit -m "feat(e2e): add template management tests"
```

---

### Task 6: Print flow E2E tests

**Files:**
- Create: `e2e/tests/print.spec.ts`

- [ ] **Step 1: Write print flow test file**

Tests require setup data (printer + container + item). We create them via API calls in a setup test.

```typescript
import { test, expect } from '../fixtures/app';

test.describe('Print flow', () => {
  test.describe.configure({ mode: 'serial' });

  let containerId: string;
  let itemId: string;
  let printerId: string;

  test('setup: create printer, container, and item via API', async ({ request, app }) => {
    // Create printer
    const printerRes = await request.post(`${app.baseURL}/api/printers`, {
      data: { name: 'E2E Printer', encoder: 'niimbot', model: 'b1', transport: 'remote', address: 'http://localhost:9999' },
    });
    expect(printerRes.ok()).toBeTruthy();
    const printer = await printerRes.json();
    printerId = printer.id;

    // Create container
    const containerRes = await request.post(`${app.baseURL}/api/containers`, {
      data: { name: 'Print Test Container' },
    });
    expect(containerRes.ok()).toBeTruthy();
    const container = await containerRes.json();
    containerId = container.id;

    // Create items
    const item1Res = await request.post(`${app.baseURL}/api/items`, {
      data: { name: 'Print Item 1', description: 'First', container_id: containerId },
    });
    expect(item1Res.ok()).toBeTruthy();
    const item1 = await item1Res.json();
    itemId = item1.id;

    const item2Res = await request.post(`${app.baseURL}/api/items`, {
      data: { name: 'Print Item 2', description: 'Second', container_id: containerId },
    });
    expect(item2Res.ok()).toBeTruthy();
  });

  test('single item print — UI flow', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui/items/${itemId}`);
    await expect(page.locator('h1')).toContainText('Print Item 1');

    // Printer and template selectors should be visible
    await expect(page.locator('#print-printer')).toBeVisible();
    await expect(page.locator('#print-template')).toBeVisible();

    // Select legacy template
    await page.selectOption('#print-template', 'simple');

    // Click print and capture API request
    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/ui/actions/items/') && r.url().includes('/print')
    );
    await page.click('#print-btn');
    const response = await responsePromise;

    // Print will likely fail (no real printer) — verify error is shown gracefully
    const resultText = await page.locator('#print-result').textContent();
    expect(resultText).toBeTruthy(); // either success or error, not empty
  });

  test('batch print from container', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui/containers/${containerId}`);
    await expect(page.locator('h2')).toContainText('Print Test Container');

    // Select printer and template for batch print
    await expect(page.locator('#container-print-printer')).toBeVisible();
    await page.selectOption('#container-print-template', 'simple');

    // Click batch print button
    await page.click('#container-print-all-btn');

    // Wait for batch print to complete (fetches items, then prints each)
    await expect(page.locator('#container-print-result')).not.toHaveText('');
    // Should show result (success or error count)
    await page.waitForFunction(() => {
      const el = document.getElementById('container-print-result');
      return el && (el.textContent?.includes('Wydrukowano') || el.textContent?.includes('Błędy') || el.textContent?.includes('Błąd'));
    }, null, { timeout: 15000 });
  });
});
```

- [ ] **Step 2: Run the test**

Run: `cd e2e && npx playwright test tests/print.spec.ts --headed`
Expected: Tests pass — print attempts are made, errors handled gracefully

- [ ] **Step 3: Commit**

```bash
git add e2e/tests/print.spec.ts
git commit -m "feat(e2e): add print flow tests (single and batch)"
```

---

### Task 7: Export E2E tests

**Files:**
- Create: `e2e/tests/export.spec.ts`

- [ ] **Step 1: Write export test file**

```typescript
import { test, expect } from '../fixtures/app';

test.describe('Data export', () => {
  test.describe.configure({ mode: 'serial' });

  test('setup: create test data', async ({ request, app }) => {
    const containerRes = await request.post(`${app.baseURL}/api/containers`, {
      data: { name: 'Export Container' },
    });
    const container = await containerRes.json();

    await request.post(`${app.baseURL}/api/items`, {
      data: { name: 'Export Item 1', description: 'Desc 1', container_id: container.id },
    });
    await request.post(`${app.baseURL}/api/items`, {
      data: { name: 'Export Item 2', description: 'Desc 2', container_id: container.id },
    });
  });

  test('export JSON contains containers and items', async ({ request, app }) => {
    const response = await request.get(`${app.baseURL}/api/export/json`);
    expect(response.ok()).toBeTruthy();

    const data = await response.json();
    expect(data.containers).toBeDefined();
    expect(data.items).toBeDefined();
    expect(data.containers.length).toBeGreaterThanOrEqual(1);
    expect(data.items.length).toBeGreaterThanOrEqual(2);

    const containerNames = data.containers.map((c: any) => c.name);
    expect(containerNames).toContain('Export Container');
  });

  test('export CSV has correct headers and rows', async ({ request, app }) => {
    const response = await request.get(`${app.baseURL}/api/export/csv`);
    expect(response.ok()).toBeTruthy();

    const csv = await response.text();
    const lines = csv.trim().split('\n');

    // Header line
    expect(lines[0]).toContain('id');
    expect(lines[0]).toContain('name');

    // At least 2 data rows
    expect(lines.length).toBeGreaterThanOrEqual(3);

    // Check content
    expect(csv).toContain('Export Item 1');
    expect(csv).toContain('Export Item 2');
  });
});
```

- [ ] **Step 2: Run the test**

Run: `cd e2e && npx playwright test tests/export.spec.ts --headed`
Expected: All tests pass

- [ ] **Step 3: Commit**

```bash
git add e2e/tests/export.spec.ts
git commit -m "feat(e2e): add JSON and CSV export tests"
```

---

### Task 8: README with setup instructions and later steps

**Files:**
- Create: `e2e/README.md`

- [ ] **Step 1: Write `e2e/README.md`**

Content should include:

**Sections:**
- Prerequisites (Node.js 20+, Go 1.22+)
- Setup (`npm install`, `npx playwright install chromium`)
- Running tests (`npm test`, `npm run test:headed`, `npm run test:ui`, `make test-e2e`)
- How it works (custom fixture builds Go binary, starts fresh server per worker, clean temp data dir)
- Test file table (inventory, templates, printers, print, export)
- **Later Steps** checklist:
  - [ ] Drag & drop tests for container/item reordering
  - [ ] Asset upload tests in template designer
  - [ ] SSE printer events testing
  - [ ] Visual regression tests (screenshot comparison)
  - [ ] CI/CD pipeline integration (GitHub Actions)
  - [ ] Multi-browser testing (Firefox, WebKit)
  - [ ] Mobile viewport testing
  - [ ] Performance/load testing
  - [ ] Accessibility (a11y) audit tests with axe-core
  - [ ] Template designer canvas interaction tests
  - [ ] Inline edit flows for container/item names
  - [ ] Move operations (items between containers, container reparenting)

- [ ] **Step 2: Commit**

```bash
git add e2e/README.md
git commit -m "docs(e2e): add README with setup instructions and later steps"
```

---

### Task 9: Makefile integration

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Add e2e targets to Makefile**

Add `test-e2e test-e2e-ui` to the `.PHONY` line. Add targets and update `clean`:

```makefile
test-e2e:
	cd e2e && npx playwright test

test-e2e-ui:
	cd e2e && npx playwright test --ui
```

Also update the `clean` target to include: `rm -f qlx-e2e-test`

- [ ] **Step 2: Run `make test-e2e` from project root**

Run: `make test-e2e`
Expected: All E2E tests run and pass

- [ ] **Step 3: Commit**

```bash
git add Makefile
git commit -m "build: add test-e2e and test-e2e-ui Makefile targets"
```

---

### Task 10: Add `qlx-e2e-test` to .gitignore

**Files:**
- Modify: `.gitignore` (root)

- [ ] **Step 1: Add binary to .gitignore**

Add `qlx-e2e-test` to the root `.gitignore` file.

- [ ] **Step 2: Commit**

```bash
git add .gitignore
git commit -m "chore: ignore E2E test binary"
```
