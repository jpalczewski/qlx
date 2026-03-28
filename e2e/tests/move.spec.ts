import { test, expect } from '../fixtures/app';

test.describe('Move to Container', () => {

  test('m opens move picker on item detail page', async ({ page, app }) => {
    // Create container + item
    await page.goto(`${app.baseURL}/`);
    await page.fill('.containers .quick-entry input[name="name"]', 'Source');
    const cr = page.waitForResponse(r => r.url().includes('/containers') && r.request().method() === 'POST');
    await page.press('.containers .quick-entry input[name="name"]', 'Enter');
    await cr;
    await page.click('#container-list a:has-text("Source")');
    await expect(page.locator('h2')).toContainText('Source');

    await page.fill('.items .quick-entry input[name="name"]', 'TestItem');
    const ir = page.waitForResponse(r => r.url().includes('/items') && r.request().method() === 'POST');
    await page.press('.items .quick-entry input[name="name"]', 'Enter');
    await ir;

    // Navigate to item detail
    await page.click('#item-list a:has-text("TestItem")');
    await expect(page.locator('h1')).toContainText('TestItem');

    // Press m — move picker should open
    await page.keyboard.press('m');
    await expect(page.locator('#move-picker')).toBeVisible();
  });

  test('m opens move picker for kb-active item in list', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.fill('.containers .quick-entry input[name="name"]', 'NavTest');
    const cr = page.waitForResponse(r => r.url().includes('/containers') && r.request().method() === 'POST');
    await page.press('.containers .quick-entry input[name="name"]', 'Enter');
    await cr;
    await page.click('#container-list a:has-text("NavTest")');
    await expect(page.locator('h2')).toContainText('NavTest');

    await page.fill('.items .quick-entry input[name="name"]', 'MoveMe');
    const ir = page.waitForResponse(r => r.url().includes('/items') && r.request().method() === 'POST');
    await page.press('.items .quick-entry input[name="name"]', 'Enter');
    await ir;

    // Arrow down to highlight item, then press m
    await page.locator('h2').click();
    await page.keyboard.press('ArrowDown');
    await expect(page.locator('.kb-active')).toHaveCount(1);
    await page.keyboard.press('m');
    await expect(page.locator('#move-picker')).toBeVisible();
  });

  test('Move to button on item detail moves item', async ({ page, app }) => {
    // Create source + target containers
    await page.goto(`${app.baseURL}/`);
    await page.fill('.containers .quick-entry input[name="name"]', 'MoveSource');
    let cr = page.waitForResponse(r => r.url().includes('/containers') && r.request().method() === 'POST');
    await page.press('.containers .quick-entry input[name="name"]', 'Enter');
    await cr;

    await page.fill('.containers .quick-entry input[name="name"]', 'MoveTarget');
    cr = page.waitForResponse(r => r.url().includes('/containers') && r.request().method() === 'POST');
    await page.press('.containers .quick-entry input[name="name"]', 'Enter');
    await cr;

    // Create item in source
    await page.click('#container-list a:has-text("MoveSource")');
    await expect(page.locator('h2')).toContainText('MoveSource');
    await page.fill('.items .quick-entry input[name="name"]', 'Movable');
    const ir = page.waitForResponse(r => r.url().includes('/items') && r.request().method() === 'POST');
    await page.press('.items .quick-entry input[name="name"]', 'Enter');
    await ir;

    // Go to item detail
    await page.click('#item-list a:has-text("Movable")');
    await expect(page.locator('h1')).toContainText('Movable');

    // Click Move to button (button with openMovePicker onclick)
    await page.click('button[onclick*="openMovePicker"]');
    await expect(page.locator('#move-picker')).toBeVisible();

    // Select target container in tree picker
    await page.click('#move-picker .tree-label:has-text("MoveTarget")');
    const moveResp = page.waitForResponse(r => r.url().includes('/move') && r.request().method() === 'PATCH');
    await page.click('#move-picker-confirm');
    await moveResp;

    // Should navigate to target container, item should be there
    await expect(page.locator('h2')).toContainText('MoveTarget');
    await expect(page.locator('#item-list')).toContainText('Movable');
  });

  test('Move to button on container detail moves container', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);

    // Create two containers
    await page.fill('.containers .quick-entry input[name="name"]', 'ChildBox');
    let cr = page.waitForResponse(r => r.url().includes('/containers') && r.request().method() === 'POST');
    await page.press('.containers .quick-entry input[name="name"]', 'Enter');
    await cr;

    await page.fill('.containers .quick-entry input[name="name"]', 'ParentBox');
    cr = page.waitForResponse(r => r.url().includes('/containers') && r.request().method() === 'POST');
    await page.press('.containers .quick-entry input[name="name"]', 'Enter');
    await cr;

    // Go to ChildBox detail
    await page.click('#container-list a:has-text("ChildBox")');
    await expect(page.locator('h2')).toContainText('ChildBox');

    // Click Move to button (button with openMovePicker onclick)
    await page.click('button[onclick*="openMovePicker"]');
    await expect(page.locator('#move-picker')).toBeVisible();

    // Select ParentBox
    await page.click('#move-picker .tree-label:has-text("ParentBox")');
    const moveResp = page.waitForResponse(r => r.url().includes('/move') && r.request().method() === 'PATCH');
    await page.click('#move-picker-confirm');
    await moveResp;

    // Should navigate to ParentBox, ChildBox should be a subcontainer
    await expect(page.locator('h2')).toContainText('ParentBox');
    await expect(page.locator('#container-list')).toContainText('ChildBox');
  });

  test('m on container detail with no selection moves the container', async ({ page, app }) => {
    await page.goto(`${app.baseURL}/`);
    await page.fill('.containers .quick-entry input[name="name"]', 'KeyMoveBox');
    const cr = page.waitForResponse(r => r.url().includes('/containers') && r.request().method() === 'POST');
    await page.press('.containers .quick-entry input[name="name"]', 'Enter');
    await cr;

    await page.click('#container-list a:has-text("KeyMoveBox")');
    await expect(page.locator('h2')).toContainText('KeyMoveBox');

    // Press m — should open move picker for the container itself
    await page.locator('h2').click(); // ensure no input focused
    await page.keyboard.press('m');
    await expect(page.locator('#move-picker')).toBeVisible();
  });
});
