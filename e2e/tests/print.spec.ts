import { test, expect } from '../fixtures/app';

test.describe('Print flow', () => {
  test.describe.configure({ mode: 'serial' });

  let containerId: string;
  let itemId: string;

  test('setup: create printer, container, and item via API', async ({ request, app }) => {
    const printerRes = await request.post(`${app.baseURL}/printers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'E2E Printer', encoder: 'niimbot', model: 'b1', transport: 'remote', address: 'http://localhost:9999' },
    });
    expect(printerRes.ok()).toBeTruthy();

    const containerRes = await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Print Test Container' },
    });
    expect(containerRes.ok()).toBeTruthy();
    const container = await containerRes.json();
    containerId = container.id;

    const item1Res = await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Print Item 1', description: 'First', container_id: containerId },
    });
    expect(item1Res.ok()).toBeTruthy();
    const item1 = await item1Res.json();
    itemId = item1.id;

    await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Print Item 2', description: 'Second', container_id: containerId },
    });
  });

  test('item print form renders with data-attribute selectors', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/items/${itemId}`, { waitUntil: 'domcontentloaded' });
    await expect(page.locator('h1')).toContainText('Print Item 1');

    const form = page.locator('[data-print-form][data-print-mode="item"]');
    await expect(form).toBeVisible();
    await expect(form.locator('[data-print-printer]')).toBeVisible();
    await expect(form.locator('[data-print-template]')).toBeVisible();
    await expect(form.locator('[data-print-btn]')).toBeVisible();
    await expect(form.locator('[data-print-preview]')).toBeVisible();
  });

  test('single item print — UI flow', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/items/${itemId}`, { waitUntil: 'domcontentloaded' });

    const form = page.locator('[data-print-form][data-print-mode="item"]');
    await form.locator('[data-print-template]').selectOption('simple');

    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/items/') && r.url().includes('/print')
    );
    await form.locator('[data-print-btn]').click();
    await responsePromise;

    // Print will likely fail (no real printer) — verify result is shown
    const resultText = await form.locator('[data-print-result]').textContent();
    expect(resultText).toBeTruthy();
  });

  test('batch print from container', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/containers/${containerId}`, { waitUntil: 'domcontentloaded' });
    await expect(page.locator('h2')).toContainText('Print Test Container');

    const panel = page.locator('.print-panel');
    // Switch to bulk tab
    await panel.locator('.print-tab[data-tab="bulk-items"]').click();

    const bulkTab = panel.locator('[data-tab-content="bulk-items"]');
    await expect(bulkTab).toBeVisible();
    await bulkTab.locator('[data-print-template]').selectOption('simple');

    await bulkTab.locator('[data-print-btn]').click();

    await expect(bulkTab.locator('[data-print-result]')).not.toHaveText('');
    await page.waitForFunction(() => {
      const el = document.querySelector('[data-tab-content="bulk-items"] [data-print-result]');
      return el && el.textContent && el.textContent.length > 0;
    }, null, { timeout: 15000 });
  });
});
