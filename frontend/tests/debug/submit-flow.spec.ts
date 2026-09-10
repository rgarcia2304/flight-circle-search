import { test, expect } from '@playwright/test';

test.describe('Debug: Search button', () => {
  test('search button should be disabled without circles and date', async ({ page }) => {
    await page.goto('/');
    const btn = page.locator('button:has-text("Find flights")');
    await expect(btn).toBeDisabled();
  });
});
