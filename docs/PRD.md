# PRD: Circle-to-Circle Flight Search

## Problem statement

Existing flight search tools require picking specific cities or airports on at least one side of a search. Google Flights and Skyscanner support flexible search from a fixed origin to "everywhere," and multi-airport search (up to several airports per side) — but none let a user specify two independently flexible *regions* and search the cheapest combination between them using a direct map interaction. The closest existing tool, PanFlights, supports flexible regions on both ends but through typed region names in form fields, not a map-drawn interaction.

The personal version of this problem: "I want to fly from anywhere in the greater Northeast US to anywhere in Western Europe on given dates, and see the actual cheapest combination" — a real, recurring search the developer has personally wanted and not found a good tool for.

## Goals

**Product goal**: let a user draw two circles on a map, pick a date, and see the 10 cheapest real flight combinations across every realistic airport pairing inside those circles, with results appearing progressively rather than after one long wait.

**Engineering goal, equally weighted**: demonstrate production-grade backend architecture (bounded concurrency, job orchestration, caching, pub/sub) and application security practice (authn/authz, rate limiting, threat modeling, secure-by-construction ownership checks) end to end, on a problem that genuinely requires them rather than performs them decoratively.

## Non-goals

- **Not competing with general-purpose AI travel agents** (Expedia Romie, Google AI Mode, ChatGPT travel integrations) on open-ended trip planning. This tool wins a narrow, deterministic, spatial search job; it does not plan itineraries, book hotels, or hold a conversation.
- **Not a commodity-trading or market-prediction tool.** Any price-history feature is explanatory (showing a documented economic pattern), not a claim of predictive trading edge.
- **Not attempting to out-market or replace PanFlights or Midway.** The goal is a better interaction model and feature combination for personal use and as an engineering showcase, not market share.

## Competitive landscape (see also: conversation record on this)

| Tool | What it does | Gap this project fills |
|---|---|---|
| Google Flights / Skyscanner | Multi-airport on one side, flexible destination on the other | Neither side is a freely-drawn region simultaneously |
| PanFlights | Flexible regions on both ends | Typed region/country names, not map interaction |
| Midway | Meet-in-the-middle for direct flights from multiple fixed cities | Optimizes for shared destination, not cheapest fare from flexible regions |

## Core user stories

**V1 (see `master-plan.md` for full sequencing)**
- As a user, I can submit a fixed Northeast-US-to-Western-Europe search for a given date and get back real prices.
- As a user, I get a job ID immediately and can poll for results rather than waiting on one long request.
- As a user, my requests are rate-limited so the system stays usable and within budget.

**V2+**
- As a user, I can draw arbitrary circles anywhere, not just the fixed corridor.
- As a user, I see results streaming in live rather than polling.
- As a user, I can save a search and get notified when the price drops.
- As a user, I can see a price-history trend for routes I search often.

## Success metrics

Since this isn't a commercial product, "success" is dual-tracked:
- **Functional**: a real search against the fixed corridor returns real, correct prices, end to end, deployed and reachable by URL.
- **Demonstrative**: the finished system and its documentation (this doc set) can credibly support the claims made in the earlier "how would you explain the value" framing — a real distributed system, not a toy, with a defensible security posture.

## Explicit out of scope

See `completeness-gaps.md` for the full list (payments, i18n, feature flags, multi-region, admin tooling, session/device management) — carried here by reference rather than duplicated.
