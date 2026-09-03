# Engineering Specification

This is the concrete design that was missing from the earlier planning — API shape, rate limits, auth mechanics, attack defenses, and git workflow. Treat this as the plan that needs your explicit approval before Phase 1 build work starts, matching the pipeline's own approval gate.

---

## 1. API design

REST, JSON, versioned under `/v1`. Consistent error envelope on every failure:

```json
{ "error": { "code": "RATE_LIMITED", "message": "Too many search requests", "requestId": "..." } }
```

Generic messages only — detail goes to server-side logs, never to the client (this is a security requirement, not just a style choice — see section 4).

**Endpoints:**

| Method & path | Purpose | Auth |
|---|---|---|
| `POST /v1/auth/callback/{provider}` | OAuth code exchange, issues tokens | none (this is login) |
| `POST /v1/auth/refresh` | Rotate refresh token, issue new access token | refresh cookie |
| `POST /v1/auth/logout` | Revoke refresh token | required |
| `GET /v1/me` | Current user profile | required |
| `POST /v1/searches` | Submit a circle-pair + date job | required |
| `GET /v1/searches/{id}` | Job status + results so far | required, owner-only |
| `GET /v1/searches/{id}/stream` | SSE live progress | required, owner-only |
| `GET /v1/searches` | List past searches (paginated) | required |
| `POST /v1/saved-searches` | Save a search definition | required |
| `GET /v1/saved-searches` | List saved searches | required |
| `DELETE /v1/saved-searches/{id}` | Remove one | required, owner-only |
| `POST /v1/alerts` | Create a price-watch alert | required |
| `GET /v1/alerts` | List alerts | required |
| `DELETE /v1/alerts/{id}` | Remove one | required, owner-only |
| `GET /v1/airports` | Reference lookup (near a point) | none, heavily cached |
| `GET /healthz`, `GET /readyz` | Liveness/readiness | none |
| `GET /metrics` | Internal observability | internal network only, never public |

**Pagination**: cursor-based (opaque token), not offset — offset pagination degrades and gets inconsistent as rows are inserted concurrently with a list being paged through.

**Idempotency**: `POST /v1/searches` accepts an optional `Idempotency-Key` header. A duplicate key within a time window returns the original job rather than creating a new one — this is a direct cost/abuse defense, not just a nicety, since duplicate submissions are the fastest way to blow through your fare-provider budget.

---

## 2. Rate limiting

**Algorithm**: token bucket, implemented as a Redis Lua script so the check-and-decrement is atomic — a plain `GET` then `SET` from your application code has a race window under concurrent requests that a Lua script closes.

| Scope | Limit | Why |
|---|---|---|
| Unauthenticated auth endpoints, per IP | ~10/min | Blunts credential/OAuth abuse attempts |
| `POST /v1/searches`, per user | ~10/day (tune later) | This is the one that maps directly to cost — it's the expensive combinatorial endpoint that consumes fare-provider quota |
| Other authenticated GETs, per user | ~120/min | Prevents runaway client bugs, not primarily a cost control |
| Background scheduler traffic | own explicit daily budget config, bypasses per-user limits | Already established — competes with live traffic, so it gets its own ceiling, not a share of users' ceilings |

Response on limit: `429` with a `Retry-After` header and the standard error envelope, `code: RATE_LIMITED`.

---

## 3. Authentication and authorization

**AuthN**: OAuth2 Authorization Code flow against GitHub and/or Google. You never touch a password.

- On successful callback: look up or create a user row keyed by `(provider, provider_user_id)`.
- Issue an access JWT: short-lived (~15 min), minimal claims (`sub`, `iat`, `exp` — nothing else, since JWT payloads are client-visible even though they're signed).
- Issue a refresh token: opaque random value, stored **hashed** in the database (never plaintext), delivered as an `httpOnly`, `Secure`, `SameSite=Strict` cookie.
- **Refresh rotation with reuse detection**: every refresh invalidates the old token and issues a new one. If an already-rotated-out token is ever presented again, that's a strong signal of theft — revoke the entire token family immediately, not just the one token.

**AuthZ**: ownership-based, not role-based — this system doesn't need RBAC at this scale, and adding it now would be speculative complexity.

- Every resource row carries a `user_id`.
- Every fetch-by-id handler filters by the authenticated user's id **inside the query itself** (`WHERE id = ? AND user_id = ?`), not "fetch, then check in application code" — the query-level filter is harder to accidentally bypass than an if-check a future edit might delete.
- On ownership mismatch, return `404`, not `403` — this avoids confirming to a non-owner that a resource exists at all.
- **Mandatory negative test per resource endpoint**: "user A requests user B's resource by id" must return 404. This is not optional test coverage — it's the direct defense against IDOR, the bug class flagged earlier as one of the most common and most systematically testable.

---

## 4. Attack/defense mapping

A concrete STRIDE pass against this specific system, not a generic checklist:

| Threat | Concrete defense here |
|---|---|
| **Spoofing** | OAuth + JWT signature verification; refresh rotation with reuse detection |
| **Tampering** | Schema validation at every boundary (zod/Pydantic); parameterized queries only, never string-built SQL; job/task status transitions are server-authoritative — no client-writable status field |
| **Repudiation** | Append-only audit log of security-relevant events (login, search submission, alert creation) with user id and timestamp |
| **Information disclosure** | Generic client-facing errors, detail server-side only; TLS everywhere; least-privileged DB credentials per service (the worker can't touch the `users` table); real secrets manager, never `.env` in the repo |
| **Denial of service** | Rate limiting (section 2); bounded worker concurrency; idempotency keys; circuit breaker + backoff on the fare provider; explicit daily budget cap on background refresh |
| **Elevation of privilege** | Query-level ownership filtering (not app-level check-then-act); never trust a client-supplied user id anywhere; worker service credentials scoped separately from the API service's |

---

## 5. Git strategy

- **Trunk-based development.** `main` is always deployable. Short-lived branches, one per function/unit — matches the dev-strategy doc's small-unit-at-a-time discipline directly.
- **`main` is protected — no direct pushes, even solo.** Every change goes through a PR. This is where CI gates actually run: tests, mutation-score threshold, lint, SAST, secret scan. This turns the "diff review" and "build gate" checkpoints from something you have to remember into something tooling enforces.
- **Conventional commits** (`feat:`, `fix:`, `chore:`, `security:`) — gives you a real changelog and a filterable history for free.
- **Squash-merge into `main`.** The branch itself can be messy (agent iteration, fixups); the merged commit is one clean, human-voiced summary — directly consistent with the Voice section already in your `AGENTS.md`.
- **Tag a milestone at the end of each phase** from the project plan (`v0.1-airport-resolution`, `v0.2-fare-adapter`, ...). Gives you rollback points and a natural trail for the eventual portfolio writeup.
- **No permanent environment branches.** `main` plus tags is enough at this scale. The remote/deploy step is a deliberate, manually-triggered, watched action — consistent with the "remote commands are Thinker-only, foreground, never backgrounded" rule already in your process document, not a branch-based automation you'd set and forget.

---

## What happens next

This is the plan-approval gate. Read it, push back on anything that doesn't sit right, and once it's approved, Phase 1 (circle → airport resolution) starts against a genuinely locked spec rather than one being invented as we go.
