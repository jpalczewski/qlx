import { test, expect } from '../fixtures/app';

test.describe('Tag autocomplete', () => {
  test('a slow response for an old query does not overwrite the latest results', async ({ page, app }) => {
    const baseURL = app.baseURL;
    const stamp = Date.now();
    const alpha = `AlphaRace${stamp}`;
    const beta = `BetaRace${stamp}`;
    for (const name of [alpha, beta]) {
      const r = await page.request.post(`${baseURL}/tags`, {
        headers: { Accept: 'application/json' },
        data: { name, color: '', icon: '' },
      });
      expect(r.status()).toBe(201);
    }
    const contResp = await page.request.post(`${baseURL}/containers`, {
      headers: { Accept: 'application/json' },
      data: { name: `Race Container ${stamp}` },
    });
    const container = await contResp.json();

    // The first GET /tags (opening the dropdown) is slow; later ones are instant.
    let calls = 0;
    await page.route('**/tags', async (route) => {
      if (route.request().method() !== 'GET') return route.fallback();
      const n = ++calls;
      const resp = await route.fetch();
      if (n === 1) await new Promise((r) => setTimeout(r, 1500));
      await route.fulfill({ response: resp });
    });

    await page.goto(`${baseURL}/containers/${container.id}`, { waitUntil: 'domcontentloaded' });
    await page.locator('.container-tags .tag-add').click();
    const input = page.locator('.tag-ac-input');
    await expect(input).toBeVisible();
    await input.fill(beta);

    await expect(page.locator('.tag-ac-option', { hasText: beta })).toBeVisible();
    // Let the slow first response (unfiltered list) arrive.
    await page.waitForTimeout(2000);
    expect(calls).toBeGreaterThanOrEqual(2);
    await expect(page.locator('.tag-ac-option', { hasText: beta })).toBeVisible();
    await expect(page.locator('.tag-ac-option', { hasText: alpha })).toHaveCount(0);
  });
});
