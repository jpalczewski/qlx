import { test, expect } from '../fixtures/app';

test.describe('Move to Container', () => {

  test('m opens move picker on item detail page', async ({ page, app }) => {
    // Create container + item
    await page.goto(`${app.baseURL}/`);
    const input = page.locator('.qe-input');
    await input.click();
    await page.keyboard.press('Tab'); // switch to container mode
    await page.keyboard.press('End');
    await page.keyboard.type('Source');
    const cr = page.waitForResponse(r => r.url().includes('/containers') && r.request().method() === 'POST');
    await page.keyboard.press('Enter');
    await cr;
    await page.click('#container-list a:has-text("Source")');
    await expect(page.locator('h2')).toContainText('Source');

    await input.click();
    await page.keyboard.press('End');
    await page.keyboard.type('TestItem');
    const ir = page.waitForResponse(r => r.url().includes('/items') && r.request().method() === 'POST');
    await page.keyboard.press('Enter');
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
    const input = page.locator('.qe-input');
    await input.click();
    await page.keyboard.press('Tab'); // switch to container mode
    await page.keyboard.press('End');
    await page.keyboard.type('NavTest');
    const cr = page.waitForResponse(r => r.url().includes('/containers') && r.request().method() === 'POST');
    await page.keyboard.press('Enter');
    await cr;
    await page.click('#container-list a:has-text("NavTest")');
    await expect(page.locator('h2')).toContainText('NavTest');

    await input.click();
    await page.keyboard.press('End');
    await page.keyboard.type('MoveMe');
    const ir = page.waitForResponse(r => r.url().includes('/items') && r.request().method() === 'POST');
    await page.keyboard.press('Enter');
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
    const input = page.locator('.qe-input');
    await input.click();
    await page.keyboard.press('Tab'); // switch to container mode
    await page.keyboard.press('End');
    await page.keyboard.type('MoveSource');
    let cr = page.waitForResponse(r => r.url().includes('/containers') && r.request().method() === 'POST');
    await page.keyboard.press('Enter');
    await cr;

    await input.click();
    await page.keyboard.press('End');
    await page.keyboard.type('MoveTarget');
    cr = page.waitForResponse(r => r.url().includes('/containers') && r.request().method() === 'POST');
    await page.keyboard.press('Enter');
    await cr;

    // Create item in source
    await page.click('#container-list a:has-text("MoveSource")');
    await expect(page.locator('h2')).toContainText('MoveSource');
    await input.click();
    await page.keyboard.press('End');
    await page.keyboard.type('Movable');
    const ir = page.waitForResponse(r => r.url().includes('/items') && r.request().method() === 'POST');
    await page.keyboard.press('Enter');
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
    const input = page.locator('.qe-input');
    await input.click();
    await page.keyboard.press('Tab'); // switch to container mode
    await page.keyboard.press('End');
    await page.keyboard.type('ChildBox');
    let cr = page.waitForResponse(r => r.url().includes('/containers') && r.request().method() === 'POST');
    await page.keyboard.press('Enter');
    await cr;

    await input.click();
    await page.keyboard.press('End');
    await page.keyboard.type('ParentBox');
    cr = page.waitForResponse(r => r.url().includes('/containers') && r.request().method() === 'POST');
    await page.keyboard.press('Enter');
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
    const input = page.locator('.qe-input');
    await input.click();
    await page.keyboard.press('Tab'); // switch to container mode
    await page.keyboard.press('End');
    await page.keyboard.type('KeyMoveBox');
    const cr = page.waitForResponse(r => r.url().includes('/containers') && r.request().method() === 'POST');
    await page.keyboard.press('Enter');
    await cr;

    await page.click('#container-list a:has-text("KeyMoveBox")');
    await expect(page.locator('h2')).toContainText('KeyMoveBox');

    // Press m — should open move picker for the container itself
    await page.locator('h2').click(); // ensure no input focused
    await page.keyboard.press('m');
    await expect(page.locator('#move-picker')).toBeVisible();
  });
});
