import { test, expect, type APIResponse } from '@playwright/test';
import { liveApiContext } from './live-auth';

interface SearchRequest {
  origin_lat: number;
  origin_lng: number;
  origin_r: number;
  dest_lat: number;
  dest_lng: number;
  dest_r: number;
  depart_from: string;
  depart_to: string;
}

interface Fare {
  Price: number;
  Currency: string;
  Airline: string;
  FlightNumber: string;
  OriginAirport: string;
  DestinationAirport: string;
  Transfers: number;
}

interface JobResponse {
  id: string;
  status: 'pending' | 'running' | 'complete' | 'failed';
  total_pairs: number;
  completed_pairs: number;
  failed_pairs: number;
  results?: Array<{
    origin: string;
    destination: string;
    departure_date: string;
    status: string;
    fare?: Fare[];
    error?: string;
  }>;
}

async function submitJob(req: SearchRequest): Promise<{ job_id: string; total_pairs: number }> {
  const ctx = await liveApiContext();
  const res: APIResponse = await ctx.post('/jobs', { data: req });
  expect(res.status(), `submit ${res.status()}`).toBe(202);
  return res.json();
}

async function pollJob(id: string, timeoutMs = 90_000): Promise<JobResponse> {
  const ctx = await liveApiContext();
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const res = await ctx.get(`/jobs/${id}?include=results`);
    expect(res.status(), `poll ${res.status()}`).toBe(200);
    const job = (await res.json()) as JobResponse;
    if (job.status === 'complete' || job.status === 'failed') return job;
    await new Promise((r) => setTimeout(r, 1500));
  }
  throw new Error(`Job ${id} did not complete within ${timeoutMs}ms`);
}

test.describe('Live E2E: small route (NYC↔LDN, 50km)', () => {
  test('all pairs return fares', async () => {
    test.setTimeout(120_000);
    const submit = await submitJob({
      origin_lat: 40.7128,
      origin_lng: -74.006,
      origin_r: 50,
      dest_lat: 51.5074,
      dest_lng: -0.1278,
      dest_r: 50,
      depart_from: '2026-09-15',
      depart_to: '2026-09-15',
    });

    expect(submit.total_pairs).toBeGreaterThan(0);
    expect(submit.total_pairs).toBeLessThanOrEqual(50);

    const job = await pollJob(submit.job_id);
    expect(job.status).toBe('complete');
    expect(job.completed_pairs).toBe(submit.total_pairs);
    expect(job.results).toBeDefined();
    expect(job.results!.length).toBe(submit.total_pairs);

    const withFares = job.results!.filter((r) => r.status === 'complete' && r.fare && r.fare.length > 0);
    const dead = job.results!.filter((r) => r.status === 'dead_letter');
    console.log(
      `[small] total=${job.total_pairs} complete=${withFares.length} dead_letter=${dead.length}`,
    );

    for (const r of withFares) {
      expect(r.fare![0].Price, `${r.origin}→${r.destination} price=0`).toBeGreaterThan(0);
      expect(r.fare![0].Currency, `${r.origin}→${r.destination} has no currency`).toBeTruthy();
      expect(r.fare![0].Airline, `${r.origin}→${r.destination} has no airline`).toBeTruthy();
    }

    for (const r of withFares.slice(0, 3)) {
      const f = r.fare![0];
      console.log(`  ${f.OriginAirport}→${f.DestinationAirport} $${(f.Price / 100).toFixed(2)} ${f.Currency} (${f.Airline} ${f.FlightNumber})`);
    }
  });
});
