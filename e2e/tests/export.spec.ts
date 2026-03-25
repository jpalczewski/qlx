import { test, expect } from '../fixtures/app';

test.describe('Data export API', () => {
  test.describe.configure({ mode: 'serial' });

  let containerId: string;
  let childContainerId: string;

  test('setup: create test data', async ({ request, app }) => {
    // Create parent container
    const cRes = await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Export Parent' },
    });
    const container = await cRes.json();
    containerId = container.id;

    // Create child container
    const childRes = await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Export Child', parent_id: containerId },
    });
    const child = await childRes.json();
    childContainerId = child.id;

    // Create items in both containers
    await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Parent Item', description: 'In parent', container_id: containerId },
    });
    await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Child Item', description: 'In child', container_id: childContainerId },
    });
  });

  test('CSV export returns correct content type and data', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/export?format=csv`);
    expect(res.ok()).toBeTruthy();
    expect(res.headers()['content-type']).toContain('text/csv');
    const csv = await res.text();
    expect(csv).toContain('item_id');
    expect(csv).toContain('Parent Item');
    expect(csv).toContain('Child Item');
  });

  test('JSON export returns correct content type and data', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/export?format=json`);
    expect(res.ok()).toBeTruthy();
    expect(res.headers()['content-type']).toContain('application/json');
    const data = await res.json();
    expect(data.containers).toBeDefined();
    expect(data.containers.length).toBeGreaterThanOrEqual(1);
  });

  test('Markdown export returns correct content type', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/export?format=md`);
    expect(res.ok()).toBeTruthy();
    expect(res.headers()['content-type']).toContain('text/markdown');
    const md = await res.text();
    expect(md).toContain('item_id');
    expect(md).toContain('Parent Item');
  });

  test('invalid format returns 400', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/export?format=xml`);
    expect(res.status()).toBe(400);
  });

  test('missing format returns 400', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/export`);
    expect(res.status()).toBe(400);
  });

  test('nonexistent container returns 404', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/export?format=csv&container=nonexistent`);
    expect(res.status()).toBe(404);
  });

  test('per-container export scopes to container', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/export?format=csv&container=${containerId}`);
    expect(res.ok()).toBeTruthy();
    const csv = await res.text();
    expect(csv).toContain('Parent Item');
    // Non-recursive: should NOT contain child item
    expect(csv).not.toContain('Child Item');
  });

  test('recursive export includes child items', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/export?format=csv&container=${containerId}&recursive=true`);
    expect(res.ok()).toBeTruthy();
    const csv = await res.text();
    expect(csv).toContain('Parent Item');
    expect(csv).toContain('Child Item');
  });

  test('download flag sets Content-Disposition', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/export?format=csv&download=true`);
    expect(res.ok()).toBeTruthy();
    expect(res.headers()['content-disposition']).toContain('attachment');
  });

  test('markdown document style works', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/export?format=md&md_style=document`);
    expect(res.ok()).toBeTruthy();
    const md = await res.text();
    expect(md).toContain('## ');
    expect(md).toContain('**Parent Item**');
  });
});
