import { test, expect } from '../fixtures/app';

test.describe('Keyboard Shortcuts — Global', () => {

  test('/ focuses global search', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.keyboard.press('/');
    await expect(page.locator('#global-search')).toBeFocused();
  });

  test('Ctrl+K focuses global search', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.keyboard.press('Control+k');
    await expect(page.locator('#global-search')).toBeFocused();
  });

  test('? opens help overlay', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.keyboard.type('?');
    await expect(page.locator('#keyboard-help')).toBeVisible();
  });

  test('Escape closes help overlay', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.keyboard.type('?');
    await expect(page.locator('#keyboard-help')).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(page.locator('#keyboard-help')).not.toBeVisible();
  });

  test('Escape blurs focused input', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.locator('#global-search').focus();
    await expect(page.locator('#global-search')).toBeFocused();
    await page.keyboard.press('Escape');
    await expect(page.locator('#global-search')).not.toBeFocused();
  });

  test('shortcuts ignored when input is focused', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.locator('#global-search').focus();
    await page.keyboard.press('s');
    await expect(page.locator('#content')).not.toHaveClass(/selection-mode/);
  });

  test('shortcuts ignored when dialog is open', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.keyboard.type('?');
    await expect(page.locator('#keyboard-help')).toBeVisible();
    await page.keyboard.press('s');
    await expect(page.locator('#content')).not.toHaveClass(/selection-mode/);
    await page.keyboard.press('Escape');
    await expect(page.locator('#keyboard-help')).not.toBeVisible();
  });

  test('m opens container navigator', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.keyboard.press('m');
    await expect(page.locator('#container-nav-picker')).toBeVisible();
  });
});

test.describe('Keyboard Shortcuts — Container View', () => {
  test.describe.configure({ mode: 'serial' });

  let containerName: string;

  test('setup: create container with items', async ({ page, app }) => {
    containerName = `KB Test ${Date.now()}`;
    await page.goto(`${app.baseURL}/`);
    await page.fill('.containers .quick-entry input[name="name"]', containerName);
    const resp = page.waitForResponse(r =>
      r.url().includes('/containers') && r.request().method() === 'POST'
    );
    await page.press('.containers .quick-entry input[name="name"]', 'Enter');
    await resp;
    await page.click(`#container-list a:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);

    for (const name of ['Item A', 'Item B']) {
      await page.fill('.items .quick-entry input[name="name"]', name);
      const itemResp = page.waitForResponse(r =>
        r.url().includes('/items') && r.request().method() === 'POST'
      );
      await page.press('.items .quick-entry input[name="name"]', 'Enter');
      await itemResp;
    }
    await expect(page.locator('#item-list li:not(.empty-state)')).toHaveCount(2);
  });

  test('i focuses item quick-entry', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.click(`#container-list a:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);
    await page.keyboard.press('i');
    await expect(page.locator('.items .quick-entry input[name="name"]')).toBeFocused();
  });

  test('c focuses container quick-entry', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.click(`#container-list a:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);
    await page.keyboard.press('c');
    await expect(page.locator('.containers .quick-entry input[name="name"]')).toBeFocused();
  });

  test('s toggles selection mode', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.click(`#container-list a:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);
    await page.locator('h2').click();
    await page.keyboard.press('s');
    await expect(page.locator('#content')).toHaveClass(/selection-mode/);
  });

  test('a selects all in selection mode', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.click(`#container-list a:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);
    await page.locator('h2').click();
    await page.keyboard.press('s');
    await expect(page.locator('#content')).toHaveClass(/selection-mode/);
    await page.keyboard.press('a');
    const checkboxes = page.locator('.bulk-select');
    const count = await checkboxes.count();
    for (let i = 0; i < count; i++) {
      await expect(checkboxes.nth(i)).toBeChecked();
    }
  });

  test('Escape exits selection mode', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.click(`#container-list a:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);
    await page.locator('h2').click();
    await page.keyboard.press('s');
    await expect(page.locator('#content')).toHaveClass(/selection-mode/);
    await page.keyboard.press('Escape');
    await expect(page.locator('#content')).not.toHaveClass(/selection-mode/);
  });

  test('arrow keys navigate list items', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.click(`#container-list a:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);
    await page.locator('h2').click();
    await page.keyboard.press('ArrowDown');
    const firstItem = page.locator('#item-list li:not(.empty-state)').first();
    await expect(firstItem).toHaveClass(/kb-active/);
    await page.keyboard.press('ArrowDown');
    const secondItem = page.locator('#item-list li:not(.empty-state)').nth(1);
    await expect(secondItem).toHaveClass(/kb-active/);
    await expect(firstItem).not.toHaveClass(/kb-active/);
  });

  test('Enter opens highlighted item', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.click(`#container-list a:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);
    await page.locator('h2').click();
    await page.keyboard.press('ArrowDown');
    const responsePromise = page.waitForResponse(r => r.url().includes('/items/'));
    await page.keyboard.press('Enter');
    await responsePromise;
    await expect(page.locator('h1')).toContainText('Item');
  });

  test('Escape clears list highlight', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.click(`#container-list a:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);
    await page.locator('h2').click();
    await page.keyboard.press('ArrowDown');
    await expect(page.locator('.kb-active')).toHaveCount(1);
    await page.keyboard.press('Escape');
    await expect(page.locator('.kb-active')).toHaveCount(0);
  });
});
