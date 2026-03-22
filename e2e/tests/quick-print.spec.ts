import { test, expect } from '../fixtures/app';

test.describe('Quick Print', () => {
  test.describe.configure({ mode: 'serial' });

  let printerId: string;

  test('setup: create printer via API', async ({ request, app }) => {
    const printerRes = await request.post(`${app.baseURL}/printers`, {
      headers: { 'Accept': 'application/json', 'Content-Type': 'application/json' },
      data: { name: 'E2E Quick Printer', encoder: 'niimbot', model: 'b1', transport: 'remote', address: 'http://localhost:9999' },
    });
    expect(printerRes.ok()).toBeTruthy();
    const printer = await printerRes.json();
    printerId = printer.id;
  });

  test('page accessible from nav', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`, { waitUntil: 'domcontentloaded' });

    // Click Quick Print nav link
    await page.click('a[href="/quick-print"]');
    await page.waitForResponse(r => r.url().includes('/quick-print'));

    await expect(page.locator('#adhoc-text')).toBeVisible();
    await expect(page.locator('#adhoc-printer')).toBeVisible();
    await expect(page.locator('#adhoc-template')).toBeVisible();
    await expect(page.locator('#adhoc-print-btn')).toBeVisible();
  });

  test('print sends request with correct body', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/quick-print`, { waitUntil: 'domcontentloaded' });

    await page.fill('#adhoc-text', 'Test label text');
    await page.selectOption('#adhoc-printer', printerId);

    // Pick the first option in the template selector
    const firstOption = await page.locator('#adhoc-template option').first().getAttribute('value');

    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/adhoc/print') && r.request().method() === 'POST'
    );
    await page.click('#adhoc-print-btn');
    const response = await responsePromise;

    const body = response.request().postDataJSON();
    expect(body.text).toBe('Test label text');
    expect(body.printer_id).toBe(printerId);
    expect(body.template).toBeTruthy();
    expect(body.print_date).toBe(false);
  });

  test('print with print_date sends correct flag', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/quick-print`, { waitUntil: 'domcontentloaded' });

    await page.fill('#adhoc-text', 'Dated label');
    await page.check('#adhoc-print-date');

    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/adhoc/print') && r.request().method() === 'POST'
    );
    await page.click('#adhoc-print-btn');
    const response = await responsePromise;

    const body = response.request().postDataJSON();
    expect(body.text).toBe('Dated label');
    expect(body.print_date).toBe(true);
  });

  test('empty text shows validation error', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/quick-print`, { waitUntil: 'domcontentloaded' });

    // Leave textarea empty and click print
    await page.click('#adhoc-print-btn');

    // The JS handler sets result text client-side for empty text
    await expect(page.locator('#adhoc-result')).toHaveText('Please enter text');
  });
});
