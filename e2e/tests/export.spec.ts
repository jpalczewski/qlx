import { test, expect } from '../fixtures/app';

test.describe('Data export', () => {
  test.describe.configure({ mode: 'serial' });

  test('setup: create test data', async ({ request, app }) => {
    const containerRes = await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Export Container' },
    });
    const container = await containerRes.json();

    await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Export Item 1', description: 'Desc 1', container_id: container.id },
    });
    await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Export Item 2', description: 'Desc 2', container_id: container.id },
    });
  });

  test('export JSON contains containers and items', async ({ request, app }) => {
    const response = await request.get(`${app.baseURL}/export/json`, {
      headers: { 'Accept': 'application/json' },
    });
    expect(response.ok()).toBeTruthy();

    const data = await response.json();
    expect(data.containers).toBeDefined();
    expect(data.items).toBeDefined();

    // containers and items are maps keyed by ID, not arrays
    const containers = Object.values(data.containers) as any[];
    const items = Object.values(data.items) as any[];

    expect(containers.length).toBeGreaterThanOrEqual(1);
    expect(items.length).toBeGreaterThanOrEqual(2);

    const containerNames = containers.map((c: any) => c.name);
    expect(containerNames).toContain('Export Container');
  });

  test('export CSV has correct headers and rows', async ({ request, app }) => {
    const response = await request.get(`${app.baseURL}/export/csv`, {
      headers: { 'Accept': 'application/json' },
    });
    expect(response.ok()).toBeTruthy();

    const csv = await response.text();
    const lines = csv.trim().split('\n');

    expect(lines[0]).toContain('id');
    expect(lines[0]).toContain('name');
    expect(lines.length).toBeGreaterThanOrEqual(3);
    expect(csv).toContain('Export Item 1');
    expect(csv).toContain('Export Item 2');
  });
});
