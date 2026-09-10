import { useEffect, useState } from 'react';
import { Check } from 'lucide-react';
import { exchangeMagicLinkToken } from '../lib/auth';

type Status = 'exchanging' | 'success' | 'error';

/** Rendered at /auth/callback — the page the emailed magic link points at.
 * Reads the token from the URL, exchanges it for a session, and reports
 * the outcome. main.tsx renders this in place of <App /> for that path. */
export function AuthCallback() {
  const [token] = useState(() => new URLSearchParams(window.location.search).get('token'));
  const [status, setStatus] = useState<Status>(token ? 'exchanging' : 'error');
  const [email, setEmail] = useState('');
  const [errorMessage, setErrorMessage] = useState(token ? '' : 'Missing sign-in token.');

  useEffect(() => {
    if (!token) return;
    exchangeMagicLinkToken(token)
      .then((session) => {
        setEmail(session.email);
        setStatus('success');
      })
      .catch((err) => {
        setStatus('error');
        setErrorMessage(err instanceof Error ? err.message : 'This link is invalid or has expired.');
      });
  }, [token]);

  return (
    <div className="fixed inset-0 flex items-center justify-center bg-[#0a0a0a] font-sans text-white">
      <div className="w-[360px] max-w-[calc(100vw-32px)] rounded-2xl border border-white/10 bg-[rgba(18,18,18,0.95)] p-6 text-center shadow-2xl">
        {status === 'exchanging' && (
          <>
            <div className="mx-auto mb-3 h-8 w-8 animate-spin rounded-full border-2 border-white/20 border-t-white" />
            <h1 className="text-base font-bold">Signing you in…</h1>
          </>
        )}
        {status === 'success' && (
          <>
            <div className="mx-auto mb-3 flex h-10 w-10 items-center justify-center rounded-full bg-white text-[#0a0a0a]">
              <Check size={20} strokeWidth={3} />
            </div>
            <h1 className="text-base font-bold">Signed in</h1>
            <p className="mt-1.5 text-sm text-white/60">
              You're signed in as <span className="font-semibold text-white">{email}</span>.
            </p>
            <a
              href="/"
              className="mt-5 block w-full rounded-lg bg-white py-2 text-sm font-bold text-[#0a0a0a] transition hover:bg-white/90"
            >
              Back to search
            </a>
          </>
        )}
        {status === 'error' && (
          <>
            <h1 className="text-base font-bold">Sign-in failed</h1>
            <p className="mt-1.5 text-sm text-white/60">{errorMessage}</p>
            <a
              href="/"
              className="mt-5 block w-full rounded-lg border border-white/10 bg-white/5 py-2 text-sm font-semibold text-white/80"
            >
              Back to search
            </a>
          </>
        )}
      </div>
    </div>
  );
}
