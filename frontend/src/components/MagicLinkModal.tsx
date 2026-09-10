import { useState, type FormEvent } from 'react';
import { requestMagicLink } from '../lib/auth';

type Status = 'idle' | 'sending' | 'sent' | 'error';

interface MagicLinkModalProps {
  open: boolean;
  onClose: () => void;
}

export function MagicLinkModal({ open, onClose }: MagicLinkModalProps) {
  const [email, setEmail] = useState('');
  const [status, setStatus] = useState<Status>('idle');
  const [errorMessage, setErrorMessage] = useState('');

  if (!open) return null;

  const reset = () => {
    setEmail('');
    setStatus('idle');
    setErrorMessage('');
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!email || status === 'sending') return;
    setStatus('sending');
    try {
      await requestMagicLink(email);
      setStatus('sent');
    } catch (err) {
      setStatus('error');
      setErrorMessage(err instanceof Error ? err.message : 'Something went wrong');
    }
  };

  return (
    <div
      className="fixed inset-0 z-[2000] flex items-center justify-center bg-black/60 backdrop-blur-sm"
      onClick={handleClose}
    >
      <div
        className="w-[360px] max-w-[calc(100vw-32px)] rounded-2xl border border-white/10 bg-[rgba(20,20,30,0.95)] p-6 text-white shadow-2xl backdrop-blur-xl"
        onClick={(e) => e.stopPropagation()}
      >
        {status === 'sent' ? (
          <div className="text-center">
            <div className="mx-auto mb-3 flex h-10 w-10 items-center justify-center rounded-full bg-gradient-to-br from-[#818cf8] to-[#f472b6] text-lg">
              ✓
            </div>
            <h2 className="text-base font-bold">Check your email</h2>
            <p className="mt-1.5 text-sm text-white/60">
              We sent a sign-in link to <span className="font-semibold text-white">{email}</span>. It expires in 15
              minutes.
            </p>
            <button
              type="button"
              onClick={handleClose}
              className="mt-5 w-full rounded-lg border border-white/10 bg-white/5 py-2 text-sm font-semibold text-white/80 transition hover:bg-white/10 hover:text-white"
            >
              Done
            </button>
          </div>
        ) : (
          <form onSubmit={handleSubmit}>
            <h2 className="text-base font-bold">Sign in to search</h2>
            <p className="mt-1.5 text-sm text-white/60">
              Enter your email and we'll send you a one-click sign-in link.
            </p>
            <input
              type="email"
              required
              autoFocus
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="you@example.com"
              disabled={status === 'sending'}
              className="mt-4 w-full rounded-lg border border-white/10 bg-white/5 px-3 py-2.5 text-sm text-white placeholder-white/30 outline-none transition focus:border-[#818cf8] focus:bg-[rgba(129,140,248,0.08)]"
            />
            {status === 'error' && <p className="mt-2 text-xs font-medium text-red-400">{errorMessage}</p>}
            <div className="mt-4 flex gap-2">
              <button
                type="button"
                onClick={handleClose}
                className="flex-1 rounded-lg border border-white/10 bg-white/5 py-2 text-sm font-semibold text-white/70 transition hover:bg-white/10 hover:text-white"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={status === 'sending' || !email}
                className="flex-1 rounded-lg bg-gradient-to-br from-[#818cf8] to-[#f472b6] py-2 text-sm font-semibold text-white shadow-[0_4px_16px_rgba(129,140,248,0.3)] transition disabled:cursor-not-allowed disabled:opacity-40"
              >
                {status === 'sending' ? 'Sending…' : 'Send link'}
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  );
}
