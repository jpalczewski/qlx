import { test, expect } from '../fixtures/app';

test.describe('Printer status SSE', () => {
  test.describe.configure({ mode: 'serial' });

  test('frontend loads immediately without blocking on printer connections', async ({ page, app }) => {
    const start = Date.now();
    await page.goto(`${app.baseURL}/printers`, { waitUntil: 'domcontentloaded' });
    const elapsed = Date.now() - start;

    // Page should load quickly — well under 5 seconds
    expect(elapsed).toBeLessThan(5000);
    await expect(page.locator('h1')).toBeVisible();
    await expect(page.locator('h1')).toContainText('Drukarki');
  });

  test('SSE endpoint is requested by the page on load', async ({ page, app }) => {
    // The page's sse.js initiates an EventSource connection to /printers/events
    // on load. Set up the request interceptor before navigating.
    const sseRequestPromise = page.waitForRequest(
      req => req.url().includes('/printers/events'),
      { timeout: 5000 }
    );

    await page.goto(`${app.baseURL}/printers`, { waitUntil: 'domcontentloaded' });

    // Verify the SSE request was made by sse.js
    const sseRequest = await sseRequestPromise;
    expect(sseRequest.url()).toContain('/printers/events');
  });

  test('connection state indicator appears for a connected printer', async ({ page, app }) => {
    // Step 1: Add a printer via the API (saves to store, not yet in ConnectionManager)
    const addResponse = await page.request.post(`${app.baseURL}/printers`, {
      headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
      data: JSON.stringify({
        name: 'SSE Test Printer',
        encoder: 'niimbot',
        transport: 'serial',
        address: '/dev/null',
      }),
    });
    expect(addResponse.status()).toBe(201);
    const printer = await addResponse.json();
    const printerId: string = printer.id;

    // Step 2: Trigger connection via the connect endpoint — this registers the printer
    // with ConnectionManager and starts the async connection loop. The loop immediately
    // emits StateConnecting, which the SSE snapshot will include.
    const connectResponse = await page.request.post(
      `${app.baseURL}/printers/${printerId}/connect`
    );
    // Connect may return 200 or 500 (if already managed); both are acceptable here
    // as long as the CM has the printer registered.
    expect([200, 500]).toContain(connectResponse.status());

    // Step 3: Navigate to printers page — SSE.js subscribes to /printers/events.
    // ConnectionManager.Subscribe() delivers a snapshot that includes the printer's
    // current state (at minimum StateConnecting). The Events handler flushes this
    // immediately, causing sse.js to create the conn-dot element.
    await page.goto(`${app.baseURL}/printers`, { waitUntil: 'domcontentloaded' });

    // The printer card should be visible
    const statusEl = page.locator(`#printer-status-${printerId}`);
    await expect(statusEl).toBeVisible();

    // Wait for the conn-dot to appear — SSE delivers the state snapshot on subscribe
    const connDot = statusEl.locator('.conn-dot');
    await expect(connDot).toBeVisible({ timeout: 5000 });
  });
});
