# Component Inventory: Traditional Backend Build

Everything you'll build, configure, or integrate, organized by category. Use this as a checklist against the phased project plan — each item below belongs to one of that plan's phases.

---

## 1. Core application services (things you write)

| Component | Purpose | Notes |
|---|---|---|
| API service | HTTP entrypoint — auth, validation, job submission, saved-search CRUD | Node/Express or Python/FastAPI; this is your main container |
| Circle → airport resolution module | Pure function: geometry in, candidate airports out | No external dependencies — build and test this first |
| Route-existence filter | Cross-references candidate airports against real route data | Static OpenFlights data, precomputed into your DB |
| Fare provider adapter | Interface + implementations (Travelpayouts, Duffel) | Keep this swappable — the abstraction that survives a provider going away |
| Job model & submission logic | Creates job + task records, enqueues work | Lives in the API service |
| Worker service | Separate process consuming the queue, bounded concurrency | Its own container — scales independently of the API |
| Progress/notification gateway | SSE or WebSocket endpoint streaming job progress to clients | Subscribes to the same queue/pub-sub backend |
| Scheduler service | Runs cron-style jobs: hot-route refresh, price-watch checks, cache pre-warm | `node-cron`/APScheduler inside its own small process |
| Notification delivery | Sends price-drop alerts | Email via a free-tier provider (Resend/Postmark) to start |

---

## 2. Data layer

**Postgres** (Neon or Supabase free tier, or self-hosted in a container for local dev)
- `users` — accounts, OAuth identity
- `saved_searches` — user's saved circle-pairs + thresholds
- `jobs` / `job_tasks` — job status, per-task results, retry counts
- `airports` — reference data from OpenFlights
- `routes` — precomputed route-existence graph
- `hot_routes` — curated + usage-derived popular corridors
- `price_history` — accumulated results from hot-route refreshes
- `alerts` — price-watch subscriptions and trigger history

**Redis** (Upstash free tier, or containerized for local dev)
- Fare result cache — key: `(origin, destination, date)`
- Rate-limit counters — per-user, per-IP
- Queue backing store — if using BullMQ

**Queue**
- BullMQ (Node, Redis-backed) or Celery (Python, Redis/RabbitMQ-backed)
- Two priority lanes: live user requests, background refresh — live always wins

---

## 3. Frontend

- Map UI — MapLibre GL (open source) + free tile source (OpenFreeMap or Protomaps)
- Circle-drawing interaction (center + radius, draggable/resizable)
- Date/date-range picker
- Live results list — updates as progress events stream in, not just at completion
- Auth UI — OAuth login flow (GitHub/Google)
- Saved searches & alert management screens
- Static hosting — Cloudflare Pages, Vercel, or Netlify free tier (frontend hosting is unaffected by the backend stack choice)

---

## 4. Infrastructure & DevOps

- Dockerfiles — one per service (API, worker, scheduler)
- `docker-compose.yml` — full local stack: Postgres, Redis, API, worker, scheduler, wired together for local dev
- CI pipeline (GitHub Actions) — lint → test → build image → push → deploy, staging and prod as separate targets
- IaC (Terraform) — for whichever cloud you deploy to; makes teardown of any temporary AWS/GCP demo trivial and auditable
- Hosting — Koyeb (primary free container host) + GCP Cloud Run (secondary, doubles as your cloud-portability demo)
- Secrets management — never in `.env` committed to the repo; a real secrets manager or at minimum injected via CI/host-level secret storage
- Environment separation — distinct config and credentials for local, staging, prod

---

## 5. Security tooling (woven through CI, not a final pass)

- SAST — Semgrep or CodeQL, runs on every PR
- Secret scanning — gitleaks
- Dependency/SCA scanning — `npm audit`/`pip-audit` or Snyk, plus Dependabot for update PRs
- Container image scanning — Trivy
- DAST — OWASP ZAP run against staging periodically
- Input validation library — zod (Node) or Pydantic (Python) at every API boundary
- Security headers/middleware — Helmet.js or equivalent; real CSP, not a wildcard
- CORS configuration — explicit allowed origins, not `*`, once auth cookies/tokens are in play
- Rate-limiting middleware — per-user and per-IP, tied into the Redis counters above

---

## 6. Testing

- Unit tests — per module, written before implementation per the dev-strategy doc
- Property-based tests — fast-check (Node) or Hypothesis (Python), for the geospatial/combinatorial logic specifically
- Mutation testing — Stryker (Node) or mutmut (Python), CI gate with a minimum score threshold
- Integration tests — real Postgres/Redis via Testcontainers, not mocked databases
- Recorded fixture tests — cassette-style (`nock`/MSW for Node, `VCR.py` for Python) against real recorded fare-provider responses
- Load testing — k6, validating your concurrency and rate-limit assumptions under real load
- End-to-end tests — Playwright, covering the full draw-circles-to-see-results user flow

---

## 7. Observability

- Structured logging — `pino`/`winston` (Node) or `structlog` (Python)
- Metrics — Prometheus client library + a dashboard (self-hosted Grafana or Grafana Cloud free tier)
- Tracing — OpenTelemetry, especially useful for following one job across API → queue → worker → provider
- Error tracking — Sentry free tier
- Health checks — liveness/readiness endpoints on every service, so your host (Koyeb/Cloud Run) can actually detect failures
- Budget dashboard — a specific, custom metric tracking fare-API calls consumed against your free-tier quota; this is unique to this project's cost model and worth building deliberately, not an afterthought

---

## 8. External integrations

- Fare data — Travelpayouts (primary), Duffel (secondary/testing)
- OAuth — GitHub and/or Google as identity providers
- Email — Resend or Postmark (free tier) for price-alert notifications
- Map tiles — OpenFreeMap or Protomaps (no vendor lock, no cost)
- Reference data — OpenFlights airport and route datasets (one-time or periodic ingestion, not a live API)

---

## 9. Documentation

- OpenAPI spec — generated from or alongside the API service, gives you a real contract and enables auto-generated client code later
- Architecture decision records (ADRs) — short, dated notes on major choices (why Postgres over just Redis, why this fare provider first, why this queue library) — genuinely useful when you're asked "why did you build it this way" in an interview
- README — setup instructions, local dev via `docker-compose up`, environment variable reference

---

## How to use this list

Work through it phase by phase per the project plan, not top to bottom — most rows in sections 5-7 (security tooling, testing, observability) apply incrementally as each phase introduces the thing they'd be checking, not as one giant setup task before you write any feature code.
