import { test, expect } from '../fixtures/app';
import type { Page } from '@playwright/test';

let seq = 0;
const uniq = (base: string) => `${base} ${Date.now()}-${++seq}`;
const JSON_HEADERS = { Accept: 'application/json' };

async function createContainer(page: Page, baseURL: string, name: string) {
  const res = await page.request.post(`${baseURL}/containers`, { headers: JSON_HEADERS, data: { name } });
  return (await res.json()) as { id: string; name: string };
}

async function createItem(page: Page, baseURL: string, containerId: string, name: string, quantity = 1) {
  const res = await page.request.post(`${baseURL}/items`, {
    headers: JSON_HEADERS,
    data: { name, container_id: containerId, quantity },
  });
  expect(res.ok()).toBeTruthy();
  return (await res.json()) as { id: string; name: string };
}

async function pin(page: Page, baseURL: string, id: string) {
  const res = await page.request.post(`${baseURL}/api/working-set/pins/${id}`);
  expect(res.status()).toBe(204);
}

async function itemsOf(page: Page, baseURL: string, containerId: string) {
  const res = await page.request.get(`${baseURL}/containers/${containerId}/items`, { headers: JSON_HEADERS });
  const body = await res.json();
  return (Array.isArray(body) ? body : (body.items ?? [])) as { name: string; quantity: number }[];
}

test.describe('Start screen card actions', () => {
  test('quick-add on a card adds an item and updates the count', async ({ page, app }) => {
    const c = await createContainer(page, app.baseURL, uniq('Add Box'));
    await createItem(page, app.baseURL, c.id, 'Existing thing');
    await pin(page, app.baseURL, c.id);
    await page.goto(`${app.baseURL}/`);

    const card = page.locator(`.home-card[data-container-id="${c.id}"]`);
    await expect(card.locator('.home-card-count')).toHaveText('1 przedmiot');
    // The field is collapsed until the "+ Dodaj przedmiot" trigger is used.
    await card.getByRole('button', { name: '+ Dodaj przedmiot' }).click();
    const input = card.getByRole('textbox', { name: `Dodaj przedmiot do: ${c.name}` });
    await expect(input).toBeFocused();
    await input.fill('Spare cable');
    const response = page.waitForResponse((r) => r.url().endsWith('/items') && r.request().method() === 'POST');
    await input.press('Enter');
    expect((await response).status()).toBe(201);

    await expect(card.locator('.home-card-count')).toHaveText('2 przedmioty');
    await expect(input).toHaveValue('');
    await expect(page.locator('.toast-success')).toContainText('Dodano: Spare cable');
    // No page reload: the start screen is still the document.
    await expect(page.locator('h1')).toHaveText('W użyciu');
    const items = await itemsOf(page, app.baseURL, c.id);
    expect(items.map((i) => i.name)).toContain('Spare cable');
  });

  test('quantity token x5 sets the quantity and is stripped from the name', async ({ page, app }) => {
    const c = await createContainer(page, app.baseURL, uniq('Qty Box'));
    await pin(page, app.baseURL, c.id);
    await page.goto(`${app.baseURL}/`);

    const card = page.locator(`.home-card[data-container-id="${c.id}"]`);
    await card.locator('.home-card-add-toggle').click();
    const input = card.locator('.home-card-add-input');
    await input.fill('Screws x5');
    await input.press('Enter');
    await expect(card.locator('.home-card-count')).toHaveText('1 przedmiot');

    const items = await itemsOf(page, app.baseURL, c.id);
    expect(items).toHaveLength(1);
    expect(items[0].name).toBe('Screws');
    expect(items[0].quantity).toBe(5);
  });

  test('failed add shows an error toast and keeps the text', async ({ page, app }) => {
    const c = await createContainer(page, app.baseURL, uniq('Fail Box'));
    await pin(page, app.baseURL, c.id);
    await page.route('**/items', (route) => {
      if (route.request().method() === 'POST') return route.fulfill({ status: 500, body: '{"error":"boom"}' });
      return route.continue();
    });
    await page.goto(`${app.baseURL}/`);

    const card = page.locator(`.home-card[data-container-id="${c.id}"]`);
    await card.locator('.home-card-add-toggle').click();
    const input = card.locator('.home-card-add-input');
    await input.fill('Lost thing');
    await input.press('Enter');

    await expect(page.locator('.toast-error')).toContainText('Nie udało się dodać');
    await expect(input).toHaveValue('Lost thing');
    await expect(card.locator('.home-card-count')).toHaveText('0 przedmiotów');
  });

  test('quick-add is collapsed, opens by keyboard, Esc collapses', async ({ page, app }) => {
    const c = await createContainer(page, app.baseURL, uniq('Kbd Box'));
    await pin(page, app.baseURL, c.id);
    await page.goto(`${app.baseURL}/`);

    const card = page.locator(`.home-card[data-container-id="${c.id}"]`);
    const toggle = card.getByRole('button', { name: '+ Dodaj przedmiot' });
    const input = card.locator('.home-card-add-input');
    await expect(input).toBeHidden();
    await expect(toggle).toHaveAttribute('aria-expanded', 'false');

    await toggle.focus();
    await page.keyboard.press('Enter');
    await expect(input).toBeFocused();
    await expect(input).toHaveAttribute('placeholder', 'kabel x5');

    await page.keyboard.press('Escape');
    await expect(input).toBeHidden();
    await expect(toggle).toBeFocused();

    await page.keyboard.press('Enter');
    await input.fill('Keyboard thing');
    const response = page.waitForResponse((r) => r.url().endsWith('/items') && r.request().method() === 'POST');
    await page.keyboard.press('Enter');
    expect((await response).status()).toBe(201);
    await expect(card.locator('.home-card-count')).toHaveText('1 przedmiot');
  });

  test('count is spelled with Polish plural forms', async ({ page, app }) => {
    const cases: [number, string][] = [[0, '0 przedmiotów'], [1, '1 przedmiot'], [3, '3 przedmioty'], [5, '5 przedmiotów']];
    const ids: [string, string][] = [];
    for (const [n, text] of cases) {
      const c = await createContainer(page, app.baseURL, uniq(`Plural ${n}`));
      for (let i = 0; i < n; i++) await createItem(page, app.baseURL, c.id, `Thing ${i}`);
      await pin(page, app.baseURL, c.id);
      ids.push([c.id, text]);
    }
    await page.goto(`${app.baseURL}/`);
    for (const [id, text] of ids) {
      await expect(page.locator(`.home-card[data-container-id="${id}"] .home-card-count`)).toHaveText(text);
    }
  });

  test('a long name wraps instead of being cut off when the card is wide', async ({ page, app }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    // Short enough for two lines even in a narrow multi-column grid, too long for one.
    const name = `Segregator medyczny ${Date.now() % 100000}`;
    const c = await createContainer(page, app.baseURL, name);
    await pin(page, app.baseURL, c.id);
    await page.goto(`${app.baseURL}/`);

    const text = page.locator(`.home-card[data-container-id="${c.id}"] .home-card-name-text`);
    await expect(text).toHaveText(name);
    const m = await text.evaluate((el) => ({ sh: el.scrollHeight, ch: el.clientHeight, sw: el.scrollWidth, cw: el.clientWidth }));
    expect(m.sh - m.ch, JSON.stringify(m)).toBeLessThanOrEqual(1);
    expect(m.sw - m.cw, JSON.stringify(m)).toBeLessThanOrEqual(1);
  });

  test('print shortcut opens the container with the print panel focused', async ({ page, app }) => {
    const printerRes = await page.request.post(`${app.baseURL}/printers`, {
      headers: JSON_HEADERS,
      form: { name: 'Card Print Printer', encoder: 'niimbot', model: 'b1', transport: 'remote', address: 'http://localhost:9999' },
    });
    expect(printerRes.ok()).toBeTruthy();
    const c = await createContainer(page, app.baseURL, uniq('Print Box'));
    await pin(page, app.baseURL, c.id);
    await page.goto(`${app.baseURL}/`);

    const card = page.locator(`.home-card[data-container-id="${c.id}"]`);
    await card.getByRole('link', { name: `Drukuj: ${c.name}` }).click();
    await expect(page).toHaveURL(new RegExp(`/containers/${c.id}`));
    await expect(page.locator('.print-panel')).toBeVisible();
    await expect(page.locator('.print-panel [data-panel-printer]')).toBeFocused();
  });

  test('direct load of ?print=1 focuses the print panel and drops the parameter', async ({ page, app }) => {
    const c = await createContainer(page, app.baseURL, uniq('Direct Print Box'));
    await page.goto(`${app.baseURL}/containers/${c.id}?print=1`);
    await expect(page.locator('.print-panel [data-panel-printer]')).toBeFocused();
    await expect(page).toHaveURL(new RegExp(`/containers/${c.id}$`));
  });

  test('wide search shows container and item groups; Enter opens the first hit', async ({ page, app }) => {
    const word = `Zorblax${Date.now()}`;
    const c = await createContainer(page, app.baseURL, `${word} Binder`);
    const other = await createContainer(page, app.baseURL, uniq('Plain Box'));
    const item = await createItem(page, app.baseURL, other.id, `${word} widget`, 3);

    await page.goto(`${app.baseURL}/`);
    const input = page.locator('#home-search');
    await input.fill(word);

    const results = page.locator('#home-search-results');
    await expect(results.locator('[data-group="containers"]')).toHaveText('Segregatory');
    await expect(results.locator('[data-group="items"]')).toHaveText('Przedmioty');
    const containerHit = results.locator(`a[href="/containers/${c.id}"]`);
    const itemHit = results.locator(`a[href^="/containers/${other.id}"]`);
    await expect(containerHit).toBeVisible();
    await expect(itemHit).toContainText(`${word} widget`);
    await expect(itemHit).toContainText(other.name);
    await expect(itemHit).toContainText('3');

    // Containers come before items; arrows walk across both groups.
    const links = results.locator('.home-result-link');
    await expect(links.first()).toHaveAttribute('href', `/containers/${c.id}`);
    await input.press('ArrowDown');
    await expect(containerHit).toBeFocused();
    await page.keyboard.press('ArrowDown');
    await expect(itemHit).toBeFocused();
    await page.keyboard.press('ArrowUp');
    await expect(containerHit).toBeFocused();
    await page.keyboard.press('ArrowUp');
    await expect(input).toBeFocused();

    // Enter in the field opens the first hit (the container).
    await input.press('Enter');
    await expect(page).toHaveURL(new RegExp(`/containers/${c.id}$`));

    // Opening the item hit lands in its container with the item highlighted.
    await page.goto(`${app.baseURL}/`);
    await page.locator('#home-search').fill(`${word} widget`);
    await page.locator(`#home-search-results a[href^="/containers/${other.id}"]`).click();
    await expect(page).toHaveURL(new RegExp(`/containers/${other.id}`));
    await expect(page.locator(`#item-list li[data-id="${item.id}"]`)).toHaveClass(/kb-active/);
  });

  test('wide search with no match shows the empty state', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.locator('#home-search').fill(`nomatch${Date.now()}`);
    await expect(page.locator('#home-search-results .home-results-empty')).toHaveText('Nic nie znaleziono');
    await expect(page.locator('#home-search-results .home-result')).toHaveCount(0);
  });
});
