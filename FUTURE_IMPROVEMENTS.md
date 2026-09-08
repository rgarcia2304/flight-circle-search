# Future Improvements — Past v1

## Phase 2: Production Hardening (weeks 2-3)

### Observability
- [ ] OpenTelemetry tracing across API → River queue → worker → fare provider
- [ ] Structured slog JSON with request_id correlation; export to Cloud Logging
- [ ] Prometheus metrics: `jobs_submitted_total`, `pairs_processed_duration_seconds`, `fare_provider_errors_total`, `cache_hit_ratio`
- [ ] Grafana dashboard (or Cloud Run built-in) with SLOs (p99 < 5s for job submit, p95 < 30s for full search)
- [ ] Alert on error rate > 1%, queue depth > 1000, cache miss spike

### Reliability
- [ ] Circuit breaker on Travelpayouts (Hystrix-style: trip after 5 failures in 30s, half-open probe)
- [ ] Retry with exponential backoff + jitter on provider 429/5xx (max 3 retries)
- [ ] Idempotency keys on `POST /v1/jobs` (client-generated, dedup in Postgres)
- [ ] Graceful degradation: if Redis cache down, bypass to provider; if provider down, serve stale cache + `X-Cache-Stale: true`
- [ ] Dead-letter queue for River jobs exceeding max attempts; manual replay UI

### Auth Hardening
- [ ] Refresh token rotation (15-min access, 30-day refresh with family revocation on reuse detection)
- [ ] OAuth 2.0 providers (Google, GitHub) as alternative to magic-link
- [ ] Device fingerprinting for anomaly detection (new device → email confirm)
- [ ] Admin API to revoke all sessions for an email

### Rate Limiting
- [ ] Token bucket per user (configured via Redis Lua script for atomicity)
- [ ] IP-based fallback for unauthenticated endpoints
- [ ] Distributed rate limit across Cloud Run instances (Redis-backed)

## Phase 3: Product Features (weeks 3-5)

### Search UX
- [ ] Arbitrary user-drawn circles (not just fixed corridors)
- [ ] Multi-city search (A→B + B→C + C→A)
- [ ] Flexible dates (±3 days, "cheapest month" calendar view)
- [ ] Hot-route pre-warming: scheduler refreshes top 50 corridors daily, results served from cache instantly

### Alerts & Notifications
- [ ] Price-drop alerts: user sets threshold, email sent when fare crosses it
- [ ] In-app notification center (history of alerts, dismiss/snooze)
- [ ] Webhook support (POST to user URL on alert) — for power users

### Data Enrichment
- [ ] Carrier name map → full carrier DB with logos (SVG from airline-logos repo)
- [ ] Aircraft type, layover duration, baggage policy from provider response
- [ ] Carbon footprint estimate per flight (use ADEME/ICAO factors)

### Mobile
- [ ] PWA manifest + service worker (offline read of cached results)
- [ ] Touch-optimized map (two-finger pan, pinch zoom, double-tap reset)
- [ ] Native share sheet for results

## Phase 4: Scale & Multi-region (month 2+)

### Infrastructure
- [ ] Terraform for all GCP resources (Cloud SQL, Memorystore, Cloud Run, Secret Manager, Artifact Registry, Cloud Build trigger)
- [ ] Separate dev/staging/prod environments (staging = 1/10 prod resources)
- [ ] Cloud SQL read replicas for GET-heavy endpoints
- [ ] Memorystore standard tier (HA) for production
- [ ] VPC-SC perimeter around Postgres + Redis

### CI/CD
- [ ] GitHub Actions → Cloud Build → Artifact Registry → Cloud Run deploy
- [ ] Preview deployments on PRs (unique subdomain per PR)
- [ ] Automated e2e test run on preview (Playwright against preview URL)
- [ ] Canary deploy: 5% traffic to new revision, auto-rollback on error rate spike

### Fare Providers
- [ ] Duffel Live integration (production-grade, requires approval)
- [ ] Provider abstraction: cache key includes provider ID; fallback chain (Duffel → Travelpayouts)
- [ ] Fare freshness badge: "Live" (direct provider call) vs "Cached" (Redis, with age)

### Analytics
- [ ] Search funnel: circles drawn → submitted → results viewed → Book clicked
- [ ] Popular corridors heatmap (aggregated, anonymized)
- [ ] Provider latency/error tracking per endpoint

## Phase 5: Compliance & Polish (ongoing)

### Legal / Compliance
- [ ] Read Travelpayouts/Duffel TOS — adjust cache TTL and attribution to comply
- [ ] OpenFlights ODbL attribution in footer ("Airport data from OpenFlights, ODbL")
- [ ] GDPR: account deletion endpoint, data export (JSON), privacy policy page
- [ ] Terms of Service page
- [ ] Cookie banner (session cookie only; no third-party tracking)

### Accessibility
- [ ] Text-based circle input fallback: "Enter city + radius" → same query as map
- [ ] WCAG AA contrast on all UI elements
- [ ] Keyboard navigation for map (arrow keys pan, +/- zoom, Enter submit)
- [ ] Screen reader labels on all interactive elements

### Security
- [ ] CSP header with strict directives (no inline scripts, report-uri to /csp-report)
- [ ] HSTS preload (after confirming TLS works)
- [ ] Dependency scanning (govulncheck, npm audit) in CI
- [ ] SAST (gosec, CodeQL) in CI
- [ ] Penetration test (annual, budget $2-5k)

### Cost Optimization
- [ ] Weekly cost report email (GCP billing export → BigQuery → Looker Studio)
- [ ] Auto-delete preview deployments after 24h
- [ ] Cloud Run min-instances = 0 for worker (scale to zero)
- [ ] Fare provider quota dashboard (daily usage vs limit)

---

## Deliberately Out of Scope (named cuts)

| Feature | Reason |
|---|---|
| Payments / billing | Not a SaaS; portfolio/demo only |
| i18n / localization | English-only is fine for demo |
| Feature flags | Config + redeploy sufficient at this scale |
| Multi-region active-active | Single region us-central1 is fine |
| Real-time WebSocket progress | Polling every 2s is simple and works |
| Admin dashboard | CLI or SQL queries sufficient for demo |
| User profiles / avatars | Not core to flight search |
| Social sharing | Book link is enough |
| Historical price charts | Alerts are the MVP version |

---

## Repo Hygiene (do once)

- [ ] Delete stale artifacts: `mutation-report.txt`, `go-mutesting-agentic.json`, `report.json`
- [ ] Delete `frontend/tests/debug/` (stale UI tests)
- [ ] Delete unused `frontend/src/assets/react.svg`, `vite.svg`
- [ ] Add `.github/dependabot.yml` for weekly dependency updates
- [ ] Add `Makefile` with `make dev`, `make test`, `make lint`, `make deploy`, `make clean`
- [ ] Add `SECURITY.md` (vulnerability reporting process)