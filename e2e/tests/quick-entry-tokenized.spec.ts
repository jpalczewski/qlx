import { test, expect } from '../fixtures/app';

test.describe('Tokenized quick entry', () => {

  test('Test 1: tokenized input is visible with prefilled container token', async ({ page, app }) => {
    const baseURL = app.baseURL;

    // Create a container to navigate into
    const contResp = await page.request.post(`${baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'QE Container 1' }
    });
    const container = await contResp.json();

    await page.goto(`${baseURL}/containers/${container.id}`, { waitUntil: 'domcontentloaded' });

    // The .qe-tokenized wrapper is visible
    await expect(page.locator('.qe-tokenized')).toBeVisible();

    // The contenteditable input is visible
    await expect(page.locator('.qe-input')).toBeVisible();

    // A container token with the container name is prefilled
    const token = page.locator('.qe-token--container');
    await expect(token).toBeVisible();
    await expect(token).toContainText('QE Container 1');
  });

  test('Test 2: basic item creation via tokenized quick entry', async ({ page, app }) => {
    const baseURL = app.baseURL;

    const contResp = await page.request.post(`${baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'QE Container Item' }
    });
    const container = await contResp.json();

    await page.goto(`${baseURL}/containers/${container.id}`, { waitUntil: 'domcontentloaded' });

    const input = page.locator('.qe-input');
    await input.click();
    // Move cursor to end of contenteditable (after the prefilled container token)
    await page.keyboard.press('End');

    const itemName = `Test Item ${Date.now()}`;

    // Type an item name
    await page.keyboard.type(itemName);

    const responsePromise = page.waitForResponse(r =>
      r.url().endsWith('/items') && r.request().method() === 'POST'
    );
    await page.keyboard.press('Enter');
    await responsePromise;

    // New item appears in the list
    await expect(page.locator('#item-list')).toContainText(itemName);
  });

  test('Test 3: Tab key toggles between item and container mode', async ({ page, app }) => {
    const baseURL = app.baseURL;

    const contResp = await page.request.post(`${baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'QE Toggle Container' }
    });
    const container = await contResp.json();

    await page.goto(`${baseURL}/containers/${container.id}`, { waitUntil: 'domcontentloaded' });

    const toggle = page.locator('.qe-type-toggle');
    const input = page.locator('.qe-input');

    // Initially shows 🏷 (item mode)
    await expect(toggle).toContainText('🏷');

    // Focus input and press Tab
    await input.click();
    await page.keyboard.press('Tab');

    // Toggle switches to 📦 (container mode)
    await expect(toggle).toContainText('📦');

    // Press Tab again
    await page.keyboard.press('Tab');

    // Toggle switches back to 🏷
    await expect(toggle).toContainText('🏷');
  });

  test('Test 4: @ trigger shows container autocomplete dropdown', async ({ page, app }) => {
    const baseURL = app.baseURL;

    const contResp = await page.request.post(`${baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'QE AC Container' }
    });
    const container = await contResp.json();

    await page.goto(`${baseURL}/containers/${container.id}`, { waitUntil: 'domcontentloaded' });

    const input = page.locator('.qe-input');
    await input.click();

    // Remove the prefilled container token by clicking ×
    const removeBtn = page.locator('.qe-token--container .qe-token-remove');
    await removeBtn.click();

    // Type @ to trigger container autocomplete
    await page.keyboard.type('@');

    // Container autocomplete dropdown is visible
    await expect(page.locator('.container-ac-dropdown')).toBeVisible();

    // Press Escape to close dropdown
    await page.keyboard.press('Escape');

    // Dropdown disappears
    await expect(page.locator('.container-ac-dropdown')).not.toBeVisible();
  });

  test('Test 5: # trigger shows tag autocomplete dropdown', async ({ page, app }) => {
    const baseURL = app.baseURL;

    // Create a tag
    await page.request.post(`${baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'QETag', color: 'blue', icon: '' }
    });

    const contResp = await page.request.post(`${baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'QE Tag Container' }
    });
    const container = await contResp.json();

    await page.goto(`${baseURL}/containers/${container.id}`, { waitUntil: 'domcontentloaded' });

    const input = page.locator('.qe-input');
    await input.click();

    // Type # to trigger tag autocomplete
    await page.keyboard.type('#');

    // Tag autocomplete dropdown is visible
    await expect(page.locator('.tag-ac-dropdown')).toBeVisible();
  });

  test('Test 6: × on container token removes it and restores default', async ({ page, app }) => {
    const baseURL = app.baseURL;

    const contResp = await page.request.post(`${baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'QE Remove Token Container' }
    });
    const container = await contResp.json();

    await page.goto(`${baseURL}/containers/${container.id}`, { waitUntil: 'domcontentloaded' });

    // Container token is present initially
    await expect(page.locator('.qe-token--container')).toBeVisible();

    // Click × on the token
    const removeBtn = page.locator('.qe-token--container .qe-token-remove');
    await removeBtn.click();

    // Token is removed but the default token is restored (same container = same token with default style)
    // The prefill is also the default, so the token should be restored as default
    await expect(page.locator('.qe-token--container')).toBeVisible();
    await expect(page.locator('.qe-token--default')).toBeVisible();
  });

  test('Test 7: container mode creates a container', async ({ page, app }) => {
    const baseURL = app.baseURL;

    const contResp = await page.request.post(`${baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'QE Parent Container' }
    });
    const container = await contResp.json();

    await page.goto(`${baseURL}/containers/${container.id}`, { waitUntil: 'domcontentloaded' });

    const toggle = page.locator('.qe-type-toggle');
    const input = page.locator('.qe-input');

    // Switch to container mode via Tab
    await input.click();
    await page.keyboard.press('Tab');
    await expect(toggle).toContainText('📦');

    // Move cursor to end before typing
    await page.keyboard.press('End');

    const subContainerName = `Sub Container ${Date.now()}`;
    await page.keyboard.type(subContainerName);

    const responsePromise = page.waitForResponse(r =>
      r.url().endsWith('/containers') && r.request().method() === 'POST'
    );
    await page.keyboard.press('Enter');
    await responsePromise;

    // New sub-container appears in the list
    await expect(page.locator('#container-list')).toContainText(subContainerName);
  });

});
