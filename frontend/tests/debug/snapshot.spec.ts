import { test } from '@playwright/test';

test('snapshot UI', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto('/');
  await page.waitForTimeout(1000);

  await page.screenshot({ path: 'screenshots/01-initial.png' });

  const map = page.locator('.leaflet-container');
  const box = await map.boundingBox();
  if (!box) throw new Error('Map has no bounding box');

  // Drag to create origin (Europe). Kept to a minimal drag distance — at this
  // zoomed-out world view even a modest drag hits MAX_RADIUS_KM (3000km) and,
  // combined with hub-routing, explodes into thousands of real fare searches
  // against the live provider once a real API key is configured.
  await page.mouse.move(box.x + 700, box.y + 300);
  await page.mouse.down();
  await page.mouse.move(box.x + 720, box.y + 320, { steps: 15 });
  await page.mouse.up();
  await page.waitForTimeout(500);
  await page.screenshot({ path: 'screenshots/02-origin.png' });

  // Drag to create destination (US East)
  await page.mouse.move(box.x + 400, box.y + 350);
  await page.mouse.down();
  await page.mouse.move(box.x + 420, box.y + 370, { steps: 15 });
  await page.mouse.up();
  await page.waitForTimeout(500);
  await page.screenshot({ path: 'screenshots/03-both.png' });

  // Set date
  await page.locator('input[type="date"]').fill('2026-09-15');
  await page.waitForTimeout(300);
  await page.screenshot({ path: 'screenshots/05-ready.png' });

  // Search
  await page.locator('button:has-text("Find flights")').click();
  await page.waitForTimeout(2200);
  await page.screenshot({ path: 'screenshots/06-results.png' });
});