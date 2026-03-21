import { test, expect } from '../fixtures/app';

test.describe('Print flow', () => {
  test.describe.configure({ mode: 'serial' });

  let containerId: string;
  let itemId: string;

  test('setup: create printer, container, and item via API', async ({ request, app }) => {
    const printerRes = await request.post(`${app.baseURL}/api/printers`, {
      data: { name: 'E2E Printer', encoder: 'niimbot', model: 'b1', transport: 'remote', address: 'http://localhost:9999' },
    });
    expect(printerRes.ok()).toBeTruthy();

    const containerRes = await request.post(`${app.baseURL}/api/containers`, {
      data: { name: 'Print Test Container' },
    });
    expect(containerRes.ok()).toBeTruthy();
    const container = await containerRes.json();
    containerId = container.id;

    const item1Res = await request.post(`${app.baseURL}/api/items`, {
      data: { name: 'Print Item 1', description: 'First', container_id: containerId },
    });
    expect(item1Res.ok()).toBeTruthy();
    const item1 = await item1Res.json();
    itemId = item1.id;

    await request.post(`${app.baseURL}/api/items`, {
      data: { name: 'Print Item 2', description: 'Second', container_id: containerId },
    });
  });

  test('single item print — UI flow', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui/items/${itemId}`, { waitUntil: 'domcontentloaded' });
    await expect(page.locator('h1')).toContainText('Print Item 1');

    await expect(page.locator('#print-printer')).toBeVisible();
    await expect(page.locator('#print-template')).toBeVisible();
    await page.selectOption('#print-template', 'simple');

    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/ui/actions/items/') && r.url().includes('/print')
    );
    await page.click('#print-btn');
    await responsePromise;

    // Print will likely fail (no real printer) — verify result is shown
    const resultText = await page.locator('#print-result').textContent();
    expect(resultText).toBeTruthy();
  });

  test('batch print from container', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui/containers/${containerId}`, { waitUntil: 'domcontentloaded' });
    await expect(page.locator('h2')).toContainText('Print Test Container');

    await expect(page.locator('#container-print-printer')).toBeVisible();
    await page.selectOption('#container-print-template', 'simple');

    await page.click('#container-print-all-btn');

    await expect(page.locator('#container-print-result')).not.toHaveText('');
    await page.waitForFunction(() => {
      const el = document.getElementById('container-print-result');
      return el && (el.textContent?.includes('Wydrukowano') || el.textContent?.includes('Błędy') || el.textContent?.includes('Błąd'));
    }, null, { timeout: 15000 });
  });
});
