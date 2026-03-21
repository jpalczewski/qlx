import { test, expect } from '../fixtures/app';

test.describe('Inventory management', () => {
  test.describe.configure({ mode: 'serial' });

  let containerName: string;
  let subContainerName: string;
  let itemName: string;

  test('create root container', async ({ page, app }) => {
    containerName = `Test Container ${Date.now()}`;
    await page.goto(`${app.baseURL}/ui`);
    await expect(page.locator('h1')).toContainText('Kontenery');

    await page.click('summary:has-text("Dodaj kontener")');
    await page.fill('input[name="name"]', containerName);
    await page.fill('textarea[name="description"]', 'E2E test container');

    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/ui/actions/containers') && r.request().method() === 'POST'
    );
    await page.click('button:has-text("Utwórz")');
    await responsePromise;

    // After creation, the app navigates into the new container
    await expect(page.locator('h2')).toContainText(containerName);
  });

  test('navigate into container from root', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui`);
    await expect(page.locator('.container-list')).toContainText(containerName);
    await page.click(`.container-item:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);
  });

  test('create sub-container', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui`);
    await page.click(`.container-item:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);

    subContainerName = `Sub ${Date.now()}`;
    await page.click('summary:has-text("Dodaj kontener")');
    await page.fill('input[name="name"]', subContainerName);

    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/ui/actions/containers') && r.request().method() === 'POST'
    );
    await page.click('button:has-text("Utwórz")');
    await responsePromise;

    // After creation, navigates into the new sub-container
    await expect(page.locator('h2')).toContainText(subContainerName);
  });

  test('create item in container', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui`);
    await page.click(`.container-item:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);

    itemName = `Item ${Date.now()}`;
    await page.click('summary:has-text("Dodaj przedmiot")');
    const itemForm = page.locator('form[hx-post="/ui/actions/items"]');
    await itemForm.locator('input[name="name"]').fill(itemName);
    await itemForm.locator('textarea[name="description"]').fill('E2E test item');

    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/ui/actions/items') && r.request().method() === 'POST'
    );
    await page.click('form:has(input[name="container_id"]) button:has-text("Utwórz")');
    await responsePromise;

    await expect(page.locator('.item-list')).toContainText(itemName);
  });

  test('navigate to item detail', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui`);
    await page.click(`.container-item:has-text("${containerName}")`);
    await page.click(`.item-item:has-text("${itemName}")`);
    await expect(page.locator('h1')).toContainText(itemName);
    await expect(page.locator('.description')).toContainText('E2E test item');
  });

  test('navigate via breadcrumbs', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui`);
    await page.click(`.container-item:has-text("${containerName}")`);
    await page.click(`.item-item:has-text("${itemName}")`);

    await page.click(`.breadcrumb a:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);
  });

  test('attempt delete non-empty container shows no delete button', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui`);
    await page.click(`.container-item:has-text("${containerName}")`);
    // Container has sub-container and item, delete button should not be visible
    await expect(page.locator('button:has-text("Usuń kontener")')).not.toBeVisible();
  });

  test('delete item', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui`);
    await page.click(`.container-item:has-text("${containerName}")`);
    await page.click(`.item-item:has-text("${itemName}")`);

    page.on('dialog', dialog => dialog.accept());
    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/ui/actions/items/') && r.request().method() === 'DELETE'
    );
    await page.click('button:has-text("Usuń")');
    await responsePromise;

    // After deleting the only item, item list is gone and empty state shows
    await expect(page.locator('.items .empty')).toBeVisible();
  });

  test('delete empty sub-container', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui`);
    await page.click(`.container-item:has-text("${containerName}")`);
    await page.click(`.container-item:has-text("${subContainerName}")`);

    page.on('dialog', dialog => dialog.accept());
    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/ui/actions/containers/') && r.request().method() === 'DELETE'
    );
    await page.click('button:has-text("Usuń kontener")');
    await responsePromise;
  });
});
