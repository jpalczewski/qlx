import { test, expect } from '../fixtures/app';

test.describe('Tags', () => {
  test.describe.configure({ mode: 'serial' });

  let parentTagName: string;
  let childTagName: string;
  let parentTagId: string;
  let childTagId: string;
  let containerId: string;
  let itemId: string;

  test('create a root tag via UI', async ({ page, app }) => {
    parentTagName = `RootTag ${Date.now()}`;
    await page.goto(`${app.baseURL}/ui/tags`);
    await expect(page.locator('h1')).toContainText('Tagi');

    await page.fill('.quick-entry input[name="name"]', parentTagName);

    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/ui/actions/tags') && r.request().method() === 'POST'
    );
    await page.press('.quick-entry input[name="name"]', 'Enter');
    await responsePromise;

    // Tag should appear in the list
    await expect(page.locator('#tag-list')).toContainText(parentTagName);
  });

  test('navigate into parent tag and create child via UI', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui/tags`);
    await page.click(`#tag-list a:has-text("${parentTagName}")`);
    await expect(page.locator('h1')).toContainText(parentTagName);

    childTagName = `ChildTag ${Date.now()}`;
    await page.fill('.quick-entry input[name="name"]', childTagName);

    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/ui/actions/tags') && r.request().method() === 'POST'
    );
    await page.press('.quick-entry input[name="name"]', 'Enter');
    await responsePromise;

    await expect(page.locator('#tag-list')).toContainText(childTagName);
  });

  test('verify breadcrumb shows parent path', async ({ page, app }) => {
    // Navigate to parent tag page first
    await page.goto(`${app.baseURL}/ui/tags`);
    await page.click(`#tag-list a:has-text("${parentTagName}")`);
    await expect(page.locator('h1')).toContainText(parentTagName);

    // Navigate into child
    await page.click(`#tag-list a:has-text("${childTagName}")`);
    await expect(page.locator('h1')).toContainText(childTagName);

    // Breadcrumb should contain the parent tag name
    await expect(page.locator('.breadcrumb')).toContainText(parentTagName);
  });

  test('create container and item via API, then assign tag', async ({ request, app }) => {
    // Get the parent tag ID
    const tagsRes = await request.get(`${app.baseURL}/api/tags`);
    const allTags = await tagsRes.json();
    const parentTag = allTags.find((t: any) => t.name === parentTagName);
    expect(parentTag).toBeTruthy();
    parentTagId = parentTag.id;

    // Get child tag
    const childTag = allTags.find((t: any) => t.name === childTagName);
    expect(childTag).toBeTruthy();
    childTagId = childTag.id;

    // Create container
    const containerRes = await request.post(`${app.baseURL}/api/containers`, {
      data: { name: 'TagTestContainer' },
    });
    expect(containerRes.status()).toBe(201);
    const container = await containerRes.json();
    containerId = container.id;

    // Create item
    const itemRes = await request.post(`${app.baseURL}/api/items`, {
      data: { name: 'TagTestItem', container_id: containerId },
    });
    expect(itemRes.status()).toBe(201);
    const item = await itemRes.json();
    itemId = item.id;

    // Assign parent tag to item
    const tagRes = await request.post(`${app.baseURL}/api/items/${itemId}/tags`, {
      data: { tag_id: parentTagId },
    });
    expect(tagRes.status()).toBe(200);
  });

  test('verify tag assignment on item', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/api/items/${itemId}`);
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.item.tag_ids).toContain(parentTagId);
  });

  test('delete leaf tag and verify removal from item', async ({ request, app }) => {
    // Assign child tag to item first
    const addRes = await request.post(`${app.baseURL}/api/items/${itemId}/tags`, {
      data: { tag_id: childTagId },
    });
    expect(addRes.status()).toBe(200);

    // Verify it's there
    let itemRes = await request.get(`${app.baseURL}/api/items/${itemId}`);
    let body = await itemRes.json();
    expect(body.item.tag_ids).toContain(childTagId);

    // Delete the child tag (leaf)
    const delRes = await request.delete(`${app.baseURL}/api/tags/${childTagId}`);
    expect(delRes.status()).toBe(200);

    // Verify tag is removed from item's tag_ids
    itemRes = await request.get(`${app.baseURL}/api/items/${itemId}`);
    body = await itemRes.json();
    expect(body.item.tag_ids).not.toContain(childTagId);
  });

  test('search for tag by name', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/api/search?q=${encodeURIComponent(parentTagName.substring(0, 8))}`);
    expect(res.status()).toBe(200);
    const body = await res.json();
    const found = body.tags.some((t: any) => t.name === parentTagName);
    expect(found).toBe(true);
  });
});
