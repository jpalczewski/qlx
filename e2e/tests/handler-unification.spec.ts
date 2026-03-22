import { test, expect } from '../fixtures/app';

// ─── Content Negotiation ─────────────────────────────────────────────────────
test.describe('Content negotiation', () => {

  test('GET / without Accept header returns HTML', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/`);
    expect(res.status()).toBe(200);
    const ct = res.headers()['content-type'] || '';
    expect(ct).toContain('text/html');
    const body = await res.text();
    expect(body).toContain('<!DOCTYPE html>');
  });

  test('GET / with Accept: application/json returns JSON', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/`, {
      headers: { 'Accept': 'application/json' },
    });
    expect(res.status()).toBe(200);
    const data = await res.json();
    // Root container list should return an array or null (empty store)
    expect(data === null || Array.isArray(data)).toBe(true);
  });

  test('GET /containers/{id} returns JSON when Accept: application/json', async ({ request, app }) => {
    const c = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'NegotiateTest' },
    })).json();

    const res = await request.get(`${app.baseURL}/containers/${c.id}`, {
      headers: { 'Accept': 'application/json' },
    });
    expect(res.status()).toBe(200);
    const data = await res.json();
    expect(data.container.name).toBe('NegotiateTest');
  });

  test('GET /containers/{id} returns HTML without Accept header', async ({ request, app }) => {
    const c = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'HTMLTest' },
    })).json();

    const res = await request.get(`${app.baseURL}/containers/${c.id}`);
    expect(res.status()).toBe(200);
    const ct = res.headers()['content-type'] || '';
    expect(ct).toContain('text/html');
    const body = await res.text();
    expect(body).toContain('HTMLTest');
  });

  test('error responses respect content negotiation', async ({ request, app }) => {
    const fakeID = '00000000-0000-0000-0000-000000000099';

    // JSON error
    const jsonRes = await request.get(`${app.baseURL}/items/${fakeID}`, {
      headers: { 'Accept': 'application/json' },
    });
    expect(jsonRes.status()).toBe(404);
    const jsonBody = await jsonRes.json();
    expect(jsonBody.error).toBeDefined();

    // HTML error (no Accept header)
    const htmlRes = await request.get(`${app.baseURL}/items/${fakeID}`);
    expect(htmlRes.status()).toBe(404);
    const ct = htmlRes.headers()['content-type'] || '';
    expect(ct).toContain('text/plain');
  });

  test('Vary: HX-Request header is set on negotiated responses', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/`);
    const vary = res.headers()['vary'] || '';
    expect(vary).toContain('HX-Request');
  });
});

// ─── Old URL scheme: /api/ and /ui/ should 404 ──────────────────────────────
test.describe('Old URL scheme is dead', () => {

  test('GET /api/containers returns 404', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/api/containers`, {
      headers: { 'Accept': 'application/json' },
    });
    expect(res.status()).toBe(404);
  });

  test('GET /ui returns 404', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/ui`);
    expect(res.status()).toBe(404);
  });

  test('GET /ui/containers returns 404', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/ui/containers`);
    expect(res.status()).toBe(404);
  });

  test('POST /api/containers returns 404', async ({ request, app }) => {
    const res = await request.post(`${app.baseURL}/api/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Should404' },
    });
    // Go ServeMux returns 404 for unregistered routes or 405 for wrong method
    expect([404, 405]).toContain(res.status());
  });
});

// ─── BindRequest edge cases ─────────────────────────────────────────────────
test.describe('BindRequest edge cases', () => {

  test('POST container with JSON body works', async ({ request, app }) => {
    const res = await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'JSONBody', description: 'via JSON' },
    });
    expect(res.status()).toBe(201);
    const c = await res.json();
    expect(c.name).toBe('JSONBody');
    expect(c.description).toBe('via JSON');
  });

  test('POST container with form body works', async ({ request, app }) => {
    const res = await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'FormBody', description: 'via form' },
    });
    expect(res.status()).toBe(201);
    const c = await res.json();
    expect(c.name).toBe('FormBody');
    expect(c.description).toBe('via form');
  });

  test('POST item with quantity as string (form) is parsed', async ({ request, app }) => {
    const c = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'QtyTest' },
    })).json();

    const res = await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      form: { name: 'FormQty', container_id: c.id, quantity: '5' },
    });
    expect(res.status()).toBe(201);
    const item = await res.json();
    expect(item.quantity).toBe(5);
  });

  test('POST item with zero quantity defaults to 1', async ({ request, app }) => {
    const c = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'ZeroQty' },
    })).json();

    const res = await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'ZeroItem', container_id: c.id, quantity: 0 },
    });
    expect(res.status()).toBe(201);
    const item = await res.json();
    expect(item.quantity).toBe(1);
  });

  test('POST container with empty name returns 400', async ({ request, app }) => {
    const res = await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: '' },
    });
    expect(res.status()).toBe(400);
  });

  test('POST item without container_id returns 400', async ({ request, app }) => {
    const res = await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Orphan' },
    });
    expect(res.status()).toBe(400);
    const body = await res.json();
    expect(body.error).toBe('container_id is required');
  });
});

// ─── HTMX partial vs full page ──────────────────────────────────────────────
test.describe('HTMX partial responses', () => {

  test('GET / with HX-Request returns partial (no doctype)', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/`, {
      headers: { 'HX-Request': 'true' },
    });
    expect(res.status()).toBe(200);
    const body = await res.text();
    // Partial should NOT have full HTML layout
    expect(body).not.toContain('<!DOCTYPE html>');
    // But should have container content
    expect(body).toContain('container-list');
  });

  test('GET /tags with HX-Request returns partial', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/tags`, {
      headers: { 'HX-Request': 'true' },
    });
    expect(res.status()).toBe(200);
    const body = await res.text();
    expect(body).not.toContain('<!DOCTYPE html>');
    expect(body).toContain('tag-list');
  });

  test('GET /settings with HX-Request returns partial', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/settings`, {
      headers: { 'HX-Request': 'true' },
    });
    expect(res.status()).toBe(200);
    const body = await res.text();
    expect(body).not.toContain('<!DOCTYPE html>');
  });

  test('full page load of /printers includes layout', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/printers`);
    expect(res.status()).toBe(200);
    const body = await res.text();
    expect(body).toContain('<!DOCTYPE html>');
    expect(body).toContain('<nav');
  });
});

// ─── Tag assignment edge cases ──────────────────────────────────────────────
test.describe('Tag assignment edge cases', () => {

  test('assign same tag twice to item is idempotent', async ({ request, app }) => {
    const tag = await (await request.post(`${app.baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Twice' },
    })).json();
    const c = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'TwiceBox' },
    })).json();
    const item = await (await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'TwiceItem', container_id: c.id },
    })).json();

    // Assign twice
    await request.post(`${app.baseURL}/items/${item.id}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { tag_id: tag.id },
    });
    await request.post(`${app.baseURL}/items/${item.id}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { tag_id: tag.id },
    });

    const detail = await (await request.get(`${app.baseURL}/items/${item.id}`, {
      headers: { 'Accept': 'application/json' },
    })).json();
    const count = detail.item.tag_ids.filter((id: string) => id === tag.id).length;
    expect(count).toBe(1);
  });

  test('assign tag to non-existent item returns 404', async ({ request, app }) => {
    const tag = await (await request.post(`${app.baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Dangling' },
    })).json();

    const res = await request.post(`${app.baseURL}/items/00000000-0000-0000-0000-ffffffffffff/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { tag_id: tag.id },
    });
    expect(res.status()).toBe(404);
  });

  test('remove non-existent tag from item returns 404', async ({ request, app }) => {
    const c = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'RemoveBox' },
    })).json();
    const item = await (await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'RemoveItem', container_id: c.id },
    })).json();

    const res = await request.delete(`${app.baseURL}/items/${item.id}/tags/nonexistent-tag-id`, {
      headers: { 'Accept': 'application/json' },
    });
    // Should return success (tag was not there) or 404 depending on implementation
    // The important thing is it doesn't 500
    expect([200, 404]).toContain(res.status());
  });

  test('assign tag to container then remove it', async ({ request, app }) => {
    const tag = await (await request.post(`${app.baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'ContainerAssign' },
    })).json();
    const c = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'TaggedC' },
    })).json();

    // Assign
    const addRes = await request.post(`${app.baseURL}/containers/${c.id}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { tag_id: tag.id },
    });
    expect(addRes.status()).toBe(200);

    // Verify
    const detail = await (await request.get(`${app.baseURL}/containers/${c.id}`, {
      headers: { 'Accept': 'application/json' },
    })).json();
    expect(detail.container.tag_ids).toContain(tag.id);

    // Remove
    const rmRes = await request.delete(`${app.baseURL}/containers/${c.id}/tags/${tag.id}`, {
      headers: { 'Accept': 'application/json' },
    });
    expect(rmRes.status()).toBe(200);

    // Verify gone
    const after = await (await request.get(`${app.baseURL}/containers/${c.id}`, {
      headers: { 'Accept': 'application/json' },
    })).json();
    expect(after.container.tag_ids || []).not.toContain(tag.id);
  });
});

// ─── Move edge cases ────────────────────────────────────────────────────────
test.describe('Move edge cases', () => {

  test('move item to non-existent container returns error', async ({ request, app }) => {
    const c = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'MoveItemBox' },
    })).json();
    const item = await (await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'MoveItem', container_id: c.id },
    })).json();

    const res = await request.patch(`${app.baseURL}/items/${item.id}/move`, {
      headers: { 'Accept': 'application/json' },
      data: { container_id: 'nonexistent-id' },
    });
    expect(res.status()).toBe(400);
  });

  test('move container to root (empty parent_id)', async ({ request, app }) => {
    const parent = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'MoveParent' },
    })).json();
    const child = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'MoveChild', parent_id: parent.id },
    })).json();

    // Move child to root
    const res = await request.patch(`${app.baseURL}/containers/${child.id}/move`, {
      headers: { 'Accept': 'application/json' },
      data: { parent_id: '' },
    });
    expect(res.status()).toBe(200);

    // Verify child is at root
    const detail = await (await request.get(`${app.baseURL}/containers/${child.id}`, {
      headers: { 'Accept': 'application/json' },
    })).json();
    expect(detail.container.parent_id).toBe('');
  });

  test('move tag to itself is rejected', async ({ request, app }) => {
    const tag = await (await request.post(`${app.baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'SelfMove' },
    })).json();

    const res = await request.patch(`${app.baseURL}/tags/${tag.id}/move`, {
      headers: { 'Accept': 'application/json' },
      data: { parent_id: tag.id },
    });
    expect(res.status()).toBe(400);
  });
});

// ─── Redirect behavior ─────────────────────────────────────────────────────
test.describe('Redirect behavior', () => {

  test('DELETE tag with JSON Accept returns ok, not redirect', async ({ request, app }) => {
    const tag = await (await request.post(`${app.baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'DeleteRedirect' },
    })).json();

    const res = await request.delete(`${app.baseURL}/tags/${tag.id}`, {
      headers: { 'Accept': 'application/json' },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.ok).toBe(true);
  });

  test('DELETE container with JSON Accept returns ok', async ({ request, app }) => {
    const c = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'DeleteC' },
    })).json();

    const res = await request.delete(`${app.baseURL}/containers/${c.id}`, {
      headers: { 'Accept': 'application/json' },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.ok).toBe(true);
  });

  test('DELETE item with JSON Accept returns ok', async ({ request, app }) => {
    const c = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'DeleteItemBox' },
    })).json();
    const item = await (await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'DeleteI', container_id: c.id },
    })).json();

    const res = await request.delete(`${app.baseURL}/items/${item.id}`, {
      headers: { 'Accept': 'application/json' },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.ok).toBe(true);
  });
});

// ─── Export edge cases ──────────────────────────────────────────────────────
test.describe('Export edge cases', () => {

  test('export JSON on empty store returns empty maps', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/export/json`);
    expect(res.status()).toBe(200);
    const data = await res.json();
    expect(data.containers).toBeDefined();
    expect(data.items).toBeDefined();
  });

  test('export CSV on empty store returns header only', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/export/csv`);
    expect(res.status()).toBe(200);
    const ct = res.headers()['content-type'] || '';
    expect(ct).toContain('text/csv');
    const text = await res.text();
    const lines = text.trim().split('\n');
    // At minimum the header row
    expect(lines.length).toBeGreaterThanOrEqual(1);
    expect(lines[0]).toContain('item_id');
  });

  test('export JSON includes items with container path', async ({ request, app }) => {
    const c = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'ExportParent' },
    })).json();
    await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'ExportChild', container_id: c.id },
    });

    const res = await request.get(`${app.baseURL}/export/json`);
    const data = await res.json();
    const items = Object.values(data.items) as any[];
    expect(items.some((i: any) => i.name === 'ExportChild')).toBe(true);
  });
});

// ─── i18n edge cases ────────────────────────────────────────────────────────
test.describe('i18n edge cases', () => {

  test('GET /i18n/pl returns object with keys', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/i18n/pl`);
    expect(res.status()).toBe(200);
    const data = await res.json();
    expect(data['nav.printers']).toBeDefined();
  });

  test('GET /i18n/en returns English translations', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/i18n/en`);
    expect(res.status()).toBe(200);
    const data = await res.json();
    expect(data['nav.printers']).toBe('Printers');
  });

  test('GET /i18n/nonexistent falls back gracefully', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/i18n/xx`);
    expect(res.status()).toBe(200);
    const data = await res.json();
    // Should return something (fallback), not empty
    expect(Object.keys(data).length).toBeGreaterThan(0);
  });

  test('POST /set-lang with JSON Accept returns ok', async ({ request, app }) => {
    const res = await request.post(`${app.baseURL}/set-lang`, {
      headers: { 'Accept': 'application/json' },
      form: { lang: 'en' },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.ok).toBe(true);
    expect(body.lang).toBe('en');
  });

  test('POST /set-lang with empty lang defaults to pl', async ({ request, app }) => {
    const res = await request.post(`${app.baseURL}/set-lang`, {
      headers: { 'Accept': 'application/json' },
      form: { lang: '' },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.lang).toBe('pl');
  });
});

// ─── Search edge cases ──────────────────────────────────────────────────────
test.describe('Search edge cases', () => {

  test('search with empty query returns empty results as JSON', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/search?q=`, {
      headers: { 'Accept': 'application/json' },
    });
    expect(res.status()).toBe(200);
    const data = await res.json();
    expect(data.containers).toBeDefined();
    expect(data.items).toBeDefined();
    expect(data.tags).toBeDefined();
  });

  test('search finds items across containers', async ({ request, app }) => {
    const c1 = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'SearchBox1' },
    })).json();
    const c2 = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'SearchBox2' },
    })).json();
    await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'UniqueSearchTerm123', container_id: c1.id },
    });
    await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'UniqueSearchTerm456', container_id: c2.id },
    });

    const res = await request.get(`${app.baseURL}/search?q=UniqueSearchTerm`, {
      headers: { 'Accept': 'application/json' },
    });
    expect(res.status()).toBe(200);
    const data = await res.json();
    expect(data.items.length).toBeGreaterThanOrEqual(2);
  });

  test('search with special characters does not crash', async ({ request, app }) => {
    const queries = ['<script>', 'a%00b', '../../etc/passwd', 'O\'Reilly', 'a"b'];
    for (const q of queries) {
      const res = await request.get(`${app.baseURL}/search?q=${encodeURIComponent(q)}`, {
        headers: { 'Accept': 'application/json' },
      });
      expect(res.status()).toBe(200);
    }
  });
});

// ─── Partials ───────────────────────────────────────────────────────────────
test.describe('Partials', () => {

  test('GET /partials/tree returns HTML fragment', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/partials/tree?parent_id=`);
    expect(res.status()).toBe(200);
    const ct = res.headers()['content-type'] || '';
    expect(ct).toContain('text/html');
  });

  test('GET /partials/tag-tree returns HTML fragment', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/partials/tag-tree?parent_id=`);
    expect(res.status()).toBe(200);
    const ct = res.headers()['content-type'] || '';
    expect(ct).toContain('text/html');
  });

  test('tree search returns matching containers', async ({ request, app }) => {
    await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'TreeSearchTarget' },
    });

    const res = await request.get(`${app.baseURL}/partials/tree/search?q=TreeSearchTarget`);
    expect(res.status()).toBe(200);
    const body = await res.text();
    expect(body).toContain('TreeSearchTarget');
  });
});

// ─── Update non-existent entities ───────────────────────────────────────────
test.describe('Update non-existent entities', () => {

  test('PUT non-existent container returns 404', async ({ request, app }) => {
    const res = await request.put(`${app.baseURL}/containers/00000000-0000-0000-0000-ffffffffffff`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Ghost' },
    });
    expect(res.status()).toBe(404);
  });

  test('PUT non-existent item returns 404', async ({ request, app }) => {
    const res = await request.put(`${app.baseURL}/items/00000000-0000-0000-0000-ffffffffffff`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Ghost' },
    });
    expect(res.status()).toBe(404);
  });

  test('PATCH move non-existent container returns error', async ({ request, app }) => {
    const res = await request.patch(`${app.baseURL}/containers/nonexistent/move`, {
      headers: { 'Accept': 'application/json' },
      data: { parent_id: '' },
    });
    expect([400, 404]).toContain(res.status());
  });

  test('PATCH move non-existent item returns error', async ({ request, app }) => {
    const res = await request.patch(`${app.baseURL}/items/nonexistent/move`, {
      headers: { 'Accept': 'application/json' },
      data: { container_id: '' },
    });
    expect([400, 404]).toContain(res.status());
  });
});

// ─── Special characters in names ────────────────────────────────────────────
test.describe('Special characters in entity names', () => {

  test('container with unicode name', async ({ request, app }) => {
    const name = 'Kontener 🎁 zażółć';
    const res = await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name },
    });
    expect(res.status()).toBe(201);
    const c = await res.json();
    expect(c.name).toBe(name);

    // Verify via GET
    const detail = await (await request.get(`${app.baseURL}/containers/${c.id}`, {
      headers: { 'Accept': 'application/json' },
    })).json();
    expect(detail.container.name).toBe(name);
  });

  test('item with HTML-like name does not cause XSS in HTML response', async ({ page, app, request }) => {
    const c = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'XSSBox' },
    })).json();
    await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      data: { name: '<img src=x onerror=alert(1)>', container_id: c.id },
    });

    await page.goto(`${app.baseURL}/containers/${c.id}`, { waitUntil: 'domcontentloaded' });

    // The text should be escaped, not rendered as HTML
    const body = await page.content();
    expect(body).not.toContain('<img src=x onerror=alert(1)>');
    expect(body).toContain('&lt;img');
  });

  test('tag with ampersand in name', async ({ request, app }) => {
    const res = await request.post(`${app.baseURL}/tags`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'R&D' },
    });
    expect(res.status()).toBe(201);
    const tag = await res.json();
    expect(tag.name).toBe('R&D');
  });
});

// ─── Quick entry HTMX flow (browser-based) ──────────────────────────────────
test.describe('Quick entry HTMX flow', () => {

  test('quick entry container appends single item, not duplicate list', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`, { waitUntil: 'domcontentloaded' });

    const name = `QE-${Date.now()}`;
    await page.fill('.containers .quick-entry input[name="name"]', name);

    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/containers') && r.request().method() === 'POST'
    );
    await page.press('.containers .quick-entry input[name="name"]', 'Enter');
    await responsePromise;

    // Should have exactly one #container-list, not two
    const lists = await page.locator('#container-list').count();
    expect(lists).toBe(1);
    await expect(page.locator('#container-list')).toContainText(name);
  });

  test('quick entry tag appends single item', async ({ page, app, request }) => {
    await page.goto(`${app.baseURL}/tags`, { waitUntil: 'domcontentloaded' });

    const name = `QETag-${Date.now()}`;
    await page.fill('.quick-entry input[name="name"]', name);

    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/tags') && r.request().method() === 'POST'
    );
    await page.press('.quick-entry input[name="name"]', 'Enter');
    await responsePromise;

    // Should have exactly one #tag-list
    const lists = await page.locator('#tag-list').count();
    expect(lists).toBe(1);
    await expect(page.locator('#tag-list')).toContainText(name);
  });
});

// ─── Concurrent-like operations ─────────────────────────────────────────────
test.describe('Rapid sequential operations', () => {

  test('create 10 containers rapidly', async ({ request, app }) => {
    const promises = Array.from({ length: 10 }, (_, i) =>
      request.post(`${app.baseURL}/containers`, {
        headers: { 'Accept': 'application/json' },
        data: { name: `Rapid-${i}` },
      })
    );
    const responses = await Promise.all(promises);
    for (const res of responses) {
      expect(res.status()).toBe(201);
    }
    // Verify all exist
    const root = await (await request.get(`${app.baseURL}/`, {
      headers: { 'Accept': 'application/json' },
    })).json();
    const names = root.map((c: any) => c.name);
    for (let i = 0; i < 10; i++) {
      expect(names).toContain(`Rapid-${i}`);
    }
  });

  test('create then immediately delete container', async ({ request, app }) => {
    const c = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'FlashContainer' },
    })).json();

    const delRes = await request.delete(`${app.baseURL}/containers/${c.id}`, {
      headers: { 'Accept': 'application/json' },
    });
    expect(delRes.status()).toBe(200);

    // Verify 404
    const getRes = await request.get(`${app.baseURL}/containers/${c.id}`, {
      headers: { 'Accept': 'application/json' },
    });
    expect(getRes.status()).toBe(404);
  });
});

// ─── Container items-json endpoint ──────────────────────────────────────────
test.describe('Container items-json endpoint', () => {

  test('items-json returns items for existing container', async ({ request, app }) => {
    const c = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'ItemsJsonBox' },
    })).json();
    const item = await (await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'IJ1', container_id: c.id },
    })).json();

    const res = await request.get(`${app.baseURL}/containers/${c.id}/items-json`, {
      headers: { 'Accept': 'application/json' },
    });
    expect(res.status()).toBe(200);
    const data = await res.json();
    expect(data.items.length).toBe(1);
    expect(data.items[0].name).toBe('IJ1');
  });

  test('items-json for non-existent container returns 404', async ({ request, app }) => {
    const res = await request.get(`${app.baseURL}/containers/nonexistent/items-json`, {
      headers: { 'Accept': 'application/json' },
    });
    expect(res.status()).toBe(404);
  });
});

// ─── Method not allowed ─────────────────────────────────────────────────────
test.describe('Wrong HTTP methods', () => {

  test('GET /containers (list) is ok but POST to /containers/{id} is not', async ({ request, app }) => {
    const c = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'MethodTest' },
    })).json();

    // POST to a specific container ID should not match any route
    const res = await request.post(`${app.baseURL}/containers/${c.id}`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Bad' },
    });
    expect(res.status()).toBe(405);
  });

  test('PUT to /containers (collection) returns 405', async ({ request, app }) => {
    const res = await request.put(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Bad' },
    });
    expect(res.status()).toBe(405);
  });
});

// ─── Item detail includes container path ────────────────────────────────────
test.describe('Item detail structure', () => {

  test('item detail JSON includes all expected fields', async ({ request, app }) => {
    const c = await (await request.post(`${app.baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'DetailBox', description: 'A box' },
    })).json();
    const item = await (await request.post(`${app.baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'DetailItem', description: 'An item', container_id: c.id, quantity: 3 },
    })).json();

    const res = await request.get(`${app.baseURL}/items/${item.id}`, {
      headers: { 'Accept': 'application/json' },
    });
    expect(res.status()).toBe(200);
    const data = await res.json();
    expect(data.item.id).toBe(item.id);
    expect(data.item.name).toBe('DetailItem');
    expect(data.item.description).toBe('An item');
    expect(data.item.quantity).toBe(3);
    expect(data.item.container_id).toBe(c.id);
    expect(data.item.created_at).toBeDefined();
  });
});
