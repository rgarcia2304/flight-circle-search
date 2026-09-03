# Cloud Strategy Decision Record

Same discipline as the architecture ADRs — context, options actually considered, decision, consequences — applied specifically to hosting and infrastructure. This was previously scattered across conversation rather than consolidated; this is the consolidation.

---

## CS-001: Environment topology — local and production only, no staging, for v1

**Context**: a full topology usually has local/staging/production. Solo project, small team of one.

**Options considered**: three-tier (local/staging/prod); two-tier (local/prod only).

**Decision**: two-tier for v1. Local development via docker-compose; a single production environment on the primary host. No standing staging environment.

**Why**: a staging environment's value is catching integration issues before they hit real users — with zero real users in v1 and a CI pipeline already running the full test suite (including integration tests against real Postgres/Redis via testcontainers) on every PR, a permanent staging environment is cost and complexity without a corresponding benefit yet.

**Consequences**: the manual deploy-approval gate in CS-005 is what substitutes for staging's safety role — a human watches the first real traffic after every deploy instead of a separate environment absorbing that risk first. Revisit this decision if real users show up and a bad deploy becomes genuinely costly to them.

---

## CS-002: Primary hosting provider — Koyeb

**Context**: the free-tier hosting landscape shifted materially in 2026 and needed to be evaluated on current terms, not older assumptions.

**Options considered**: Fly.io, Render, Railway, Koyeb.

**Decision**: Koyeb, for the primary Go API and worker services.

**Why**: Fly.io no longer offers a free tier for new accounts at all — ruled out immediately. Render's free web services sleep after 15 minutes of inactivity, which is a bad fit for a service expected to handle background job processing and eventually live progress streaming — a cold-started service mid-job is a real user-facing problem. Railway has no permanent free tier, only a small trial credit — not durable enough to build on as a foundation. Koyeb still offers a genuinely free, always-on container (1 vCPU/512MB) plus a free scale-to-zero Postgres, making it the closest thing left to a durable, no-sleep, no-trial-expiry free tier.

**Consequences**: 512MB is a real, tight ceiling — the Go binary's small footprint (a direct benefit of ADR-001) matters concretely here, not just abstractly. Revisit if the app's real memory usage approaches that ceiling under load.

---

## CS-003: Region — single region, US East

**Context**: never actually decided. The target corridor spans Northeast US and Western Europe, so no single region is equidistant from both the developer, the users, and the fare providers.

**Options considered**: US East (closest to the developer and the Northeast US side of the corridor); EU West (closest to the Western Europe side and to Duffel's UK base).

**Decision**: US East, single region, for v1.

**Why**: the developer's own testing and iteration happens from the US — fast local feedback loops during active development outweigh a small latency difference on the European side of a request that's already dominated by a 200ms+ external fare-API call regardless of which region calls it. This is a "good enough for now" call, explicitly, not a rigorously optimized one.

**Consequences**: European users will see a few extra tens of milliseconds of round-trip latency to your API — invisible next to the fare-provider call time. Multi-region is a real v3+ consideration if this ever needs to serve meaningfully latency-sensitive traffic; not justified now.

---

## CS-004: Secrets management — platform-native for v1, not a dedicated secrets manager

**Context**: the engineering spec already said "never `.env` in the repo, use a real secrets manager" — but didn't specify what that actually means at this scale.

**Options considered**: a dedicated secrets manager (Doppler, HashiCorp Vault, cloud-native Secret Manager); the hosting platform's own built-in environment-variable/secret storage.

**Decision**: Koyeb's built-in secret storage for v1 — injected as environment variables at deploy time, never committed, never logged.

**Why**: a dedicated secrets manager earns its complexity when multiple services or environments need to share and rotate secrets centrally — at one host, one environment, one developer, that coordination problem doesn't exist yet. Platform-native storage still satisfies the actual requirement (secrets never in git, never in logs) without adding a system to operate for coordination this project doesn't have yet.

**Consequences**: graduate to a dedicated secrets manager if a staging environment gets added (CS-001 revisited), a second host/provider becomes permanent rather than a demo, or secret rotation needs to be automated rather than manual.

---

## CS-005: Deployment mechanics — CI runs automatically, production deploy requires a manual step

**Context**: how does a merged PR actually reach production, and should it happen automatically?

**Options considered**: fully automatic deploy on every merge to `main` (continuous deployment); manual trigger required for the production deploy step specifically.

**Decision**: GitHub Actions runs the full CI suite (tests, lint, security scans) automatically on every PR and on every merge to `main`. The production deploy itself requires a manual trigger (a `workflow_dispatch` step or Koyeb's own manual promote action) — never silently automatic.

**Why**: this is the same principle already established in `AGENTS.md` — anything touching a remote environment is a deliberate, watched, foreground action, never fire-and-forget. A merge passing CI proves the code is correct in isolation; it doesn't prove now is the right moment to ship it, especially solo with no staging environment absorbing first-contact risk.

**Consequences**: one extra manual click per deploy — a small, deliberate cost in exchange for a human always being present for the moment a change actually goes live.

---

## CS-006: Scaling strategy — horizontal, and it falls out of an earlier decision for free

**Context**: what happens if this needs to handle more load than one instance can.

**Decision**: horizontal scaling (more replicas), not vertical (bigger instance), for both the API and worker services.

**Why**: this isn't a new design — it's a direct consequence of ADR-004 (Redis Streams with consumer groups). The worker pool is already safe to run as multiple replicas, since consumer groups guarantee a task is claimed by exactly one worker regardless of how many worker processes are running. The API service is stateless (JWTs carry auth state, rate-limit counters live in shared Redis), so it scales the same way. No new design work is required to support this — it's a property the architecture already has.

**Consequences**: none beyond Koyeb's own per-instance pricing if it's ever actually needed. Worth noting explicitly as a payoff of an earlier decision, not a new problem to solve later.

---

## CS-007: Disaster recovery at the infrastructure level — single provider, risk accepted explicitly

**Context**: what happens if Koyeb itself has an outage.

**Options considered**: active multi-cloud failover; single provider with accepted risk.

**Decision**: single provider, no failover. This risk is accepted consciously, not overlooked.

**Why**: multi-cloud failover is real engineering effort in exchange for protection against an outage of a specific free-tier host, for a project with no uptime SLA and no paying users. That trade doesn't clear the bar. This sits in the same category as the deliberately-out-of-scope items in the completeness gap analysis — a conscious cut, written down so it reads as a decision later, not a gap discovered during an actual outage.

**Consequences**: if Koyeb has a bad day, the project is down until it recovers. Acceptable for what this is right now.

---

## CS-008: CDN for the frontend — already satisfied, no new design needed

**Decision**: no separate CDN decision required — static hosting on Cloudflare Pages, Vercel, or Netlify (already chosen for the frontend) provides CDN distribution as a built-in property of the platform, not something to configure on top of it.

---

## CS-009: Secondary cloud (GCP/AWS) — scoped, temporary, and torn down on a real calendar date

**Context**: the earlier plan to demo cloud-portability on GCP/AWS needs the same rigor as everything else, not just good intentions.

**Decision**: a single named service (the worker) gets a parallel deployment to GCP Cloud Run, built from the same container image as the Koyeb deployment. This is provisioned via Terraform specifically so `terraform destroy` is a real, auditable, one-command teardown. A concrete end date gets set the day it's stood up — not "when I get around to it" — with a calendar reminder independent of memory, plus a billing alert at a low threshold (e.g. $5) as a tripwire.

**Why**: this needed to move from "a nice idea for later" to an actual decision with a teardown mechanism, given that a temporary cloud resource without a forcing function to remove it is exactly how people get surprised by a bill.

**Consequences**: this is explicitly a portfolio/demo artifact, not part of the production topology — CS-002 through CS-007 describe where this system actually lives; this is a deliberate, time-boxed side deployment.

---

## CS-010: Cost governance — one dashboard, two failure modes it watches

**Decision**: the fare-API budget dashboard (already specified in the component inventory) and a hosting-cost billing alert (Koyeb's own usage, plus the CS-009 demo's billing alert) are treated as the same category of monitoring — both are "a finite budget this project can silently exceed if nobody's watching," and both get an explicit, independent-of-memory alert, not a mental note to check occasionally.

---

## What was actually missing before this document existed

Region selection (CS-003) and the manual-deploy-gate decision (CS-005) hadn't been decided at all — they were gaps, not just undocumented decisions. The rest existed as reasoning scattered across the conversation; this is what it looks like consolidated into the same decision-record discipline as the architecture itself.
