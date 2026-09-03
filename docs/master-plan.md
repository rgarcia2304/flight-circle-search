# Master plan: build this fast, in the right order

This is the "start here" document. Everything else already produced — architecture, dev strategy, project plan, component inventory, engineering spec, harness setup, AGENTS.md, day-one kickoff — still stands. This adds the one thing missing: an explicit cut list and sequencing designed for speed, not just correctness.

## The core idea: ship a real, ugly, working slice before generalizing anything

The single biggest threat to this project isn't any individual piece of engineering — it's the scope we built up across this whole conversation. The fastest path to something real is to deliberately under-build the first version, then layer the rest of the already-designed architecture on top of something that already works end to end.

## V1 definition of done (the thing that makes this real)

A user can submit **one fixed origin cluster (Northeast US) to one fixed destination cluster (Western Europe), one date**, and get back real prices from a real fare provider, through your own API, with basic bounded concurrency and basic auth. That's it. That's the milestone that proves the whole concept before you spend another hour on anything else.

## What's IN for v1 — build this first, in this order

1. **Python offline ingestion** (`scripts/ingest_openflights.py`) — pull the OpenFlights airport/route data into `data/*.csv`. Do this first and separately; everything else depends on this data existing, and it has zero dependency on anything else being built yet.
2. **`internal/geo` — circle → airport resolution** (today's kickoff task, unchanged).
3. **`internal/routes` — route-existence filtering** against the ingested data.
4. **`internal/fareprovider` — one adapter, Travelpayouts only.** Synchronous calls. No queue yet. Prove real fares come back for a few hardcoded JFK/LHR-style pairs before anything else.
5. **`internal/cache` — cache-aside on `(origin, destination, date)`.** Simple TTL, no fancy invalidation yet.
6. **Bounded worker pool, generalized to the full fixed-corridor combinatorics** (the ~20-60 realistic route-pairs after route-existence filtering). This is where Go's goroutines/channels earn their place.
7. **Job submission + polling** — `POST /v1/searches` and `GET /v1/searches/{id}`. **Cut for v1: skip SSE/live streaming.** Polling every few seconds from the client is fine for a first working version and is dramatically less to build than a progress-event pub/sub pipeline. Add streaming in v2 once the core loop is proven.
8. **Minimal auth** — enough to attribute rate limits to a user, not the full OAuth-rotation-detection system yet. A single hardcoded API key or the simplest possible login is fine for v1; harden it before this is ever public.
9. **Deploy somewhere reachable** — a single container on Koyeb's free tier is enough. Skip the AWS/GCP demo entirely for v1.

## What's OUT of v1 — real, designed, deliberately deferred

- SSE/WebSocket live progress streaming
- Hot-route pre-warming and price-watch alerts
- Second fare provider (Duffel) as fallback
- Arbitrary user-drawn circles (stay on the fixed corridor)
- Full OAuth rotation/reuse-detection, refresh-token family revocation
- GCP/AWS multi-cloud demo
- Full CI/CD polish, Terraform, mutation-testing gates, SAST/DAST — start with tests you write and review yourself; add the automated gates once there's a real codebase for them to run against, not before there's anything to scan

None of this is cut from the plan — it's sequenced. Everything above has a designed home in the existing project-plan.md; v1 just proves the core before you spend time on the rest.

## Rough time-boxing (calibrate against your own actual pace after step 1-2)

| Step | Estimate | Why |
|---|---|---|
| Python ingestion | 1 short session | One-off script, no architecture |
| Geo resolution (today's task) | 1 session | Pure function, but do the test-review properly |
| Route filtering | Half a session | Straightforward once ingestion data exists |
| Fare adapter (single provider, sync) | 1-2 sessions | First real external integration, expect friction |
| Cache | Half a session | Small, well-understood pattern |
| Worker pool | 2-3 sessions | The architecturally densest piece — don't rush the concurrency tests |
| Job submission + polling | 1 session | Simple compared to the streaming version you're deferring |
| Minimal auth | 1 session | Deliberately minimal — full version is v2 |
| Deploy to Koyeb | Half a session | Should be close to trivial if Dockerfiles are clean |

That's roughly 9-12 focused sessions to a real, demoable v1 — track your actual time per step and use it to recalibrate the rest of the project plan's phases, since this is the first real data point on your actual agent-assisted pace.

## The v2+ path, once v1 works

Return to `project-plan.md` exactly as written — Phase 6 onward (pub/sub streaming, hardening pass, generalization to arbitrary circles) is unchanged. The only thing this document changed is what comes first and what waits. Once v1 is live and you've personally watched it return real prices for a real search, every subsequent addition is layered onto something proven, not imagined.

## The one discipline to hold onto through all of this

Every step above still goes through the loop from the dev-strategy doc: contract first, agent writes tests, you review the test list before implementation exists, fresh-context review before merge. Moving fast doesn't mean skipping that — the whole reason v1 is small is so that loop stays fast per step, not so it gets skipped.
