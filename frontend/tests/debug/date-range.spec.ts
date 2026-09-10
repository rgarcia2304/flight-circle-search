import { test, expect } from '@playwright/test';

test.describe('Debug: Date input', () => {
  test('should have a single depart date input', async ({ page }) => {
    await page.goto('/');
    const dates = page.locator('input[type="date"]');
    await expect(dates).toHaveCount(1);
  });
});
