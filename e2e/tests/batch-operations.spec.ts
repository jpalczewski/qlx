import { test, expect } from '../fixtures/app';

test.describe('Quick Entry', () => {
  test.describe.configure({ mode: 'serial' });

  let containerName: string;

  test('add container via quick entry at root', async ({ page, app }) => {
    containerName = `QE Container ${Date.now()}`;
    await page.goto(`${app.baseURL}/ui`);
    await expect(page.locator('h1')).toContainText('Kontenery');

    await page.fill('.containers .quick-entry input[name="name"]', containerName);

    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/ui/actions/containers') && r.request().method() === 'POST'
    );
    await page.press('.containers .quick-entry input[name="name"]', 'Enter');
    await responsePromise;

    // Container should appear in list without full page reload
    await expect(page.locator('#container-list')).toContainText(containerName);
    // Page title should still show root (no navigation happened)
    await expect(page.locator('h1')).toContainText('Kontenery');
  });

  test('navigate into container and add item via quick entry', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui`);
    await page.click(`#container-list a:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);

    const itemName = `QE Item ${Date.now()}`;
    await page.fill('.items .quick-entry input[name="name"]', itemName);

    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/ui/actions/items') && r.request().method() === 'POST'
    );
    await page.press('.items .quick-entry input[name="name"]', 'Enter');
    await responsePromise;

    await expect(page.locator('#item-list')).toContainText(itemName);
  });

  test('add another item and verify both visible', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui`);
    await page.click(`#container-list a:has-text("${containerName}")`);
    await expect(page.locator('h2')).toContainText(containerName);

    const itemName2 = `QE Item2 ${Date.now()}`;
    await page.fill('.items .quick-entry input[name="name"]', itemName2);

    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/ui/actions/items') && r.request().method() === 'POST'
    );
    await page.press('.items .quick-entry input[name="name"]', 'Enter');
    await responsePromise;

    await expect(page.locator('#item-list')).toContainText(itemName2);
    // Both items visible (at least 2 <li> that are not empty-state)
    const itemCount = await page.locator('#item-list li:not(.empty-state)').count();
    expect(itemCount).toBeGreaterThanOrEqual(2);
  });
});

test.describe('Bulk Operations via API', () => {
  test.describe.configure({ mode: 'serial' });

  let container1Id: string;
  let container2Id: string;
  let item1Id: string;
  let item2Id: string;
  let item3Id: string;

  test('setup: create containers and items', async ({ request, app }) => {
    const c1 = await request.post(`${app.baseURL}/api/containers`, {
      data: { name: 'BulkSource' },
    });
    container1Id = (await c1.json()).id;

    const c2 = await request.post(`${app.baseURL}/api/containers`, {
      data: { name: 'BulkTarget' },
    });
    container2Id = (await c2.json()).id;

    const i1 = await request.post(`${app.baseURL}/api/items`, {
      data: { name: 'BulkItem1', container_id: container1Id },
    });
    item1Id = (await i1.json()).id;

    const i2 = await request.post(`${app.baseURL}/api/items`, {
      data: { name: 'BulkItem2', container_id: container1Id },
    });
    item2Id = (await i2.json()).id;

    const i3 = await request.post(`${app.baseURL}/api/items`, {
      data: { name: 'BulkItem3', container_id: container1Id },
    });
    item3Id = (await i3.json()).id;
  });

  test('bulk move items to another container', async ({ request, app }) => {
    const res = await request.post(`${app.baseURL}/api/bulk/move`, {
      data: {
        ids: [
          { id: item1Id, type: 'item' },
          { id: item2Id, type: 'item' },
        ],
        target_container_id: container2Id,
      },
    });
    expect(res.status()).toBe(200);

    // Verify items moved to container2
    const c2Items = await request.get(`${app.baseURL}/api/containers/${container2Id}/items`);
    const body = await c2Items.json();
    const itemIds = body.items.map((i: any) => i.id);
    expect(itemIds).toContain(item1Id);
    expect(itemIds).toContain(item2Id);

    // Verify item3 still in container1
    const c1Items = await request.get(`${app.baseURL}/api/containers/${container1Id}/items`);
    const body1 = await c1Items.json();
    const itemIds1 = body1.items.map((i: any) => i.id);
    expect(itemIds1).toContain(item3Id);
    expect(itemIds1).not.toContain(item1Id);
  });

  test('bulk delete item', async ({ request, app }) => {
    const res = await request.post(`${app.baseURL}/api/bulk/delete`, {
      data: {
        ids: [{ id: item3Id, type: 'item' }],
      },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.deleted.length).toBeGreaterThanOrEqual(1);

    // Verify item3 is gone
    const itemRes = await request.get(`${app.baseURL}/api/items/${item3Id}`);
    expect(itemRes.status()).toBe(404);
  });

  test('bulk tag items', async ({ request, app }) => {
    // Create a tag
    const tagRes = await request.post(`${app.baseURL}/api/tags`, {
      data: { name: 'BulkTag' },
    });
    expect(tagRes.status()).toBe(201);
    const tag = await tagRes.json();

    // Bulk tag items
    const bulkRes = await request.post(`${app.baseURL}/api/bulk/tags`, {
      data: {
        ids: [
          { id: item1Id, type: 'item' },
          { id: item2Id, type: 'item' },
        ],
        tag_id: tag.id,
      },
    });
    expect(bulkRes.status()).toBe(200);

    // Verify both items have the tag
    const i1Res = await request.get(`${app.baseURL}/api/items/${item1Id}`);
    const i1 = await i1Res.json();
    expect(i1.item.tag_ids).toContain(tag.id);

    const i2Res = await request.get(`${app.baseURL}/api/items/${item2Id}`);
    const i2 = await i2Res.json();
    expect(i2.item.tag_ids).toContain(tag.id);
  });
});

test.describe('Search', () => {
  test.describe.configure({ mode: 'serial' });

  let containerId: string;

  test('setup: create container and item for search', async ({ request, app }) => {
    const cRes = await request.post(`${app.baseURL}/api/containers`, {
      data: { name: 'Elektronika' },
    });
    expect(cRes.status()).toBe(201);
    containerId = (await cRes.json()).id;

    const iRes = await request.post(`${app.baseURL}/api/items`, {
      data: { name: 'Arduino Nano', container_id: containerId },
    });
    expect(iRes.status()).toBe(201);
  });

  test('API search finds item by partial name', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/api/search?q=ardui`);
    expect(res.status()).toBe(200);
    const body = await res.json();
    const found = body.items.some((i: any) => i.name === 'Arduino Nano');
    expect(found).toBe(true);
  });

  test('API search finds container by partial name', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/api/search?q=elektro`);
    expect(res.status()).toBe(200);
    const body = await res.json();
    const found = body.containers.some((c: any) => c.name === 'Elektronika');
    expect(found).toBe(true);
  });

  test('API search returns empty for nonexistent query', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/api/search?q=nonexistent99999`);
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.items ?? []).toHaveLength(0);
    expect(body.containers ?? []).toHaveLength(0);
    expect(body.tags ?? []).toHaveLength(0);
  });

  test('UI search shows results', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui`);

    // Type in the global search input
    await page.fill('#global-search', 'Arduino');

    // Wait for the HTMX search response
    await page.waitForResponse(r =>
      r.url().includes('/ui/search') && r.request().method() === 'GET'
    );

    // The search results page should show
    await expect(page.locator('#content')).toContainText('Arduino Nano');
  });
});
