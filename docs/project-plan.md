# Project Plan: Circle-to-Circle Flight Search

Two focus points drive every phase below: **backend architecture** (concurrency, caching, pub/sub, cloud-native design) and **application security** (defending against real attack classes as you build, not after). Each phase pairs a feature milestone with a security requirement — treat both as part of the same contract, not separate tracks.

Scope for the whole plan: single fixed corridor first (your real use case — e.g. Philly/NYC/DC ↔ London/Amsterdam/Paris), generalizing to arbitrary circles only in the final phase. This controls combinatorial cost while you prove the pipeline. See the companion dev-strategy document for the function-by-function agent loop (contract → adversarial tests → implementation → mutation-score gate → diff review) — this plan sequences *what* to build in that loop, phase by phase.

---

## Phase 0 — Foundations & threat model
**Feature work:** Repo scaffold, CI skeleton (lint/test/build, empty but wired), mutation-testing tool configured with a baseline threshold, secrets management approach chosen before any key exists.

**Security work:** Write a one-page STRIDE threat model *before* writing feature code. Identify your trust boundaries explicitly: browser → edge API, edge API → external fare providers, background workers → database/cache. This document is what you check every later phase's mitigations against — keep it visible, not filed away.

**Your checkpoint:** You write or tightly approve the threat model yourself. This is the one document in the whole project the agent should draft only after you've stated the trust boundaries, not before.

---

## Phase 1 — Data layer: circle → airport → route resolution
**Feature work:** Resolve a circle to candidate airports (static OpenFlights data, no network calls), filter against a route-existence graph. Pure functions, fully offline, zero cost.

**Security work:** Input validation on all geometry — reject NaN, out-of-range lat/long, negative or absurdly large radius. This is a low-stakes, high-value first security lesson: validating untrusted input even when it's "just numbers," before there's any external API to blame for bad behavior.

**Testing emphasis:** Property-based tests (fast-check/Hypothesis) for geospatial edge cases — antimeridian crossing, zero radius, overlapping circles — the exact class of edge case example-based tests miss.

**Deliverable:** A standalone, exhaustively tested resolution library you'd trust even without the rest of the app built.

---

## Phase 2 — Fare provider adapter (single corridor, single provider)
**Feature work:** Adapter interface + one implementation (Travelpayouts), synchronous calls only — no queue yet. Prove real fares come back end-to-end for your fixed corridor.

**Security work:** Treat every provider response as untrusted — strict schema validation before anything downstream uses it. Error responses returned to your client must not leak upstream internals (stack traces, provider error bodies, API key fragments).

**Testing emphasis:** Recorded real-response fixtures (cassette-style), not agent-imagined mocks — includes recording actual error and rate-limit responses so you're testing against reality.

---

## Phase 3 — Caching layer
**Feature work:** Cache-aside on `(origin, destination, date)`, TTL-based expiry.

**Security work:** Sanitize anything that becomes a cache key or backend command — unsanitized input flowing into a Redis key/command is a real injection surface, not a theoretical one. Consider the thundering-herd case (two simultaneous cache misses both hitting the provider) as a cost/availability issue, not just a performance one.

**Your checkpoint:** Review the cache key design specifically — this is a small decision with outsized blast radius if done carelessly.

---

## Phase 4 — Job engine: queue + bounded worker pool
**Feature work:** Move from synchronous single-corridor calls to async job submission, scaling to full combinatorics for the fixed corridor. Bounded concurrency (not unbounded) is the whole point here.

**Security work:** Per-user rate limiting and quota enforcement — this doubles as DoS protection and cost protection. Idempotency keys on job submission so duplicate/replayed submissions can't multiply cost or create duplicate side effects.

**Testing emphasis:** Failure injection is mandatory here, not optional — simulate task timeout, partial batch failure, duplicate submission, and two workers racing on the same cache key. These are exactly the scenarios a happy-path test suite skips and production will hit immediately.

---

## Phase 5 — Auth
**Feature work:** OAuth social login, JWT sessions, per-user identity tied to rate limits from Phase 4.

**Security work:** This is your deep appsec phase. Explicitly test against: JWT algorithm confusion, missing expiry validation, token replay, brute-force login attempts without lockout. Write authorization tests as *negative* tests — "user A requests user B's saved search by ID" must return 403/404, never 200. This IDOR class of bug is one of the most common in real systems and the easiest to systematically test for once you name it.

**Your checkpoint:** Review the negative-test list specifically before implementation — an agent will readily test "correct user can access their own data" and just as readily forget "wrong user cannot access someone else's" unless you require it by name.

---

## Phase 6 — Pub/sub progress streaming
**Feature work:** SSE/WebSocket gateway subscribing to job-progress events, live "top 10 so far" updates to the client.

**Security work:** Authenticate the stream connection itself — an unauthenticated or under-authenticated subscriber must not be able to watch another user's job progress by guessing a job ID. Use unguessable identifiers (UUIDs), not sequential integers, for anything referenced this way.

---

## Phase 7 — Frontend map integration
**Feature work:** Wire the real map UI (MapLibre + open tiles) to the real backend, replacing any mocked data from earlier prototyping.

**Security work:** XSS review on anything rendering provider-derived or user-derived text (airport names, saved search labels). Set a real CSP. Review CORS configuration now that auth tokens/cookies are actually in play — this is the phase where a wildcard origin left over from prototyping becomes a real vulnerability instead of a harmless default.

---

## Phase 8 — Cron jobs: cache pre-warming + price-watch alerts
**Feature work:** Scheduled pre-warm of popular routes, scheduled re-checks of saved price-watch alerts, notification delivery on threshold triggers.

**Security work:** Confirm notification delivery can't leak one user's search data to another. Confirm a failed scheduled run fails loudly (observability) rather than silently dropping alerts a user is relying on.

---

## Phase 9 — Hardening pass (dedicated, not skipped)
**Feature work:** Load testing (k6) to validate your concurrency and rate-limit assumptions actually hold under real load, not just in unit tests.

**Security work:** Run the full automated stack in CI from here forward — SAST (Semgrep/CodeQL), secret scanning (gitleaks), dependency scanning (`pip-audit`/`npm audit`/Snyk), container scanning (Trivy). Run a lightweight DAST pass (OWASP ZAP) against staging. Then go back to the Phase 0 threat model line by line and confirm each identified threat has a real, implemented mitigation — note any gap explicitly rather than assuming it's covered.

---

## Phase 10 — Generalize + go live
**Feature work:** Widen from the fixed corridor to arbitrary user-drawn circles. Full CI/CD to production, custom domain, monitoring dashboards live and actually checked.

**Security work:** Re-run the full Phase 9 automated stack against the generalized version — new combinatorics can surface new edge cases the fixed-corridor version never exercised.

---

## The throughline

Every phase's contract has two sections before any code gets written: what it does, and what it must defend against. That's the concrete mechanism — not a good intention — that keeps security a first-class focus alongside the architecture, phase by phase, instead of a rushed audit at the end.
