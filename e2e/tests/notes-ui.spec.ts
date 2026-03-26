import { test, expect } from '../fixtures/app';

test.describe('Notes UI', () => {

  test('tabs visible on item detail', async ({ page, app }) => {
    const baseURL = app.baseURL;

    // Create container via API
    const contResp = await page.request.post(`${baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Note Test Container' },
    });
    const container = await contResp.json();

    // Create item via API
    const itemResp = await page.request.post(`${baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Note Test Item', container_id: container.id },
    });
    const item = await itemResp.json();

    // Navigate to item detail page
    await page.goto(`${baseURL}/items/${item.id}`, { waitUntil: 'domcontentloaded' });

    // Tab bar must be present
    const tabBar = page.locator('.tab-bar');
    await expect(tabBar).toBeVisible();

    // Detail tab and Notes tab must both exist
    const detailTab = page.locator('.tab-btn[data-tab="detail"]');
    const notesTab = page.locator('.tab-btn[data-tab="notes"]');
    await expect(detailTab).toBeVisible();
    await expect(notesTab).toBeVisible();
  });

  test('create note via quick-entry on item', async ({ page, app }) => {
    const baseURL = app.baseURL;

    // Create container + item via API
    const contResp = await page.request.post(`${baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'QE Note Container' },
    });
    const container = await contResp.json();

    const itemResp = await page.request.post(`${baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'QE Note Item', container_id: container.id },
    });
    const item = await itemResp.json();

    // Navigate to item detail
    await page.goto(`${baseURL}/items/${item.id}`, { waitUntil: 'domcontentloaded' });

    // Click Notes tab and wait for the quick-entry form to appear
    const notesTab = page.locator('.tab-btn[data-tab="notes"]');
    await notesTab.click();

    // Wait for HTMX to load the notes tab content
    const titleInput = page.locator('.note-quick-entry-title');
    await expect(titleInput).toBeVisible({ timeout: 15000 });
    await titleInput.fill('My Test Note');

    const contentInput = page.locator('.note-quick-entry-content');
    await contentInput.fill('Note content here');

    // Submit and wait for POST /notes (HTMX returns 200, JSON API returns 201)
    const submitResp = page.waitForResponse(r =>
      r.url().includes('/notes') && r.request().method() === 'POST'
    );
    await page.locator('.note-quick-entry .btn-primary').click();
    await submitResp;

    // Note card must appear in the list
    const noteList = page.locator('.note-list');
    await expect(noteList).toContainText('My Test Note');
  });

  test('delete note via note card button', async ({ page, app }) => {
    const baseURL = app.baseURL;

    // Create container + item via API
    const contResp = await page.request.post(`${baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Delete Note Container' },
    });
    const container = await contResp.json();

    const itemResp = await page.request.post(`${baseURL}/items`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Delete Note Item', container_id: container.id },
    });
    const item = await itemResp.json();

    // Create note via API
    const noteResp = await page.request.post(`${baseURL}/notes`, {
      headers: { 'Accept': 'application/json', 'Content-Type': 'application/json' },
      data: { item_id: item.id, title: 'Note To Delete', content: 'Will be removed' },
    });
    const note = await noteResp.json();

    // Navigate to item detail
    await page.goto(`${baseURL}/items/${item.id}`, { waitUntil: 'domcontentloaded' });

    // Click Notes tab and wait for content to load
    const notesTab = page.locator('.tab-btn[data-tab="notes"]');
    await notesTab.click();

    // Wait for note list to appear
    await expect(page.locator('.note-list')).toBeVisible({ timeout: 15000 });

    // Verify note appears
    await expect(page.locator('.note-list')).toContainText('Note To Delete');

    // Hover over the note card to reveal action buttons (hidden by default, shown on hover)
    const noteCard = page.locator(`.note-card[data-note-id="${note.id}"]`);
    await noteCard.hover();

    // Accept the hx-confirm dialog and delete
    page.on('dialog', dialog => dialog.accept());
    const deleteResp = page.waitForResponse(r =>
      r.url().includes(`/notes/${note.id}`) && r.request().method() === 'DELETE'
    );
    await noteCard.locator('.note-card-action-btn.danger').click();
    await deleteResp;

    // Note card must no longer be in the list
    await expect(page.locator(`.note-card[data-note-id="${note.id}"]`)).toHaveCount(0);
  });

  test('notes appear in search results', async ({ page, app }) => {
    const baseURL = app.baseURL;

    // Create container via API
    const contResp = await page.request.post(`${baseURL}/containers`, {
      headers: { 'Accept': 'application/json' },
      data: { name: 'Search Note Container' },
    });
    const container = await contResp.json();

    // Create note attached to container via API
    const uniqueTitle = `SearchableNote-${Date.now()}`;
    const noteResp = await page.request.post(`${baseURL}/notes`, {
      headers: { 'Accept': 'application/json', 'Content-Type': 'application/json' },
      data: { container_id: container.id, title: uniqueTitle, content: 'Findable content' },
    });

    // Navigate to search page with query matching the note title
    await page.goto(`${baseURL}/search?q=${uniqueTitle}`, { waitUntil: 'domcontentloaded' });

    // Notes section must show the note title
    const noteTitle = page.locator('.note-title', { hasText: uniqueTitle });
    await expect(noteTitle).toBeVisible();
  });

});
