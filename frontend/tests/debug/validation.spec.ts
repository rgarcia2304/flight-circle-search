import { test, expect } from '@playwright/test';

test.describe('Debug: Controls', () => {
  test('date input should be present', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('input[type="date"]')).toBeVisible();
  });

  test('find flights button should be present', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByText('Find flights')).toBeVisible();
  });
});
