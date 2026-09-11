import { request, type APIRequestContext } from '@playwright/test';

// Shared by every live-*.spec.ts and by live-global-setup.ts.
export const API = 'http://localhost:8080';
export const TEST_EMAIL = 'e2e@example.com';
export const AUTH_STATE_PATH = 'tests/e2e/.auth/live-state.json';

/**
 * Returns an authenticated API request context for the live backend.
 *
 * `request.newContext()` (the module-level factory, as opposed to the
 * per-test `request` fixture) doesn't inherit the Playwright config's
 * `use.storageState` — it has to be passed explicitly, which is what this
 * wraps. The state file it points at is produced by live-global-setup.ts.
 */
export function liveApiContext(): Promise<APIRequestContext> {
  return request.newContext({ baseURL: API, storageState: AUTH_STATE_PATH });
}
