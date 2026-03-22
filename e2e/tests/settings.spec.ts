import { test, expect } from '../fixtures/app';

test.describe('Settings page', () => {
  test('settings page loads with language section', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/settings`, { waitUntil: 'domcontentloaded' });
    await expect(page.locator('h1')).toContainText('Ustawienia');
    await expect(page.locator('h2').first()).toContainText('Język');
    await expect(page.getByRole('button', { name: /Polski/ })).toBeVisible();
    await expect(page.getByRole('button', { name: /English/ })).toBeVisible();
  });

  test('settings page has data export links', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/settings`, { waitUntil: 'domcontentloaded' });
    await expect(page.getByRole('link', { name: /Eksportuj JSON/ })).toBeVisible();
    await expect(page.getByRole('link', { name: /Eksportuj CSV/ })).toBeVisible();
  });

  test('settings accessible via HTMX navigation', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`, { waitUntil: 'domcontentloaded' });
    const settingsLink = page.locator('nav a[title]').last();
    await settingsLink.click();
    await expect(page.locator('h1')).toContainText('Ustawienia');
  });
});

test.describe('Language switching', () => {
  test('switch to English and verify UI changes', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/settings`, { waitUntil: 'domcontentloaded' });

    // Polish is active by default (cookie set in fixture)
    const polskiBtn = page.getByRole('button', { name: /Polski/ });
    await expect(polskiBtn).toHaveClass(/lang-active/);

    // Switch to English — triggers reload
    const englishBtn = page.getByRole('button', { name: /English/ });
    await englishBtn.click();

    // Page reloads back to settings — should be in English now
    await expect(page.locator('h1')).toContainText('Settings');
  });

  test('language persists across navigation after switch', async ({ page, app }) => {
    // Override cookie to English
    await page.context().addCookies([{
      name: 'lang',
      value: 'en',
      url: app.baseURL,
    }]);

    await page.goto(`${app.baseURL}/`, { waitUntil: 'domcontentloaded' });
    await expect(page.locator('h1')).toContainText('Containers');

    // Navigate to printers via HTMX
    await page.click('a[href="/printers"]');
    await expect(page.locator('h1')).toContainText('Printers');

    // Navigate to templates
    await page.click('a[href="/templates"]');
    await expect(page.locator('h1')).toContainText('Templates');
  });

  test('navbar translates with language cookie', async ({ page, app }) => {
    // Default Polish (from fixture)
    await page.goto(`${app.baseURL}/`, { waitUntil: 'domcontentloaded' });
    await expect(page.getByRole('link', { name: 'Drukarki' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Szablony' })).toBeVisible();

    // Switch to English via cookie
    await page.context().addCookies([{
      name: 'lang',
      value: 'en',
      url: app.baseURL,
    }]);
    await page.reload({ waitUntil: 'domcontentloaded' });
    await expect(page.getByRole('link', { name: 'Printers' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Templates' })).toBeVisible();
  });
});

test.describe('i18n API', () => {
  test('GET /i18n/pl returns Polish translations', async ({ request, app }) => {
    const resp = await request.get(`${app.baseURL}/i18n/pl`, {
      headers: { 'Accept': 'application/json' },
    });
    expect(resp.status()).toBe(200);
    const data = await resp.json();
    expect(data['nav.printers']).toBe('Drukarki');
    expect(data['action.delete']).toBe('Usuń');
  });

  test('GET /i18n/en returns English translations', async ({ request, app }) => {
    const resp = await request.get(`${app.baseURL}/i18n/en`, {
      headers: { 'Accept': 'application/json' },
    });
    expect(resp.status()).toBe(200);
    const data = await resp.json();
    expect(data['nav.printers']).toBe('Printers');
    expect(data['action.delete']).toBe('Delete');
  });

  test('GET /i18n/xx falls back to English', async ({ request, app }) => {
    const resp = await request.get(`${app.baseURL}/i18n/xx`, {
      headers: { 'Accept': 'application/json' },
    });
    expect(resp.status()).toBe(200);
    const data = await resp.json();
    expect(data['nav.printers']).toBe('Printers');
  });
});
