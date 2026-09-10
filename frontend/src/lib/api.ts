export interface SearchRequest {
  origin_lat: number;
  origin_lng: number;
  origin_r: number;
  dest_lat: number;
  dest_lng: number;
  dest_r: number;
  depart_from: string;
  depart_to: string;
}

export interface SubmitResponse {
  job_id: string;
  total_pairs: number;
}

export interface FareOption {
  Price: number;
  Currency: string;
  Airline: string;
  FlightNumber: string;
  Origin: string;
  Destination: string;
  OriginAirport: string;
  DestinationAirport: string;
  DepartureAt: string;
  Duration: number;
  Transfers: number;
  Link: string;
}

export interface Result {
  origin: string;
  destination: string;
  departure_date: string;
  status: string;
  fare?: FareOption[];
  error?: string;
}

export interface JobResponse {
  id: string;
  status: 'pending' | 'running' | 'complete' | 'failed';
  submitted_at: string;
  completed_at?: string;
  total_pairs: number;
  completed_pairs: number;
  failed_pairs: number;
  error_summary?: string;
  results?: Result[];
}

const API_BASE = 'http://localhost:8080';

export async function submitJob(req: SearchRequest): Promise<SubmitResponse> {
  const res = await fetch(`${API_BASE}/jobs`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ message: 'request failed' }));
    throw new Error(err.message || `HTTP ${res.status}`);
  }
  return res.json();
}

export async function getJob(id: string, includeResults = true): Promise<JobResponse> {
  const qs = includeResults ? '?include=results' : '';
  const res = await fetch(`${API_BASE}/jobs/${id}${qs}`);
  if (!res.ok) {
    const err = await res.json().catch(() => ({ message: 'request failed' }));
    throw new Error(err.message || `HTTP ${res.status}`);
  }
  return res.json();
}
