# Documentation index

Everything needed to build this project, in the order it's most useful to read.

## Start here
1. **`PRD.md`** — what this is, why it exists, what it isn't trying to be.
2. **`master-plan.md`** — the actual v1 cut list and sequencing. Read this before any other planning doc; it tells you what order to build things in and what's deliberately deferred.
3. **`day-one-kickoff.md`** — the literal first session: environment setup, repo structure, the first real coding contract.

## Architecture and decisions
4. **`architecture-decisions.md`** — the ADR log. Every non-trivial technical decision, its alternatives, and its consequences. Living document — new decisions get appended here as they come up.
5. **`cloud-strategy.md`** — the same ADR discipline applied specifically to hosting, regions, secrets, and deployment mechanics.
6. **`DATA_MODEL.md`** — the actual schema: every table, every column, referenced by the ADRs above.
7. **`component-inventory.md`** — the full build checklist, organized by category (services, data layer, frontend, infra, security tooling, testing, observability).

## Process and security
8. **`engineering-spec.md`** — concrete API design, rate-limiting algorithm, auth mechanics, and the STRIDE attack-defense mapping.
9. **`dev-strategy.md`** — the function-by-function agent-driven development loop, and specifically how to catch an agent writing happy-path-only tests.
10. **`project-plan.md`** — the full phased plan with a security requirement built into every phase, not bolted on at the end.
11. **`completeness-gaps.md`** — what's missing to be a genuinely complete system: legal/compliance, accessibility, operational readiness, and what's deliberately out of scope.

## Running the agent harness
12. **`AGENTS.md`** — drop this at the project root; opencode reads it automatically. The working philosophy: roles, the review-vs-red-team distinction, security rules, voice.
13. **`free-agent-harness-setup.md`** — how to actually run this for free: local Ollama models sized for your hardware, OpenRouter for the rationed Thinker seat, and how Claude Code fits in as a rare, deliberate escalation without threatening your subscription's usage pool.

## How these fit together

The PRD says what and why. The master plan says in what order. The ADR logs (architecture and cloud) say exactly what was decided and why, permanently — append to them, don't let a future decision get made silently outside this system. The data model, component inventory, and engineering spec are the concrete things being built. The dev strategy and AGENTS.md are how the building actually happens, function by function, with you as the adversarial reviewer at every checkpoint. The completeness gaps doc is the honest "what would still be missing even if all of the above is done."

Nothing here is final in the sense of unchangeable — the ADR logs exist specifically because decisions will keep getting made once real code surfaces real questions. This is the state of the plan now, not a contract against future learning.
