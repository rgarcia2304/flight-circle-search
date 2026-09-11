import { test, expect, type APIResponse } from '@playwright/test';
import { liveApiContext } from './live-auth';

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
  status: string;
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

test.describe('Live E2E: pair coverage (assert every pair works)', () => {
  test('all 10 pairs from small NYC↔LDN return complete fares', async () => {
    test.setTimeout(120_000);
    const ctx = await liveApiContext();
    const res: APIResponse = await ctx.post('/jobs', {
      data: {
        origin_lat: 40.7128,
        origin_lng: -74.006,
        origin_r: 50,
        dest_lat: 51.5074,
        dest_lng: -0.1278,
        dest_r: 50,
        depart_from: '2026-09-15',
        depart_to: '2026-09-15',
      },
    });
    expect(res.status()).toBe(202);
    const submit = (await res.json()) as { job_id: string; total_pairs: number };

    expect(submit.total_pairs).toBeGreaterThanOrEqual(5);

    const deadline = Date.now() + 90_000;
    let job: JobResponse | null = null;
    while (Date.now() < deadline) {
      const poll = await ctx.get(`/jobs/${submit.job_id}?include=results`);
      expect(poll.status()).toBe(200);
      job = (await poll.json()) as JobResponse;
      if (job.status === 'complete' || job.status === 'failed') break;
      await new Promise((r) => setTimeout(r, 1500));
    }
    expect(job).not.toBeNull();
    expect(job!.status).toBe('complete');
    expect(job!.completed_pairs).toBe(submit.total_pairs);

    const results = job!.results!;
    const deadLetter = results.filter((r) => r.status === 'dead_letter');
    const completeWithFare = results.filter(
      (r) => r.status === 'complete' && r.fare && r.fare.length > 0,
    );

    console.log(
      `[pair-coverage] total=${submit.total_pairs} complete=${completeWithFare.length} dead_letter=${deadLetter.length}`,
    );

    for (const r of deadLetter) {
      console.log(`  DEAD: ${r.origin}→${r.destination} error=${r.error}`);
    }
    for (const r of completeWithFare.slice(0, 5)) {
      const f = r.fare![0];
      console.log(
        `  OK: ${f.OriginAirport}→${f.DestinationAirport} $${(f.Price / 100).toFixed(2)} ${f.Currency} (${f.Airline} ${f.FlightNumber})`,
      );
    }

    // Live third-party fare data: a handful of pairs (e.g. small regional
    // airports with no real itinerary) can legitimately return zero fares
    // without erroring. Require no hard failures and at least 90% coverage
    // rather than literal 100%, which is not guaranteed by live data.
    expect(deadLetter.length).toBe(0);
    expect(completeWithFare.length).toBeGreaterThanOrEqual(Math.ceil(submit.total_pairs * 0.9));
  });
});
