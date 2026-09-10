import { useState, useMemo, useEffect, useCallback, useRef } from 'react';
import { MapView, type CircleData } from './components/MapView';
import { MagicLinkModal } from './components/MagicLinkModal';
import { ResultsPanel, type ParsedFare } from './components/ResultsPanel';
import { submitJob, getJob, type Result } from './lib/api';
import { getSession, logout, onUnauthorized, type Session } from './lib/auth';
import './App.css';

type Step = 'origin' | 'dest' | 'ready';
type ConfirmKind = 'reset' | null;

interface HistoryEntry {
  origin: CircleData | null;
  dest: CircleData | null;
}

function App() {
  const [origin, setOrigin] = useState<CircleData | null>(null);
  const [dest, setDest] = useState<CircleData | null>(null);
  const [step, setStep] = useState<Step>('origin');
  const [departDate, setDepartDate] = useState('');
  const [searching, setSearching] = useState(false);
  const [results, setResults] = useState<ParsedFare[]>([]);
  const [jobProgress, setJobProgress] = useState<{ completed: number; total: number } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [hasSearched, setHasSearched] = useState(false);
  const [searchError, setSearchError] = useState<string | null>(null);
  const [confirm, setConfirm] = useState<ConfirmKind>(null);
  const [session, setSession] = useState<Session | null>(null);
  const [showAuthModal, setShowAuthModal] = useState(false);
  const historyRef = useRef<HistoryEntry[]>([]);
  const lastCommittedRef = useRef<{ type: 'origin' | 'dest' } | null>(null);

  const refreshSession = useCallback(() => {
    getSession().then(setSession).catch(() => setSession(null));
  }, []);

  useEffect(() => {
    refreshSession();
  }, [refreshSession]);

  useEffect(() => onUnauthorized(() => setShowAuthModal(true)), []);

  const handleLogout = useCallback(async () => {
    try {
      await logout();
    } finally {
      setSession(null);
    }
  }, []);

  const activeStep: Step = useMemo(() => {
    if (!origin) return 'origin';
    if (!dest) return 'dest';
    return 'ready';
  }, [origin, dest]);

  useEffect(() => { setStep(activeStep); }, [activeStep]);

  const pushHistory = useCallback((entry: HistoryEntry) => {
    historyRef.current.push(entry);
  }, []);

  const settleOrigin = useCallback((data: CircleData) => {
    pushHistory({ origin, dest });
    setOrigin(data);
    lastCommittedRef.current = { type: 'origin' };
  }, [origin, dest, pushHistory]);

  const settleDest = useCallback((data: CircleData) => {
    pushHistory({ origin, dest });
    setDest(data);
    lastCommittedRef.current = { type: 'dest' };
  }, [origin, dest, pushHistory]);

  const moveOrigin = useCallback((data: CircleData) => {
    setOrigin(data);
  }, []);

  const moveDest = useCallback((data: CircleData) => {
    setDest(data);
  }, []);

  const handleCancelDraw = useCallback(() => {
    // In-progress draw is dropped in MapView; nothing to commit here
  }, []);

  const undo = useCallback(() => {
    const last = historyRef.current.pop();
    if (!last) return;
    setOrigin(last.origin);
    setDest(last.dest);
    lastCommittedRef.current = null;
  }, []);

  const reset = useCallback(() => {
    if (!origin && !dest) return;
    historyRef.current = [];
    setOrigin(null);
    setDest(null);
    setDepartDate('');
    setResults([]);
    setJobProgress(null);
    setHasSearched(false);
    setSearchError(null);
    lastCommittedRef.current = null;
  }, [origin, dest]);

  const requestReset = useCallback(() => {
    if (!origin && !dest) return;
    setConfirm('reset');
  }, [origin, dest]);

  const cancelReset = useCallback(() => {
    setConfirm(null);
  }, []);

  const confirmReset = useCallback(() => {
    reset();
    setConfirm(null);
  }, [reset]);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement | null;
      if (target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA')) return;
      if (e.key === 'Escape' && confirm) {
        e.preventDefault();
        cancelReset();
        return;
      }
      if (confirm) return;
      if (e.key === 'Escape' && (origin || dest)) {
        e.preventDefault();
        if (dest) {
          pushHistory({ origin, dest });
          setDest(null);
          lastCommittedRef.current = null;
        } else if (origin) {
          pushHistory({ origin, dest });
          setOrigin(null);
          lastCommittedRef.current = null;
        }
        return;
      }
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'z') {
        e.preventDefault();
        undo();
        return;
      }
      if (e.key.toLowerCase() === 'r') {
        e.preventDefault();
        requestReset();
        return;
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [origin, dest, undo, requestReset, confirm, cancelReset, pushHistory]);

  useEffect(() => {
    if (confirm) {
      const t = setTimeout(() => setConfirm(null), 6000);
      return () => clearTimeout(t);
    }
  }, [confirm]);

  const handleSearch = async () => {
    if (!origin || !dest || !departDate) {
      setError('Pick a departure date');
      setTimeout(() => setError(null), 2500);
      return;
    }
    setError(null);
    setSearchError(null);
    setSearching(true);
    setResults([]);
    setJobProgress(null);
    setHasSearched(true);

    try {
      const submit = await submitJob({
        origin_lat: origin.lat,
        origin_lng: origin.lng,
        origin_r: origin.radius,
        dest_lat: dest.lat,
        dest_lng: dest.lng,
        dest_r: dest.radius,
        depart_from: departDate,
        depart_to: departDate,
      });

      const poll = async (): Promise<void> => {
        const job = await getJob(submit.job_id, true);
        setJobProgress({ completed: job.completed_pairs, total: job.total_pairs });
        if (job.status === 'complete' || job.status === 'failed') {
          const fares: ParsedFare[] = (job.results ?? [])
            .filter((r: Result) => r.status === 'complete' && r.fare && r.fare.length > 0)
            .flatMap((r: Result) =>
              (r.fare ?? []).map((f) => ({
                id: `${r.origin}-${r.destination}-${f.FlightNumber}-${f.DepartureAt}`,
                origin: f.OriginAirport || r.origin,
                dest: f.DestinationAirport || r.destination,
                date: r.departure_date,
                price: f.Price / 100,
                airline: f.Airline,
                flightNumber: f.FlightNumber,
                transfers: f.Transfers,
                departureAt: f.DepartureAt,
                link: f.Link,
                cached: f.Cached,
              })),
            )
            .sort((a, b) => a.price - b.price)
            .slice(0, 20);
          setResults(fares);
          setSearching(false);
          return;
        }
        await new Promise((r) => setTimeout(r, 1500));
        return poll();
      };

      await poll();
    } catch (e: any) {
      setSearchError(e?.message || 'Search failed');
      setSearching(false);
    }
  };

  const canSearch = origin && dest && departDate && !searching;
  const hasAnyCircle = origin || dest;

  return (
    <div className="app-shell">
      <div className="app-map">
        <MapView
          originCircle={origin}
          destCircle={dest}
          onOriginSettled={settleOrigin}
          onDestSettled={settleDest}
          onOriginChange={moveOrigin}
          onDestChange={moveDest}
          onCancelDraw={handleCancelDraw}
          activeStep={step}
        />
      </div>

      <header className="topbar">
        <div className="brand">
          <div className="brand-mark">
            <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M17.8 19.2 16 11l3.5-3.5C21 6 21.5 4 21 3c-1-.5-3 0-4.5 1.5L13 8 4.8 6.2c-.5-.1-.9.1-1.1.5l-.3.5c-.2.5-.1 1 .3 1.3L9 12l-2 3H4l-1 1 3 2 2 3 1-1v-3l3-2 3.5 5.3c.3.4.8.5 1.3.3l.5-.2c.4-.3.6-.7.5-1.2z"/>
            </svg>
          </div>
          <div className="brand-text">
            <div className="brand-title">Circles</div>
          </div>
        </div>

        {hasAnyCircle && (
          <button
            type="button"
            onClick={requestReset}
            className="reset-button"
            aria-label="Reset circles"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
              <path d="M3 12a9 9 0 1 0 3-6.7L3 8" />
              <path d="M3 3v5h5" />
            </svg>
            <span>Reset</span>
          </button>
        )}

        {session && (
          <div className="account-pill">
            <span className="account-email">{session.email}</span>
            <button type="button" onClick={handleLogout} className="account-logout">
              Sign out
            </button>
          </div>
        )}

        <div className="search-panel">
          <div className="date-field">
            <label className="date-label" htmlFor="depart-date">Depart</label>
            <input
              id="depart-date"
              type="date"
              value={departDate}
              onChange={(e) => setDepartDate(e.target.value)}
              className="date-input"
            />
          </div>
          <button
            onClick={handleSearch}
            disabled={!canSearch}
            className="search-button"
          >
            {searching ? (
              <span className="loading-content">
                <span className="spinner" />
                <span>Searching</span>
              </span>
            ) : (
              <span className="search-content">
                <span>Find flights</span>
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                  <line x1="5" y1="12" x2="19" y2="12" />
                  <polyline points="12 5 19 12 12 19" />
                </svg>
              </span>
            )}
          </button>
        </div>
      </header>

      <MagicLinkModal
        open={showAuthModal}
        onClose={() => {
          setShowAuthModal(false);
          refreshSession();
        }}
      />

      {error && <div className="toast">{error}</div>}

      {confirm === 'reset' && (
        <div className="confirm-bar">
          <div className="confirm-text">Clear all circles?</div>
          <div className="confirm-actions">
            <button type="button" onClick={cancelReset} className="confirm-btn confirm-btn--cancel">Cancel</button>
            <button type="button" onClick={confirmReset} className="confirm-btn confirm-btn--confirm">Clear</button>
          </div>
        </div>
      )}

      <div className="hint-bar">
        {!origin && (
          <div className="hint">
            <span className="hint-icon">✕</span>
            <span><strong>Click & drag</strong> on the map to set your <strong>origin</strong> · <kbd>Space</kbd> pan</span>
          </div>
        )}
        {origin && !dest && (
          <div className="hint">
            <span className="hint-icon">◎</span>
            <span><strong>Click & drag</strong> to set your <strong>destination</strong> · <kbd>Esc</kbd> to undo</span>
          </div>
        )}
        {origin && dest && !departDate && (
          <div className="hint">
            <span className="hint-icon">📅</span>
            <span>Pick a <strong>departure date</strong> · <kbd>Esc</kbd> undo · <kbd>Space</kbd> pan · trackpad: two-finger drag</span>
          </div>
        )}
        {canSearch && (
          <div className="hint hint--ready">
            <span className="hint-icon">🚀</span>
            <span>Ready — hit Find Flights · <kbd>Esc</kbd> undo · <kbd>R</kbd> reset</span>
          </div>
        )}
      </div>

      {hasSearched && (
        <ResultsPanel
          searching={searching}
          results={results}
          jobProgress={jobProgress}
          error={searchError}
          onClose={() => {
            setHasSearched(false);
            setResults([]);
            setSearchError(null);
          }}
        />
      )}

      <style>{`
        .app-shell {
          position: fixed;
          inset: 0;
          background: #0a0a14;
          font-family: -apple-system, BlinkMacSystemFont, "Inter", "SF Pro Display", system-ui, sans-serif;
          color: white;
          -webkit-font-smoothing: antialiased;
          overflow: hidden;
        }
        .app-map { position: absolute; inset: 0; }

        .topbar {
          position: absolute;
          top: 0;
          left: 0;
          right: 0;
          display: flex;
          justify-content: space-between;
          align-items: center;
          padding: 16px 20px;
          z-index: 1000;
          pointer-events: none;
          gap: 12px;
        }
        .topbar > * { pointer-events: auto; }

        .brand {
          display: flex;
          align-items: center;
          gap: 10px;
          background: rgba(20, 20, 30, 0.75);
          backdrop-filter: blur(20px) saturate(180%);
          -webkit-backdrop-filter: blur(20px) saturate(180%);
          border: 1px solid rgba(255,255,255,0.08);
          padding: 10px 14px;
          border-radius: 12px;
          box-shadow: 0 8px 32px rgba(0,0,0,0.4);
        }
        .brand-mark {
          width: 30px;
          height: 30px;
          background: linear-gradient(135deg, #818cf8 0%, #f472b6 100%);
          border-radius: 8px;
          display: flex;
          align-items: center;
          justify-content: center;
          color: white;
        }
        .brand-title {
          font-size: 16px;
          font-weight: 700;
          letter-spacing: -0.02em;
          line-height: 1;
        }

        .reset-button {
          display: flex;
          align-items: center;
          gap: 6px;
          background: rgba(20, 20, 30, 0.75);
          backdrop-filter: blur(20px) saturate(180%);
          -webkit-backdrop-filter: blur(20px) saturate(180%);
          border: 1px solid rgba(255,255,255,0.08);
          color: rgba(255,255,255,0.8);
          padding: 10px 14px;
          border-radius: 12px;
          font-size: 13px;
          font-weight: 600;
          cursor: pointer;
          transition: all 0.2s ease;
          box-shadow: 0 8px 32px rgba(0,0,0,0.4);
          animation: hint-pop 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
        }
        .reset-button:hover {
          background: rgba(40, 40, 60, 0.9);
          color: white;
          border-color: rgba(239, 68, 68, 0.4);
        }

        .account-pill {
          display: flex;
          align-items: center;
          gap: 8px;
          background: rgba(20, 20, 30, 0.75);
          backdrop-filter: blur(20px) saturate(180%);
          -webkit-backdrop-filter: blur(20px) saturate(180%);
          border: 1px solid rgba(255,255,255,0.08);
          padding: 8px 8px 8px 14px;
          border-radius: 12px;
          box-shadow: 0 8px 32px rgba(0,0,0,0.4);
          animation: hint-pop 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
        }
        .account-email {
          font-size: 12px;
          font-weight: 600;
          color: rgba(255,255,255,0.8);
          max-width: 160px;
          overflow: hidden;
          text-overflow: ellipsis;
          white-space: nowrap;
        }
        .account-logout {
          background: rgba(255,255,255,0.06);
          border: none;
          color: rgba(255,255,255,0.7);
          padding: 6px 10px;
          border-radius: 8px;
          font-size: 11px;
          font-weight: 600;
          cursor: pointer;
          transition: all 0.2s ease;
        }
        .account-logout:hover {
          background: rgba(239, 68, 68, 0.15);
          color: #fca5a5;
        }

        .search-panel {
          display: flex;
          align-items: flex-end;
          gap: 10px;
          background: rgba(20, 20, 30, 0.75);
          backdrop-filter: blur(20px) saturate(180%);
          -webkit-backdrop-filter: blur(20px) saturate(180%);
          border: 1px solid rgba(255,255,255,0.08);
          padding: 10px 12px 10px 14px;
          border-radius: 12px;
          box-shadow: 0 8px 32px rgba(0,0,0,0.4);
        }
        .date-field {
          display: flex;
          flex-direction: column;
          gap: 4px;
        }
        .date-label {
          font-size: 10px;
          font-weight: 700;
          color: rgba(255,255,255,0.5);
          text-transform: uppercase;
          letter-spacing: 0.08em;
        }
        .date-input {
          background: rgba(255,255,255,0.05);
          border: 1px solid rgba(255,255,255,0.1);
          color: white;
          padding: 8px 10px;
          border-radius: 8px;
          font-size: 13px;
          font-weight: 500;
          outline: none;
          transition: all 0.2s ease;
          color-scheme: dark;
          width: 140px;
        }
        .date-input:focus {
          border-color: #818cf8;
          background: rgba(129, 140, 248, 0.08);
        }
        .search-button {
          background: linear-gradient(135deg, #818cf8 0%, #f472b6 100%);
          border: none;
          color: white;
          padding: 10px 18px;
          border-radius: 9px;
          font-size: 13px;
          font-weight: 600;
          letter-spacing: -0.01em;
          cursor: pointer;
          transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
          box-shadow: 0 4px 16px rgba(129, 140, 248, 0.3);
          white-space: nowrap;
        }
        .search-button:hover:not(:disabled) {
          transform: translateY(-1px);
          box-shadow: 0 6px 20px rgba(129, 140, 248, 0.5);
        }
        .search-button:disabled {
          background: rgba(255,255,255,0.08);
          color: rgba(255,255,255,0.3);
          cursor: not-allowed;
          box-shadow: none;
        }
        .search-content, .loading-content {
          display: flex;
          align-items: center;
          justify-content: center;
          gap: 8px;
        }
        .spinner {
          width: 13px;
          height: 13px;
          border: 2px solid rgba(255,255,255,0.3);
          border-top-color: white;
          border-radius: 50%;
          animation: spin 0.8s linear infinite;
        }
        @keyframes spin { to { transform: rotate(360deg); } }

        .hint-bar {
          position: absolute;
          bottom: 24px;
          left: 50%;
          transform: translateX(-50%);
          z-index: 1000;
        }
        .hint {
          background: rgba(20, 20, 30, 0.85);
          backdrop-filter: blur(20px);
          -webkit-backdrop-filter: blur(20px);
          border: 1px solid rgba(255,255,255,0.08);
          padding: 12px 20px;
          border-radius: 999px;
          display: flex;
          align-items: center;
          gap: 10px;
          font-size: 14px;
          color: rgba(255,255,255,0.9);
          box-shadow: 0 8px 32px rgba(0,0,0,0.4);
          animation: hint-pop 0.4s cubic-bezier(0.34, 1.56, 0.64, 1);
        }
        .hint--ready {
          background: linear-gradient(135deg, rgba(129, 140, 248, 0.2) 0%, rgba(244, 114, 182, 0.2) 100%);
          border-color: rgba(129, 140, 248, 0.3);
        }
        .hint strong { color: white; font-weight: 700; }
        .hint-icon {
          width: 22px;
          height: 22px;
          border-radius: 50%;
          background: linear-gradient(135deg, #818cf8 0%, #f472b6 100%);
          display: inline-flex;
          align-items: center;
          justify-content: center;
          font-size: 11px;
          font-weight: 700;
          color: white;
        }
        .hint kbd {
          display: inline-block;
          padding: 1px 6px;
          margin: 0 2px;
          background: rgba(255,255,255,0.08);
          border: 1px solid rgba(255,255,255,0.15);
          border-radius: 4px;
          font-family: inherit;
          font-size: 11px;
          font-weight: 600;
          color: rgba(255,255,255,0.85);
        }
        @keyframes hint-pop {
          from { opacity: 0; transform: translateY(8px); }
          to { opacity: 1; transform: translateY(0); }
        }

        .toast {
          position: absolute;
          top: 84px;
          left: 50%;
          transform: translateX(-50%);
          background: rgba(239, 68, 68, 0.95);
          color: white;
          padding: 10px 20px;
          border-radius: 10px;
          font-size: 13px;
          font-weight: 500;
          z-index: 1100;
          box-shadow: 0 8px 24px rgba(239, 68, 68, 0.3);
          animation: toast-pop 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
        }
        @keyframes toast-pop {
          from { opacity: 0; transform: translateX(-50%) translateY(-10px); }
          to { opacity: 1; transform: translateX(-50%) translateY(0); }
        }

        .confirm-bar {
          position: absolute;
          top: 84px;
          left: 50%;
          transform: translateX(-50%);
          display: flex;
          align-items: center;
          gap: 14px;
          background: rgba(20, 20, 30, 0.95);
          backdrop-filter: blur(20px);
          -webkit-backdrop-filter: blur(20px);
          border: 1px solid rgba(239, 68, 68, 0.4);
          padding: 10px 14px 10px 18px;
          border-radius: 14px;
          z-index: 1100;
          box-shadow: 0 12px 40px rgba(0,0,0,0.5);
          animation: confirm-pop 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
        }
        @keyframes confirm-pop {
          from { opacity: 0; transform: translateX(-50%) translateY(-10px); }
          to { opacity: 1; transform: translateX(-50%) translateY(0); }
        }
        .confirm-text {
          font-size: 14px;
          font-weight: 600;
          color: white;
        }
        .confirm-actions {
          display: flex;
          gap: 6px;
        }
        .confirm-btn {
          border: none;
          padding: 7px 14px;
          border-radius: 8px;
          font-size: 13px;
          font-weight: 600;
          cursor: pointer;
          transition: all 0.15s ease;
        }
        .confirm-btn--cancel {
          background: rgba(255,255,255,0.08);
          color: rgba(255,255,255,0.85);
        }
        .confirm-btn--cancel:hover {
          background: rgba(255,255,255,0.15);
          color: white;
        }
        .confirm-btn--confirm {
          background: #ef4444;
          color: white;
        }
        .confirm-btn--confirm:hover {
          background: #dc2626;
        }
      `}</style>
    </div>
  );
}

export default App;
