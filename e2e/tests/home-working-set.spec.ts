import { test, expect } from '../fixtures/app';
import type { Page } from '@playwright/test';

let seq = 0;
const uniq = (base: string) => `${base} ${Date.now()}-${++seq}`;

async function createContainer(page: Page, baseURL: string, name: string, parentId?: string) {
  const res = await page.request.post(`${baseURL}/containers`, {
    headers: { Accept: 'application/json' },
    data: { name, ...(parentId ? { parent_id: parentId } : {}) },
  });
  return (await res.json()) as { id: string; name: string };
}

async function pin(page: Page, baseURL: string, id: string) {
  const res = await page.request.post(`${baseURL}/api/working-set/pins/${id}`);
  expect(res.status()).toBe(204);
}

async function recordOpen(page: Page, baseURL: string, id: string) {
  const res = await page.request.post(`${baseURL}/api/working-set/recents/${id}`);
  expect(res.status()).toBe(204);
}

test.describe('Home start screen', () => {
  test('empty state guides the user and links to the tree', async ({ page, app }) => {
    // Fresh data dir per worker: remove whatever earlier tests in this worker pinned.
    const snap = await (await page.request.get(`${app.baseURL}/api/working-set?recent=50`)).json();
    for (const p of snap.pins) await page.request.delete(`${app.baseURL}/api/working-set/pins/${p.id}`);
    // Recents cannot be cleared through the API; this test only asserts the pins-empty path
    // when nothing was opened, otherwise it checks the section is absent.
    await page.goto(`${app.baseURL}/`);
    await expect(page.locator('h1')).toHaveText('W użyciu');
    await expect(page.locator('[data-home="pins"]')).toHaveCount(0);
    if ((snap.recents as unknown[]).length === 0) {
      const empty = page.locator('[data-home="empty"]');
      await expect(empty).toContainText('Przypnij segregator');
      await empty.getByRole('link').click();
      await expect(page.locator('#container-list')).toBeVisible();
      await expect(page).toHaveURL(/\/containers$/);
    }
  });

  test('pinned and recent containers are listed and open in one click', async ({ page, app }) => {
    const parent = await createContainer(page, app.baseURL, uniq('Hall'));
    const pinned = await createContainer(page, app.baseURL, uniq('Pinned Binder'), parent.id);
    const recent = await createContainer(page, app.baseURL, uniq('Recent Binder'));
    await pin(page, app.baseURL, pinned.id);
    await recordOpen(page, app.baseURL, recent.id);

    await page.goto(`${app.baseURL}/`);

    const pinCard = page.locator(`[data-home="pins"] .home-card[data-container-id="${pinned.id}"]`);
    await expect(pinCard).toBeVisible();
    await expect(pinCard).toContainText(pinned.name);
    await expect(pinCard.locator('.home-card-path')).toContainText(parent.name);
    await expect(pinCard.locator('[data-pin-toggle]')).toHaveAttribute('aria-pressed', 'true');

    const recentCard = page.locator(`[data-home="recents"] .home-card[data-container-id="${recent.id}"]`);
    await expect(recentCard).toBeVisible();
    await expect(recentCard.locator('[data-pin-toggle]')).toHaveAttribute('aria-pressed', 'false');

    // No items view on the start screen.
    await expect(page.locator('#item-list')).toHaveCount(0);
    await expect(page.getByRole('heading', { name: 'Przedmioty' })).toHaveCount(0);

    await pinCard.locator('.home-card-link').click();
    await expect(page).toHaveURL(new RegExp(`/containers/${pinned.id}$`));
    await expect(page.locator('h2', { hasText: pinned.name })).toBeVisible();
  });

  test('search finds a container and opens it', async ({ page, app }) => {
    const name = uniq('Zebra Locker');
    const c = await createContainer(page, app.baseURL, name);

    await page.goto(`${app.baseURL}/`);
    const input = page.locator('#home-search');
    await expect(input).toBeFocused();
    await input.fill('Zebra Locker');
    const result = page.locator(`#home-search-results a[href="/containers/${c.id}"]`);
    await expect(result).toBeVisible();
    await expect(page.locator('#home-lists')).toBeHidden();
    await result.click();
    await expect(page).toHaveURL(new RegExp(`/containers/${c.id}$`));
  });

  test('"Wszystkie segregatory" opens the tree', async ({ page, app }) => {
    await createContainer(page, app.baseURL, uniq('Tree Root'));
    await page.goto(`${app.baseURL}/`);
    await page.getByRole('link', { name: 'Wszystkie segregatory' }).click();
    await expect(page).toHaveURL(/\/containers$/);
    await expect(page.locator('#container-list')).toBeVisible();
  });

  test('pin star on a recent card pins it', async ({ page, app }) => {
    const c = await createContainer(page, app.baseURL, uniq('To Pin'));
    await recordOpen(page, app.baseURL, c.id);
    await page.goto(`${app.baseURL}/`);
    const toggle = page.locator(`.home-card[data-container-id="${c.id}"] [data-pin-toggle]`);
    await toggle.click();
    await expect(toggle).toHaveAttribute('aria-pressed', 'true');
    const snap = await (await page.request.get(`${app.baseURL}/api/working-set`)).json();
    expect(snap.pins.map((p: { id: string }) => p.id)).toContain(c.id);
  });

  test('nav links point to home and to the tree', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/containers`);
    await expect(page.locator('nav a.brand')).toHaveAttribute('href', '/');
    await expect(page.locator('nav a[href="/containers"]').first()).toContainText('Segregatory');
    await page.locator('nav a.brand').click();
    await expect(page).toHaveURL(new RegExp('/$'));
    await expect(page.locator('.home-view')).toBeVisible();
  });
});

test.describe('Container tree root', () => {
  test('root shows only child containers: no items section, no item toggle', async ({ page, app }) => {
    await createContainer(page, app.baseURL, uniq('Root Box'));
    await page.goto(`${app.baseURL}/containers`);
    await expect(page.locator('#container-list')).toBeVisible();
    await expect(page.locator('#item-list')).toHaveCount(0);
    await expect(page.locator('section.items')).toHaveCount(0);
    await expect(page.getByText('Brak przedmiotów')).toHaveCount(0);
    await expect(page.locator('.qe-type-toggle')).toHaveCount(0);
    const input = page.locator('.qe-input');
    await expect(input).toHaveAttribute('data-placeholder', /Nowy kontener/);
    await expect(input).not.toHaveAttribute('data-placeholder', /Nowy item/);
  });

  test('root quick entry creates containers only, even after Tab', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/containers`);
    const name = uniq('Quick Root');
    const input = page.locator('.qe-input');
    await input.click();
    await page.keyboard.press('Tab');
    await expect(input).toHaveAttribute('data-placeholder', /Nowy kontener/);
    await page.keyboard.type(name);
    const resp = page.waitForResponse((r) => r.url().endsWith('/containers') && r.request().method() === 'POST');
    await page.keyboard.press('Enter');
    expect((await resp).status()).toBeLessThan(300);
    await expect(page.locator('#container-list')).toContainText(name);
  });

  test('inside a container the item section and toggle remain', async ({ page, app }) => {
    const c = await createContainer(page, app.baseURL, uniq('Has Items'));
    await page.goto(`${app.baseURL}/containers/${c.id}`);
    await expect(page.locator('#item-list')).toBeVisible();
    await expect(page.locator('.qe-type-toggle')).toBeVisible();
  });
});
