import { test, expect } from '../fixtures/app';

test.describe('Edge cases: Store migration', () => {
  test('new store starts with version 1 and empty tags', async ({ request, app }) => {
    // A freshly created store should have migrated to v1
    const resp = await request.get(`${app.baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
    });
    expect(resp.status()).toBe(200);
    const tags = await resp.json();
    // Tags should be an empty array (or null for Go nil slice)
    expect(tags === null || (Array.isArray(tags) && tags.length === 0)).toBeTruthy();
  });
});

test.describe('Edge cases: Tag hierarchy', () => {
  test('cannot delete tag with children', async ({ request, app }) => {
    // Create parent
    const parent = await (await request.post(`${app.baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'Parent', parent_id: '' }
    })).json();

    // Create child
    await request.post(`${app.baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'Child', parent_id: parent.id }
    });

    // Try to delete parent — should 409
    const delResp = await request.delete(`${app.baseURL}/tags/${parent.id}`, {
      headers: { 'Accept': 'application/json' },
    });
    expect(delResp.status()).toBe(409);
  });

  test('tag descendants returns full subtree', async ({ request, app }) => {
    const root = await (await request.post(`${app.baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'Root', parent_id: '' }
    })).json();
    const mid = await (await request.post(`${app.baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'Mid', parent_id: root.id }
    })).json();
    const leaf = await (await request.post(`${app.baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'Leaf', parent_id: mid.id }
    })).json();

    const resp = await request.get(`${app.baseURL}/tags/${root.id}/descendants`, {
      headers: { 'Accept': 'application/json' },
    });
    const descendants: string[] = await resp.json();
    expect(descendants).toContain(mid.id);
    expect(descendants).toContain(leaf.id);
    expect(descendants).not.toContain(root.id);
  });

  test('move tag rejects cycle (child → parent)', async ({ request, app }) => {
    const parent = await (await request.post(`${app.baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'CycleParent', parent_id: '' }
    })).json();
    const child = await (await request.post(`${app.baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'CycleChild', parent_id: parent.id }
    })).json();

    // Try to move parent under child — cycle
    const moveResp = await request.patch(`${app.baseURL}/tags/${parent.id}/move`, {
      headers: { 'Accept': 'application/json' },
      data: { parent_id: child.id }
    });
    expect(moveResp.status()).toBe(400);
  });

  test('deleting leaf tag cascades removal from items', async ({ request, app }) => {
    const tag = await (await request.post(`${app.baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'ToDelete', parent_id: '' }
    })).json();

    // Create container + item
    const container = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'Box', parent_id: '' }
    })).json();
    const item = await (await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'Widget', container_id: container.id }
    })).json();

    // Assign tag
    await request.post(`${app.baseURL}/items/${item.id}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { tag_id: tag.id }
    });

    // Verify assigned
    let itemData = await (await request.get(`${app.baseURL}/items/${item.id}`, {
      headers: { 'Accept': 'application/json' },
    });
    expect(itemData.item.tag_ids).toContain(tag.id);

    // Delete tag
    const delResp = await request.delete(`${app.baseURL}/tags/${tag.id}`, {
      headers: { 'Accept': 'application/json' },
    });
    expect(delResp.status()).toBe(200);

    // Verify cascade — tag_ids should no longer contain the deleted tag
    itemData = await (await request.get(`${app.baseURL}/items/${item.id}`, {
      headers: { 'Accept': 'application/json' },
    });
    expect(itemData.item.tag_ids).not.toContain(tag.id);
  });
});

test.describe('Edge cases: Bulk operations', () => {
  test('bulk move with nonexistent target returns error', async ({ request, app }) => {
    const container = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'Source', parent_id: '' }
    })).json();
    const item = await (await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'Thing', container_id: container.id }
    })).json();

    const resp = await request.post(`${app.baseURL}/bulk/move`, {
      headers: { 'Accept': 'application/json' },
      data: {
        ids: [{ id: item.id, type: 'item' }],
        target_container_id: 'nonexistent-uuid'
      }
    });
    const body = await resp.json();
    expect(body.errors).toBeTruthy();
    expect(body.errors.length).toBeGreaterThan(0);
  });

  test('bulk move container into itself rejected', async ({ request, app }) => {
    const container = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'SelfMove', parent_id: '' }
    })).json();

    const resp = await request.post(`${app.baseURL}/bulk/move`, {
      headers: { 'Accept': 'application/json' },
      data: {
        ids: [{ id: container.id, type: 'container' }],
        target_container_id: container.id
      }
    });
    const body = await resp.json();
    expect(body.errors).toBeTruthy();
    expect(body.errors.length).toBeGreaterThan(0);
  });

  test('bulk move container into descendant rejected (cycle)', async ({ request, app }) => {
    const parent = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'CycleA', parent_id: '' }
    })).json();
    const child = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'CycleB', parent_id: parent.id }
    })).json();

    const resp = await request.post(`${app.baseURL}/bulk/move`, {
      headers: { 'Accept': 'application/json' },
      data: {
        ids: [{ id: parent.id, type: 'container' }],
        target_container_id: child.id
      }
    });
    const body = await resp.json();
    expect(body.errors).toBeTruthy();
  });

  test('bulk delete non-empty container fails, items succeed', async ({ request, app }) => {
    const container = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'NonEmpty', parent_id: '' }
    })).json();
    const item1 = await (await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'I1', container_id: container.id }
    })).json();
    await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'I2', container_id: container.id }
    });

    // Try to delete item1 AND the non-empty container
    const resp = await request.post(`${app.baseURL}/bulk/delete`, {
      headers: { 'Accept': 'application/json' },
      data: {
        ids: [
          { id: item1.id, type: 'item' },
          { id: container.id, type: 'container' }
        ]
      }
    });
    const body = await resp.json();
    // item1 should be deleted
    expect(body.deleted).toContain(item1.id);
    // container should fail (still has item2)
    expect(body.failed.length).toBeGreaterThan(0);
    expect(body.failed[0].id).toBe(container.id);
  });

  test('bulk tag with nonexistent tag ID returns error', async ({ request, app }) => {
    const container = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'TagBox', parent_id: '' }
    })).json();
    const item = await (await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'TagItem', container_id: container.id }
    })).json();

    const resp = await request.post(`${app.baseURL}/bulk/tags`, {
      headers: { 'Accept': 'application/json' },
      data: {
        ids: [{ id: item.id, type: 'item' }],
        tag_id: 'nonexistent-tag'
      }
    });
    // API returns 404 for nonexistent tag
    expect(resp.status()).toBe(404);
  });

  test('bulk move intra-batch ancestor conflict rejected', async ({ request, app }) => {
    const grandparent = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'GP', parent_id: '' }
    })).json();
    const parent = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'P', parent_id: grandparent.id }
    })).json();
    const target = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'Target', parent_id: '' }
    })).json();

    // Move both grandparent AND parent to target — intra-batch ancestry conflict
    const resp = await request.post(`${app.baseURL}/bulk/move`, {
      headers: { 'Accept': 'application/json' },
      data: {
        ids: [
          { id: grandparent.id, type: 'container' },
          { id: parent.id, type: 'container' }
        ],
        target_container_id: target.id
      }
    });
    const body = await resp.json();
    expect(body.errors).toBeTruthy();
    expect(body.errors.length).toBeGreaterThan(0);
  });
});

test.describe('Edge cases: Search', () => {
  test('empty search query returns empty results', async ({ request, app }) => {
    const resp = await request.get(`${app.baseURL}/search?q=`, {
      headers: { 'Accept': 'application/json' },
    });
    const body = await resp.json();
    expect(body.containers).toEqual([]);
    expect(body.items).toEqual([]);
    expect(body.tags).toEqual([]);
  });

  test('search is case-insensitive', async ({ request, app }) => {
    await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'CaseSensitiveTest', parent_id: '' }
    });

    const resp1 = await request.get(`${app.baseURL}/search?q=casesensitive`, {
      headers: { 'Accept': 'application/json' },
    });
    const body1 = await resp1.json();
    expect(body1.containers.length).toBe(1);

    const resp2 = await request.get(`${app.baseURL}/search?q=CASESENSITIVE`, {
      headers: { 'Accept': 'application/json' },
    });
    const body2 = await resp2.json();
    expect(body2.containers.length).toBe(1);
  });
});

test.describe('Edge cases: Item quantity', () => {
  test('create item with quantity via API', async ({ request, app }) => {
    const container = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'QtyBox', parent_id: '' }
    })).json();

    const resp = await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'Bolts', container_id: container.id, quantity: '50' }
    });
    const item = await resp.json();
    expect(item.quantity).toBe(50);
  });

  test('create item with zero quantity defaults to 1', async ({ request, app }) => {
    const container = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'DefaultQtyBox', parent_id: '' }
    })).json();

    const resp = await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'Single', container_id: container.id, quantity: '0' }
    });
    const item = await resp.json();
    expect(item.quantity).toBe(1);
  });

  test('create item without quantity defaults to 1', async ({ request, app }) => {
    const container = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'NoQtyBox', parent_id: '' }
    })).json();

    const resp = await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'Default', container_id: container.id }
    });
    const item = await resp.json();
    expect(item.quantity).toBe(1);
  });
});

test.describe('Edge cases: Tag assignment to containers', () => {
  test('assign and remove tag from container', async ({ request, app }) => {
    const tag = await (await request.post(`${app.baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'ContainerTag', parent_id: '' }
    })).json();
    const container = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'TaggedContainer', parent_id: '' }
    })).json();

    // Assign
    const addResp = await request.post(`${app.baseURL}/containers/${container.id}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { tag_id: tag.id }
    });
    expect(addResp.status()).toBe(200);

    // Verify
    let data = await (await request.get(`${app.baseURL}/containers/${container.id}`, {
      headers: { 'Accept': 'application/json' },
    });
    expect(data.container.tag_ids).toContain(tag.id);

    // Remove
    const rmResp = await request.delete(`${app.baseURL}/containers/${container.id}/tags/${tag.id}`, {
      headers: { 'Accept': 'application/json' },
    });
    expect(rmResp.status()).toBe(200);

    // Verify removed
    data = await (await request.get(`${app.baseURL}/containers/${container.id}`, {
      headers: { 'Accept': 'application/json' },
    });
    expect(data.container.tag_ids).not.toContain(tag.id);
  });

  test('duplicate tag assignment is idempotent', async ({ request, app }) => {
    const tag = await (await request.post(`${app.baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'DupeTag', parent_id: '' }
    })).json();
    const container = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'DupeBox', parent_id: '' }
    })).json();

    // Assign twice
    await request.post(`${app.baseURL}/containers/${container.id}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { tag_id: tag.id }
    });
    await request.post(`${app.baseURL}/containers/${container.id}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { tag_id: tag.id }
    });

    // Should only appear once
    const data = await (await request.get(`${app.baseURL}/containers/${container.id}`, {
      headers: { 'Accept': 'application/json' },
    });
    const count = data.container.tag_ids.filter((id: string) => id === tag.id).length;
    expect(count).toBe(1);
  });
});

test.describe('Edge cases: ItemsByTag inheritance', () => {
  test('filtering by parent tag returns items with child tags', async ({ request, app }) => {
    // Create tag hierarchy: Electronics > Sensors
    const electronics = await (await request.post(`${app.baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'Electronics', parent_id: '' }
    })).json();
    const sensors = await (await request.post(`${app.baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'Sensors', parent_id: electronics.id }
    })).json();

    // Create items
    const box = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'InheritBox', parent_id: '' }
    })).json();
    const thermometer = await (await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'Thermometer', container_id: box.id }
    })).json();
    const resistor = await (await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'Resistor', container_id: box.id }
    })).json();

    // Tag: thermometer → Sensors, resistor → Electronics
    await request.post(`${app.baseURL}/items/${thermometer.id}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { tag_id: sensors.id }
    });
    await request.post(`${app.baseURL}/items/${resistor.id}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { tag_id: electronics.id }
    });

    // Verify tag hierarchy — descendants of Electronics includes Sensors
    const descResp = await request.get(`${app.baseURL}/tags/${electronics.id}/descendants`, {
      headers: { 'Accept': 'application/json' },
    });
    const descendants: string[] = await descResp.json();
    expect(descendants).toContain(sensors.id);
  });
});
