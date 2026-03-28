import { test, expect } from '../fixtures/app';

test.describe('Print options controls', () => {
  test.describe.configure({ mode: 'serial' });

  let printerId: string;
  let itemId: string;

  test('setup: create printer, container, and item via API', async ({ request, app }) => {
    const printerRes = await request.post(`${app.baseURL}/printers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Options Test Printer', encoder: 'niimbot', model: 'b1', transport: 'remote', address: 'http://localhost:9999' },
    });
    expect(printerRes.ok()).toBeTruthy();
    const printer = await printerRes.json();
    printerId = printer.id;

    const containerRes = await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Options Test Container' },
    });
    expect(containerRes.ok()).toBeTruthy();
    const container = await containerRes.json();

    const itemRes = await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Options Test Item', description: 'For print options test', container_id: container.id },
    });
    expect(itemRes.ok()).toBeTruthy();
    const item = await itemRes.json();
    itemId = item.id;
  });

  test('default state: copies visible, density/cut_every/high_res hidden', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/items/${itemId}`);

    const form = page.locator('[data-print-form][data-print-mode="item"]');
    await expect(form).toBeVisible();

    // copies input is always visible
    await expect(form.locator('[data-copies]')).toBeVisible();

    // optional controls start hidden (display:none set in template)
    await expect(form.locator('[data-density-wrap]')).toBeHidden();
    await expect(form.locator('[data-cut-every-wrap]')).toBeHidden();
    await expect(form.locator('[data-high-res-wrap]')).toBeHidden();
  });

  test('capabilities endpoint is called when printer is selected', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/items/${itemId}`);

    const form = page.locator('[data-print-form][data-print-mode="item"]');
    await expect(form).toBeVisible();

    // Change printer selection to trigger a fresh capabilities fetch
    const capsPromise = page.waitForResponse(r =>
      r.url().includes('/printers/') && r.url().includes('/capabilities')
    );
    await form.locator('[data-print-printer]').selectOption(printerId);
    const capsResponse = await capsPromise;

    // The server may return an error (printer not connected), but the request is made
    expect([200, 404, 500, 503]).toContain(capsResponse.status());
  });

  test('print request body includes copies field', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/items/${itemId}`);

    const form = page.locator('[data-print-form][data-print-mode="item"]');
    await expect(form).toBeVisible();

    // Set copies to 2
    await form.locator('[data-copies]').fill('2');

    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/items/') && r.url().includes('/print') && r.request().method() === 'POST'
    );
    await form.locator('[data-print-btn]').click();
    const response = await responsePromise;

    const body = response.request().postDataJSON();
    expect(body.copies).toBe(2);
  });

  test('print request body includes default print options fields', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/items/${itemId}`);

    const form = page.locator('[data-print-form][data-print-mode="item"]');
    await expect(form).toBeVisible();

    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/items/') && r.url().includes('/print') && r.request().method() === 'POST'
    );
    await form.locator('[data-print-btn]').click();
    const response = await responsePromise;

    const body = response.request().postDataJSON();
    // With optional controls hidden, density/cut_every/high_res default to off values
    expect(body.copies).toBe(1);
    expect(body.density).toBe(0);
    expect(body.cut_every).toBe(0);
    expect(body.high_res).toBe(false);
  });
});
