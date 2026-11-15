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

test.describe('Working set rail', () => {
  test('rail renders with headings and empty states', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/containers`);
    const rail = page.locator('#working-rail');
    await expect(rail).toBeVisible();
    await expect(rail).toHaveAttribute('aria-label', 'W użyciu');
    await expect(rail.getByRole('heading', { name: 'Przypięte' })).toBeVisible();
    await expect(rail.getByRole('heading', { name: 'Ostatnie' })).toBeVisible();
    await expect(rail.locator('h1, h2, nav')).toHaveCount(0);
  });

  test('pin from container page appears in rail without reload, unpin from rail', async ({ page, app }) => {
    const parent = await createContainer(page, app.baseURL, uniq('Shelf'));
    const box = await createContainer(page, app.baseURL, uniq('Binder'), parent.id);

    await page.goto(`${app.baseURL}/containers/${box.id}`);
    const star = page.locator('[data-pin-toggle]');
    await expect(star).toHaveAttribute('aria-pressed', 'false');
    await expect(star).toHaveAttribute('aria-label', 'Przypnij do zestawu roboczego');

    const pins = page.locator('#working-rail [data-rail="pins"]');
    await expect(pins.locator('.rail-entry')).toHaveCount(0);

    await star.click();
    await expect(star).toHaveAttribute('aria-pressed', 'true');
    await expect(star).toHaveAttribute('aria-label', 'Odepnij');
    const entry = pins.locator('.rail-entry', { hasText: box.name });
    await expect(entry).toBeVisible();
    await expect(entry.locator('.rail-path')).toHaveText(parent.name);
    await expect(entry.locator('.rail-count')).toHaveText('0');
    await expect(entry.locator('.rail-link')).toHaveAttribute('aria-current', 'page');

    // Pin state survives a reload (IsPinned decides initial state).
    await page.reload();
    await expect(page.locator('[data-pin-toggle]')).toHaveAttribute('aria-pressed', 'true');

    // Unpin from the rail; the star on the page follows.
    await pins.locator('.rail-entry', { hasText: box.name }).getByRole('button', { name: /Odepnij/ }).click();
    await expect(pins.locator('.rail-entry', { hasText: box.name })).toHaveCount(0);
    await expect(page.locator('[data-pin-toggle]')).toHaveAttribute('aria-pressed', 'false');
  });

  test('star is keyboard operable', async ({ page, app }) => {
    const box = await createContainer(page, app.baseURL, uniq('Keyed'));
    await page.goto(`${app.baseURL}/containers/${box.id}`);
    const star = page.locator('[data-pin-toggle]');
    await star.focus();
    await page.keyboard.press('Enter');
    await expect(star).toHaveAttribute('aria-pressed', 'true');
    await expect(page.locator('#working-rail [data-rail="pins"]').getByText(box.name)).toBeVisible();
    await page.keyboard.press('Space');
    await expect(star).toHaveAttribute('aria-pressed', 'false');
    await expect(page.locator('#working-rail [data-rail="pins"]').getByText(box.name)).toHaveCount(0);
  });

  test('visiting a container adds it to recents; rail link navigates', async ({ page, app }) => {
    const a = await createContainer(page, app.baseURL, uniq('Alpha'));
    const b = await createContainer(page, app.baseURL, uniq('Beta'));

    await page.goto(`${app.baseURL}/containers/${a.id}`);
    const recents = page.locator('#working-rail [data-rail="recents"]');
    await expect(recents.locator('.rail-entry', { hasText: a.name })).toBeVisible();

    // HTMX navigation (no reload) to another container records it too.
    await page.goto(`${app.baseURL}/containers/${b.id}`);
    await expect(recents.locator('.rail-entry', { hasText: b.name })).toBeVisible();

    // Click the rail link for A: navigates via #content, URL follows, A is current.
    await recents.locator('.rail-link', { hasText: a.name }).click();
    await expect(page).toHaveURL(new RegExp(`/containers/${a.id}$`));
    await expect(page.locator('#content h2')).toContainText(a.name);
    await expect(recents.locator('.rail-entry', { hasText: a.name }).locator('.rail-link'))
      .toHaveAttribute('aria-current', 'page');
  });
});
