import type { Locator } from '@playwright/test';
import { test, expect } from '../fixtures/app';

// Row markup with the transient flash class removed, so a freshly appended row
// can be compared with the same row after a reload.
async function normalizedRow(row: Locator): Promise<string> {
  return row.evaluate((el) => {
    const clone = el.cloneNode(true) as HTMLElement;
    clone.classList.remove('flash');
    if (!clone.getAttribute('class')) clone.removeAttribute('class');
    return clone.outerHTML.replace(/\s+/g, ' ').trim();
  });
}

test.describe('List rows', () => {
  test('quick-entry row matches the same row after reload', async ({ page, app }) => {
    const baseURL = app.baseURL;

    const contResp = await page.request.post(`${baseURL}/containers`, {
      headers: { Accept: 'application/json' },
      data: { name: 'Rows Container' },
    });
    const container = await contResp.json();

    await page.request.post(`${baseURL}/items`, {
      headers: { Accept: 'application/json' },
      data: { name: 'Api Item', description: 'Api description', quantity: 3, container_id: container.id },
    });

    await page.goto(`${baseURL}/containers/${container.id}`, { waitUntil: 'domcontentloaded' });

    const apiRow = page.locator('#item-list li', { hasText: 'Api Item' });
    await expect(apiRow.locator('.name')).toContainText('Api Item');
    await expect(apiRow.locator('.desc')).toHaveText('Api description');
    await expect(apiRow.locator('.badge')).toHaveText('x3');
    await expect(apiRow).not.toHaveClass(/flash/);

    // Quick entry: name plus quantity token.
    const input = page.locator('.qe-input');
    await input.click();
    await page.keyboard.press('End');
    await page.keyboard.type('Quick Item x4');
    const responsePromise = page.waitForResponse(
      (r) => r.url().endsWith('/items') && r.request().method() === 'POST',
    );
    await page.keyboard.press('Enter');
    await responsePromise;

    const quickRow = page.locator('#item-list li', { hasText: 'Quick Item' });
    await expect(quickRow).toHaveCount(1);
    await expect(quickRow).toHaveClass(/flash/);
    await expect(quickRow.locator('.name')).toContainText('Quick Item');
    await expect(quickRow.locator('.badge')).toHaveText('x4');
    await expect(quickRow.locator('.bulk-select')).toHaveCount(1);
    await expect(quickRow).toHaveAttribute('data-type', 'item');
    await expect(quickRow).toHaveAttribute('draggable', 'true');

    const appended = await normalizedRow(quickRow);

    await page.reload({ waitUntil: 'domcontentloaded' });

    const reloaded = page.locator('#item-list li', { hasText: 'Quick Item' });
    await expect(reloaded).not.toHaveClass(/flash/);
    expect(await normalizedRow(reloaded)).toBe(appended);

    // The API-created row still shows name, description and quantity the same way.
    const apiAfter = page.locator('#item-list li', { hasText: 'Api Item' });
    await expect(apiAfter.locator('.desc')).toHaveText('Api description');
    await expect(apiAfter.locator('.badge')).toHaveText('x3');
    await expect(page.locator('#item-list li.empty-state')).toHaveCount(0);
  });
});
