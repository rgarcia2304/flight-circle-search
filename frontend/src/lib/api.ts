import { apiFetch, parseJSON } from './auth';

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

export async function submitJob(req: SearchRequest): Promise<SubmitResponse> {
  const res = await apiFetch('/jobs', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  });
  return parseJSON<SubmitResponse>(res);
}

export async function getJob(id: string, includeResults = true): Promise<JobResponse> {
  const qs = includeResults ? '?include=results' : '';
  const res = await apiFetch(`/jobs/${id}${qs}`);
  return parseJSON<JobResponse>(res);
}
