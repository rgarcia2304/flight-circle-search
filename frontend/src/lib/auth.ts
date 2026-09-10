export interface Session {
  email: string;
}

export const API_BASE = (import.meta.env.VITE_API_BASE as string | undefined) ?? 'http://localhost:8080';

type UnauthorizedListener = () => void;
const unauthorizedListeners = new Set<UnauthorizedListener>();

/** Subscribe to "the server just told us we're not logged in" events, fired
 * by apiFetch on a 401 (unless the caller passed silentUnauthorized). Returns
 * an unsubscribe function. */
export function onUnauthorized(listener: UnauthorizedListener): () => void {
  unauthorizedListeners.add(listener);
  return () => unauthorizedListeners.delete(listener);
}

function notifyUnauthorized() {
  for (const listener of unauthorizedListeners) listener();
}

interface ApiFetchOptions {
  /** Suppress the onUnauthorized notification for an expected 401 (e.g. a
   * session check on page load, or an expired magic-link token) rather than
   * treating it as "the user got logged out mid-action". */
  silentUnauthorized?: boolean;
}

/** Shared fetch wrapper for every API call: resolves against API_BASE,
 * always sends the session cookie, and fires onUnauthorized on a 401. */
export async function apiFetch(path: string, init?: RequestInit, opts?: ApiFetchOptions): Promise<Response> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    credentials: 'include',
  });
  if (res.status === 401 && !opts?.silentUnauthorized) {
    notifyUnauthorized();
  }
  return res;
}

/** Parses a JSON response, throwing an Error (using the server's message
 * when present) if the response wasn't ok. */
export async function parseJSON<T>(res: Response): Promise<T> {
  if (!res.ok) {
    const body = await res.json().catch(() => ({ message: 'request failed' }));
    throw new Error(body.message || `HTTP ${res.status}`);
  }
  return res.json();
}

export async function requestMagicLink(email: string): Promise<void> {
  const res = await apiFetch('/v1/auth/magic-link', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email }),
  });
  await parseJSON(res);
}

/** Returns the current session, or null if not logged in. */
export async function getSession(): Promise<Session | null> {
  const res = await apiFetch('/v1/auth/session', undefined, { silentUnauthorized: true });
  if (res.status === 401) return null;
  return parseJSON<Session>(res);
}

/** Exchanges a magic-link token (from the emailed callback URL) for a
 * session. Throws if the token is missing, invalid, or expired. */
export async function exchangeMagicLinkToken(token: string): Promise<Session> {
  const res = await apiFetch(`/v1/auth/callback?token=${encodeURIComponent(token)}`, undefined, {
    silentUnauthorized: true,
  });
  return parseJSON<Session>(res);
}

export async function logout(): Promise<void> {
  const res = await apiFetch('/v1/auth/logout', { method: 'POST' });
  if (!res.ok) {
    await parseJSON(res);
  }
}
