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

  test('without a printer: delete button sits below the print section', async ({ page, request, app }) => {
    // The worker-scoped server may hold printers from other specs; remove them.
    const list = await request.get(`${app.baseURL}/printers`, { headers: json });
    const body = await list.json();
    const printers: { id: string }[] = Array.isArray(body) ? body : Object.values(body ?? {});
    for (const p of printers) {
      await request.delete(`${app.baseURL}/printers/${p.id}`, { headers: json });
    }
    const id = await createContainer(request, app.baseURL, 'Delete Spacing No Printer');
    await page.goto(`${app.baseURL}/containers/${id}`);
    await assertButtonBelow(page, '.print-section');
  });

  test('with a printer: delete button sits below the print panel', async ({ page, request, app }) => {
    const pr = await request.post(`${app.baseURL}/printers`, {
      headers: json,
      form: { name: 'E2E Spacing Printer', encoder: 'niimbot', model: 'b1', transport: 'remote', address: 'http://localhost:9999' },
    });
    expect(pr.ok()).toBeTruthy();
    const id = await createContainer(request, app.baseURL, 'Delete Spacing With Printer');
    await page.goto(`${app.baseURL}/containers/${id}`);
    await assertButtonBelow(page, '.print-panel');
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
