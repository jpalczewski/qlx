import { test, expect } from '../fixtures/app';

test.describe('Container print', () => {
  test.describe.configure({ mode: 'serial' });

  let containerId: string;
  let printerId: string;

  test('setup: create printer, container with items and tags', async ({ request, app }) => {
    const printerRes = await request.post(`${app.baseURL}/printers`, {
      headers: { 'Accept': 'application/json', 'Content-Type': 'application/json' },
      data: { name: 'E2E Print Printer', encoder: 'niimbot', model: 'b1', transport: 'remote', address: 'http://localhost:9999' },
    });
    expect(printerRes.ok()).toBeTruthy();
    const printer = await printerRes.json();
    printerId = printer.id;

    const tagRes = await request.post(`${app.baseURL}/tags`, {
      headers: { 'Accept': 'application/json', 'Content-Type': 'application/json' },
      data: { name: 'print-test-tag', color: '#ff0000' },
    });
    expect(tagRes.ok()).toBeTruthy();
    const tag = await tagRes.json();

    const containerRes = await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json', 'Content-Type': 'application/json' },
      data: { name: 'Print Container', description: 'Container for print tests' },
    });
    expect(containerRes.ok()).toBeTruthy();
    const container = await containerRes.json();
    containerId = container.id;

    // Assign tag to container
    await request.post(`${app.baseURL}/containers/${containerId}/tags`, {
      headers: { 'Accept': 'application/json', 'Content-Type': 'application/json' },
      data: { tag_id: tag.id },
    });

    // Create items inside container
    const item1Res = await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json', 'Content-Type': 'application/json' },
      data: { name: 'Print Item A', description: 'First item', container_id: containerId },
    });
    expect(item1Res.ok()).toBeTruthy();

    const item2Res = await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json', 'Content-Type': 'application/json' },
      data: { name: 'Print Item B', description: 'Second item', container_id: containerId },
    });
    expect(item2Res.ok()).toBeTruthy();
  });

  test('print section visible on container detail', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/containers/${containerId}`, { waitUntil: 'domcontentloaded' });
    await expect(page.locator('h2')).toContainText('Print Container');

    // Container label print section elements
    await expect(page.locator('#cprint-printer')).toBeVisible();
    await expect(page.locator('#cprint-templates')).toBeVisible();
    await expect(page.locator('#cprint-btn')).toBeVisible();
    await expect(page.locator('#cprint-date')).toBeVisible();
    await expect(page.locator('#cprint-children')).toBeVisible();
  });

  test('container print sends request with correct body', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/containers/${containerId}`, { waitUntil: 'domcontentloaded' });

    // Select the first schema checkbox
    const firstCheckbox = page.locator('input[name="cprint-schema"]').first();
    await firstCheckbox.check();
    const schemaValue = await firstCheckbox.getAttribute('value');

    const responsePromise = page.waitForResponse(r =>
      r.url().includes(`/containers/${containerId}/print`) && r.request().method() === 'POST'
    );
    await page.click('#cprint-btn');
    const response = await responsePromise;

    const body = response.request().postDataJSON();
    expect(body.printer_id).toBeTruthy();
    expect(body.templates).toEqual([schemaValue]);
    expect(body.print_date).toBe(false);
    expect(body.show_children).toBe(false);
  });

  test('print_date checkbox is included in request when checked', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/containers/${containerId}`, { waitUntil: 'domcontentloaded' });

    // Select a schema and check print date
    await page.locator('input[name="cprint-schema"]').first().check();
    await page.check('#cprint-date');

    const responsePromise = page.waitForResponse(r =>
      r.url().includes(`/containers/${containerId}/print`) && r.request().method() === 'POST'
    );
    await page.click('#cprint-btn');
    const response = await responsePromise;

    const body = response.request().postDataJSON();
    expect(body.print_date).toBe(true);
  });

  test('show_children checkbox is included in request when checked', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/containers/${containerId}`, { waitUntil: 'domcontentloaded' });

    // Select a schema and check show children
    await page.locator('input[name="cprint-schema"]').first().check();
    await page.check('#cprint-children');

    const responsePromise = page.waitForResponse(r =>
      r.url().includes(`/containers/${containerId}/print`) && r.request().method() === 'POST'
    );
    await page.click('#cprint-btn');
    const response = await responsePromise;

    const body = response.request().postDataJSON();
    expect(body.show_children).toBe(true);
  });

  test('batch print all items section is visible', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/containers/${containerId}`, { waitUntil: 'domcontentloaded' });

    // Batch item print section elements
    await expect(page.locator('#container-print-printer')).toBeVisible();
    await expect(page.locator('#container-print-template')).toBeVisible();
    await expect(page.locator('#container-print-all-btn')).toBeVisible();
  });
});
