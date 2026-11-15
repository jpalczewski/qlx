import { test, expect } from '../fixtures/app';

test.describe('Inventory list layout', () => {
  test('empty placeholder is compact, colourless dot hidden, tag button labelled', async ({ page, app }) => {
    const baseURL = app.baseURL;

    const parentResp = await page.request.post(`${baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Layout Parent' }
    });
    const parent = await parentResp.json();
    await page.request.post(`${baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Layout Child', parent_id: parent.id }
    });

    await page.goto(`${baseURL}/containers/${parent.id}`, { waitUntil: 'domcontentloaded' });

    // The empty items list shows a single quiet line, not a tall block
    const placeholder = page.locator('#item-list > .empty-state');
    await expect(placeholder).toHaveCount(1);
    const box = await placeholder.boundingBox();
    expect(box).not.toBeNull();
    expect(box!.height).toBeLessThan(40);
    await expect(placeholder).toHaveCSS('text-align', 'left');

    // A container without a colour does not draw a grey dot
    const row = page.locator('#container-list > li', { hasText: 'Layout Child' });
    await expect(row).toHaveCount(1);
    await expect(row.locator('.color-dot')).toBeHidden();

    // The "+" button explains itself on hover
    const add = row.locator('.tag-add');
    await add.hover();
    await expect(add).toHaveAttribute('title', /.+/);
    await expect(add).toHaveAttribute('aria-label', /.+/);
  });
});
