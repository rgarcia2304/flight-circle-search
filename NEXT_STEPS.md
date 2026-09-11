# Next Steps — Get this to a demo-ready v1

## Day 1: World hubs + config
- [x] `internal/config/config.go`: typed Config struct — done, and grew beyond the original field list (`ALLOWED_EMAILS`, `PORT`-aware `APIAddr` for Cloud Run, etc.)
- [x] `go test ./internal/config/...` passes
- [~] World hub list — `internal/jobengine.DefaultHubs` covers 34 hubs, not the literal 27-city list originally specified (missing e.g. SYD/MEL, has a few not on the original list). Functionally fine, never reconciled — see backlog below.

## Day 2: Magic-link auth
- [x] `internal/auth/tokens.go`, `internal/auth/sessions.go`, `internal/email/sender.go`, `internal/http/auth_handlers.go`, `internal/http/middleware.go`, typed errors — all done, PR #10
- [x] `go test ./internal/auth/ ./internal/http/ ./internal/email/...` passes

## Day 3: HTTP + worker wiring
- [x] `internal/api/server.go`, `cmd/api/main.go` rewrite, `cmd/worker/main.go` rewrite via `internal/worker.Runner` — all done, PR #11
- [x] `go build ./...` and `go test -race ./...` pass

## Day 4: Frontend auth UI
- [x] `MagicLinkModal.tsx`, `lib/auth.ts`, `lib/api.ts` rewrite — done, PR #12
- [x] `ResultsPanel.tsx`, `carriers.json` — done, PR #13 (split out as its own PR rather than bundled into the auth-UI PR)
- [x] `npm run build && npm run lint` passes
- [x] `frontend/tests/e2e/magic-link.spec.ts` — done, fully mocked network (no live backend needed)

## Day 5: Deploy + docs
- [x] GCP setup: Cloud SQL (db-f1-micro), Memorystore (1GB basic), Cloud Run (`flight-api` + `flight-worker`), Firebase Hosting — all live
- [x] Secret Manager: `database-url`, `session-signing-key`, `travelpayouts-api-key`, `resend-api-key`
- [x] Deployed via `gcloud run deploy` (now automated through `.github/workflows/deploy.yml`, manual-trigger)
- [x] `firebase deploy --only hosting` — live at https://circle-search-508212.web.app
- [x] Billing budget alert — done, but $300 (this repo's actual GCP free-trial credit), not the originally-specified $30
- [ ] `README.md` — drafted during the session, never finalized/committed. Still open.
- [~] `make dev` / `make deploy` as originally specified were never built as literal targets. What exists instead: local dev is `docker compose up -d` + `go run ./cmd/api` + `go run ./cmd/worker` + `npm run dev` (no single wrapper), real deploys go through `.github/workflows/deploy.yml` (manual `gh workflow run` / Actions tab), and `make demo-up`/`make demo-down`/`make demo-status` handle toggling Cloud SQL + the worker between demo sessions. The acceptance-criteria block below is stale against this — kept for historical record, not a literal script to run.
- [ ] Record 60-second demo walkthrough video — still on you.

## Acceptance Criteria (historical — see note above, `make dev`/`make deploy` don't exist as written)

```bash
# 1. Config loads
go test ./internal/config/... -count=1  # exits 0

# 2. Auth compiles + tests pass
go test ./internal/auth/ ./internal/http/ ./internal/email/ -count=1  # exits 0

# 3. Full build + tests
go build ./...  # exits 0
go test -race ./...  # exits 0

# 4. Frontend builds + lints
cd frontend && npm run build && npm run lint  # exits 0

# 5. Smoke test (real deploy)
curl https://flight-api-1048739205149.us-central1.run.app/readyz  # returns 200
curl https://circle-search-508212.web.app/  # returns 200
```

---

## Day 6+: Post-launch backlog

The app is deployed and demo-ready. What's left, prioritized.

### Tier 1 — real gaps, clear scope, worth doing next
1. [x] **Runtime service-account least-privilege.** Default compute SA had project-wide `roles/editor`. Fixed: dedicated `flight-runtime@...` SA with exactly `roles/cloudsql.client` + `roles/secretmanager.secretAccessor` on the 4 secrets, both services redeployed with `--service-account`. See PR #18.
2. [x] **Fare-data provider decision.** Travelpayouts' free tier returns real but stale/aggregated data, not live GDS pricing. Investigated AeroDataBox (wrong category of API — flight status/schedule, not fares), Duffel (already built in `internal/fareprovider/duffel.go`, but live data needs partner/business account activation, not just an API key), Amadeus Self-Service (free tier shut down July 2026), FlightAPI.io (unverified, small free quota). **Decision: stay on Travelpayouts for now.** No free, self-serve option returns live GDS pricing — switching to Duffel means going through their partner-approval process, which isn't worth it until there's a real audience to justify it. Revisit if that changes.
3. [x] **Data retention for `search_jobs`/`search_job_results`.** No cleanup existed — grew forever. Fixed: a daily River periodic job deletes `search_jobs` older than 30 days; `search_job_results` cleans up via the existing `ON DELETE CASCADE` FK. See PR #20.
4. [x] **Dependabot config** (`.github/dependabot.yml`) for Go modules + npm. See PR #21.
5. [x] **Fix `tests/e2e/live-*.spec.ts`.** Was stale since Day 3 — submitted jobs with no login step against the now-auth-gated `/jobs`. Fixed via a test-only `E2E_TEST_MODE`-gated token endpoint + a Playwright global-setup that authenticates and writes a `storageState`. Run manually with `npm run test:e2e:live` against a locally running backend.

### Tier 2 — documentation/completion
6. **Finish `README.md`** — pitch, architecture diagram (mermaid), quick start, magic-link flow, deploy steps, trade-offs, what's missing.
7. **Demo video.**

### Tier 3 — real, but lower priority for a solo demo project
8. **Basic alerting** — an uptime check on `/readyz`, a log-based alert on worker dead-letters. Nothing pages anyone today.
9. **World-hub list reconciliation** — see Day 1 note above. Cosmetic.
10. **Infra-as-code (Terraform).** Deliberately deferred from Day 5 — the point was learning the GCP primitives by hand. Revisit only if this needs to be reproduced or handed off to someone else.
11. **Staging environment.** Not worth it at current scale; revisit if this gets real users.
