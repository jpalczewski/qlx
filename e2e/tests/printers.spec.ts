import { test, expect } from '../fixtures/app';

test.describe('Printer management', () => {
  test.describe.configure({ mode: 'serial' });

  const printerName = `Test Printer ${Date.now()}`;

  test('printers page loads', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui/printers`);
    await expect(page.locator('h1')).toContainText('Drukarki');
  });

  test('add printer via form', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui/printers`);

    await page.click('summary:has-text("Dodaj drukarkę")');
    await page.fill('#name', printerName);
    await page.selectOption('#encoder', 'niimbot');
    await page.selectOption('#transport', 'ble');
    await page.fill('#address', 'AA:BB:CC:DD:EE:FF');

    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/ui/actions/printers') && r.request().method() === 'POST'
    );
    await page.click('button:has-text("Dodaj")');
    await responsePromise;

    await expect(page.locator(`.printer-card:has-text("${printerName}")`)).toBeVisible();
  });

  test('BLE scan button triggers request', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui/printers`);

    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/api/bluetooth/scan')
    );

    await page.click('button:has-text("Skanuj Bluetooth")');
    await expect(page.locator('#ble-results')).toContainText('Skanowanie');

    const response = await responsePromise;
    // On non-BLE builds, scan returns 404 or error — UI handles gracefully
    if (!response.ok()) {
      await expect(page.locator('#ble-results')).toContainText('Błąd');
    }
  });

  test('BLE scan with mocked results populates form', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui/printers`);

    // Mock the BLE scan endpoint
    await page.route('**/api/bluetooth/scan', async route => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([
          { name: 'Niimbot B1', address: '11:22:33:44:55:66', rssi: -55 },
        ]),
      });
    });

    await page.click('button:has-text("Skanuj Bluetooth")');
    await expect(page.locator('#ble-results')).toContainText('Niimbot B1');

    // Click scan result to fill form
    await page.click('#ble-results .container-item:has-text("Niimbot B1")');

    // Verify form was filled — fillPrinter() sets values via .value property
    await expect(page.locator('#address')).toHaveValue('11:22:33:44:55:66');
    await expect(page.locator('#transport')).toHaveValue('ble');
  });

  test('delete printer', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/ui/printers`);

    page.on('dialog', dialog => dialog.accept());
    const responsePromise = page.waitForResponse(r =>
      r.url().includes('/ui/actions/printers/') && r.request().method() === 'DELETE'
    );
    await page.click(`.printer-card:has-text("${printerName}") button:has-text("Usuń")`);
    await responsePromise;

    // Printer should be gone (other printers from other tests may still exist)
    await expect(page.locator(`.printer-card:has-text("${printerName}")`)).not.toBeVisible();
  });
});
