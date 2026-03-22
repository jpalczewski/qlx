import { test, expect } from '../fixtures/app';

test.describe('Icon and color system', () => {
  // Blue hex from the palette
  const BLUE_HEX = '#4d9de0';

  test('container edit form shows color picker with 10 swatches and a selected one', async ({ request, page, app }) => {
    // Create a container via API with a specific color so the picker shows it as selected
    const res = await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: `ColorPickerTest ${Date.now()}`, color: 'green' },
    });
    expect(res.status()).toBe(201);
    const container = await res.json();

    await page.goto(`${app.baseURL}/containers/${container.id}/edit`, { waitUntil: 'domcontentloaded' });

    // Color picker grid is visible
    const grid = page.locator("[data-picker='color']");
    await expect(grid).toBeVisible();

    // There are exactly 10 color swatches
    const swatches = grid.locator('.color-swatch');
    await expect(swatches).toHaveCount(10);

    // At least one swatch has the selected class (random default)
    const selectedSwatch = grid.locator('.color-swatch.selected');
    await expect(selectedSwatch).toHaveCount(1);

    // Hidden input has a non-empty value
    const hiddenInput = page.locator("input[name='color']");
    const colorValue = await hiddenInput.inputValue();
    expect(colorValue).toBeTruthy();
    expect(colorValue.length).toBeGreaterThan(0);
  });

  test('clicking a color swatch updates hidden input and marks only that swatch selected', async ({ request, page, app }) => {
    const res = await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: `ColorSelectTest ${Date.now()}` },
    });
    expect(res.status()).toBe(201);
    const container = await res.json();

    await page.goto(`${app.baseURL}/containers/${container.id}/edit`, { waitUntil: 'domcontentloaded' });

    const grid = page.locator("[data-picker='color']");
    await expect(grid).toBeVisible();

    // Click the teal swatch
    const tealSwatch = grid.locator('.color-swatch[data-value="teal"]');
    await tealSwatch.click();

    // That swatch now has the selected class
    await expect(tealSwatch).toHaveClass(/selected/);

    // Hidden input value is "teal"
    const hiddenInput = page.locator("input[name='color']");
    await expect(hiddenInput).toHaveValue('teal');

    // Other swatches do not have selected class
    const otherSwatches = grid.locator('.color-swatch:not([data-value="teal"])');
    const count = await otherSwatches.count();
    for (let i = 0; i < count; i++) {
      await expect(otherSwatches.nth(i)).not.toHaveClass(/selected/);
    }
  });

  test('icon picker first category is open; clicking another category header opens it', async ({ request, page, app }) => {
    const res = await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: `IconPickerTest ${Date.now()}` },
    });
    expect(res.status()).toBe(201);
    const container = await res.json();

    await page.goto(`${app.baseURL}/containers/${container.id}/edit`, { waitUntil: 'domcontentloaded' });

    const iconPicker = page.locator("[data-picker='icon']");
    await expect(iconPicker).toBeVisible();

    // First category (tools) is open
    const firstCategory = iconPicker.locator('.icon-picker-category').first();
    await expect(firstCategory).toHaveClass(/open/);

    // Click the second category header to open it
    const secondCategory = iconPicker.locator('.icon-picker-category').nth(1);
    const secondHeader = secondCategory.locator('.icon-picker-category-header');
    await secondHeader.click();

    await expect(secondCategory).toHaveClass(/open/);
  });

  test('created container shows entity-icon with svg in container detail view', async ({ request, page, app }) => {
    const res = await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: `BlueWrenchContainer ${Date.now()}`, color: 'blue', icon: 'wrench' },
    });
    expect(res.status()).toBe(201);
    const container = await res.json();

    // Navigate to the container's detail page (not root list)
    await page.goto(`${app.baseURL}/containers/${container.id}`, { waitUntil: 'domcontentloaded' });

    // Container header shows entity-icon with an svg
    const entityIcon = page.locator('.container-header .entity-icon');
    await expect(entityIcon).toBeVisible();
    await expect(entityIcon.locator('svg')).toBeVisible();

    // Container header border uses the blue hex color (inline style)
    const containerHeader = page.locator('.container-header');
    const style = await containerHeader.getAttribute('style');
    expect(style).toContain(BLUE_HEX);
  });

  test('tag chip shows color and entity-icon svg on item list', async ({ request, page, app }) => {
    // 1. Create a container
    const containerRes = await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: `TagChipContainer ${Date.now()}` },
    });
    expect(containerRes.status()).toBe(201);
    const container = await containerRes.json();

    // 2. Create an item in that container
    const itemRes = await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      data: { name: `TagChipItem ${Date.now()}`, container_id: container.id },
    });
    expect(itemRes.status()).toBe(201);
    const item = await itemRes.json();

    // 3. Create a tag with color and icon
    const tagName = `RedWarningTag ${Date.now()}`;
    const tagRes = await request.post(`${app.baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { name: tagName, color: 'red', icon: 'warning' },
    });
    expect(tagRes.status()).toBe(201);
    const tag = await tagRes.json();

    // 4. Assign tag to item
    const assignRes = await request.post(`${app.baseURL}/items/${item.id}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { tag_id: tag.id },
    });
    expect(assignRes.status()).toBe(200);

    // 5. Navigate to the container view which lists items with their tag chips
    await page.goto(`${app.baseURL}/containers/${container.id}`, { waitUntil: 'domcontentloaded' });

    // The tag chip containing the tag name is visible
    const tagChip = page.locator('.tag-chip').filter({ hasText: tagName });
    await expect(tagChip).toBeVisible();

    // The chip has an entity-icon with an svg
    const chipIcon = tagChip.locator('.entity-icon svg');
    await expect(chipIcon).toBeVisible();
  });
});
