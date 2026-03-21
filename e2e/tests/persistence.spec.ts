import { test, expect } from '../fixtures/app';

test.describe('Data persistence across page reloads', () => {
  test.describe.configure({ mode: 'serial' });

  let containerName: string;
  let itemName: string;

  test('create container and item', async ({ request, app }) => {
    containerName = `Persist Test ${Date.now()}`;
    itemName = `Persist Item ${Date.now()}`;

    const containerRes = await request.post(`${app.baseURL}/api/containers`, {
      data: { name: containerName, description: 'persistence test' },
    });
    const container = await containerRes.json();

    await request.post(`${app.baseURL}/api/items`, {
      data: { name: itemName, description: 'persist item desc', container_id: container.id },
    });
  });

  test('container visible after full page reload', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui`);
    await expect(page.locator('.container-list')).toContainText(containerName);

    // Full reload — not HTMX, actual browser reload
    await page.reload();
    await expect(page.locator('.container-list')).toContainText(containerName);
  });

  test('item visible after navigating away and back', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui`);
    const containerLink = page.locator(`.container-item:has-text("${containerName}")`);
    await containerLink.click();

    await expect(page.locator('.item-list')).toContainText(itemName);

    // Navigate to printers and back
    await page.goto(`${app.baseURL}/ui/printers`);
    await expect(page.locator('h1')).toContainText('Drukarki');

    await page.goto(`${app.baseURL}/ui`);
    await page.click(`.container-item:has-text("${containerName}")`);
    await expect(page.locator('.item-list')).toContainText(itemName);
  });

  test('item description persists correctly', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui`);
    await page.click(`.container-item:has-text("${containerName}")`);
    await page.click(`.item-item:has-text("${itemName}")`);
    await expect(page.locator('.description')).toContainText('persist item desc');
  });
});

test.describe('UI error states', () => {

  test('navigating to non-existent container shows 404', async ({ page, app }) => {
    const res = await page.goto(`${app.baseURL}/ui/containers/00000000-0000-0000-0000-000000000000`);
    expect(res?.status()).toBe(404);
  });

  test('navigating to non-existent item shows 404', async ({ page, app }) => {
    const res = await page.goto(`${app.baseURL}/ui/items/00000000-0000-0000-0000-000000000000`);
    expect(res?.status()).toBe(404);
  });

  test('root containers page loads without JS errors', async ({ page, app }) => {
    // Collect uncaught page errors (thrown exceptions)
    const pageErrors: string[] = [];
    page.on('pageerror', err => pageErrors.push(err.message));

    // Collect failed network requests, ignoring known SSE endpoint (no printer manager in test)
    const failedRequests: string[] = [];
    page.on('requestfailed', req => {
      if (!req.url().includes('printers/events') && !req.url().includes('favicon')) {
        failedRequests.push(`${req.url()} - ${req.failure()?.errorText}`);
      }
    });

    await page.goto(`${app.baseURL}/ui`);
    await expect(page.locator('h1')).toContainText('Kontenery');

    expect(pageErrors).toHaveLength(0);
    expect(failedRequests).toHaveLength(0);
  });

  test('printers page loads without JS errors', async ({ page, app }) => {
    const pageErrors: string[] = [];
    page.on('pageerror', err => pageErrors.push(err.message));

    const failedRequests: string[] = [];
    page.on('requestfailed', req => {
      if (!req.url().includes('printers/events') && !req.url().includes('favicon')) {
        failedRequests.push(`${req.url()} - ${req.failure()?.errorText}`);
      }
    });

    await page.goto(`${app.baseURL}/ui/printers`);
    await expect(page.locator('h1')).toContainText('Drukarki');

    expect(pageErrors).toHaveLength(0);
    expect(failedRequests).toHaveLength(0);
  });

  test('templates page loads without JS errors', async ({ page, app }) => {
    const pageErrors: string[] = [];
    page.on('pageerror', err => pageErrors.push(err.message));

    const failedRequests: string[] = [];
    page.on('requestfailed', req => {
      if (!req.url().includes('printers/events') && !req.url().includes('favicon')) {
        failedRequests.push(`${req.url()} - ${req.failure()?.errorText}`);
      }
    });

    await page.goto(`${app.baseURL}/ui/templates`);
    await expect(page.locator('h1')).toContainText('Szablony');

    expect(pageErrors).toHaveLength(0);
    expect(failedRequests).toHaveLength(0);
  });
});
