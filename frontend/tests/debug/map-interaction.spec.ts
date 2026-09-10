import { test, expect } from '@playwright/test';

test.describe('Debug: Map interaction', () => {
  test('clicking map should place origin circle', async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto('/');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);

    const map = page.locator('.leaflet-container');
    const box = await map.boundingBox();
    if (!box) throw new Error('Map has no bounding box');

    await page.mouse.click(box.x + 700, box.y + 300);
    await page.waitForTimeout(500);

    // Reset button should appear (circle placed)
    await expect(page.locator('button.reset-button')).toBeVisible();
  });

  test('should show initial placement hint', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByText('Click & drag on the map to set your origin')).toBeVisible();
  });
});
