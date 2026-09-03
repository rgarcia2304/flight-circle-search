# Data Model

Consolidates schema decisions from ADR-002 (Postgres), ADR-004 (Redis Streams), and ADR-005 (job/task model) into one authoritative reference. This is what the first migration (ADR-014) should implement.

## PostgreSQL — system of record

### `users`
| Column | Type | Notes |
|---|---|---|
| id | uuid, PK | |
| provider | text | `github` or `google` |
| provider_user_id | text | |
| email | text | from OAuth identity, used for alert delivery — never a user-editable free-text field (see completeness-gaps.md) |
| created_at | timestamptz | |

Unique constraint on `(provider, provider_user_id)`.

### `refresh_tokens`
| Column | Type | Notes |
|---|---|---|
| id | uuid, PK | |
| user_id | uuid, FK → users | |
| token_hash | text | hashed, never plaintext |
| family_id | uuid | groups a rotation chain; reuse of a rotated-out token revokes the whole family |
| expires_at | timestamptz | |
| revoked_at | timestamptz, nullable | |

### `jobs`
| Column | Type | Notes |
|---|---|---|
| id | uuid, PK | |
| user_id | uuid, FK → users | every query filtered by this — ADR-009 |
| status | text | `pending`, `running`, `completed`, `partial`, `failed` |
| params | jsonb | circle definitions, date, corridor identifiers |
| created_at | timestamptz | |
| idempotency_key | text, nullable, unique per user | ADR from engineering-spec section 1 |

### `job_tasks`
| Column | Type | Notes |
|---|---|---|
| id | uuid, PK | |
| job_id | uuid, FK → jobs | |
| origin_iata | text | |
| destination_iata | text | |
| status | text | `pending`, `succeeded`, `failed` |
| result | jsonb, nullable | fare details when succeeded |
| error | text, nullable | |
| attempt_count | int | capped at 3 per ADR-007 |

One row per route-pair sub-task — this is the durable record price-history reads from later, not the cache.

### `saved_searches`
| Column | Type | Notes |
|---|---|---|
| id | uuid, PK | |
| user_id | uuid, FK → users | |
| origin_circle | jsonb | `{center, radiusKm}` |
| destination_circle | jsonb | |
| created_at | timestamptz | |

### `alerts`
| Column | Type | Notes |
|---|---|---|
| id | uuid, PK | |
| user_id | uuid, FK → users | |
| saved_search_id | uuid, FK → saved_searches | |
| threshold_price | numeric | |
| notify_email | — | **not a column** — always resolved from `users.email` at send time, never stored or accepted as input (completeness-gaps.md) |
| last_triggered_at | timestamptz, nullable | |

### `airports` (reference data, from OpenFlights ingestion)
| Column | Type | Notes |
|---|---|---|
| iata | text, PK | |
| name | text | |
| lat | double precision | |
| lng | double precision | |

### `routes` (reference data, route-existence graph)
| Column | Type | Notes |
|---|---|---|
| origin_iata | text | |
| destination_iata | text | |

Composite index on `(origin_iata, destination_iata)`.

### `hot_routes`
| Column | Type | Notes |
|---|---|---|
| origin_iata | text | |
| destination_iata | text | |
| last_refreshed_at | timestamptz | |

### `audit_log`
| Column | Type | Notes |
|---|---|---|
| id | uuid, PK | |
| user_id | uuid, nullable | null for unauthenticated events (failed logins) |
| event_type | text | `login`, `search_submitted`, `alert_created`, etc. |
| metadata | jsonb | |
| created_at | timestamptz | |

Append-only — no updates or deletes, per the tampering/repudiation defenses in the attack-defense mapping.

## Redis — cache, rate limiting, queue transport

- `fare:{origin}:{destination}:{date}` → cached fare result, TTL 6-24h
- `ratelimit:{scope}:{key}` → token bucket state, per the algorithm in the engineering spec
- `job-tasks` stream (Redis Streams) with a consumer group per worker pool — durable task distribution, per ADR-004

## What's deliberately not modeled yet

Multi-tenancy beyond per-user ownership, session/device tracking, and any billing-related tables — all explicitly deferred per `completeness-gaps.md` and the PRD's non-goals.
