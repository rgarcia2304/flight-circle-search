import { mkdir, writeFile } from 'node:fs/promises';
import { dirname } from 'node:path';
import { API, AUTH_STATE_PATH, TEST_EMAIL } from './live-auth';

/**
 * Authenticates against a live, already-running backend and writes a
 * Playwright storageState file the live-*.spec.ts tests point at.
 *
 * Requires the backend to be running locally with E2E_TEST_MODE=true, e.g.:
 *   E2E_TEST_MODE=true go run ./cmd/api
 *
 * That flag gates a test-only endpoint (POST /v1/auth/test-token) that
 * issues a real magic-link token directly, skipping the allowlist and email
 * delivery — see internal/http/auth_handlers.go's IssueTestToken. It's never
 * enabled in any real deployment.
 */
export default async function globalSetup(): Promise<void> {
  let tokenRes: Response;
  try {
    tokenRes = await fetch(`${API}/v1/auth/test-token`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: TEST_EMAIL }),
    });
  } catch (err) {
    throw new Error(
      `Could not reach the backend at ${API}. Live e2e tests need a real backend running locally ` +
        `(E2E_TEST_MODE=true go run ./cmd/api). Original error: ${String(err)}`,
    );
  }

  if (tokenRes.status === 404) {
    throw new Error(
      `${API}/v1/auth/test-token returned 404 — the backend is running without E2E_TEST_MODE=true. ` +
        `Restart it with: E2E_TEST_MODE=true go run ./cmd/api`,
    );
  }
  if (!tokenRes.ok) {
    throw new Error(`POST /v1/auth/test-token failed: ${tokenRes.status} ${await tokenRes.text()}`);
  }
  const { token } = (await tokenRes.json()) as { token: string };

  const callbackRes = await fetch(`${API}/v1/auth/callback?token=${encodeURIComponent(token)}`, {
    redirect: 'manual',
  });
  if (callbackRes.status !== 200) {
    throw new Error(`GET /v1/auth/callback failed: ${callbackRes.status} ${await callbackRes.text()}`);
  }

  const setCookie =
    typeof callbackRes.headers.getSetCookie === 'function'
      ? callbackRes.headers.getSetCookie()[0]
      : callbackRes.headers.get('set-cookie');
  if (!setCookie) {
    throw new Error('/v1/auth/callback response had no Set-Cookie header');
  }
  const [nameValue] = setCookie.split(';');
  const eqIdx = nameValue.indexOf('=');
  const name = nameValue.slice(0, eqIdx).trim();
  const value = nameValue.slice(eqIdx + 1).trim();

  await mkdir(dirname(AUTH_STATE_PATH), { recursive: true });
  await writeFile(
    AUTH_STATE_PATH,
    JSON.stringify({
      cookies: [
        {
          name,
          value,
          domain: 'localhost',
          path: '/',
          expires: -1,
          httpOnly: true,
          secure: false,
          sameSite: 'Lax' as const,
        },
      ],
      origins: [],
    }),
  );
}
