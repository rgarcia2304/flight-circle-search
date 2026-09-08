# Next Steps — Get this to a demo-ready v1

## Day 1: World hubs + config
- [ ] Edit `internal/jobengine/service.go` lines 36-47: replace `DefaultHubs` with 27-hub world list (LHR, FRA, AMS, CDG, IST, DUB, MAD, MUC, ZRH, BCN + YYZ, YVR, MEX, DFW, ORD, ATL, JFK, LAX, SFO, GRU, EZE, DXB, PEK, PVG, HND, NRT, ICN, BKK, DEL, BOM, SYD, MEL, CAI, DOH, BRU, MIL)
- [ ] Create `internal/config/config.go`: typed Config struct with DATABASE_URL, REDIS_URL, API_ADDR, APP_ORIGIN, HUBS, CACHE_TTL_HOURS, RATE_LIMIT_PER_DAY, RESEND_API_KEY, SESSION_SIGNING_KEY
- [ ] Run `go test ./internal/config/...` to verify defaults

## Day 2: Magic-link auth
- [ ] Create `internal/auth/tokens.go` (32-byte base64url tokens, SHA-256 hash + 15-min Redis TTL)
- [ ] Create `internal/auth/sessions.go` (Redis `session:{id} -> {email, created_at}`, 30-day TTL)
- [ ] Create `internal/email/sender.go` (interface + resend.go + stdout fallback)
- [ ] Create `internal/http/auth_handlers.go` (POST /v1/auth/magic-link, GET /v1/auth/callback, POST /v1/auth/logout)
- [ ] Create `internal/http/middleware.go` (RequireAuth, request ID, structured access log, security headers, CORS from config)
- [ ] Add typed errors: ErrRateLimited, ErrNotFound, ErrUnauthorized
- [ ] Run `go test ./internal/auth/ ./internal/http/ ./internal/email/...` verify

## Day 3: HTTP + worker wiring
- [ ] Create `internal/api/server.go` (NewServer(cfg config.Config) *Server with middleware chain + graceful shutdown)
- [ ] Rewrite `cmd/api/main.go` to call `api.NewServer(cfg).Listen(ctx)` (remove inline handlers)
- [ ] Rewrite `cmd/worker/main.go` to use `worker.NewRunner(cfg config.Config) *Runner` (remove inline idleWorker hack)
- [ ] Run `go build ./...` and `go test -race ./...` verify

## Day 4: Frontend auth UI
- [ ] Create `frontend/src/components/MagicLinkModal.tsx` (email input + send link)
- [ ] Create `frontend/src/lib/auth.ts` (getSession, logout, requestMagicLink, cookie reading)
- [ ] Rewrite `frontend/src/lib/api.ts` (replace hardcoded localhost, add credentials: 'include', 401 -> modal pop)
- [ ] Update `frontend/src/components/ResultsPanel.tsx` (empty/loading/error states + cached badge)
- [ ] Add `frontend/src/lib/carriers.json` (50-entry IATA→carrier name map)
- [ ] Run `npm run build && npm run lint` verify
- [ ] Create `frontend/tests/e2e/magic-link.spec.ts` (mock email send, walk full flow)

## Day 5: Deploy + docs
- [ ] GCP setup: Cloud SQL (db-f1-micro) + Memorystore (1GB basic) + Cloud Run (api + worker) + Firebase Hosting
- [ ] Create Secret Manager entries (TRAVELPAYOUTS_API_KEY, SESSION_SIGNING_KEY, RESEND_API_KEY)
- [ ] Run `gcloud run deploy api --source . --region us-central1 --allow-unauthenticated`
- [ ] Run `firebase init` + `firebase deploy --only hosting`
- [ ] Create billing alert: `gcloud billing budgets create --billing-account=XXXXXX --display-name="flight-circles" --budget-amount=30`
- [ ] Write `README.md` at repo root (1-paragraph pitch, architecture diagram mermaid, quick start, magic-link flow, deploy steps, trade-offs, what's missing)
- [ ] Final pass: `make dev` → app loads; `make deploy` → URL reachable
- [ ] Record 60-second demo walkthrough video

## Acceptance Criteria (run to verify)

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

# 5. Smoke test (local dev)
make dev                    # starts Postgres + Redis + API + worker + Vite
curl http://localhost:8080/readyz  # returns 200
# Submit a search → get real fares → click Book → aviasales.com opens

# 5. Deploy
make deploy                 # succeeds
curl https://YOUR-SUBDOMAIN/readyz  # returns 200
```