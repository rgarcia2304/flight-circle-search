import { test, expect } from '@playwright/test';

test.describe('Live E2E: full UI flow with backend', () => {
  test('place circles, submit, see real results with booking link', async ({ page }) => {
    test.setTimeout(180_000);

    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto('/');
    await page.waitForTimeout(1500);

    const map = page.locator('.leaflet-container');
    const box = await map.boundingBox();
    if (!box) throw new Error('Map has no bounding box');

    // Place origin (US East coast) — visible on zoom-3 map around 28-40°N, 70-80°W
    await page.mouse.move(box.x + 320, box.y + 300);
    await page.mouse.down();
    await page.mouse.move(box.x + 420, box.y + 360, { steps: 12 });
    await page.mouse.up();
    await page.waitForTimeout(500);

    // Place dest (London) — visible around 50-55°N, 0-5°W
    await page.mouse.move(box.x + 760, box.y + 230);
    await page.mouse.down();
    await page.mouse.move(box.x + 840, box.y + 280, { steps: 12 });
    await page.mouse.up();
    await page.waitForTimeout(500);

    // Set date
    await page.locator('input[type="date"]').fill('2026-09-15');

    // Submit
    await page.locator('button:has-text("Find flights")').click();

    // Should see "Searching..." or progress
    await expect(page.getByText('Searching…')).toBeVisible({ timeout: 5000 });

    // Wait for completion (results tray title changes to "Cheapest one-way fares")
    await expect(page.getByText('Cheapest one-way fares')).toBeVisible({ timeout: 120_000 });

    // Verify at least one real result rendered with a price
    const firstCard = page.locator('.result-card').first();
    await expect(firstCard).toBeVisible({ timeout: 5000 });
    const price = await firstCard.locator('.result-price').textContent();
    expect(price).toMatch(/\$\d+/);

    // Verify the result card exposes a booking link
    const bookLink = firstCard.locator('a.result-book-btn');
    await expect(bookLink).toBeVisible({ timeout: 5000 });
    const href = await bookLink.getAttribute('href');
    expect(href, 'booking link should point at aviasales.com').toMatch(/^https:\/\/www\.aviasales\.com\/search\//);
    const target = await bookLink.getAttribute('target');
    expect(target, 'booking link should open in a new tab').toBe('_blank');

    // Take screenshot of live results
    await page.screenshot({ path: 'screenshots/live-ui-results.png', fullPage: true });
  });
});
