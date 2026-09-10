import { test, expect, type Page, type Route } from '@playwright/test';

const API = 'http://localhost:8080';
const APP_ORIGIN = 'http://localhost:5173';

const CORS_HEADERS = {
  'Access-Control-Allow-Origin': APP_ORIGIN,
  'Access-Control-Allow-Credentials': 'true',
  'Access-Control-Allow-Methods': 'GET, POST, OPTIONS',
  'Access-Control-Allow-Headers': 'Content-Type',
};

async function fulfillJSON(
  route: Route,
  status: number,
  body: unknown,
  extraHeaders: Record<string, string> = {},
): Promise<void> {
  if (route.request().method() === 'OPTIONS') {
    await route.fulfill({ status: 204, headers: CORS_HEADERS });
    return;
  }
  await route.fulfill({
    status,
    headers: { ...CORS_HEADERS, 'Content-Type': 'application/json', ...extraHeaders },
    body: JSON.stringify(body),
  });
}

/** Places an origin + destination circle and picks a departure date, the
 * same mouse-drag pattern used in tests/e2e/live-ui.spec.ts. */
async function placeCirclesAndDate(page: Page): Promise<void> {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto('/');
  await page.waitForTimeout(1500);

  const map = page.locator('.leaflet-container');
  const box = await map.boundingBox();
  if (!box) throw new Error('Map has no bounding box');

  await page.mouse.move(box.x + 320, box.y + 300);
  await page.mouse.down();
  await page.mouse.move(box.x + 420, box.y + 360, { steps: 12 });
  await page.mouse.up();
  await page.waitForTimeout(500);

  await page.mouse.move(box.x + 760, box.y + 230);
  await page.mouse.down();
  await page.mouse.move(box.x + 840, box.y + 280, { steps: 12 });
  await page.mouse.up();
  await page.waitForTimeout(500);

  await page.locator('input[type="date"]').fill('2026-10-15');
}

test.describe('Magic-link auth UI', () => {
  test('401 pops the modal; request, callback, and a retried search all work', async ({ page }) => {
    test.setTimeout(60_000);

    // Shared fake auth state, flipped once the mocked callback "completes login".
    const state = { loggedIn: false, jobsPostCount: 0 };

    // GET /v1/auth/session — never touches a real backend, mirrors whatever
    // the fake login state currently is.
    await page.route(`${API}/v1/auth/session`, async (route) => {
      if (state.loggedIn) {
        await fulfillJSON(route, 200, { email: 'e2e@example.com' });
      } else {
        await fulfillJSON(route, 401, { error: 'unauthorized', message: 'unauthorized' });
      }
    });

    // POST /jobs — 401 the first time (triggers the modal), 202 after login.
    await page.route(`${API}/jobs`, async (route) => {
      if (route.request().method() === 'OPTIONS') {
        await route.fulfill({ status: 204, headers: CORS_HEADERS });
        return;
      }
      state.jobsPostCount += 1;
      if (!state.loggedIn) {
        await fulfillJSON(route, 401, { error: 'unauthorized', message: 'unauthorized' });
        return;
      }
      // Verify the session cookie actually rode along on the authenticated retry.
      const cookieHeader = route.request().headers()['cookie'] ?? '';
      expect(cookieHeader, 'submit request should carry the session cookie').toContain('session=test-session-id');
      await fulfillJSON(route, 202, { job_id: 'job-1', total_pairs: 1 });
    });

    // GET /jobs/:id — immediately "complete" with one fare, so polling
    // resolves right away and the results tray has something to show.
    await page.route(`${API}/jobs/*`, async (route) => {
      await fulfillJSON(route, 200, {
        id: 'job-1',
        status: 'complete',
        submitted_at: new Date().toISOString(),
        total_pairs: 1,
        completed_pairs: 1,
        failed_pairs: 0,
        results: [
          {
            origin: 'JFK',
            destination: 'LHR',
            departure_date: '2026-10-15',
            status: 'complete',
            fare: [
              {
                Price: 45000,
                Currency: 'USD',
                Airline: 'AA',
                FlightNumber: 'AA100',
                Origin: 'JFK',
                Destination: 'LHR',
                OriginAirport: 'JFK',
                DestinationAirport: 'LHR',
                DepartureAt: '2026-10-15T10:00:00Z',
                Duration: 420,
                Transfers: 0,
                Link: '/search/JFKLHR1510?marker=test',
              },
            ],
          },
        ],
      });
    });

    await placeCirclesAndDate(page);
    await page.locator('button:has-text("Find flights")').click();

    // 401 -> modal pops.
    await expect(page.getByText('Sign in to search')).toBeVisible({ timeout: 5000 });
    expect(state.jobsPostCount).toBe(1);

    // Request the magic link.
    await page.route(`${API}/v1/auth/magic-link`, async (route) => {
      if (route.request().method() === 'OPTIONS') {
        await route.fulfill({ status: 204, headers: CORS_HEADERS });
        return;
      }
      const body = route.request().postDataJSON() as { email: string };
      expect(body.email).toBe('e2e@example.com');
      await fulfillJSON(route, 202, { status: 'sent' });
    });

    await page.locator('input[type="email"]').fill('e2e@example.com');
    await page.locator('button:has-text("Send link")').click();
    await expect(page.getByText('Check your email')).toBeVisible({ timeout: 5000 });

    // Simulate opening the emailed link in the callback page — the mocked
    // response sets a real session cookie via Set-Cookie, exactly like the
    // real backend does.
    await page.route(`${API}/v1/auth/callback**`, async (route) => {
      state.loggedIn = true;
      await fulfillJSON(
        route,
        200,
        { email: 'e2e@example.com' },
        { 'Set-Cookie': 'session=test-session-id; Path=/; HttpOnly; SameSite=Lax' },
      );
    });

    await page.goto('/auth/callback?token=fake-token');
    await expect(page.getByRole('heading', { name: 'Signed in' })).toBeVisible({ timeout: 5000 });

    // Back to the app (a fresh SPA load, so circles need placing again) —
    // the retried search should now succeed with no modal.
    await placeCirclesAndDate(page);
    await page.locator('button:has-text("Find flights")').click();

    await expect(page.getByText('Cheapest one-way fares')).toBeVisible({ timeout: 10_000 });
    await expect(page.getByText('Sign in to search')).not.toBeVisible();
    expect(state.jobsPostCount).toBe(2);
  });
});
