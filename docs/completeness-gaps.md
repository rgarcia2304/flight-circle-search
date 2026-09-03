# Completeness gap analysis

Everything designed so far is real and correct, but a "full end system" needs several categories of thing that never show up in an architecture diagram. Organized by how urgent each is.

---

## Must address before any real user touches this

### Fare provider terms of service — read them before writing the adapter, not after

Travelpayouts and Duffel both have usage terms governing exactly what you're allowed to do with their data: whether you can cache results (for how long), whether you're required to link back to them for booking, attribution requirements, and restrictions on reselling or redistributing pricing data. This project's entire caching strategy (ADR-003, ADR-004) could conflict with a provider's terms if you haven't actually read them. Do this before Phase 2, not as a retrofit — a caching architecture built first and reconciled with the terms second is much more expensive to fix.

### OpenFlights data license — attribution is required, not optional

The OpenFlights airport/route database is distributed under the Open Database License (ODbL), which requires attribution and has share-alike implications for the database itself (not necessarily your application code, but the derived data). A visible attribution line in your app ("Airport data from OpenFlights, ODbL") is a small addition now and a real compliance gap if skipped.

### A genuine security gap this exercise just surfaced: alert notifications must never target an arbitrary address

The price-watch alert feature sends an email when a threshold triggers. If the "where to send it" field is ever a free-text address rather than always the authenticated user's own verified account email, you've built a harassment/spam vector — someone could set up alerts addressed to a stranger's inbox. This needs to be structurally impossible, not policy-enforced: the notification path should only ever read the address from the authenticated user's account record, never accept one as input. Add this as a named negative test in Phase 8, the same way IDOR got a named negative test in Phase 5.

### Privacy policy and terms of service for your own site

You're storing OAuth identity, email addresses, search history, and saved preferences. That needs a real (even if simple) privacy policy and terms of service before real users sign up — not because of enterprise-scale compliance theater, but because you're handling personal data and should say what you do with it.

### GDPR consideration — sharpened by your own corridor choice

Your test corridor is Northeast US to Western Europe, meaning EU users are a realistic part of your actual user base, not a hypothetical. That means: a real basis for processing their data, the ability to export their data on request, and the ability to delete an account and its associated data completely. This doesn't need to be elaborate for a small project, but "can a user delete their account and have their data actually removed" needs to be a real, tested code path, not an assumption.

---

## Should address before this goes beyond a personal demo

### Accessibility — the map-circle interaction has no fallback

Drawing a circle on a map is not accessible to a keyboard-only or screen-reader user by default. A genuinely complete version of this needs an alternative input path — a text-based "enter a city or coordinates and a radius" fallback that produces the exact same query as drawing a circle. Worth designing in from the start rather than retrofitting, since retrofitting usually means building a second, worse code path instead of one clean shared one.

### Domain, TLS, and DNS

Not yet discussed anywhere: an actual domain name, TLS certificate (typically free and automatic via your host — Koyeb and Cloud Run both handle this), and DNS configuration. Small, but it's the literal difference between "runs on my machine" and "a full end system."

### Backup and disaster recovery for Postgres specifically

Redis is pure cache and queue transport — losing it is an inconvenience, not data loss, since nothing in it is the source of truth. Postgres is different: it's where users, jobs, and search history actually live. This needs a real backup policy (most managed Postgres hosts, including Neon and Supabase, offer point-in-time recovery — confirm it's actually enabled, don't assume) and a stated, even if informal, recovery expectation: how much data loss is acceptable if something goes wrong, and how long you're willing to be down while restoring.

### A minimal incident runbook

Even solo: when something breaks, what do you actually do? A short, real document — how to check `/healthz`/`/readyz`, how to view recent error logs, how to roll back a bad deploy, where an alert would even notify you (a webhook to your own phone/email on an error-rate spike is enough at this scale). This turns "something's wrong" from a panic into a checklist.

### Cost/billing alerts on the hosting side, not just the fare-API budget

The component inventory already calls for a dashboard tracking fare-API quota consumption. The same discipline applies to hosting costs themselves — a billing alert on whichever cloud account is live, independent of your own memory, the same pattern already used for the GCP/AWS demo teardown plan.

---

## Deliberately out of scope — named so it's a conscious cut, not a blind spot

- **Payments/billing** — not building this; if the project ever became a real product, this would be the first addition, but there's no reason to design for it now.
- **Internationalization** — English-only is a fine, deliberate scope limit for a portfolio project.
- **Feature flagging system** — unnecessary at solo scale; a config value and a redeploy is sufficient.
- **Multi-region deployment** — single region is fine; nothing about this project's use case needs global low-latency presence.
- **Admin dashboard / moderation tooling** — no user-generated content beyond saved-search labels and alert thresholds, which don't need moderation at this scale.
- **Session/device management (view and revoke active logins)** — a real feature of mature auth systems, genuinely deferred to a "v3" polish pass rather than needed for a working, secure v1.

---

## The pattern here, generalized

Everything in the first section shares a property: it's not a technical risk, it's a **people-and-data risk** — real users' emails, real personal data, another company's API terms. Those categories are easy to skip entirely when you're deep in architecture and concurrency design, precisely because they don't show up in a system diagram. Worth a standing habit: before any phase that touches real user data or a third party's service, ask "what's the non-technical obligation here," not just "what's the technical design."
