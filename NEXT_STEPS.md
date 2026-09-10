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
1. **Runtime service-account least-privilege.** `flight-api`/`flight-worker` still run under GCP's default compute service account (broad permissions). Day 5's follow-up CI/CD work only scoped a *deploy-time* identity (`github-deployer`) — never a *runtime* one. Fix: create a dedicated runtime SA with exactly Secret Manager accessor + Cloud SQL client roles, redeploy both services with `--service-account`.
2. **Fare-data provider decision.** Travelpayouts' free tier returns real but stale/aggregated data, not live GDS pricing. Investigated AeroDataBox (wrong category of API — flight status/schedule, not fares), Duffel (already built in `internal/fareprovider/duffel.go`, but live data needs partner/business account activation, not just an API key), Amadeus Self-Service (free tier shut down July 2026), FlightAPI.io (unverified, small free quota). Needs an actual decision, not more research.
3. **Data retention for `search_jobs`/`search_job_results`.** No cleanup exists today — grows forever. Needs a scheduled `DELETE ... WHERE submitted_at < now() - interval '30 days'` (Cloud Scheduler → a small endpoint, or just a manual/cron job at current scale).
4. **Dependabot config** (`.github/dependabot.yml`) for Go modules + npm — nothing currently flags stale/vulnerable dependencies.
5. **Fix `tests/e2e/live-*.spec.ts`.** Stale since Day 3 — they submit jobs with no login step, and `/jobs` has required auth since Day 3. Not part of CI (manual-only), so this hasn't blocked anything, but they don't pass today.

### Tier 2 — documentation/completion
6. **Finish `README.md`** — pitch, architecture diagram (mermaid), quick start, magic-link flow, deploy steps, trade-offs, what's missing.
7. **Demo video.**

### Tier 3 — real, but lower priority for a solo demo project
8. **Basic alerting** — an uptime check on `/readyz`, a log-based alert on worker dead-letters. Nothing pages anyone today.
9. **World-hub list reconciliation** — see Day 1 note above. Cosmetic.
10. **Infra-as-code (Terraform).** Deliberately deferred from Day 5 — the point was learning the GCP primitives by hand. Revisit only if this needs to be reproduced or handed off to someone else.
11. **Staging environment.** Not worth it at current scale; revisit if this gets real users.
