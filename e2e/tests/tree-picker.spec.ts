import { test, expect } from '../fixtures/app';

test.describe('Tree picker search', () => {
  test.describe.configure({ mode: 'serial' });

  let alphaId: string;
  let itemId: string;

  test('setup: create containers and item via API', async ({ request, app }) => {
    const cRes = await request.post(`${app.baseURL}/containers`, {
      data: { name: 'Alpha', description: '' },
      headers: { 'Accept': 'application/json' },
    });
    expect(cRes.ok()).toBeTruthy();
    const c = await cRes.json();
    alphaId = c.id;

    const bRes = await request.post(`${app.baseURL}/containers`, {
      data: { name: 'Beta', description: '' },
      headers: { 'Accept': 'application/json' },
    });
    expect(bRes.ok()).toBeTruthy();

    const iRes = await request.post(`${app.baseURL}/items`, {
      data: { name: 'Widget', container_id: alphaId },
      headers: { 'Accept': 'application/json' },
    });
    expect(iRes.ok()).toBeTruthy();
    const i = await iRes.json();
    itemId = i.id;
  });

  test('search in tree picker updates results on input', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/containers/${alphaId}`);

    // Enter selection mode via keyboard shortcut, then select Widget
    await page.locator('h2').click();
    await page.keyboard.press('s');
    const checkbox = page.locator(`[data-id="${itemId}"] input[type="checkbox"]`).first();
    await checkbox.click();

    // Action bar should appear with move button
    await expect(page.locator('#action-bar button:has-text("Przenieś do")')).toBeVisible();

    // Open move picker
    await page.click('#action-bar button:has-text("Przenieś do")');
    const dialog = page.locator('dialog#move-picker');
    await expect(dialog).toBeVisible();
    await expect(dialog.locator('.tree-label').first()).toBeVisible();

    // Type "alph" — search should update the tree and return Alpha
    const searchInput = dialog.locator('.tree-search');
    const searchResponse = page.waitForResponse(r =>
      r.url().includes('/partials/tree/search') && r.url().includes('q=alph')
    );
    await searchInput.fill('alph');
    await searchResponse;

    // Only "Alpha" should appear in results
    await expect(dialog.locator('.tree-label')).toHaveCount(1);
    await expect(dialog.locator('.tree-label')).toContainText('Alpha');
  });

  test('clearing search restores full tree', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/containers/${alphaId}`);

    await page.locator('h2').click();
    await page.keyboard.press('s');
    const checkbox = page.locator(`[data-id="${itemId}"] input[type="checkbox"]`).first();
    await checkbox.click();
    await expect(page.locator('#action-bar button:has-text("Przenieś do")')).toBeVisible();
    await page.click('#action-bar button:has-text("Przenieś do")');

    const dialog = page.locator('dialog#move-picker');
    await expect(dialog).toBeVisible();

    const searchInput = dialog.locator('.tree-search');

    // Search to filter
    const searchResponse = page.waitForResponse(r =>
      r.url().includes('/partials/tree/search')
    );
    await searchInput.fill('alph');
    await searchResponse;
    await expect(dialog.locator('.tree-label')).toHaveCount(1);

    // Clear — tree should reload from root (both Alpha and Beta visible)
    const treeResponse = page.waitForResponse(r =>
      r.url().includes('/partials/tree') && r.url().includes('parent_id=') && !r.url().includes('search')
    );
    await searchInput.fill('');
    await treeResponse;

    await expect(dialog.locator('.tree-label:has-text("Alpha")')).toBeVisible();
    await expect(dialog.locator('.tree-label:has-text("Beta")')).toBeVisible();
  });

  test('select container in picker and confirm moves item', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/containers/${alphaId}`);
    await expect(page.locator('#item-list')).toContainText('Widget');

    // Enter selection mode via keyboard shortcut and select Widget
    await page.locator('h2').click();
    await page.keyboard.press('s');
    const checkbox = page.locator(`[data-id="${itemId}"] input[type="checkbox"]`).first();
    await checkbox.click();
    await expect(page.locator('#action-bar button:has-text("Przenieś do")')).toBeVisible();

    // Open picker
    await page.click('#action-bar button:has-text("Przenieś do")');
    const dialog = page.locator('dialog#move-picker');
    await expect(dialog).toBeVisible();

    // Wait for root tree and click Beta
    await expect(dialog.locator('.tree-label:has-text("Beta")')).toBeVisible();

    const moveResponse = page.waitForResponse(r =>
      r.url().includes('/bulk/move') && r.request().method() === 'POST'
    );
    const reloadResponse = page.waitForResponse(r =>
      r.url().includes(`/containers/${alphaId}`) && r.request().method() === 'GET' && !r.url().includes('/notes')
    );
    await dialog.locator('.tree-label:has-text("Beta")').click();
    await dialog.locator('button:has-text("Przenieś")').click();
    await moveResponse;
    await reloadResponse;

    // Widget should no longer be in Alpha
    await expect(page.locator('#item-list')).not.toContainText('Widget');
  });
});
