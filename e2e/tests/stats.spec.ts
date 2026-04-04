import { test, expect } from '../fixtures/app';

test.describe('Stats page', () => {
  test('stats page loads with stat cards', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/stats`, { waitUntil: 'domcontentloaded' });
    await expect(page.locator('h1')).toContainText('Statystyki');

    const cards = page.locator('.stat-card');
    await expect(cards).toHaveCount(4);
  });

  test('stat cards show numeric values', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/stats`, { waitUntil: 'domcontentloaded' });

    const values = page.locator('.stat-value');
    await expect(values).toHaveCount(4);

    // Each value should be a non-negative integer
    const texts = await values.allTextContents();
    for (const text of texts) {
      const n = parseInt(text.trim(), 10);
      expect(Number.isInteger(n) && n >= 0).toBe(true);
    }
  });

  test('refresh button triggers HTMX reload', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/stats`, { waitUntil: 'domcontentloaded' });

    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/stats') && r.request().method() === 'GET'
    );

    await page.getByRole('button', { name: /Odśwież/ }).click();
    const response = await responsePromise;

    expect(response.status()).toBe(200);
    await expect(page.locator('.stat-card').first()).toBeVisible();
  });

  test('stats page accessible via nav link', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`, { waitUntil: 'domcontentloaded' });

    const statsLink = page.locator('nav a[href="/stats"]');
    await expect(statsLink).toBeVisible();

    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/stats') && r.request().method() === 'GET'
    );
    await statsLink.click();
    await responsePromise;

    await expect(page.locator('.stat-card').first()).toBeVisible();
  });

  test('stats reflect created data', async ({ request, page, app }) => {
    // Create a container via API
    const containerRes = await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      form: { name: `StatTest ${Date.now()}`, parent_id: '' }
    });
    expect(containerRes.ok()).toBe(true);

    await page.goto(`${app.baseURL}/stats`, { waitUntil: 'domcontentloaded' });

    // Containers count should be at least 1
    const values = page.locator('.stat-value');
    const containerText = await values.nth(0).textContent();
    expect(parseInt(containerText!.trim(), 10)).toBeGreaterThanOrEqual(1);
  });
});
