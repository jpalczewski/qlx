import { test, expect } from '../fixtures/app';

test.describe('Tag UI improvements', () => {

  test('tag chip links navigate to tag detail page', async ({ page, app }) => {
    // Setup: create a container, item, tag, and assign tag
    const baseURL = app.baseURL;
    await page.request.post(`${baseURL}/api/containers`, {
      data: { name: 'Test Container' }
    });
    const containerResp = await page.request.get(`${baseURL}/api/containers`);
    const containers = await containerResp.json();
    const containerId = containers[0].id;

    const itemResp = await page.request.post(`${baseURL}/api/items`, {
      data: { name: 'Test Item', container_id: containerId }
    });
    const item = await itemResp.json();
    const itemId = item.id;

    await page.request.post(`${baseURL}/api/tags`, {
      data: { name: 'TestTag', color: 'blue', icon: '' }
    });
    const tagsResp = await page.request.get(`${baseURL}/api/tags`);
    const tags = await tagsResp.json();
    const tagId = tags[0].id;

    await page.request.post(`${baseURL}/api/items/${itemId}/tags`, {
      data: { tag_id: tagId }
    });

    // Navigate to container view
    await page.goto(`${baseURL}/ui/containers/${containerId}`, { waitUntil: 'domcontentloaded' });

    // Click the tag chip name
    const tagLink = page.locator('.tag-chip .tag-name', { hasText: 'TestTag' });
    await expect(tagLink).toBeVisible();

    const responsePromise = page.waitForResponse(r =>
      r.url().includes(`/ui/tags/${tagId}`) && r.status() === 200
    );
    await tagLink.click();
    await responsePromise;

    // Verify tag detail page
    await expect(page.locator('h1')).toContainText('TestTag');
    await expect(page.locator('.tag-stats')).toBeVisible();
    await expect(page.locator('.stat-value').first()).toContainText('1'); // 1 item
  });

  test('tag detail page shows statistics and tagged objects', async ({ page, app }) => {
    const baseURL = app.baseURL;

    // Setup: create tag, container, items with tag
    const tagResp = await page.request.post(`${baseURL}/api/tags`, {
      data: { name: 'StatsTag', color: 'green', icon: '' }
    });
    const tag = await tagResp.json();

    const contResp = await page.request.post(`${baseURL}/api/containers`, {
      data: { name: 'Tagged Container' }
    });
    const container = await contResp.json();
    await page.request.post(`${baseURL}/api/containers/${container.id}/tags`, {
      data: { tag_id: tag.id }
    });

    const itemResp = await page.request.post(`${baseURL}/api/items`, {
      data: { name: 'Tagged Item', container_id: container.id, quantity: 5 }
    });
    const item = await itemResp.json();
    await page.request.post(`${baseURL}/api/items/${item.id}/tags`, {
      data: { tag_id: tag.id }
    });

    // Navigate to tag detail
    await page.goto(`${baseURL}/ui/tags/${tag.id}`, { waitUntil: 'domcontentloaded' });

    // Verify stats
    await expect(page.locator('.tag-stats')).toContainText('1'); // 1 item
    await expect(page.locator('.tag-stats')).toContainText('1'); // 1 container
    await expect(page.locator('.tag-stats')).toContainText('5'); // total qty

    // Verify listed objects
    await expect(page.locator('.container-list')).toContainText('Tagged Container');
    await expect(page.locator('.item-list')).toContainText('Tagged Item');
  });

  test('inline + button opens autocomplete and assigns tag', async ({ page, app }) => {
    const baseURL = app.baseURL;

    // Setup
    const tagResp = await page.request.post(`${baseURL}/api/tags`, {
      data: { name: 'InlineTag', color: 'red', icon: '' }
    });
    const tag = await tagResp.json();

    const contResp = await page.request.post(`${baseURL}/api/containers`, {
      data: { name: 'Inline Container' }
    });
    const container = await contResp.json();
    const itemResp = await page.request.post(`${baseURL}/api/items`, {
      data: { name: 'Inline Item', container_id: container.id }
    });
    const item = await itemResp.json();

    // Assign a tag first so + button is visible (tag-chips renders only if TagIDs not empty)
    const otherTag = await (await page.request.post(`${baseURL}/api/tags`, {
      data: { name: 'OtherTag', color: 'blue', icon: '' }
    })).json();
    await page.request.post(`${baseURL}/api/items/${item.id}/tags`, {
      data: { tag_id: otherTag.id }
    });

    // Navigate to container
    await page.goto(`${baseURL}/ui/containers/${container.id}`, { waitUntil: 'domcontentloaded' });

    // Click + button
    const addBtn = page.locator('.tag-add').first();
    await expect(addBtn).toBeVisible();
    await addBtn.click();

    // Input should appear
    const input = page.locator('.tag-ac-input');
    await expect(input).toBeVisible();
    await input.fill('Inline');

    // Dropdown should show InlineTag
    const option = page.locator('.tag-ac-option', { hasText: 'InlineTag' });
    await expect(option).toBeVisible();
    await option.click();

    // Tag chip should appear
    await expect(page.locator('.tag-chip', { hasText: 'InlineTag' })).toBeVisible();
  });

  test('container list shows tag chips', async ({ page, app }) => {
    const baseURL = app.baseURL;

    const tagResp = await page.request.post(`${baseURL}/api/tags`, {
      data: { name: 'ContTag', color: 'purple', icon: '' }
    });
    const tag = await tagResp.json();

    const contResp = await page.request.post(`${baseURL}/api/containers`, {
      data: { name: 'Root' }
    });
    const root = await contResp.json();

    const childResp = await page.request.post(`${baseURL}/api/containers`, {
      data: { name: 'Tagged Child', parent_id: root.id }
    });
    const child = await childResp.json();
    await page.request.post(`${baseURL}/api/containers/${child.id}/tags`, {
      data: { tag_id: tag.id }
    });

    await page.goto(`${baseURL}/ui/containers/${root.id}`, { waitUntil: 'domcontentloaded' });
    await expect(page.locator('.tag-chip', { hasText: 'ContTag' })).toBeVisible();
  });
});
