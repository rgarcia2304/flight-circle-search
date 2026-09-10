import { test, expect } from '@playwright/test';

test('end-to-end with backend', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto('/');
  await page.waitForTimeout(1000);

  const map = page.locator('.leaflet-container');
  const box = await map.boundingBox();
  if (!box) throw new Error('Map has no bounding box');

  // Place origin (sparse North Atlantic coastal area — keeps the resolved
  // airport count, and therefore the real pair count submitted to the live
  // fare provider, small regardless of whether an API key is configured)
  await page.mouse.move(box.x + 320, box.y + 300);
  await page.mouse.down();
  await page.mouse.move(box.x + 420, box.y + 360, { steps: 10 });
  await page.mouse.up();
  await page.waitForTimeout(500);

  // Place dest (Iceland — one airport, no combinatorial hub-routing blowup)
  await page.mouse.move(box.x + 760, box.y + 230);
  await page.mouse.down();
  await page.mouse.move(box.x + 840, box.y + 280, { steps: 10 });
  await page.mouse.up();
  await page.waitForTimeout(500);

  // Set date
  await page.locator('input[type="date"]').fill('2026-09-15');

  // Submit
  await page.locator('button:has-text("Find flights")').click();
  await page.waitForTimeout(500);

  // The results tray should appear with "Searching..." or progress
  await expect(page.getByText('Searching…').or(page.getByText('Cheapest one-way fares'))).toBeVisible({ timeout: 5000 });

  // Wait for results or completion (without API key, it'll fail fast)
  await page.waitForTimeout(8000);
  await page.screenshot({ path: 'screenshots/07-e2e.png' });
});
