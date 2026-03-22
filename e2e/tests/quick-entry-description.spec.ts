import { test, expect } from '../fixtures/app';

test.describe('Quick-entry description', () => {
  test.describe.configure({ mode: 'serial' });

  let containerName: string;

  test('create container then test description toggle', async ({ page, app }) => {
    containerName = `Desc Test ${Date.now()}`;
    await page.goto(`${app.baseURL}/ui`, { waitUntil: 'domcontentloaded' });

    // Create a container first
    const nameInput = page.locator('.containers .quick-entry input[name="name"]');
    await nameInput.fill(containerName);
    const resp = page.waitForResponse(r =>
      r.url().includes('/ui/actions/containers') && r.request().method() === 'POST'
    );
    await nameInput.press('Enter');
    await resp;
    await expect(page.locator('#container-list')).toContainText(containerName);
  });

  test('container description trigger expands on click', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui`, { waitUntil: 'domcontentloaded' });
    const trigger = page.locator('.containers .quick-entry-desc-trigger');
    const body = page.locator('.containers .quick-entry-desc-body');

    // Initially collapsed
    await expect(body).not.toBeVisible();

    // Click trigger to expand
    await trigger.click();
    await expect(body).toBeVisible();

    // Textarea is focused
    const textarea = page.locator('.containers .quick-entry-desc-body textarea');
    await expect(textarea).toBeFocused();
  });

  test('escape collapses description', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui`, { waitUntil: 'domcontentloaded' });
    const trigger = page.locator('.containers .quick-entry-desc-trigger');
    const body = page.locator('.containers .quick-entry-desc-body');

    await trigger.click();
    await expect(body).toBeVisible();

    // Escape collapses
    await page.keyboard.press('Escape');
    await expect(body).not.toBeVisible();

    // Focus returns to trigger
    await expect(trigger).toBeFocused();
  });

  test('submit container with description', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui`, { waitUntil: 'domcontentloaded' });
    const name = `WithDesc ${Date.now()}`;

    const nameInput = page.locator('.containers .quick-entry input[name="name"]');
    await nameInput.fill(name);

    // Expand and fill description
    const trigger = page.locator('.containers .quick-entry-desc-trigger');
    await trigger.click();
    const textarea = page.locator('.containers .quick-entry-desc-body textarea');
    await textarea.fill('Test description text');

    const resp = page.waitForResponse(r =>
      r.url().includes('/ui/actions/containers') && r.request().method() === 'POST'
    );
    await page.locator('.containers .quick-entry-submit').click();
    await resp;

    // Container appears in list with name and description
    await expect(page.locator('#container-list')).toContainText(name);
    await expect(page.locator('#container-list')).toContainText('Test description text');
  });

  test('expanded state persists after submit', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui`, { waitUntil: 'domcontentloaded' });

    // Expand description
    const trigger = page.locator('.containers .quick-entry-desc-trigger');
    await trigger.click();

    const nameInput = page.locator('.containers .quick-entry input[name="name"]');
    await nameInput.fill(`Persist ${Date.now()}`);

    const resp = page.waitForResponse(r =>
      r.url().includes('/ui/actions/containers') && r.request().method() === 'POST'
    );
    await nameInput.press('Enter');
    await resp;

    // Description should still be expanded after reset
    const body = page.locator('.containers .quick-entry-desc-body');
    await expect(body).toBeVisible();
  });

  test('item description toggle works', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui`, { waitUntil: 'domcontentloaded' });
    // Navigate into the container
    await page.click(`a:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);

    const trigger = page.locator('.items .quick-entry-desc-trigger');
    const body = page.locator('.items .quick-entry-desc-body');

    // Click to expand
    await trigger.click();
    await expect(body).toBeVisible();

    // Fill and submit item with description
    const nameInput = page.locator('.items .quick-entry input[name="name"]');
    await nameInput.fill(`Item Desc ${Date.now()}`);
    const textarea = page.locator('.items .quick-entry-desc-body textarea');
    await textarea.fill('Item description here');

    const resp = page.waitForResponse(r =>
      r.url().includes('/ui/actions/items') && r.request().method() === 'POST'
    );
    await page.locator('.items .quick-entry-submit').click();
    await resp;

    // Item appears in list
    await expect(page.locator('#item-list')).toContainText(`Item Desc`);
  });
});
