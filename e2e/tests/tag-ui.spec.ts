import { test, expect } from '../fixtures/app';

test.describe('Tag pill styling', () => {

  test('tags page shows tag pills linking to tag detail', async ({ page, app }) => {
    const baseURL = app.baseURL;

    const tagResp = await page.request.post(`${baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'PillTag', color: 'blue', icon: '' }
    });
    const tag = await tagResp.json();

    await page.goto(`${baseURL}/tags`, { waitUntil: 'domcontentloaded' });

    const pill = page.locator('.tag-pill', { hasText: 'PillTag' });
    await expect(pill).toBeVisible();

    // Pill links to /tags/{id} (tag detail), not /tags?parent_id=
    await expect(pill).toHaveAttribute('href', `/tags/${tag.id}`);
  });

  test('tags page shows child count badge for tags with children', async ({ page, app }) => {
    const baseURL = app.baseURL;

    const parentResp = await page.request.post(`${baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'BadgeParent', color: 'green', icon: '' }
    });
    const parent = await parentResp.json();

    await page.request.post(`${baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'BadgeChild1', parent_id: parent.id }
    });
    await page.request.post(`${baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'BadgeChild2', parent_id: parent.id }
    });

    await page.goto(`${baseURL}/tags`, { waitUntil: 'domcontentloaded' });

    const badge = page.locator('.tag-pill', { hasText: 'BadgeParent' }).locator('.tag-pill-badge');
    await expect(badge).toBeVisible();
    await expect(badge).toContainText('2');
  });

  test('tag chip shows parent name and links to parent tag', async ({ page, app }) => {
    const baseURL = app.baseURL;

    const parentResp = await page.request.post(`${baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'ParentVis', color: 'red', icon: '' }
    });
    const parentTag = await parentResp.json();

    const childResp = await page.request.post(`${baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'ChildVis', parent_id: parentTag.id }
    });
    const childTag = await childResp.json();

    // Create container + item, assign child tag
    const contResp = await page.request.post(`${baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'ParentVisContainer' }
    });
    const container = await contResp.json();

    const itemResp = await page.request.post(`${baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'ParentVisItem', container_id: container.id }
    });
    const item = await itemResp.json();

    await page.request.post(`${baseURL}/items/${item.id}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { tag_id: childTag.id }
    });

    // Navigate to item detail
    await page.goto(`${baseURL}/items/${item.id}`, { waitUntil: 'domcontentloaded' });

    // Parent name visible in chip
    const parentLink = page.locator('.tag-chip .tag-parent', { hasText: 'ParentVis' });
    await expect(parentLink).toBeVisible();

    // Parent link navigates to parent tag detail
    await expect(parentLink).toHaveAttribute('href', `/tags/${parentTag.id}`);
  });

  test('tag detail children section uses pill styling', async ({ page, app }) => {
    const baseURL = app.baseURL;

    const parentResp = await page.request.post(`${baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'DetailParent', color: 'teal', icon: '' }
    });
    const parent = await parentResp.json();

    const childResp = await page.request.post(`${baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'DetailChild', parent_id: parent.id, color: 'purple', icon: '' }
    });
    const child = await childResp.json();

    await page.goto(`${baseURL}/tags/${parent.id}`, { waitUntil: 'domcontentloaded' });

    const pill = page.locator('.tag-pill-list .tag-pill', { hasText: 'DetailChild' });
    await expect(pill).toBeVisible();
    await expect(pill).toHaveAttribute('href', `/tags/${child.id}`);
  });
});

test.describe('Tag UI improvements', () => {

  test('tag chip links navigate to tag detail page', async ({ page, app }) => {
    // Setup: create a container, item, tag, and assign tag
    const baseURL = app.baseURL;
    const containerResp = await page.request.post(`${baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Test Container' }
    });
    const container = await containerResp.json();
    const containerId = container.id;

    const itemResp = await page.request.post(`${baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Test Item', container_id: containerId }
    });
    const item = await itemResp.json();
    const itemId = item.id;

    const tagResp = await page.request.post(`${baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'TestTag', color: 'blue', icon: '' }
    });
    const tagObj = await tagResp.json();
    const tagId = tagObj.id;

    await page.request.post(`${baseURL}/items/${itemId}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { tag_id: tagId }
    });

    // Navigate to container view
    await page.goto(`${baseURL}/containers/${containerId}`, { waitUntil: 'domcontentloaded' });

    // Click the tag chip name
    const tagLink = page.locator('.tag-chip .tag-name', { hasText: 'TestTag' });
    await expect(tagLink).toBeVisible();

    const responsePromise = page.waitForResponse(r =>
      r.url().includes(`/tags/${tagId}`) && r.status() === 200
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
    const tagResp = await page.request.post(`${baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'StatsTag', color: 'green', icon: '' }
    });
    const tag = await tagResp.json();

    const contResp = await page.request.post(`${baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Tagged Container' }
    });
    const container = await contResp.json();
    await page.request.post(`${baseURL}/containers/${container.id}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { tag_id: tag.id }
    });

    const itemResp = await page.request.post(`${baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Tagged Item', container_id: container.id, quantity: 5 }
    });
    const item = await itemResp.json();
    await page.request.post(`${baseURL}/items/${item.id}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { tag_id: tag.id }
    });

    // Navigate to tag detail
    await page.goto(`${baseURL}/tags/${tag.id}`, { waitUntil: 'domcontentloaded' });

    // Verify stats
    await expect(page.locator('.tag-stats')).toContainText('1'); // 1 item
    await expect(page.locator('.tag-stats')).toContainText('1'); // 1 container
    await expect(page.locator('.tag-stats')).toContainText('5'); // total qty

    // Verify listed objects
    await expect(page.locator('.container-list')).toContainText('Tagged Container');
    await expect(page.locator('.item-list')).toContainText('Tagged Item');
  });

  test('container detail always shows tag + button even with no tags', async ({ page, app }) => {
    const baseURL = app.baseURL;

    const contResp = await page.request.post(`${baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Empty Tag Container' }
    });
    const container = await contResp.json();

    await page.goto(`${baseURL}/containers/${container.id}`, { waitUntil: 'domcontentloaded' });

    // .container-tags section must exist with a + button, regardless of tag count
    const addBtn = page.locator('.container-tags .tag-add');
    await expect(addBtn).toBeVisible();
  });

  test('add tag from container detail + button', async ({ page, app }) => {
    const baseURL = app.baseURL;

    const tagResp = await page.request.post(`${baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'DetailTag', color: 'teal', icon: '' }
    });
    const tag = await tagResp.json();

    const contResp = await page.request.post(`${baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Detail Container' }
    });
    const container = await contResp.json();

    await page.goto(`${baseURL}/containers/${container.id}`, { waitUntil: 'domcontentloaded' });

    // Click + in the container detail header tag section
    const addBtn = page.locator('.container-tags .tag-add');
    await expect(addBtn).toBeVisible();
    await addBtn.click();

    // Autocomplete input appears
    const input = page.locator('.tag-ac-input');
    await expect(input).toBeVisible();
    await input.fill('Detail');

    // Select matching option
    const option = page.locator('.tag-ac-option', { hasText: 'DetailTag' });
    await expect(option).toBeVisible();
    await option.click();

    // Chip appears in the container detail header
    await expect(page.locator('.container-tags .tag-chip', { hasText: 'DetailTag' })).toBeVisible();
  });

  test('item list does not show tag + button when item has no tags', async ({ page, app }) => {
    const baseURL = app.baseURL;

    const contResp = await page.request.post(`${baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Parent Container' }
    });
    const container = await contResp.json();

    await page.request.post(`${baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Untagged Item', container_id: container.id }
    });

    await page.goto(`${baseURL}/containers/${container.id}`, { waitUntil: 'domcontentloaded' });

    // Item appears in list
    await expect(page.locator('.item-list')).toContainText('Untagged Item');

    // No + button inside the item list (item has no tags)
    await expect(page.locator('.item-list .tag-add')).toHaveCount(0);
  });

  test('item detail always shows tag + button even with no tags', async ({ page, app }) => {
    const baseURL = app.baseURL;

    const contResp = await page.request.post(`${baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Container' }
    });
    const container = await contResp.json();

    const itemResp = await page.request.post(`${baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Bare Item', container_id: container.id }
    });
    const item = await itemResp.json();

    await page.goto(`${baseURL}/items/${item.id}`, { waitUntil: 'domcontentloaded' });

    // .item-tags section must exist with a + button
    const addBtn = page.locator('.item-tags .tag-add');
    await expect(addBtn).toBeVisible();
  });

  test('inline + button opens autocomplete and assigns tag (item in list)', async ({ page, app }) => {
    const baseURL = app.baseURL;

    const tagResp = await page.request.post(`${baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'InlineTag', color: 'red', icon: '' }
    });
    const tag = await tagResp.json();

    const contResp = await page.request.post(`${baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Inline Container' }
    });
    const container = await contResp.json();
    const itemResp = await page.request.post(`${baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Inline Item', container_id: container.id }
    });
    const item = await itemResp.json();

    // Pre-assign a tag so the + button appears in the item list
    const seedTag = await (await page.request.post(`${baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'SeedTag', color: 'blue', icon: '' }
    })).json();
    await page.request.post(`${baseURL}/items/${item.id}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { tag_id: seedTag.id }
    });

    await page.goto(`${baseURL}/containers/${container.id}`, { waitUntil: 'domcontentloaded' });

    // Click + button in the item list row
    const addBtn = page.locator('.item-list .tag-add').first();
    await expect(addBtn).toBeVisible();
    await addBtn.click();

    const input = page.locator('.tag-ac-input');
    await expect(input).toBeVisible();
    await input.fill('Inline');

    const option = page.locator('.tag-ac-option', { hasText: 'InlineTag' });
    await expect(option).toBeVisible();
    await option.click();

    await expect(page.locator('.tag-chip', { hasText: 'InlineTag' })).toBeVisible();
  });

  test('container list shows tag chips', async ({ page, app }) => {
    const baseURL = app.baseURL;

    const tagResp = await page.request.post(`${baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'ContTag', color: 'purple', icon: '' }
    });
    const tag = await tagResp.json();

    const contResp = await page.request.post(`${baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Root' }
    });
    const root = await contResp.json();

    const childResp = await page.request.post(`${baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Tagged Child', parent_id: root.id }
    });
    const child = await childResp.json();
    await page.request.post(`${baseURL}/containers/${child.id}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { tag_id: tag.id }
    });

    await page.goto(`${baseURL}/containers/${root.id}`, { waitUntil: 'domcontentloaded' });
    await expect(page.locator('.tag-chip', { hasText: 'ContTag' })).toBeVisible();
  });
});
