import { X } from 'lucide-react';
import carriers from '../lib/carriers.json';

export interface ParsedFare {
  id: string;
  origin: string;
  dest: string;
  date: string;
  price: number;
  airline: string;
  flightNumber: string;
  transfers: number;
  departureAt: string;
  link: string;
  cached: boolean;
}

interface ResultsPanelProps {
  searching: boolean;
  results: ParsedFare[];
  jobProgress: { completed: number; total: number } | null;
  error: string | null;
  onClose: () => void;
}

const airlineNames: Record<string, string> = carriers;

function airlineName(code: string): string {
  return airlineNames[code] ?? code;
}

/** Fare.Link is relative for Travelpayouts (e.g. "/search/EWR...") but a
 * complete URL for other providers (Duffel, the local-only Google Flights
 * scraper) — use it as-is when it already has a scheme, otherwise treat it
 * as an Aviasales-relative path. */
function bookingUrl(link: string): string {
  return /^https?:\/\//.test(link) ? link : `https://www.aviasales.com${link}`;
}

function formatDepartureTime(iso: string, fallbackDate: string): string {
  if (!iso) return fallbackDate;
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return fallbackDate;
  const time = d.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit', hour12: true });
  return `${fallbackDate} · ${time}`;
}

type Phase = 'searching' | 'error' | 'empty' | 'results';

export function ResultsPanel({ searching, results, jobProgress, error, onClose }: ResultsPanelProps) {
  const phase: Phase = searching ? 'searching' : error ? 'error' : results.length === 0 ? 'empty' : 'results';

  return (
    <div className="results-tray">
      <div className="results-header">
        <div>
          {phase === 'searching' && (
            <>
              <div className="results-title">Searching…</div>
              <div className="results-sub">
                {jobProgress ? `${jobProgress.completed} / ${jobProgress.total} pairs checked` : 'Submitting job…'}
              </div>
            </>
          )}
          {phase === 'error' && (
            <>
              <div className="results-title">Search failed</div>
              <div className="results-sub">{error}</div>
            </>
          )}
          {phase === 'empty' && (
            <>
              <div className="results-title">No fares found</div>
              <div className="results-sub">Try a wider radius or a different date</div>
            </>
          )}
          {phase === 'results' && (
            <>
              <div className="results-title">Cheapest one-way fares</div>
              <div className="results-sub">{results.length} routes found</div>
            </>
          )}
        </div>
        {phase !== 'searching' && (
          <button className="results-close" onClick={onClose} aria-label="Close results">
            <X size={16} strokeWidth={2.5} />
          </button>
        )}
      </div>
      <div className="results-list">
        {phase === 'searching' && <div className="results-empty">Finding the best fares…</div>}
        {phase === 'error' && <div className="results-empty results-empty--error">{error}</div>}
        {phase === 'empty' && (
          <div className="results-empty">No fares matched your search. Try widening your circles or picking a different date.</div>
        )}
        {phase === 'results' &&
          results.map((r, i) => (
            <div key={r.id} className="result-card" style={{ animationDelay: `${i * 60}ms` }}>
              <div className="result-route">
                <div className="result-airport">{r.origin}</div>
                <div className="result-line">
                  <div className="result-dot" />
                  <div className="result-dash" />
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M21 16v-2l-8-5V3.5c0-.83-.67-1.5-1.5-1.5S10 2.67 10 3.5V9l-8 5v2l8-2.5V19l-2 1.5V22l3.5-1 3.5 1v-1.5L13 19v-5.5l8 2.5z" />
                  </svg>
                  <div className="result-dash" />
                  <div className="result-dot" />
                </div>
                <div className="result-airport">{r.dest}</div>
              </div>
              <div className="result-airline">
                <span className="result-airline-line">
                  <span>
                    {airlineName(r.airline)} {r.flightNumber}
                  </span>
                  <span className="result-transfers">
                    {r.transfers === 0 ? 'Direct' : `${r.transfers} stop${r.transfers > 1 ? 's' : ''}`}
                  </span>
                  {r.cached && <span className="result-cached-badge">Cached</span>}
                </span>
                <span className="result-date">{formatDepartureTime(r.departureAt, r.date)}</span>
              </div>
              <div className="result-price">${r.price}</div>
              {r.link && (
                <a
                  className="result-book-btn"
                  href={bookingUrl(r.link)}
                  target="_blank"
                  rel="noopener noreferrer"
                  aria-label={`Book ${r.origin} to ${r.dest} for $${r.price}`}
                >
                  <span>Book</span>
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                    <line x1="7" y1="17" x2="17" y2="7" />
                    <polyline points="7 7 17 7 17 17" />
                  </svg>
                </a>
              )}
            </div>
          ))}
      </div>

      <style>{`
        .results-tray {
          position: absolute;
          bottom: 24px;
          left: 20px;
          right: 20px;
          max-height: 50vh;
          background: rgba(18, 18, 18, 0.9);
          backdrop-filter: blur(24px) saturate(180%);
          -webkit-backdrop-filter: blur(24px) saturate(180%);
          border: 1px solid rgba(255,255,255,0.08);
          border-radius: 18px;
          z-index: 998;
          box-shadow: 0 -8px 32px rgba(0,0,0,0.4);
          display: flex;
          flex-direction: column;
          overflow: hidden;
          animation: slide-up 0.4s cubic-bezier(0.34, 1.56, 0.64, 1);
        }
        @keyframes slide-up {
          from { opacity: 0; transform: translateY(20px); }
          to { opacity: 1; transform: translateY(0); }
        }
        .results-header {
          display: flex;
          justify-content: space-between;
          align-items: center;
          padding: 16px 20px;
          border-bottom: 1px solid rgba(255,255,255,0.06);
        }
        .results-title {
          font-size: 14px;
          font-weight: 700;
          letter-spacing: -0.01em;
        }
        .results-sub {
          font-size: 12px;
          color: rgba(255,255,255,0.5);
          margin-top: 2px;
        }
        .results-close {
          background: rgba(255,255,255,0.05);
          border: none;
          color: rgba(255,255,255,0.6);
          width: 28px;
          height: 28px;
          border-radius: 8px;
          display: flex;
          align-items: center;
          justify-content: center;
          cursor: pointer;
          transition: all 0.2s ease;
        }
        .results-close:hover {
          background: rgba(255,255,255,0.1);
          color: white;
        }
        .results-list {
          overflow-y: auto;
          padding: 12px;
          display: flex;
          flex-direction: column;
          gap: 8px;
        }
        .result-card {
          display: grid;
          grid-template-columns: 1.4fr 1.2fr auto auto;
          align-items: center;
          gap: 16px;
          padding: 14px 16px;
          background: rgba(255,255,255,0.03);
          border: 1px solid rgba(255,255,255,0.05);
          border-radius: 12px;
          transition: all 0.2s ease;
          animation: card-pop 0.4s cubic-bezier(0.34, 1.56, 0.64, 1) both;
        }
        .result-card:hover {
          background: rgba(255,255,255,0.06);
          border-color: rgba(255,255,255,0.25);
          transform: translateX(2px);
        }
        @keyframes card-pop {
          from { opacity: 0; transform: translateY(8px); }
          to { opacity: 1; transform: translateY(0); }
        }
        .result-route {
          display: flex;
          align-items: center;
          gap: 10px;
        }
        .result-airport {
          font-size: 15px;
          font-weight: 700;
          letter-spacing: -0.01em;
        }
        .result-line {
          display: flex;
          align-items: center;
          gap: 2px;
          color: rgba(255,255,255,0.4);
        }
        .result-dot {
          width: 4px;
          height: 4px;
          border-radius: 50%;
          background: currentColor;
        }
        .result-dash {
          width: 16px;
          height: 1px;
          background: currentColor;
        }
        .result-airline {
          font-size: 12px;
          font-weight: 600;
          color: rgba(255,255,255,0.7);
          display: flex;
          flex-direction: column;
          gap: 3px;
        }
        .result-airline-line {
          display: flex;
          align-items: center;
          gap: 8px;
        }
        .result-transfers {
          font-size: 10px;
          font-weight: 600;
          color: rgba(255, 255, 255, 0.75);
          background: rgba(255, 255, 255, 0.08);
          border: 1px solid rgba(255, 255, 255, 0.15);
          padding: 1px 7px;
          border-radius: 999px;
        }
        .result-cached-badge {
          font-size: 10px;
          font-weight: 600;
          color: rgba(255, 255, 255, 0.75);
          background: transparent;
          border: 1px dashed rgba(255, 255, 255, 0.3);
          padding: 1px 7px;
          border-radius: 999px;
        }
        .result-date {
          font-size: 10px;
          font-weight: 500;
          color: rgba(255,255,255,0.4);
        }
        .result-book-btn {
          display: inline-flex;
          align-items: center;
          gap: 5px;
          background: #fff;
          color: #0a0a0a;
          text-decoration: none;
          font-size: 12px;
          font-weight: 700;
          letter-spacing: 0.01em;
          padding: 8px 14px;
          border-radius: 8px;
          white-space: nowrap;
          box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
          transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
        }
        .result-book-btn:hover {
          transform: translateY(-1px);
          background: rgba(255, 255, 255, 0.88);
          box-shadow: 0 6px 18px rgba(0, 0, 0, 0.4);
        }
        .result-book-btn:active {
          transform: translateY(0);
        }
        .results-empty {
          padding: 32px 16px;
          text-align: center;
          color: rgba(255,255,255,0.5);
          font-size: 13px;
        }
        .results-empty--error {
          color: rgba(255, 255, 255, 0.85);
          font-weight: 600;
        }
        .result-price {
          font-size: 18px;
          font-weight: 700;
          color: #fff;
          letter-spacing: -0.02em;
        }
      `}</style>
    </div>
  );
}
