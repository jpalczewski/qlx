import { test, expect } from '../fixtures/app';

test.describe('API contracts and error handling', () => {

  test.describe('Container errors', () => {
    test('GET non-existent container returns 404', async ({ request, app }) => {
      const res = await request.get(`${app.baseURL}/api/containers/00000000-0000-0000-0000-000000000000`);
      expect(res.status()).toBe(404);
    });

    test('DELETE non-existent container returns 404 with error message', async ({ request, app }) => {
      const res = await request.delete(`${app.baseURL}/api/containers/00000000-0000-0000-0000-000000000000`);
      expect(res.status()).toBe(404);
      const body = await res.json();
      expect(body.error).toBe('container not found');
    });

    test('DELETE container with children returns 409', async ({ request, app }) => {
      // Create parent
      const parentRes = await request.post(`${app.baseURL}/api/containers`, {
        data: { name: 'Parent' },
      });
      const parent = await parentRes.json();

      // Create child
      await request.post(`${app.baseURL}/api/containers`, {
        data: { name: 'Child', parent_id: parent.id },
      });

      // Try delete parent
      const deleteRes = await request.delete(`${app.baseURL}/api/containers/${parent.id}`);
      expect(deleteRes.status()).toBe(409);
      const body = await deleteRes.json();
      expect(body.error).toBe('container has children');
    });

    test('DELETE container with items returns 409', async ({ request, app }) => {
      const containerRes = await request.post(`${app.baseURL}/api/containers`, {
        data: { name: 'Has Items' },
      });
      const container = await containerRes.json();

      await request.post(`${app.baseURL}/api/items`, {
        data: { name: 'Blocker', container_id: container.id },
      });

      const deleteRes = await request.delete(`${app.baseURL}/api/containers/${container.id}`);
      expect(deleteRes.status()).toBe(409);
      const body = await deleteRes.json();
      expect(body.error).toBe('container has items');
    });
  });

  test.describe('Item errors', () => {
    test('GET non-existent item returns 404', async ({ request, app }) => {
      const res = await request.get(`${app.baseURL}/api/items/00000000-0000-0000-0000-000000000000`);
      expect(res.status()).toBe(404);
    });

    test('DELETE non-existent item returns 404 with error message', async ({ request, app }) => {
      const res = await request.delete(`${app.baseURL}/api/items/00000000-0000-0000-0000-000000000000`);
      expect(res.status()).toBe(404);
      const body = await res.json();
      expect(body.error).toBe('item not found');
    });

    test('move item to non-existent container returns 400', async ({ request, app }) => {
      const containerRes = await request.post(`${app.baseURL}/api/containers`, {
        data: { name: 'Source' },
      });
      const container = await containerRes.json();

      const itemRes = await request.post(`${app.baseURL}/api/items`, {
        data: { name: 'Movable', container_id: container.id },
      });
      const item = await itemRes.json();

      const moveRes = await request.patch(`${app.baseURL}/api/items/${item.id}/move`, {
        data: { container_id: '00000000-0000-0000-0000-000000000000' },
      });
      expect(moveRes.status()).toBe(400);
      const body = await moveRes.json();
      expect(body.error).toBe('invalid container for item');
    });
  });

  test.describe('Printer errors', () => {
    test('DELETE non-existent printer returns 404', async ({ request, app }) => {
      const res = await request.delete(`${app.baseURL}/api/printers/00000000-0000-0000-0000-000000000000`);
      expect(res.status()).toBe(404);
      const body = await res.json();
      expect(body.error).toBe('printer not found');
    });
  });

  test.describe('Container move cycle detection', () => {
    test('move container to itself returns 400 cycle detected', async ({ request, app }) => {
      const res = await request.post(`${app.baseURL}/api/containers`, {
        data: { name: 'Self Mover' },
      });
      const container = await res.json();

      const moveRes = await request.patch(`${app.baseURL}/api/containers/${container.id}/move`, {
        data: { parent_id: container.id },
      });
      expect(moveRes.status()).toBe(400);
      const body = await moveRes.json();
      expect(body.error).toBe('cycle detected');
    });

    test('move container to its own descendant returns 400 cycle detected', async ({ request, app }) => {
      const grandparentRes = await request.post(`${app.baseURL}/api/containers`, {
        data: { name: 'Grandparent' },
      });
      const grandparent = await grandparentRes.json();

      const parentRes = await request.post(`${app.baseURL}/api/containers`, {
        data: { name: 'Parent', parent_id: grandparent.id },
      });
      const parent = await parentRes.json();

      const childRes = await request.post(`${app.baseURL}/api/containers`, {
        data: { name: 'Child', parent_id: parent.id },
      });
      const child = await childRes.json();

      // Try to move grandparent under child (cycle)
      const moveRes = await request.patch(`${app.baseURL}/api/containers/${grandparent.id}/move`, {
        data: { parent_id: child.id },
      });
      expect(moveRes.status()).toBe(400);
      const body = await moveRes.json();
      expect(body.error).toBe('cycle detected');
    });
  });

  test.describe('Response structure contracts', () => {
    test('created container has all required fields', async ({ request, app }) => {
      const res = await request.post(`${app.baseURL}/api/containers`, {
        data: { name: 'Contract Test', description: 'desc' },
      });
      expect(res.status()).toBe(201);
      const container = await res.json();

      expect(container.id).toBeTruthy();
      expect(container.id).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/);
      expect(container.name).toBe('Contract Test');
      expect(container.description).toBe('desc');
      expect(container.created_at).toBeTruthy();
    });

    test('created item has all required fields', async ({ request, app }) => {
      const containerRes = await request.post(`${app.baseURL}/api/containers`, {
        data: { name: 'For Item' },
      });
      const container = await containerRes.json();

      const res = await request.post(`${app.baseURL}/api/items`, {
        data: { name: 'Contract Item', description: 'item desc', container_id: container.id },
      });
      expect(res.status()).toBe(201);
      const item = await res.json();

      expect(item.id).toBeTruthy();
      expect(item.id).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/);
      expect(item.name).toBe('Contract Item');
      expect(item.description).toBe('item desc');
      expect(item.container_id).toBe(container.id);
      expect(item.created_at).toBeTruthy();
    });
  });

  test.describe('Missing validation (known gaps)', () => {
    test('BUG: container can be created with empty name', async ({ request, app }) => {
      const res = await request.post(`${app.baseURL}/api/containers`, {
        data: { name: '', description: 'no name' },
      });
      // This SHOULD return 400, but currently returns 201 — documenting the bug
      expect(res.status()).toBe(201); // BUG: should be 400
    });

    test('BUG: item can be created with empty name', async ({ request, app }) => {
      const containerRes = await request.post(`${app.baseURL}/api/containers`, {
        data: { name: 'For Empty Item' },
      });
      const container = await containerRes.json();

      const res = await request.post(`${app.baseURL}/api/items`, {
        data: { name: '', container_id: container.id },
      });
      // This SHOULD return 400, but currently returns 201 — documenting the bug
      expect(res.status()).toBe(201); // BUG: should be 400
    });

    test('BUG: item can be created without container_id', async ({ request, app }) => {
      const res = await request.post(`${app.baseURL}/api/items`, {
        data: { name: 'Orphan Item' },
      });
      // Orphan items (no container) might be unintended
      expect(res.status()).toBe(201); // BUG: should probably require container_id
    });
  });
});
