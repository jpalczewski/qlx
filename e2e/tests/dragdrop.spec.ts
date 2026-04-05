import { test, expect } from '../fixtures/app';
import type { Page, Locator } from '@playwright/test';

/**
 * Simulates HTML5 drag and drop using the Playwright-recommended approach:
 * create a real DataTransfer via evaluateHandle and dispatch drag events
 * through locator.dispatchEvent(). This correctly fires all drag listeners.
 */
async function dragTo(page: Page, source: Locator, target: Locator): Promise<void> {
  const dataTransfer = await page.evaluateHandle(() => new DataTransfer());
  await source.dispatchEvent('dragstart', { dataTransfer });
  await target.dispatchEvent('dragenter', { dataTransfer });
  await target.dispatchEvent('dragover',  { dataTransfer });
  await target.dispatchEvent('drop',      { dataTransfer });
  await source.dispatchEvent('dragend',   { dataTransfer });
}

test.describe('Drag and drop', () => {
  test.describe.configure({ mode: 'serial' });

  test('drag item into sibling container (uses PATCH, not POST)', async ({ page, app }) => {
    // Regression: drag-drop was sending POST /items/{id}/move → 405.
    // Fix: change fetch method to PATCH to match PATCH /items/{id}/move route.
    const parentName = `DnD-P-${Date.now()}`;
    const targetName = `DnD-T-${Date.now()}`;
    const itemName   = `DnD-I-${Date.now()}`;

    // Create parent container via API and navigate into it
    await page.goto(`${app.baseURL}/`, { waitUntil: 'domcontentloaded' });
    const r1 = page.waitForResponse(r => r.url().includes('/containers') && r.request().method() === 'POST');
    const input = page.locator('.qe-input');
    await input.click();
    await page.keyboard.press('Tab'); // switch to container mode
    await page.keyboard.press('End');
    await page.keyboard.type(parentName);
    await page.keyboard.press('Enter');
    await r1;
    await page.click(`#container-list a:has-text("${parentName}")`);
    await expect(page.locator('h2')).toContainText(parentName);

    // Create item (item mode is default)
    const r2 = page.waitForResponse(r => r.url().includes('/items') && r.request().method() === 'POST');
    await input.click();
    await page.keyboard.press('End');
    await page.keyboard.type(itemName);
    await page.keyboard.press('Enter');
    await r2;
    await expect(page.locator('#item-list')).toContainText(itemName);

    // Create target sub-container (switch to container mode)
    const r3 = page.waitForResponse(r => r.url().includes('/containers') && r.request().method() === 'POST');
    await input.click();
    await page.keyboard.press('Tab'); // switch to container mode
    await page.keyboard.press('End');
    await page.keyboard.type(targetName);
    await page.keyboard.press('Enter');
    await r3;
    await expect(page.locator('#container-list')).toContainText(targetName);

    const source = page.locator(`li[data-type="item"]:has-text("${itemName}")`);
    const target = page.locator(`[data-drop-type="container"]:has-text("${targetName}")`).first();

    const moveResp = page.waitForResponse(
      r => /\/items\/[^/]+\/move/.test(r.url()) && r.request().method() === 'PATCH',
    );
    await dragTo(page, source, target);
    expect((await moveResp).status()).toBe(200);

    await expect(page.locator('#item-list')).not.toContainText(itemName);
  });

  test('drag container into another container (uses PATCH, not POST)', async ({ page, app }) => {
    const rootName   = `DnD-R-${Date.now()}`;
    const sourceName = `DnD-S-${Date.now()}`;
    const destName   = `DnD-D-${Date.now()}`;

    await page.goto(`${app.baseURL}/`, { waitUntil: 'domcontentloaded' });
    const r1 = page.waitForResponse(r => r.url().includes('/containers') && r.request().method() === 'POST');
    const input = page.locator('.qe-input');
    await input.click();
    await page.keyboard.press('Tab'); // switch to container mode
    await page.keyboard.press('End');
    await page.keyboard.type(rootName);
    await page.keyboard.press('Enter');
    await r1;
    await page.click(`#container-list a:has-text("${rootName}")`);
    await expect(page.locator('h2')).toContainText(rootName);

    const r2 = page.waitForResponse(r => r.url().includes('/containers') && r.request().method() === 'POST');
    await input.click();
    await page.keyboard.press('Tab'); // switch to container mode
    await page.keyboard.press('End');
    await page.keyboard.type(sourceName);
    await page.keyboard.press('Enter');
    await r2;

    const r3 = page.waitForResponse(r => r.url().includes('/containers') && r.request().method() === 'POST');
    await input.click();
    // mode is already 'container' from the previous entry — no Tab needed
    await page.keyboard.press('End');
    await page.keyboard.type(destName);
    await page.keyboard.press('Enter');
    await r3;
    await expect(page.locator('#container-list')).toContainText(sourceName);
    await expect(page.locator('#container-list')).toContainText(destName);

    const source = page.locator('#container-list li').filter({ hasText: sourceName });
    const target = page.locator('#container-list').locator('[data-drop-type="container"]').filter({ hasText: destName }).first();

    await source.waitFor({ state: 'attached' });
    await target.waitFor({ state: 'attached' });

    const moveResp = page.waitForResponse(
      r => /\/containers\/[^/]+\/move/.test(r.url()) && r.request().method() === 'PATCH',
    );
    await dragTo(page, source, target);
    expect((await moveResp).status()).toBe(200);

    await expect(page.locator('#container-list')).not.toContainText(sourceName);
  });

  test('container added via quick-entry becomes a valid drop target', async ({ page, app }) => {
    // Regression: initDragDrop() was only called after full #content HTMX swaps.
    // Containers created via quick-entry arrive via a partial swap into
    // #container-list, so they never got drop listeners. Fix: call initDragDrop()
    // after every HTMX swap regardless of target element.
    const parentName  = `DnD-P3-${Date.now()}`;
    const newContName = `DnD-N3-${Date.now()}`;
    const itemName    = `DnD-I3-${Date.now()}`;

    await page.goto(`${app.baseURL}/`, { waitUntil: 'domcontentloaded' });
    const r1 = page.waitForResponse(r => r.url().includes('/containers') && r.request().method() === 'POST');
    const input = page.locator('.qe-input');
    await input.click();
    await page.keyboard.press('Tab'); // switch to container mode
    await page.keyboard.press('End');
    await page.keyboard.type(parentName);
    await page.keyboard.press('Enter');
    await r1;
    await page.click(`#container-list a:has-text("${parentName}")`);
    await expect(page.locator('h2')).toContainText(parentName);

    // Create item (item mode is default)
    const r2 = page.waitForResponse(r => r.url().includes('/items') && r.request().method() === 'POST');
    await input.click();
    await page.keyboard.press('End');
    await page.keyboard.type(itemName);
    await page.keyboard.press('Enter');
    await r2;
    await expect(page.locator('#item-list')).toContainText(itemName);

    // Add sub-container via quick-entry — partial HTMX swap into #container-list
    const r3 = page.waitForResponse(r => r.url().includes('/containers') && r.request().method() === 'POST');
    await input.click();
    await page.keyboard.press('Tab'); // switch to container mode
    await page.keyboard.press('End');
    await page.keyboard.type(newContName);
    await page.keyboard.press('Enter');
    await r3;
    await expect(page.locator('#container-list')).toContainText(newContName);

    // Verify the newly injected element has the drop-target attribute
    const newDropTarget = page.locator(`[data-drop-type="container"]:has-text("${newContName}")`);
    await expect(newDropTarget).toBeAttached();

    // Drag item onto the freshly-added container (tests re-init after partial swap)
    const source = page.locator(`li[data-type="item"]:has-text("${itemName}")`);

    const moveResp = page.waitForResponse(
      r => /\/items\/[^/]+\/move/.test(r.url()) && r.request().method() === 'PATCH',
    );
    await dragTo(page, source, newDropTarget);
    expect((await moveResp).status()).toBe(200);

    await expect(page.locator('#item-list')).not.toContainText(itemName);
  });
});
