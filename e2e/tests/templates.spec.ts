import { test, expect } from '../fixtures/app';

test.describe('Template management', () => {
  test.describe.configure({ mode: 'serial' });

  const templateName = `Tpl ${Date.now()}`;
  const updatedName = `Tpl Updated ${Date.now()}`;
  const tag1 = 'etykieta';
  const tag2 = 'test';

  test('shows empty template list', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui/templates`);
    await expect(page.locator('h1')).toContainText('Szablony');
    await expect(page.locator('p.empty')).toContainText('Brak szablonów');
  });

  test('navigate to template designer', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui/templates`);
    await page.click('a:has-text("Nowy szablon")');
    await expect(page.locator('#designer-app')).toBeVisible();
    await expect(page.locator('#template-name')).toBeVisible();
    await expect(page.locator('#save-template')).toBeVisible();
  });

  test('create template via designer', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui/templates/new`);
    await expect(page.locator('#designer-app')).toBeVisible();

    await page.fill('#template-name', templateName);
    await page.fill('#template-tags', `${tag1}, ${tag2}`);

    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/ui/actions/templates') && r.request().method() === 'POST'
    );
    await page.click('#save-template');
    const response = await responsePromise;
    expect(response.status()).toBeLessThan(400);

    // After save, designer JS navigates back to templates list via htmx
    await expect(page.locator('h1')).toContainText('Szablony');
  });

  test('template appears in list', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui/templates`);
    const card = page.locator(`.template-card:has(.name:has-text("${templateName}"))`).first();
    await expect(card).toBeVisible();
    await expect(card.locator('.name')).toContainText(templateName);
    await expect(card.locator('.tag').first()).toBeVisible();
  });

  test('filter templates by tag', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui/templates`);
    await expect(page.locator('.tag-filter-bar')).toBeVisible();

    // Click on one of the tags in the filter bar
    await page.click(`.tag-filter-bar a.tag:has-text("${tag1}")`);
    // Template should still be visible (it has this tag)
    const card = page.locator(`.template-card:has(.name:has-text("${templateName}"))`).first();
    await expect(card).toBeVisible();
    // The active tag should be marked
    await expect(page.locator('.tag-filter-bar a.tag.active')).toContainText(tag1);
  });

  test('edit existing template', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui/templates`);
    const card = page.locator(`.template-card:has(.name:has-text("${templateName}"))`).first();
    await card.locator('a:has-text("Edytuj")').click();
    await expect(page.locator('#designer-app')).toBeVisible();

    // The name field should have the current name
    await expect(page.locator('#template-name')).toHaveValue(templateName);

    // Update the name
    await page.fill('#template-name', updatedName);

    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/ui/actions/templates/') && r.request().method() === 'PUT'
    );
    await page.click('#save-template');
    const response = await responsePromise;
    expect(response.status()).toBeLessThan(400);

    // After save, navigates back to list
    await expect(page.locator('h1')).toContainText('Szablony');
    const updatedCard = page.locator(`.template-card:has(.name:has-text("${updatedName}"))`).first();
    await expect(updatedCard).toBeVisible();
  });

  test('delete template', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui/templates`);
    const card = page.locator(`.template-card:has(.name:has-text("${updatedName}"))`).first();
    await expect(card).toBeVisible();

    page.on('dialog', dialog => dialog.accept());
    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/ui/actions/templates/') && r.request().method() === 'DELETE'
    );
    await card.locator('button:has-text("Usuń")').click();
    await responsePromise;

    // After deletion, verify the template is gone
    await expect(page.locator(`.template-card:has(.name:has-text("${updatedName}"))`)).toHaveCount(0);
  });
});
