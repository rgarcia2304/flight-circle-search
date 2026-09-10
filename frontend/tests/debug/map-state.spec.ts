import { test } from '@playwright/test';

test('debug map state', async ({ page }) => {
  page.on('console', (msg) => console.log(`[${msg.type()}]`, msg.text()));
  page.on('pageerror', (err) => console.log('[pageerror]', err.message));

  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto('/');
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(800);

  const map = page.locator('.leaflet-container');
  const box = await map.boundingBox();
  if (!box) throw new Error('Map has no bounding box');

  await page.mouse.click(box.x + 700, box.y + 300);
  await page.waitForTimeout(800);

  // Inspect leaflet layers
  const layers = await page.evaluate(() => {
    const map = (window as any)._leaflet_map;
    if (!map) return { error: 'No map on window' };
    const layers: any[] = [];
    map.eachLayer((layer: any) => {
      layers.push({
        type: layer.constructor.name,
        latlng: layer.getLatLng ? layer.getLatLng() : null,
        radius: layer.getRadius ? layer.getRadius() : null,
      });
    });
    return { count: layers.length, layers };
  });
  console.log('Leaflet layers:', JSON.stringify(layers, null, 2));
});
