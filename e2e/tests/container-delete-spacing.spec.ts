import { test, expect } from '../fixtures/app';

// Regression for #82: the delete button of an empty container must sit below
// the print panel/section with a gap and stay inside .container-view.
test.describe('Container delete button layout', () => {
  test.describe.configure({ mode: 'serial' });

  const json = { Accept: 'application/json' };

  async function createContainer(request: any, baseURL: string, name: string): Promise<string> {
    const res = await request.post(`${baseURL}/containers`, { headers: json, form: { name } });
    expect(res.ok()).toBeTruthy();
    return (await res.json()).id;
  }

  async function assertButtonBelow(page: any, anchorSelector: string) {
    const anchor = page.locator(anchorSelector);
    await expect(anchor).toBeVisible();
    const button = page.locator('.container-actions button');
    await expect(button).toBeVisible();

    const a = await anchor.boundingBox();
    const b = await button.boundingBox();
    const view = await page.locator('.container-view').boundingBox();
    expect(a && b && view).toBeTruthy();

    // Gap of at least 8px below the panel.
    expect(b!.y).toBeGreaterThanOrEqual(a!.y + a!.height + 8);
    // Right edge stays inside the container view.
    expect(b!.x + b!.width).toBeLessThanOrEqual(view!.x + view!.width + 0.5);
    expect(b!.x).toBeGreaterThanOrEqual(view!.x);
  }

  test('empty container: delete button sits below whichever print block is shown', async ({ page, request, app }) => {
    // Does not touch printers: the worker-scoped server may or may not have
    // some from other specs, so accept either the panel or the "no printers" section.
    const id = await createContainer(request, app.baseURL, 'Delete Spacing Default');
    await page.goto(`${app.baseURL}/containers/${id}`);
    await assertButtonBelow(page, '.print-panel, .print-section');
  });

  test('with a printer: delete button sits below the print panel', async ({ page, request, app }) => {
    const pr = await request.post(`${app.baseURL}/printers`, {
      headers: json,
      form: { name: `E2E Spacing Printer ${Date.now()}`, encoder: 'niimbot', model: 'b1', transport: 'remote', address: 'http://localhost:9999' },
    });
    expect(pr.ok()).toBeTruthy();
    const printerId = (await pr.json()).id;
    try {
      const id = await createContainer(request, app.baseURL, 'Delete Spacing With Printer');
      await page.goto(`${app.baseURL}/containers/${id}`);
      await assertButtonBelow(page, '.print-panel');
    } finally {
      await request.delete(`${app.baseURL}/printers/${printerId}`, { headers: json });
    }
  });

  test('delete button is hidden once the container has an item', async ({ page, request, app }) => {
    const id = await createContainer(request, app.baseURL, 'Delete Spacing With Item');
    const item = await request.post(`${app.baseURL}/items`, {
      headers: json,
      form: { name: 'Blocker item', container_id: id },
    });
    expect(item.ok()).toBeTruthy();
    await page.goto(`${app.baseURL}/containers/${id}`);
    await expect(page.locator('.container-view')).toBeVisible();
    await expect(page.locator('.container-actions')).toHaveCount(0);
  });
});
