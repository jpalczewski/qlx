import { test, expect } from '../fixtures/app';

test.describe('Label preview', () => {
  test.describe.configure({ mode: 'serial' });

  let containerId: string;
  let itemId: string;

  test('setup: create printer, container, and item', async ({ request, app }) => {
    await request.post(`${app.baseURL}/printers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Preview Printer', encoder: 'niimbot', model: 'b1', transport: 'remote', address: 'http://localhost:9999' },
    });

    const containerRes = await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Preview Container', description: 'For preview tests' },
    });
    const container = await containerRes.json();
    containerId = container.id;

    const itemRes = await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Preview Item', description: 'Test item', container_id: containerId },
    });
    const item = await itemRes.json();
    itemId = item.id;
  });

  // --- Item preview ---

  test('item preview dialog opens and shows PNG', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/items/${itemId}`, { waitUntil: 'domcontentloaded' });

    const form = page.locator('[data-print-form][data-print-mode="item"]');
    await form.locator('[data-print-template]').selectOption('simple');

    const previewResponse = page.waitForResponse(r =>
      r.url().includes(`/items/${itemId}/preview`) && r.request().method() === 'GET'
    );
    await form.locator('[data-print-preview]').click();
    await previewResponse;

    const dialog = page.locator('[data-preview-dialog]');
    await expect(dialog).toBeVisible();

    // Should show a preview image (PNG from server)
    const img = dialog.locator('.preview-image');
    await expect(img).toBeVisible({ timeout: 10000 });
  });

  test('preview dialog has dither toggle', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/items/${itemId}`, { waitUntil: 'domcontentloaded' });

    const form = page.locator('[data-print-form][data-print-mode="item"]');
    await form.locator('[data-print-template]').selectOption('simple');

    const previewResponse = page.waitForResponse(r =>
      r.url().includes(`/items/${itemId}/preview`)
    );
    await form.locator('[data-print-preview]').click();
    await previewResponse;

    const dialog = page.locator('[data-preview-dialog]');
    await expect(dialog).toBeVisible();

    // Dither checkbox should be present and unchecked
    const ditherCheckbox = dialog.locator('[data-preview-dither]');
    await expect(ditherCheckbox).toBeVisible();
    await expect(ditherCheckbox).not.toBeChecked();
  });

  test('preview dialog closes on close button', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/items/${itemId}`, { waitUntil: 'domcontentloaded' });

    const form = page.locator('[data-print-form][data-print-mode="item"]');

    const previewResponse = page.waitForResponse(r =>
      r.url().includes(`/items/${itemId}/preview`)
    );
    await form.locator('[data-print-preview]').click();
    await previewResponse;

    const dialog = page.locator('[data-preview-dialog]');
    await expect(dialog).toBeVisible();

    // Close via the X button
    await dialog.locator('.preview-close').first().click();
    await expect(dialog).not.toBeVisible();
  });

  test('preview dialog print button triggers print', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/items/${itemId}`, { waitUntil: 'domcontentloaded' });

    const form = page.locator('[data-print-form][data-print-mode="item"]');
    await form.locator('[data-print-template]').selectOption('simple');

    const previewResponse = page.waitForResponse(r =>
      r.url().includes(`/items/${itemId}/preview`)
    );
    await form.locator('[data-print-preview]').click();
    await previewResponse;

    const dialog = page.locator('[data-preview-dialog]');
    await expect(dialog).toBeVisible();
    await expect(dialog.locator('.preview-image')).toBeVisible({ timeout: 10000 });

    // Click print in dialog — should close dialog and trigger print POST
    const printResponse = page.waitForResponse(r =>
      r.url().includes(`/items/${itemId}/print`) && r.request().method() === 'POST'
    );
    await dialog.locator('[data-preview-print]').click();
    await printResponse;

    // Dialog should be closed after print
    await expect(dialog).not.toBeVisible();

    // Print result should be shown
    const result = form.locator('[data-print-result]');
    await expect(result).not.toHaveText('');
  });

  // --- Container preview ---

  test('container label preview works', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/containers/${containerId}`, { waitUntil: 'domcontentloaded' });

    const form = page.locator('[data-print-form][data-print-mode="container-label"]');
    await form.locator('input[name="print-schema"]').first().check();

    const previewResponse = page.waitForResponse(r =>
      r.url().includes(`/containers/${containerId}/preview`) && r.request().method() === 'GET'
    );
    await form.locator('[data-print-preview]').click();
    await previewResponse;

    const dialog = page.locator('[data-preview-dialog]');
    await expect(dialog).toBeVisible();
    await expect(dialog.locator('.preview-image')).toBeVisible({ timeout: 10000 });
  });

  // --- Preview API endpoints ---

  test('item preview endpoint returns PNG for built-in schema', async ({ request, app }) => {
    const response = await request.get(
      `${app.baseURL}/items/${itemId}/preview?template=simple`,
      { headers: { 'Accept': 'image/png' } }
    );
    expect(response.ok()).toBeTruthy();
    expect(response.headers()['content-type']).toContain('image/png');

    const body = await response.body();
    // PNG magic bytes: 89 50 4E 47
    expect(body[0]).toBe(0x89);
    expect(body[1]).toBe(0x50);
    expect(body[2]).toBe(0x4e);
    expect(body[3]).toBe(0x47);
  });

  test('container preview endpoint returns PNG for built-in schema', async ({ request, app }) => {
    const response = await request.get(
      `${app.baseURL}/containers/${containerId}/preview?template=simple&show_children=true`,
      { headers: { 'Accept': 'image/png' } }
    );
    expect(response.ok()).toBeTruthy();
    expect(response.headers()['content-type']).toContain('image/png');
  });

  test('adhoc preview endpoint returns PNG', async ({ request, app }) => {
    const response = await request.get(
      `${app.baseURL}/adhoc/preview?template=simple&text=Hello+World`,
      { headers: { 'Accept': 'image/png' } }
    );
    expect(response.ok()).toBeTruthy();
    expect(response.headers()['content-type']).toContain('image/png');
  });

  test('preview endpoint returns 400 for missing template', async ({ request, app }) => {
    const response = await request.get(
      `${app.baseURL}/items/${itemId}/preview`,
      { headers: { 'Accept': 'application/json' } }
    );
    expect(response.status()).toBe(400);
  });

  test('adhoc preview returns 400 for missing text', async ({ request, app }) => {
    const response = await request.get(
      `${app.baseURL}/adhoc/preview?template=simple`,
      { headers: { 'Accept': 'application/json' } }
    );
    expect(response.status()).toBe(400);
  });
});
