import { test, expect } from '../fixtures/app';

// Paired labels: every item/container prints as two labels (identity + context)
// that share one header bar. All data here is synthetic.

function pngSize(body: Buffer): { w: number; h: number } {
  return { w: body.readUInt32BE(16), h: body.readUInt32BE(20) };
}

test.describe('Paired labels', () => {
  test.describe.configure({ mode: 'serial' });

  let containerId: string;
  let itemId: string;

  test('setup: printer, container with a child and an item', async ({ request, app }) => {
    const printerRes = await request.post(`${app.baseURL}/printers`, {
      headers: { Accept: 'application/json' },
      data: { name: 'Pair Printer', encoder: 'niimbot', model: 'b1', transport: 'remote', address: 'http://localhost:9999' },
    });
    expect(printerRes.ok()).toBeTruthy();

    const containerRes = await request.post(`${app.baseURL}/containers`, {
      headers: { Accept: 'application/json' },
      data: { name: 'Pair Shelf', description: 'Synthetic shelf for pair label tests' },
    });
    expect(containerRes.ok()).toBeTruthy();
    containerId = (await containerRes.json()).id;

    const subRes = await request.post(`${app.baseURL}/containers`, {
      headers: { Accept: 'application/json' },
      data: { name: 'Pair Drawer', parent_id: containerId },
    });
    expect(subRes.ok()).toBeTruthy();

    const itemRes = await request.post(`${app.baseURL}/items`, {
      headers: { Accept: 'application/json' },
      data: { name: 'Pair Widget', description: 'Synthetic widget with a reasonably long description text', container_id: containerId },
    });
    expect(itemRes.ok()).toBeTruthy();
    itemId = (await itemRes.json()).id;
  });

  test('pair is the default label style and hides the single-label template picker', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/items/${itemId}`);
    const form = page.locator('[data-print-form][data-print-mode="item"]');

    await expect(form.locator('[data-print-layout]')).toHaveValue('pair');
    await expect(form.locator('[data-print-template]')).toBeHidden();
    await expect(form.locator('[data-print-description]')).toBeVisible();
    await expect(form.locator('[data-print-description]')).not.toBeChecked();
  });

  test('preview shows both pages of the pair', async ({ page, request, app }) => {
    await page.goto(`${app.baseURL}/items/${itemId}`);
    const form = page.locator('[data-print-form][data-print-mode="item"]');

    const previewResponse = page.waitForResponse(r =>
      r.url().includes(`/items/${itemId}/preview`) && r.request().method() === 'GET'
    );
    await form.locator('[data-print-preview]').click();
    const response = await previewResponse;

    const url = new URL(response.url());
    expect(url.searchParams.get('template')).toBe('pair');
    expect(url.searchParams.get('page')).toBe('all');
    expect(response.headers()['x-qlx-pages']).toBe('2');

    const dialog = page.locator('[data-preview-dialog]');
    await expect(dialog).toBeVisible();
    const img = dialog.locator('img[data-preview-pair]');
    await expect(img).toBeVisible({ timeout: 10000 });
    await expect(dialog.locator('[data-preview-warning]')).toHaveCount(0);

    // The image is both pages stacked: twice one page plus the gap between them.
    const shown = await img.evaluate((el: HTMLImageElement) => ({ w: el.naturalWidth, h: el.naturalHeight }));
    const single = await request.get(response.url().replace('page=all', 'page=1'));
    const one = pngSize(await single.body());
    expect(shown.w).toBe(one.w);
    expect(shown.h).toBe(2 * one.h + 12);
  });

  test('preview endpoint serves page=1, page=2 and page=all', async ({ request, app }) => {
    const base = `${app.baseURL}/items/${itemId}/preview?template=pair&width=696`;
    const get = async (page: string) => {
      const res = await request.get(`${base}&page=${page}`, { headers: { Accept: 'image/png' } });
      expect(res.ok()).toBeTruthy();
      expect(res.headers()['content-type']).toContain('image/png');
      return res.body();
    };

    const p1 = await get('1');
    const p2 = await get('2');
    const all = await get('all');

    expect(pngSize(p1)).toEqual(pngSize(p2));
    expect(pngSize(all).w).toBe(pngSize(p1).w);
    expect(pngSize(all).h).toBeGreaterThan(2 * pngSize(p1).h);
    expect(p1.equals(p2)).toBe(false);

    const bad = await request.get(`${base}&page=7`);
    expect(bad.status()).toBe(400);
  });

  test('a pair that does not fit the media falls back to a single label with a warning', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/items/${itemId}/preview?template=pair&width=8`, {
      headers: { Accept: 'image/png' },
    });
    expect(res.status()).toBe(200);
    expect(res.headers()['x-qlx-warning']).toBe('pair_fallback');
    expect(res.headers()['x-qlx-pages']).toBe('1');
  });

  test('description option reaches the preview and changes the label', async ({ page, request, app }) => {
    await page.goto(`${app.baseURL}/items/${itemId}`);
    const form = page.locator('[data-print-form][data-print-mode="item"]');
    await form.locator('[data-print-description]').check();

    const previewResponse = page.waitForResponse(r => r.url().includes(`/items/${itemId}/preview`));
    await form.locator('[data-print-preview]').click();
    const response = await previewResponse;
    expect(new URL(response.url()).searchParams.get('print_description')).toBe('true');

    const base = `${app.baseURL}/items/${itemId}/preview?template=pair&width=696&page=1`;
    const without = await (await request.get(base)).body();
    const withDesc = await (await request.get(`${base}&print_description=true`)).body();
    expect(without.equals(withDesc)).toBe(false);
  });

  test('item print request carries the pair template, copies and description', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/items/${itemId}`);
    const form = page.locator('[data-print-form][data-print-mode="item"]');
    await form.locator('[data-print-description]').check();
    await form.locator('[data-copies]').fill('3');

    const responsePromise = page.waitForResponse(r =>
      r.url().includes(`/items/${itemId}/print`) && r.request().method() === 'POST'
    );
    await form.locator('[data-print-btn]').click();
    const response = await responsePromise;

    const body = response.request().postDataJSON();
    expect(body.template).toBe('pair');
    expect(body.print_description).toBe(true);
    expect(body.copies).toBe(3);
    // The printer is offline: the server answers with a message, never a crash page.
    await expect(form.locator('[data-print-result]')).not.toHaveText('');
  });

  test('container prints as a pair without choosing a template', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/containers/${containerId}`);
    const containerTab = page.locator('[data-tab-content="container-label"]');

    await expect(containerTab.locator('[data-print-layout]')).toHaveValue('pair');
    await expect(containerTab.locator('[data-print-checkboxes]')).toBeHidden();
    await containerTab.locator('[data-copies]').fill('2');

    const responsePromise = page.waitForResponse(r =>
      r.url().includes(`/containers/${containerId}/print`) && r.request().method() === 'POST'
    );
    await containerTab.locator('[data-print-btn]').click();
    const response = await responsePromise;

    const body = response.request().postDataJSON();
    expect(body.templates).toEqual(['pair']);
    expect(body.copies).toBe(2);
    expect(body.show_children).toBe(true);
    expect(body.print_description).toBe(false);
  });

  test('container pair preview includes both pages', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/containers/${containerId}`);
    const containerTab = page.locator('[data-tab-content="container-label"]');

    const previewResponse = page.waitForResponse(r =>
      r.url().includes(`/containers/${containerId}/preview`) && r.request().method() === 'GET'
    );
    await containerTab.locator('[data-print-preview]').click();
    const response = await previewResponse;
    const url = new URL(response.url());
    expect(url.searchParams.get('template')).toBe('pair');
    expect(url.searchParams.get('page')).toBe('all');
    expect(url.searchParams.get('show_children')).toBe('true');

    await expect(page.locator('[data-preview-dialog] img[data-preview-pair]')).toBeVisible({ timeout: 10000 });
  });

  test('single mode still prints one label with the chosen template', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/items/${itemId}`);
    const form = page.locator('[data-print-form][data-print-mode="item"]');

    await form.locator('[data-print-layout]').selectOption('single');
    await expect(form.locator('[data-print-template]')).toBeVisible();
    await expect(form.locator('[data-print-description]')).toBeHidden();
    await form.locator('[data-print-template]').selectOption('simple');

    const responsePromise = page.waitForResponse(r =>
      r.url().includes(`/items/${itemId}/print`) && r.request().method() === 'POST'
    );
    await form.locator('[data-print-btn]').click();
    const response = await responsePromise;

    const body = response.request().postDataJSON();
    expect(body.template).toBe('simple');
    expect(body.print_description).toBe(false);

    const previewResponse = page.waitForResponse(r => r.url().includes(`/items/${itemId}/preview`));
    await form.locator('[data-print-preview]').click();
    const preview = await previewResponse;
    expect(preview.headers()['x-qlx-pages']).toBe('1');
    expect(new URL(preview.url()).searchParams.get('page')).toBeNull();
    await expect(page.locator('[data-preview-dialog] .preview-image')).toBeVisible({ timeout: 10000 });
  });

  test('label style choice is remembered', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/items/${itemId}`);
    const form = page.locator('[data-print-form][data-print-mode="item"]');
    await form.locator('[data-print-layout]').selectOption('single');

    await page.reload();
    const reloaded = page.locator('[data-print-form][data-print-mode="item"]');
    await expect(reloaded.locator('[data-print-layout]')).toHaveValue('single');
    await expect(reloaded.locator('[data-print-template]')).toBeVisible();
  });
});
