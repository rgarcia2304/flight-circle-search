import { test, expect } from '@playwright/test';
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
  OriginAirport: string;
  DestinationAirport: string;
  Transfers: number;
}

interface JobResponse {
  id: string;
  status: string;
  total_pairs: number;
  completed_pairs: number;
  failed_pairs: number;
  results?: Array<{
    origin: string;
    destination: string;
    status: string;
    fare?: Fare[];
    error?: string;
  }>;
}

test.describe('Live E2E: medium route (NYC↔LAX, 200km)', () => {
  test('completes with no hard failures and surfaces real fares', async () => {
    test.setTimeout(200_000);
    const ctx = await liveApiContext();
    const res = await ctx.post('/jobs', {
      data: {
        origin_lat: 40.7128,
        origin_lng: -74.006,
        origin_r: 200,
        dest_lat: 34.0522,
        dest_lng: -118.2437,
        dest_r: 200,
        depart_from: '2026-09-15',
        depart_to: '2026-09-15',
      } satisfies SearchRequest,
    });
    expect(res.status()).toBe(202);
    const submit = (await res.json()) as { job_id: string; total_pairs: number };

    expect(submit.total_pairs).toBeGreaterThan(10);

    const deadline = Date.now() + 180_000;
    let job: JobResponse | null = null;
    while (Date.now() < deadline) {
      const poll = await ctx.get(`/jobs/${submit.job_id}?include=results`);
      expect(poll.status()).toBe(200);
      job = (await poll.json()) as JobResponse;
      if (job.status === 'complete' || job.status === 'failed') break;
      await new Promise((r) => setTimeout(r, 2000));
    }
    expect(job).not.toBeNull();
    expect(job!.status).toBe('complete');
    expect(job!.completed_pairs).toBe(submit.total_pairs);

    const withFares = job!.results!.filter((r) => r.status === 'complete' && r.fare && r.fare.length > 0);
    const dead = job!.results!.filter((r) => r.status === 'dead_letter');
    console.log(
      `[medium] total=${job!.total_pairs} complete=${withFares.length} dead_letter=${dead.length}`,
    );

    // A 200km radius sweeps in small regional airports that legitimately have
    // no affiliate fare data on a given day — that's live third-party data
    // sparsity, not a pipeline error. Assert no hard failures and that the
    // search actually surfaces real fares, rather than an arbitrary coverage %.
    expect(dead.length, 'no pair should hard-fail').toBe(0);
    expect(withFares.length, 'search should surface at least some fares').toBeGreaterThan(0);

    for (const r of withFares.slice(0, 5)) {
      const f = r.fare![0];
      console.log(
        `  ${f.OriginAirport}→${f.DestinationAirport} $${(f.Price / 100).toFixed(2)} ${f.Currency} (${f.Airline})`,
      );
    }
  });
});
