# Architecture Decision Records

A living log. Every non-trivial technical decision gets an entry: context, options actually considered, the decision, and its consequences. This is a real practice at well-run engineering orgs specifically so reasoning survives past the moment it was made — append to this file as new decisions come up during the build, don't just write it once now.

---

## ADR-001: Language and runtime — Go for services, Python for offline prep

**Context**: needed a primary backend language for a mostly I/O-bound system (network-bound fare lookups) with a concurrency-heavy core (bounded worker pool).

**Options considered**: Go, Python/FastAPI, C++.

**Decision**: Go for all live services (API, worker, scheduler); Python only for the one-off OpenFlights ingestion script.

**Why**: goroutines + channels are the idiomatic, minimal-ceremony implementation of a bounded worker pool — the architectural core of this system. Compiles to a static binary (small Docker image, fast cold start, fits a 512MB free-tier container comfortably). Memory-safe by default, which matters given the project's explicit security focus. Weighed against the developer's actual deep Go fluency versus zero FastAPI experience — review quality is bounded by language depth, and Go is where that depth already exists. C++ was rejected outright: this system is network-bound, not compute-bound, so C++'s performance advantage is inapplicable, while its manual memory model directly increases the attack surface the project is trying to learn to defend against.

**Consequences**: no dependency-injection framework magic — auth/rate-limit checks as cross-cutting concerns will be explicit middleware, not `Depends()`-style injection. Slightly more boilerplate for OpenAPI generation (via `swaggo/swag` or `huma`, not automatic-by-default). Acceptable trade for a language the developer can actually adversarially review.

---

## ADR-002: Primary datastore — PostgreSQL

**Context**: need to persist users, saved searches, jobs, per-task results, and reference data with real relational structure and ownership constraints.

**Options considered**: PostgreSQL, MongoDB, SQLite.

**Decision**: PostgreSQL.

**Why**: the data model is inherently relational — jobs have tasks, tasks have owning jobs, resources have owning users, and ownership-filtered queries (`WHERE id = ? AND user_id = ?`) are a core security mechanism (ADR-009). MongoDB was rejected — no schema flexibility requirement exists here, and its weaker transactional guarantees don't help this workload. SQLite was rejected for anything past local single-process testing — its single-writer model would bottleneck the moment a separate worker process needs to write task results concurrently with the API reading job status.

**Consequences**: requires running a real Postgres instance even in local dev (via docker-compose) rather than a zero-setup embedded file. Worth it for the concurrency and constraint guarantees.

---

## ADR-003: Cache and coordination store — Redis

**Context**: need atomic rate-limit counters, a fare-result cache with TTL, and a mechanism for coordinating the job queue.

**Options considered**: Redis, Memcached, in-process Go map.

**Decision**: Redis, serving three roles: cache, rate-limiter backing store, and job queue transport (see ADR-004).

**Why**: an in-process map was rejected immediately — the worker process and API process are separate, so cache/rate-limit state must be shared across processes, not held in one. Memcached was rejected because it lacks the atomic scripting (Lua) needed for race-free token-bucket rate limiting, and lacks the stream data structure needed for ADR-004 — using it would mean running a second system (e.g. a separate queue broker) alongside it anyway, which is more operational surface for no benefit.

**Consequences**: Redis becomes a single point of failure for three distinct concerns at once. Acceptable at this scale; worth revisiting only if the project ever needs to scale each concern independently.

---

## ADR-004: Job queue implementation — Redis Streams, not an in-process channel or a dedicated broker

**Context**: this wasn't fully specified before. A job's route-pair sub-tasks need to be distributed to a worker pool. The open question: does "queue" mean an in-process Go channel, or a real durable message broker?

**Options considered**: in-process Go channel only; a dedicated broker (RabbitMQ, Kafka); Redis Streams with consumer groups.

**Decision**: Redis Streams, using consumer groups for durable task distribution to the worker pool; in-process channels are still used, but only *within* a single worker process to fan out that worker's claimed tasks concurrently to the fare-provider adapter.

**Why**: an in-process-channel-only design was rejected because it doesn't survive a process restart — an in-flight job would silently lose its remaining tasks if the worker process crashed or redeployed, and it can't be split across a separate API and worker service, which the architecture already calls for. A dedicated broker (RabbitMQ/Kafka) was rejected as disproportionate — this project doesn't need Kafka's ordering/partitioning guarantees or RabbitMQ's routing topology, and standing up either is a whole extra service to operate for capability this workload doesn't use. Redis Streams gets durable, at-least-once task distribution with consumer groups using infrastructure you're already running for cache and rate limiting.

**Consequences**: task handlers must be idempotent (a redelivered task after a crash must be safe to reprocess) — this was already true given the fare-cache-aside design, but it's now a hard requirement, not a nice property. Worth an explicit test case in Phase 4.

---

## ADR-005: Job/task data model

**Context**: how a job with N sub-tasks is represented in storage.

**Decision**: two tables. `jobs` (id, user_id, status, params, created_at) holds the overall request. `job_tasks` (job_id, route_pair, status, result, error, attempt_count) holds one row per route-pair sub-task, in Postgres — not just in the cache.

**Why**: task results need to be durable and queryable independent of the cache's TTL, because the price-history feature (deferred to v2, but designed for) depends on a durable record of what was actually returned, when — the cache is for cross-job reuse of identical lookups, not for history.

**Consequences**: a job's final result is reconstructed by querying its `job_tasks` rows, not stored as one denormalized blob — slightly more query complexity, but keeps per-task retry/failure state clean and independently inspectable.

---

## ADR-006: Concurrency bound — global, not per-job

**Context**: how many concurrent outbound fare-provider calls are allowed, and is that limit scoped per-job or globally across the whole worker process?

**Options considered**: per-job semaphore (fair between simultaneous users); global semaphore across the whole worker process.

**Decision**: a global bounded semaphore (start at 5-10 concurrent outbound calls) across the entire worker process for v1.

**Why**: the thing actually being protected is the fare provider's shared rate limit, which doesn't care which job a call belongs to — a global bound protects that limit correctly regardless of how many jobs are in flight. A per-job bound would be more fair to simultaneous users but adds real complexity (tracking fairness across jobs) that has no payoff until there's actually more than one user hitting the system concurrently.

**Consequences**: if two jobs run at once, they compete for the same global concurrency budget — one user's large search could slow another's. Acceptable and explicitly deferred; revisit if real multi-user contention shows up.

---

## ADR-007: Retry and circuit-breaker policy — specific numbers, not "add retries"

**Context**: "handle failures" isn't a real spec — the actual numbers matter and were left unspecified before.

**Decision**: per-task retry: exponential backoff with full jitter, base delay 500ms, multiplier 2, max delay 8s, maximum 3 attempts before the task is marked `failed` (not the whole job). Provider-level circuit breaker: if more than 50% of calls fail within a rolling 1-minute window, open the circuit for 30 seconds and fail fast rather than continuing to call a struggling provider.

**Why**: this is the standard, well-tested backoff shape (the same one AWS's own SDKs use) — full jitter specifically avoids synchronized retry storms across concurrent workers. The circuit breaker protects both your API budget and the provider relationship — hammering a provider that's already failing wastes quota on calls likely to fail anyway.

**Consequences**: a job can complete with some tasks `failed` rather than succeeded — the API and UI must handle and display partial results honestly, not treat any failure as a whole-job failure.

---

## ADR-008: Consistency model for job status

**Context**: needed to state explicitly, not leave implicit, how job status reads and writes behave under concurrency.

**Decision**: job status is eventually consistent from the client's polling perspective — a poll returns a snapshot, not a guaranteed-latest value. Task status *writes* are safe by construction: Redis Streams' consumer-group semantics guarantee a given task is claimed by exactly one worker at a time, so no two workers ever write the same `job_tasks` row concurrently — there is no write-write race to defend against, by design of the distribution mechanism, not by adding locking after the fact.

**Consequences**: this is exactly the kind of claim that needs a concurrency test proving it, not just asserting it — a named test case for Phase 4: "two workers cannot claim the same task" is a required adversarial test, not an assumption to trust.

---

## ADR-009: Ownership-based authorization, 404 over 403

Already covered in the engineering spec — restated here because it belongs in the same decision log as everything else: every resource query filters by `user_id` at the query level, and a mismatch returns 404, not 403, to avoid confirming a resource's existence to a non-owner.

---

## ADR-010: API versioning — URL path, not headers

**Decision**: `/v1/` in the URL path.

**Why**: curl-able and debuggable without needing to inspect headers, cacheable by path at any layer that cares to. A new version is only cut on a genuine breaking change — additive fields never bump the version.

---

## ADR-011: Configuration — twelve-factor, fail-fast at startup

**Decision**: all configuration via environment variables, no config files. Every required variable is validated at process startup — the service refuses to start and logs exactly which variable is missing or malformed, rather than starting successfully and failing on the first request that needs it.

**Why**: "discover a missing `DATABASE_URL` on the first real request" is a bad failure mode you can eliminate for free by checking at boot instead.

---

## ADR-012: Structured logging from the first line of code

**Decision**: JSON structured logs from day one (a Go structured logging library, not `fmt.Println`), even before any log aggregator exists to consume them.

**Why**: this costs nothing extra to do from the start and is expensive to retrofit later — the first time you actually need to debug a production issue is the worst time to discover your logs aren't queryable.

---

## ADR-013: Test pyramid shape

**Decision**: most tests are pure unit tests with no I/O (`internal/geo`, the retry/backoff logic) — fast, and where the adversarial-test discipline from the dev-strategy doc does the most work. A smaller layer of integration tests uses real Postgres/Redis via `testcontainers-go`, not mocks, for anything touching the database or cache. A handful of true end-to-end tests hit a fully running instance — deliberately few, since they're the slowest and most flaky layer.

---

## ADR-014: Database migrations — versioned from day one

**Decision**: use a migration tool (`golang-migrate` or equivalent) from the very first table, even solo. Every schema change is a numbered, reversible migration file committed to git — never a hand-edited schema.

**Why**: this is cheap to start right and expensive to retrofit once there's real data you can't casually drop and recreate.

---

## ADR-015: Local development environment

**Decision**: `docker-compose.yml` brings up Postgres and Redis; a `Makefile` wraps the common commands (`make dev`, `make test`, `make migrate`) so the actual commands aren't something you have to remember or re-derive each session.

---

## ADR-016: Frontend framework — React with react-map-gl

**Context**: needed a framework to wrap an imperative WebGL map library (MapLibre GL JS) plus a non-React-aware circle-drawing plugin (`maplibre-gl-draw-circle`, built on the mapbox-gl-draw compatibility layer).

**Options considered**: vanilla TypeScript with no framework; React; Svelte.

**Decision**: React, using `react-map-gl` (MapLibre backend) for the base map lifecycle, with the draw-circle plugin attached imperatively to the underlying map instance via a ref, inside a `useEffect` with an empty dependency array.

**Why**: the developer has real prior React experience — rusty, not absent. That's a materially different position than the FastAPI case (ADR-001): the core mental models are already there and recoverable through the same review discipline already applied to the backend, not something being learned from zero. `react-map-gl` specifically solves the classic reactive-render-vs-imperative-WebGL-lifecycle mismatch correctly out of the box, so only the non-React-aware draw plugin needs manual escape-hatch handling, not the whole map.

**Consequences**: the `useEffect` that attaches the draw control is the single highest-risk line in the frontend — an incorrect or unstable dependency array causes the draw control to be silently destroyed and recreated on every re-render. This is the direct frontend equivalent of the async-route-blocking-call trap flagged for FastAPI: a specific, named, well-understood mistake, and a required first checkpoint in review whenever this file changes.

---

## What this list demonstrates as a pattern

Notice the shape repeating across these: **state the actual numbers, name the rejected alternatives, and write down why — not just what.** That's the whole discipline. The next decision that comes up mid-build (and several will) gets the same treatment: a new entry here, not a choice made silently in a commit message.
