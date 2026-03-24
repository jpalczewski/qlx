import { test, expect } from '../fixtures/app';

test.describe('Quick Print', () => {
  test.describe.configure({ mode: 'serial' });

  let printerId: string;

  test('setup: create printer via API', async ({ request, app }) => {
    const printerRes = await request.post(`${app.baseURL}/printers`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'E2E Quick Printer', encoder: 'niimbot', model: 'b1', transport: 'remote', address: 'http://localhost:9999' },
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

    const form = page.locator('[data-print-form][data-print-mode="adhoc"]');
    await expect(form.locator('[data-print-text]')).toBeVisible();
    await expect(form.locator('[data-print-printer]')).toBeVisible();
    await expect(form.locator('[data-print-template]')).toBeVisible();
    await expect(form.locator('[data-print-btn]')).toBeVisible();
    await expect(form.locator('[data-print-preview]')).toBeVisible();
  });

  test('print sends request with correct body', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/quick-print`, { waitUntil: 'domcontentloaded' });

    const form = page.locator('[data-print-form][data-print-mode="adhoc"]');
    await form.locator('[data-print-text]').fill('Test label text');
    await form.locator('[data-print-printer]').selectOption(printerId);

    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/adhoc/print') && r.request().method() === 'POST'
    );
    await form.locator('[data-print-btn]').click();
    const response = await responsePromise;

    const body = response.request().postDataJSON();
    expect(body.text).toBe('Test label text');
    expect(body.printer_id).toBe(printerId);
    expect(body.template).toBeTruthy();
    expect(body.print_date).toBe(false);
  });

  test('print with print_date sends correct flag', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/quick-print`, { waitUntil: 'domcontentloaded' });

    const form = page.locator('[data-print-form][data-print-mode="adhoc"]');
    await form.locator('[data-print-text]').fill('Dated label');
    await form.locator('[data-print-date]').check();

    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/adhoc/print') && r.request().method() === 'POST'
    );
    await form.locator('[data-print-btn]').click();
    const response = await responsePromise;

    const body = response.request().postDataJSON();
    expect(body.text).toBe('Dated label');
    expect(body.print_date).toBe(true);
  });

  test('empty text shows validation message', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/quick-print`, { waitUntil: 'domcontentloaded' });

    const form = page.locator('[data-print-form][data-print-mode="adhoc"]');
    await form.locator('[data-print-btn]').click();

    const result = form.locator('[data-print-result]');
    await expect(result).not.toHaveText('');
  });
});
