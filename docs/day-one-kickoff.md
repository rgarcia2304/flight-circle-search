# Day one kickoff plan

Everything decided, nothing hypothetical: Go for services, Python for offline data prep, M1 Pro 16GB, Northeast US ↔ Western Europe as the real corridor. Follow this top to bottom.

---

## 1. Environment setup

```bash
# Go
brew install go

# Docker (for local Postgres/Redis via docker-compose)
brew install --cask docker

# Ollama, for local models
curl -fsSL https://ollama.ai/install.sh | sh
ollama pull qwen2.5-coder:7b

# opencode
curl -fsSL https://opencode.ai/install | bash
```

**On the 7B-only call**: at 16GB unified memory, a Q4-quantized 14B model runs ~9GB just for the model — workable in isolation, but tight once opencode, your terminal, and a browser are also open, and you'll be running this for hours at a stretch. Start on `qwen2.5-coder:7b` for both Scout and Orchestrator roles. If it feels comfortable after a few sessions, try 14B for Orchestrator specifically and see if it holds up under your real usage — but 7B is the default, not the fallback.

Python for the offline scripts is almost certainly already on your Mac (`python3 --version` to check); if not, `brew install python@3.12`.

---

## 2. Repo structure

```
flight-circle-search/
  cmd/
    api/main.go
    worker/main.go
    scheduler/main.go
  internal/
    geo/              # Phase 1 lives here — pure, no I/O
      geo.go
      geo_test.go
    routes/           # route-existence graph
    fareprovider/      # adapter interface + implementations
    cache/
    queue/
    auth/
    api/              # HTTP handlers
  data/
    airports.csv       # from OpenFlights, via the Python ingest script
    routes.csv
  scripts/
    ingest_openflights.py
  docker-compose.yml
  Dockerfile.api
  Dockerfile.worker
  go.mod
  AGENTS.md
  .opencode/
    agent/
      thinker.md
  opencode.json
  .env.example
  .gitignore
  README.md
```

```bash
mkdir flight-circle-search && cd flight-circle-search
git init
go mod init github.com/<you>/flight-circle-search
mkdir -p cmd/api cmd/worker cmd/scheduler internal/{geo,routes,fareprovider,cache,queue,auth,api} data scripts .opencode/agent
```

`.gitignore` — at minimum: `.env`, `*.env.local`, `/data/*.csv` if you don't want raw reference data in git history, Go's `bin/`, Python's `__pycache__/` and `venv/`.

---

## 3. Wire the harness

`opencode.json` at the project root:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "provider": {
    "ollama": {
      "npm": "@ai-sdk/openai-compatible",
      "options": { "baseURL": "http://localhost:11434/v1" }
    },
    "openrouter": {
      "npm": "@ai-sdk/openai-compatible",
      "options": { "baseURL": "https://openrouter.ai/api/v1", "apiKey": "{env:OPENROUTER_API_KEY}" }
    }
  },
  "agent": {
    "build": { "model": "ollama/qwen2.5-coder:7b" },
    "explore": { "model": "ollama/qwen2.5-coder:7b" }
  }
}
```

`.opencode/agent/thinker.md` — a custom agent for the rare, deliberate review pass:

```markdown
---
mode: subagent
model: openrouter/qwen/qwen3-next-80b-a3b-instruct:free
---
You are the Thinker: judgment only, not implementation. Review the plan or diff
you're given against its stated contract. Actively look for happy-path-only
test coverage, missing edge cases, and concurrency/ownership mistakes. Name
every gap with file:line evidence. Do not write implementation code.
```

Verify this schema against opencode's current docs before relying on it — it changes over time, as we saw with the V1/V2 instructions-field change earlier.

Copy the `AGENTS.md` from the earlier harness setup into the project root as-is — it's already framework-agnostic and needs no Go-specific changes.

Get an OpenRouter API key at openrouter.ai/keys (no card needed for the free tier), export it: `export OPENROUTER_API_KEY=...` — put this in your shell profile, not committed anywhere.

---

## 4. Git setup

```bash
git add .
git commit -m "chore: initial project scaffold"
gh repo create flight-circle-search --private --source=. --push
```

Then on GitHub: Settings → Branches → add a protection rule for `main` requiring a PR and passing status checks before merge — this is what turns your dev-strategy doc's review gates from something you have to remember into something enforced. You can add the actual CI checks (tests, mutation score, lint) as the very first PR, before Phase 1 feature work.

---

## 5. Today's actual coding task: Phase 1 contract

This is the first thing to hand to opencode, following the loop from the dev-strategy doc exactly: contract first, then you review the agent's test list before any implementation exists.

**Package**: `internal/geo`

**Function**: `ResolveAirports(center Point, radiusKm float64, airports []Airport) ([]Airport, error)`

```go
type Point struct {
    Lat float64
    Lng float64
}

type Airport struct {
    IATA string
    Name string
    Location Point
}
```

Pure function. No network calls, no database — that's deliberate, this is the zero-cost, zero-dependency phase from the project plan.

**Required test cases** (give this list to the agent explicitly — don't let it invent its own and stop there):

- Happy path: a real circle around JFK/EWR/LGA with a realistic radius returns exactly the expected Northeast airports from your test fixture set.
- Zero or negative radius → empty result or a named error, your call, but it must be deliberate, not accidental.
- Invalid coordinates: lat outside [-90, 90], lng outside [-180, 180] → returns an error, doesn't silently compute garbage.
- Antimeridian edge case: a circle whose radius crosses the 180°/-180° longitude boundary must still correctly include airports just on the other side — this is the property-based-testing-shaped bug class from the earlier docs, worth an explicit case even before you wire up Hypothesis-equivalent tooling in Go (`gopter`, if you want it here).
- Empty airports slice in → empty slice out, no panic.
- Very large radius (bigger than half of Earth's circumference) → still returns a sane result, no overflow.
- Duplicate airports in the input slice → no duplicates in the output.

**Test fixture data** — use your real corridor, not synthetic points: JFK, EWR, LGA, PHL, BOS, BWI, DCA as the Northeast US set; LHR, CDG, AMS, FRA, MAD as the Western Europe set. Real IATA codes and real coordinates, pulled from the OpenFlights dataset you'll ingest via the Python script in `scripts/ingest_openflights.py`.

**The actual first prompt to opencode**, once the above is in front of you as the agreed contract:

> Implement `internal/geo.ResolveAirports` per this contract. Write the test file first, covering every case listed above using the real JFK/EWR/LGA/PHL/BOS/BWI/DCA and LHR/CDG/AMS/FRA/MAD fixtures. Stop after the tests are written — do not implement yet.

Review that test list yourself before saying "go ahead and implement" — this is the single highest-leverage checkpoint in the whole project, per the dev-strategy doc, and it's happening on the very first function you build.

---

## What today actually gets you

By the end of today, if you work straight through this: a real Go module, a working local+free agent harness, a protected repo with the git strategy actually enforced, and one fully tested, adversarially-reviewed pure function that's the literal foundation everything else in the project sits on top of. That's a real, complete unit — not a stub, not a scaffold you'll redo later.
