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

async function createItem(page: Page, baseURL: string, containerId: string, name: string) {
  await page.request.post(`${baseURL}/items`, {
    headers: { Accept: 'application/json' },
    data: { name, container_id: containerId },
  });
}

async function pin(page: Page, baseURL: string, id: string) {
  const res = await page.request.post(`${baseURL}/api/working-set/pins/${id}`);
  expect(res.ok()).toBeTruthy();
}

test.describe('Quick switcher', () => {
  // The data dir is shared per worker: leave no pins behind (other specs
  // expect an empty rail) and start each test without any.
  const clearPins = async ({ page, app }: { page: Page; app: { baseURL: string } }) => {
    const res = await page.request.get(`${app.baseURL}/api/working-set`, { headers: { Accept: 'application/json' } });
    const snap = (await res.json()) as { pins: { id: string }[] };
    for (const p of snap.pins) await page.request.delete(`${app.baseURL}/api/working-set/pins/${p.id}`);
  };
  test.beforeEach(clearPins);
  test.afterEach(clearPins);

  test('Ctrl+K opens the dialog with combobox semantics and Esc restores focus', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/containers`);
    const search = page.locator('#global-search');
    await search.focus();
    await page.keyboard.press('Control+k');

    const dialog = page.locator('#quick-switcher');
    const input = page.locator('#qs-input');
    await expect(dialog).toBeVisible();
    await expect(input).toBeFocused();
    await expect(input).toHaveAttribute('role', 'combobox');
    await expect(input).toHaveAttribute('aria-expanded', 'true');
    await expect(page.locator('#qs-list')).toHaveAttribute('role', 'listbox');

    // Focus is trapped inside the dialog.
    await page.keyboard.press('Tab');
    await expect(input).toBeFocused();

    await page.keyboard.press('Escape');
    await expect(dialog).not.toBeVisible();
    await expect(search).toBeFocused();
  });

  test('empty query lists pins first, then recents', async ({ page, app }) => {
    const pinned = await createContainer(page, app.baseURL, uniq('Pinned'));
    const recent = await createContainer(page, app.baseURL, uniq('Recent'));
    await createItem(page, app.baseURL, pinned.id, uniq('Thing'));
    await pin(page, app.baseURL, pinned.id);
    await page.request.post(`${app.baseURL}/api/working-set/recents/${recent.id}`);

    await page.goto(`${app.baseURL}/containers`);
    await page.keyboard.press('Control+k');
    const options = page.locator('#qs-list [role="option"]');
    // Pins come first; recents (possibly more from earlier tests) follow.
    await expect(options.nth(0).locator('.qs-name')).toHaveText(pinned.name);
    await expect(options.nth(0).locator('.qs-pin')).toBeVisible();
    await expect(options.nth(0).locator('.qs-count')).toHaveText('1');
    const recentOpt = options.filter({ hasText: recent.name });
    await expect(recentOpt).toHaveCount(1);
    await expect(recentOpt.locator('.qs-pin')).toHaveCount(0);
    await expect(page.locator('#qs-input')).toHaveAttribute('aria-activedescendant', 'qs-opt-0');
  });

  test('typing finds containers ignoring Polish diacritics, Enter navigates', async ({ page, app }) => {
    const parent = await createContainer(page, app.baseURL, uniq('Regał'));
    const target = await createContainer(page, app.baseURL, 'Żółty segregator', parent.id);

    await page.goto(`${app.baseURL}/containers`);
    await page.keyboard.press('Control+k');
    await page.locator('#qs-input').fill('zolty');
    const options = page.locator('#qs-list [role="option"]');
    await expect(options).toHaveCount(1);
    await expect(options.first().locator('.qs-name')).toHaveText('Żółty segregator');
    await expect(options.first().locator('.qs-path')).toHaveText(parent.name);

    await page.keyboard.press('Enter');
    await expect(page.locator('#quick-switcher')).not.toBeVisible();
    await expect(page).toHaveURL(new RegExp(`/containers/${target.id}$`));
    await expect(page.locator('#content h2')).toHaveText('Żółty segregator');
  });

  test('arrow keys move the active option', async ({ page, app }) => {
    const base = uniq('Arrow');
    const a = await createContainer(page, app.baseURL, `${base} A`);
    await createContainer(page, app.baseURL, `${base} B`);

    await page.goto(`${app.baseURL}/containers`);
    await page.keyboard.press('Control+k');
    await page.locator('#qs-input').fill(base);
    const options = page.locator('#qs-list [role="option"]');
    await expect(options).toHaveCount(2);
    await expect(options.nth(0)).toHaveAttribute('aria-selected', 'true');
    await page.keyboard.press('ArrowDown');
    await expect(options.nth(1)).toHaveAttribute('aria-selected', 'true');
    await expect(page.locator('#qs-input')).toHaveAttribute('aria-activedescendant', 'qs-opt-1');
    await page.keyboard.press('ArrowUp');
    await expect(options.nth(0)).toHaveAttribute('aria-selected', 'true');
    expect(a.id).toBeTruthy();
  });

  test('digit keys jump to the nth pinned container', async ({ page, app }) => {
    const first = await createContainer(page, app.baseURL, uniq('Pin one'));
    const second = await createContainer(page, app.baseURL, uniq('Pin two'));
    await pin(page, app.baseURL, first.id);
    await pin(page, app.baseURL, second.id);

    await page.goto(`${app.baseURL}/containers`);
    const entries = page.locator('#working-rail [data-rail="pins"] .rail-entry');
    await expect(entries).toHaveCount(2);
    await expect(entries.nth(1)).toHaveAttribute('data-digit', '2');

    await page.locator('h1').click();
    await page.keyboard.press('2');
    await expect(page).toHaveURL(new RegExp(`/containers/${second.id}$`));
    await page.locator('h2').first().click();
    await page.keyboard.press('1');
    await expect(page).toHaveURL(new RegExp(`/containers/${first.id}$`));
  });

  test('digits do not jump while typing in an input', async ({ page, app }) => {
    const first = await createContainer(page, app.baseURL, uniq('Pin typing'));
    await pin(page, app.baseURL, first.id);

    await page.goto(`${app.baseURL}/containers`);
    await expect(page.locator('#working-rail [data-rail="pins"] .rail-entry')).toHaveCount(1);
    const search = page.locator('#global-search');
    await search.focus();
    await page.keyboard.type('1');
    await expect(search).toHaveValue('1');
    await page.waitForTimeout(300);
    await expect(page).not.toHaveURL(/\/containers\//);
  });
});
