import { test, expect } from '../fixtures/app';

test.describe('Tag API contracts', () => {

  test('GET non-existent tag returns 404', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/api/tags/00000000-0000-0000-0000-000000000000`);
    expect(res.status()).toBe(404);
  });

  test('DELETE non-existent tag returns 404', async ({ request, app }) => {
    const res = await request.delete(`${app.baseURL}/api/tags/00000000-0000-0000-0000-000000000000`);
    expect(res.status()).toBe(404);
    const body = await res.json();
    expect(body.error).toBe('tag not found');
  });

  test('DELETE parent tag with children returns 409', async ({ request, app }) => {
    const parentRes = await request.post(`${app.baseURL}/api/tags`, {
      data: { name: 'ParentTag' },
    });
    expect(parentRes.status()).toBe(201);
    const parent = await parentRes.json();

    const childRes = await request.post(`${app.baseURL}/api/tags`, {
      data: { name: 'ChildTag', parent_id: parent.id },
    });
    expect(childRes.status()).toBe(201);

    const deleteRes = await request.delete(`${app.baseURL}/api/tags/${parent.id}`);
    expect(deleteRes.status()).toBe(409);
    const body = await deleteRes.json();
    expect(body.error).toBe('tag has children');
  });

  test('POST /api/tags creates tag with correct response', async ({ request, app }) => {
    const res = await request.post(`${app.baseURL}/api/tags`, {
      data: { name: 'NewAPITag' },
    });
    expect(res.status()).toBe(201);
    const tag = await res.json();
    expect(tag.id).toBeTruthy();
    expect(tag.id).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/);
    expect(tag.name).toBe('NewAPITag');
    expect(tag.parent_id).toBe('');
    expect(tag.created_at).toBeTruthy();
  });

  test('PUT /api/tags/{id} updates tag name', async ({ request, app }) => {
    const createRes = await request.post(`${app.baseURL}/api/tags`, {
      data: { name: 'BeforeRename' },
    });
    const tag = await createRes.json();

    const updateRes = await request.put(`${app.baseURL}/api/tags/${tag.id}`, {
      data: { name: 'AfterRename' },
    });
    expect(updateRes.status()).toBe(200);
    const updated = await updateRes.json();
    expect(updated.name).toBe('AfterRename');
    expect(updated.id).toBe(tag.id);
  });

  test('PATCH /api/tags/{id}/move moves tag to new parent', async ({ request, app }) => {
    const parent1Res = await request.post(`${app.baseURL}/api/tags`, {
      data: { name: 'MoveParent1' },
    });
    const parent1 = await parent1Res.json();

    const parent2Res = await request.post(`${app.baseURL}/api/tags`, {
      data: { name: 'MoveParent2' },
    });
    const parent2 = await parent2Res.json();

    const childRes = await request.post(`${app.baseURL}/api/tags`, {
      data: { name: 'MoveChild', parent_id: parent1.id },
    });
    const child = await childRes.json();
    expect(child.parent_id).toBe(parent1.id);

    // Move child to parent2
    const moveRes = await request.patch(`${app.baseURL}/api/tags/${child.id}/move`, {
      data: { parent_id: parent2.id },
    });
    expect(moveRes.status()).toBe(200);

    // Verify child is now under parent2
    const getRes = await request.get(`${app.baseURL}/api/tags/${child.id}`);
    const moved = await getRes.json();
    expect(moved.parent_id).toBe(parent2.id);
  });

  test('GET /api/tags/{id}/descendants returns descendants', async ({ request, app }) => {
    const rootRes = await request.post(`${app.baseURL}/api/tags`, {
      data: { name: 'DescRoot' },
    });
    const root = await rootRes.json();

    const midRes = await request.post(`${app.baseURL}/api/tags`, {
      data: { name: 'DescMid', parent_id: root.id },
    });
    const mid = await midRes.json();

    const leafRes = await request.post(`${app.baseURL}/api/tags`, {
      data: { name: 'DescLeaf', parent_id: mid.id },
    });
    const leaf = await leafRes.json();

    const descRes = await request.get(`${app.baseURL}/api/tags/${root.id}/descendants`);
    expect(descRes.status()).toBe(200);
    const descendants = await descRes.json();

    // descendants endpoint returns an array of tag ID strings, not objects
    expect(descendants).toContain(mid.id);
    expect(descendants).toContain(leaf.id);
    expect(descendants).not.toContain(root.id); // root itself should not be in descendants
  });

  test('PUT non-existent tag returns 404', async ({ request, app }) => {
    const res = await request.put(`${app.baseURL}/api/tags/00000000-0000-0000-0000-000000000000`, {
      data: { name: 'Ghost' },
    });
    expect(res.status()).toBe(404);
    const body = await res.json();
    expect(body.error).toBe('tag not found');
  });

  test('item tag add/remove via API', async ({ request, app }) => {
    const cRes = await request.post(`${app.baseURL}/api/containers`, {
      data: { name: 'TagAPIContainer' },
    });
    const container = await cRes.json();

    const iRes = await request.post(`${app.baseURL}/api/items`, {
      data: { name: 'TagAPIItem', container_id: container.id },
    });
    const item = await iRes.json();

    const tRes = await request.post(`${app.baseURL}/api/tags`, {
      data: { name: 'TagForItem' },
    });
    const tag = await tRes.json();

    // Add tag
    const addRes = await request.post(`${app.baseURL}/api/items/${item.id}/tags`, {
      data: { tag_id: tag.id },
    });
    expect(addRes.status()).toBe(200);

    // Verify
    let getRes = await request.get(`${app.baseURL}/api/items/${item.id}`);
    let body = await getRes.json();
    expect(body.item.tag_ids).toContain(tag.id);

    // Remove tag
    const removeRes = await request.delete(`${app.baseURL}/api/items/${item.id}/tags/${tag.id}`);
    expect(removeRes.status()).toBe(200);

    // Verify removed
    getRes = await request.get(`${app.baseURL}/api/items/${item.id}`);
    body = await getRes.json();
    expect(body.item.tag_ids).not.toContain(tag.id);
  });

  test('add tag to non-existent item returns 404', async ({ request, app }) => {
    const tRes = await request.post(`${app.baseURL}/api/tags`, {
      data: { name: 'OrphanTag' },
    });
    const tag = await tRes.json();

    const res = await request.post(`${app.baseURL}/api/items/00000000-0000-0000-0000-000000000000/tags`, {
      data: { tag_id: tag.id },
    });
    expect(res.status()).toBe(404);
  });
});
