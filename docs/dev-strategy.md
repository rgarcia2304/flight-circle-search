# Development Strategy: Circle-to-Circle Flight Search

## 1. Architecture

### Component responsibilities

**Map UI (client)**
- User draws two circles (center + radius) and picks a date or date range
- Subscribes to a progress stream once a search is submitted
- Renders results live as they arrive, not just at the end

**Edge API**
- Authenticates the request (JWT from OAuth login)
- Rate-limits per user (protects your fare-provider quota — this is the real reason auth exists here)
- Resolves each circle to a candidate airport list:
  1. Query a static airport reference table (from OpenFlights) for all airports within the circle's radius
  2. Filter that list against a precomputed route-existence graph (also from OpenFlights route data) so you only keep airport pairs that are actually flown, even indirectly
  3. Rank remaining candidates by hub size so the worker pool can search major hubs first
- Writes a job record (status: `pending`) and enqueues one task per surviving airport pair × date
- Returns a job ID immediately — this is an async submission, not a synchronous response

**Job engine (queue + worker pool)**
- Workers pull tasks with a **bounded concurrency limit** (e.g., max 5 in flight), not unbounded — this is what keeps you inside the fare provider's rate limit
- Each worker: check cache first → on miss, call the fare provider adapter → write result → publish a progress event → update job record
- On task failure (timeout, 429, malformed response): retry with backoff up to N times, then mark that specific task `failed` without failing the whole job — partial results are still useful

**Fare provider adapter**
- A thin interface — `search(origin, destination, date) -> Fare[]` — with Travelpayouts as the first implementation and Duffel as a second, swappable without touching business logic
- Owns the cache-aside logic: `(origin, destination, date)` is the cache key, TTL 6–24h
- Owns rate-limit backoff and circuit-breaking against the upstream provider

**External fare API**
- Third-party, untrusted-by-default — every response gets schema-validated before anything downstream trusts it

**Progress stream (pub/sub)**
- Worker task completions publish events; the client's SSE/WebSocket connection subscribes and pushes "top 10 so far" updates
- A Cron Trigger separately pre-warms popular circle-pairs overnight and re-checks saved price-watch alerts, publishing to the same notification path

### Data stores
- **Relational (D1/Postgres)**: users, saved searches, job records, airport reference data, route-existence graph
- **Cache (KV/Redis)**: fare results keyed by `(origin, destination, date)`, rate-limit counters
- **Queue**: task distribution, decoupled from the request/response cycle

---

## 2. Development strategy: function by function with an agent

The core discipline: **the agent implements, you own the spec and the adversarial review.** Never let the agent define what "correct" means for a function — only how to achieve a definition you've already pinned down.

### The loop, per function or small unit of work

1. **You write (or tightly review) the contract first** — inputs, outputs, and explicitly, the error/edge cases it must handle. This is the cheapest point to catch scope drift or missing edge cases, before any code exists.
2. **Agent writes tests against the contract, before implementation.** Require it to explicitly enumerate: the happy path, boundary conditions, malformed/adversarial input, and failure modes of anything external (timeouts, rate limits, partial data).
3. **You review the test list — not the implementation yet.** This is the single highest-leverage checkpoint in the whole loop (see Section 3). Reject and send back if the tests only cover the happy path.
4. **Agent implements against the approved tests.**
5. **You review the diff.** Small unit size (one function or tightly-scoped module at a time) makes this actually readable — don't let the agent batch multiple functions into one unreviewable change.
6. **Run the objective checks** (mutation score, coverage, integration test) before merge. Treat these as gates, not suggestions.
7. **Merge, then move to the next unit.**

### Suggested build order

1. Airport reference data ingestion + circle→airport resolution (pure function, no external dependencies, easy to test exhaustively)
2. Route-existence graph filtering
3. Fare provider adapter interface + one concrete implementation, tested against **recorded real responses**, not agent-imagined mocks
4. Cache-aside layer (key design, TTL, invalidation)
5. Job model + queue submission + bounded-concurrency worker consumption
6. Progress aggregation + pub/sub streaming to the client
7. Auth + per-user rate limiting
8. Frontend map integration
9. Cron jobs: cache pre-warming, price-watch alerts
10. Observability, load testing, security hardening pass
11. CI/CD and deployment

Building in this order means every later stage has a tested, trustworthy foundation under it — you're never building concurrency logic on top of an unverified data layer.

---

## 3. How to catch an agent writing happy-path-only tests

This is the part most people get wrong, so treat it as seriously as the code itself. Passing tests and coverage percentage tell you almost nothing about test quality — an agent can write tests that pass 100% of the time and verify almost nothing.

**Mutation testing — the single best objective signal.** Tools like Stryker (JS/TS) or mutmut (Python) automatically introduce small bugs into your code (flip a comparison, off-by-one an index, invert a boolean) and check whether your test suite catches them. A high line-coverage percentage with a low mutation score is the exact fingerprint of happy-path-only tests — the lines got executed, but nothing actually checked the result was right. Set a minimum mutation-score threshold in CI and treat it as a hard gate, the same way you'd treat a failing test.

**Require named adversarial cases in the spec, not left to agent discretion.** For every function, explicitly list what must be tested: empty input, maximum-size input, malformed/unexpected shape, concurrent access to the same resource, upstream timeout, upstream returning garbage, negative numbers where you expect positive, the boundary values exactly at your limits. If it's not named, the agent won't reliably think of it — this is not a criticism unique to AI agents, it's true of most developers under deadline pressure too, but it's especially worth being explicit about with an agent since it will happily stop at "tests pass."

**Use recorded real responses (fixture/cassette-style testing) for external integrations.** When testing your fare provider adapter, don't let the agent write its own mock of what it imagines the API returns — record actual responses (including error responses, rate-limit responses, and malformed edge cases you can provoke) and test against those. An agent mocking its own assumed shape of an API is one of the most common sources of tests that pass in development and fail in production.

**Do a manual spot-check by deliberately breaking the implementation yourself.** Pick a function, introduce an obvious bug on purpose (flip a condition, off-by-one a loop), and run the test suite. If it still passes, the tests aren't doing their job — full stop, regardless of what coverage tools report. This takes two minutes and catches things automated tooling sometimes misses.

**Have a second agent session adversarially review the first agent's tests.** A fresh session prompted explicitly as "find gaps in this test suite, assume the author only tested the happy path" will often surface real gaps that a same-context agent, anchored on its own prior reasoning, will miss.

**Require explicit failure-injection tests for anything concurrent or distributed.** For the worker pool and job engine specifically: simulate a provider timeout mid-batch, a duplicate job submission, two workers racing on the same cache key, a partial batch failure (12 of 80 tasks fail). These are exactly the scenarios happy-path tests skip, and exactly the scenarios that will actually happen in production.

**Property-based testing for the geospatial and combinatorial logic.** Tools like fast-check (JS) or Hypothesis (Python) generate hundreds of randomized inputs against invariants you define (e.g., "every returned airport is actually within the radius," "the route graph never returns a pair with no real route"). This surfaces edge cases — antimeridian crossing, radius of zero, overlapping circles — that a human or agent writing individual examples will rarely think to include.

---

## 4. Your role at each checkpoint, summarized

| Checkpoint | What you're checking | Why it's the right gate |
|---|---|---|
| Contract review | Is the spec complete — are edge cases and error modes named? | Cheapest point to catch a missing requirement |
| Test list review (before implementation) | Do the tests cover more than the happy path? | Highest-leverage review in the whole loop |
| Diff review | Does the implementation match the approved tests, and is it readable? | Keeps unit size honest and catches logic issues tests didn't |
| Mutation score / CI gates | Do the tests actually catch injected bugs? | Objective signal, doesn't rely on your judgment alone |
| Manual break-test (spot check) | Does an obvious deliberate bug get caught? | Fast, cheap sanity check independent of tooling |
| Adversarial second-agent review | Are there gaps a fresh perspective can find? | Counteracts anchoring from the implementing agent's own context |

The throughline: you're the spec owner and adversarial reviewer, the agent is the implementer. That division of labor is what lets you move fast without losing the thing that actually matters — knowing the system does what you think it does, for the right reasons, not just that a test suite reports green.
