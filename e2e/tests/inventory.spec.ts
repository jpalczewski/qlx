import { test, expect } from '../fixtures/app';

test.describe('Inventory management', () => {
  test.describe.configure({ mode: 'serial' });

  let containerName: string;
  let subContainerName: string;
  let itemName: string;

  test('create root container via quick entry', async ({ page, app }) => {
    containerName = `Test Container ${Date.now()}`;
    await page.goto(`${app.baseURL}/`, { waitUntil: 'domcontentloaded' });
    await expect(page.locator('h1')).toContainText('Kontenery');

    // Switch to container mode (Tab), then type name and submit
    const input = page.locator('.qe-input');
    await input.click();
    await page.keyboard.press('Tab'); // switch to container mode
    await page.keyboard.press('End');
    await page.keyboard.type(containerName);

    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/containers') && r.request().method() === 'POST'
    );
    await page.keyboard.press('Enter');
    await responsePromise;

    // Quick entry appends to list, stays on same page
    await expect(page.locator('#container-list')).toContainText(containerName);
  });

  test('navigate into container from root', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`, { waitUntil: 'domcontentloaded' });
    await expect(page.locator('#container-list')).toContainText(containerName);
    await page.click(`a:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);
  });

  test('create sub-container', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`, { waitUntil: 'domcontentloaded' });
    await page.click(`a:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);

    subContainerName = `Sub ${Date.now()}`;
    // Switch to container mode (Tab), then type name and submit
    const input = page.locator('.qe-input');
    await input.click();
    await page.keyboard.press('Tab'); // switch to container mode
    await page.keyboard.press('End');
    await page.keyboard.type(subContainerName);

    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/containers') && r.request().method() === 'POST'
    );
    await page.keyboard.press('Enter');
    await responsePromise;

    await expect(page.locator('#container-list')).toContainText(subContainerName);
  });

  test('create item in container', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`, { waitUntil: 'domcontentloaded' });
    await page.click(`a:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);

    itemName = `Item ${Date.now()}`;
    // Item mode is default — just click input, type name, and submit
    const input = page.locator('.qe-input');
    await input.click();
    await page.keyboard.press('End');
    await page.keyboard.type(itemName);

    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/items') && r.request().method() === 'POST'
    );
    await page.keyboard.press('Enter');
    await responsePromise;

    await expect(page.locator('#item-list')).toContainText(itemName);
  });

  test('navigate to item detail', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`, { waitUntil: 'domcontentloaded' });
    await page.click(`a:has-text("${containerName}")`);
    await page.click(`a:has-text("${itemName}")`);
    await expect(page.locator('h1')).toContainText(itemName);
  });

  test('navigate via breadcrumbs', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`, { waitUntil: 'domcontentloaded' });
    await page.click(`a:has-text("${containerName}")`);
    await page.click(`a:has-text("${itemName}")`);

    await page.click(`.breadcrumb a:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);
  });

  test('attempt delete non-empty container shows no delete button', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`, { waitUntil: 'domcontentloaded' });
    await page.click(`a:has-text("${containerName}")`);
    // Container has sub-container and item, delete button should not be visible
    await expect(page.locator('button:has-text("Usuń kontener")')).not.toBeVisible();
  });

  test('delete item', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`, { waitUntil: 'domcontentloaded' });
    await page.click(`a:has-text("${containerName}")`);
    await page.click(`a:has-text("${itemName}")`);

    page.on('dialog', dialog => dialog.accept());
    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/items/') && r.request().method() === 'DELETE'
    );
    await page.click('button:has-text("Usuń")');
    await responsePromise;
  });

  test('delete empty sub-container', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`, { waitUntil: 'domcontentloaded' });
    await page.click(`a:has-text("${containerName}")`);
    await page.click(`a:has-text("${subContainerName}")`);

    page.on('dialog', dialog => dialog.accept());
    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/containers/') && r.request().method() === 'DELETE'
    );
    await page.click('button:has-text("Usuń kontener")');
    await responsePromise;
  });
});
