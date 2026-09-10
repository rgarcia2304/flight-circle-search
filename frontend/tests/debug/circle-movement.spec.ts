import { test, expect } from '@playwright/test';

test.describe('Debug: UI elements', () => {
  test('should render the brand', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByText('Circle Search')).toBeVisible();
  });

  test('should render the depart input', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('input[type="date"]')).toHaveCount(1);
  });

  test('should render the find flights button', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByText('Find flights')).toBeVisible();
  });
});