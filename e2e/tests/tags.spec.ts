import { test, expect } from '../fixtures/app';

test.describe('Tags', () => {
  test.describe.configure({ mode: 'serial' });

  let parentTagName: string;
  let childTagName: string;
  let parentTagId: string;
  let childTagId: string;
  let containerId: string;
  let itemId: string;

  test('create a root tag via API and UI child', async ({ request, page, app }) => {
    // Create parent tag via API (reliable)
    parentTagName = `RootTag ${Date.now()}`;
    const parentRes = await request.post(`${app.baseURL}/api/tags`, {
      form: { name: parentTagName, parent_id: '' }
    });
    const parent = await parentRes.json();
    parentTagId = parent.id;

    // Navigate to parent tag page directly (full page load, no HTMX)
    await page.goto(`${app.baseURL}/ui/tags?parent_id=${parentTagId}`, { waitUntil: 'domcontentloaded' });
    await expect(page.locator('h1')).toContainText(parentTagName);

    // Create child via quick entry
    childTagName = `ChildTag ${Date.now()}`;
    const nameInput = page.locator('.quick-entry input[name="name"]');
    await nameInput.waitFor({ state: 'visible' });
    await nameInput.fill(childTagName);

    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/ui/actions/tags') && r.request().method() === 'POST'
    );
    await nameInput.press('Enter');
    await responsePromise;

    await expect(page.locator('#tag-list')).toContainText(childTagName);
  });

  test('verify breadcrumb shows parent path', async ({ request, page, app }) => {
    // Get child tag ID if not yet set
    if (!childTagId) {
      const tagsRes = await request.get(`${app.baseURL}/api/tags`);
      const allTags = await tagsRes.json();
      const childTag = allTags.find((t: any) => t.name === childTagName);
      expect(childTag).toBeTruthy();
      childTagId = childTag.id;
    }

    // Navigate to child tag page directly
    await page.goto(`${app.baseURL}/ui/tags?parent_id=${childTagId}`, { waitUntil: 'domcontentloaded' });
    await expect(page.locator('h1')).toContainText(childTagName);

    // Breadcrumb should contain the parent tag name
    await expect(page.locator('.breadcrumb')).toContainText(parentTagName);
  });

  test('create container and item via API, then assign tag', async ({ request, app }) => {
    // Get child tag ID via API
    const tagsRes = await request.get(`${app.baseURL}/api/tags`);
    const allTags = await tagsRes.json();
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
