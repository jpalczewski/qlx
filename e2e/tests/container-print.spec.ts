import { test, expect } from '../fixtures/app';

test.describe('Container print', () => {
  test.describe.configure({ mode: 'serial' });

  let containerId: string;
  let printerId: string;

  test('setup: create printer, container with items and tags', async ({ request, app }) => {
    const printerRes = await request.post(`${app.baseURL}/printers`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'E2E Print Printer', encoder: 'niimbot', model: 'b1', transport: 'remote', address: 'http://localhost:9999' },
    });
    expect(printerRes.ok()).toBeTruthy();
    const printer = await printerRes.json();
    printerId = printer.id;

    const tagRes = await request.post(`${app.baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'print-test-tag', color: 'red' },
    });
    expect(tagRes.ok()).toBeTruthy();
    const tag = await tagRes.json();

    const containerRes = await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'Print Container', description: 'Container for print tests' },
    });
    expect(containerRes.ok()).toBeTruthy();
    const container = await containerRes.json();
    containerId = container.id;

    // Assign tag to container
    await request.post(`${app.baseURL}/containers/${containerId}/tags`, {
      headers: { 'Accept': 'application/json' },
      form: { tag_id: tag.id },
    });

    // Create items inside container
    const item1Res = await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'Print Item A', description: 'First item', container_id: containerId },
    });
    expect(item1Res.ok()).toBeTruthy();

    const item2Res = await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'Print Item B', description: 'Second item', container_id: containerId },
    });
    expect(item2Res.ok()).toBeTruthy();
  });

  test('print panel with shared printer and tabs visible', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/containers/${containerId}`, { waitUntil: 'domcontentloaded' });

    const panel = page.locator('.print-panel');
    await expect(panel).toBeVisible();

    // Single shared printer select
    await expect(panel.locator('[data-panel-printer]')).toBeVisible();

    // Two tabs
    await expect(panel.locator('.print-tab')).toHaveCount(2);

    // Container label tab is active by default
    const containerTab = panel.locator('[data-tab-content="container-label"]');
    await expect(containerTab).toBeVisible();
    await expect(containerTab.locator('[data-print-checkboxes]')).toBeVisible();
    await expect(containerTab.locator('[data-print-date]')).toBeVisible();
    await expect(containerTab.locator('[data-print-children]')).toBeVisible();
  });

  test('switching to bulk tab shows template select', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/containers/${containerId}`, { waitUntil: 'domcontentloaded' });

    const panel = page.locator('.print-panel');

    // Click bulk tab
    await panel.locator('.print-tab[data-tab="bulk-items"]').click();

    // Container tab hidden, bulk tab shown
    await expect(panel.locator('[data-tab-content="container-label"]')).not.toBeVisible();
    const bulkTab = panel.locator('[data-tab-content="bulk-items"]');
    await expect(bulkTab).toBeVisible();
    await expect(bulkTab.locator('[data-print-template]')).toBeVisible();
    await expect(bulkTab.locator('[data-print-btn]')).toBeVisible();
  });

  test('container print sends request with correct body', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/containers/${containerId}`, { waitUntil: 'domcontentloaded' });

    const panel = page.locator('.print-panel');
    const containerTab = panel.locator('[data-tab-content="container-label"]');

    // Uncheck default-checked boxes
    await containerTab.locator('[data-print-date]').uncheck();
    await containerTab.locator('[data-print-children]').uncheck();

    // Select the first schema checkbox
    const firstCheckbox = containerTab.locator('input[name="print-schema"]').first();
    await firstCheckbox.check();
    const schemaValue = await firstCheckbox.getAttribute('value');

    const responsePromise = page.waitForResponse(r =>
      r.url().includes(`/containers/${containerId}/print`) && r.request().method() === 'POST'
    );
    await containerTab.locator('[data-print-btn]').click();
    const response = await responsePromise;

    const body = response.request().postDataJSON();
    expect(body.printer_id).toBeTruthy();
    expect(body.templates).toEqual([schemaValue]);
    expect(body.print_date).toBe(false);
    expect(body.show_children).toBe(false);
  });

  test('print_date checkbox is included in request when checked', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/containers/${containerId}`, { waitUntil: 'domcontentloaded' });

    const containerTab = page.locator('[data-tab-content="container-label"]');
    await containerTab.locator('input[name="print-schema"]').first().check();

    const responsePromise = page.waitForResponse(r =>
      r.url().includes(`/containers/${containerId}/print`) && r.request().method() === 'POST'
    );
    await containerTab.locator('[data-print-btn]').click();
    const response = await responsePromise;

    const body = response.request().postDataJSON();
    expect(body.print_date).toBe(true);
  });

  test('show_children checkbox is included in request when checked', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/containers/${containerId}`, { waitUntil: 'domcontentloaded' });

    const containerTab = page.locator('[data-tab-content="container-label"]');
    await containerTab.locator('input[name="print-schema"]').first().check();

    const responsePromise = page.waitForResponse(r =>
      r.url().includes(`/containers/${containerId}/print`) && r.request().method() === 'POST'
    );
    await containerTab.locator('[data-print-btn]').click();
    const response = await responsePromise;

    const body = response.request().postDataJSON();
    expect(body.show_children).toBe(true);
  });

  test('batch print tab has template and print button', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/containers/${containerId}`, { waitUntil: 'domcontentloaded' });

    const panel = page.locator('.print-panel');
    await panel.locator('.print-tab[data-tab="bulk-items"]').click();

    const bulkTab = panel.locator('[data-tab-content="bulk-items"]');
    await expect(bulkTab.locator('[data-print-template]')).toBeVisible();
    await expect(bulkTab.locator('[data-print-btn]')).toBeVisible();
    await expect(bulkTab.locator('[data-print-preview]')).toBeVisible();
  });
});
