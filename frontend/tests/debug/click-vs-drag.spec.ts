import { test, expect } from '@playwright/test';

test.describe('Debug: Click vs drag', () => {
  test('click (no drag) should still place a min-radius circle', async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto('/');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);

    const map = page.locator('.leaflet-container');
    const box = await map.boundingBox();
    if (!box) throw new Error('Map has no bounding box');

    // Simple click — no drag at all
    await page.mouse.click(box.x + 700, box.y + 300);
    await page.waitForTimeout(500);

    // Reset button should appear (a circle is placed)
    await expect(page.locator('button.reset-button')).toBeVisible();
  });

  test('click+drag should size circle based on drag distance', async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto('/');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);

    const map = page.locator('.leaflet-container');
    const box = await map.boundingBox();
    if (!box) throw new Error('Map has no bounding box');

    // Click and drag a long distance
    await page.mouse.move(box.x + 700, box.y + 300);
    await page.mouse.down();
    await page.mouse.move(box.x + 900, box.y + 400, { steps: 15 });
    await page.mouse.up();
    await page.waitForTimeout(500);

    // Circle should be placed
    await expect(page.locator('button.reset-button')).toBeVisible();
  });

  test('release without moving should commit min radius', async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto('/');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);

    const map = page.locator('.leaflet-container');
    const box = await map.boundingBox();
    if (!box) throw new Error('Map has no bounding box');

    // Press and release at same position (mousedown then immediate mouseup)
    await page.mouse.move(box.x + 500, box.y + 300);
    await page.mouse.down();
    await page.waitForTimeout(50);
    await page.mouse.up();
    await page.waitForTimeout(500);

    // Should still place a circle
    await expect(page.locator('button.reset-button')).toBeVisible();
  });
});
