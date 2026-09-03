# Free agent-driven development harness: opencode + Qwen

## The honest starting point

Qwen Code's own official free tier (OAuth login at qwen.ai) went from 1,000 requests/day to 100/day on April 13, 2026, then was discontinued entirely two days later. Anyone who built a workflow around it had it disappear with days of notice. This isn't a reason to avoid Qwen — it's the reason to build on the layer that can't be taken away: **weights you run yourself.** That's the plan below. Cloud free tiers are listed too, but as a supplement, not the foundation — exactly the "free tiers can change without notice" caveat your own philosophy document should apply to itself.

This means the answer to "if I can do it, anybody can" is genuinely yes — but the honest version is: anybody with a reasonably modern computer, not anybody with any computer. Hardware tiers below.

---

## Part 1: Install opencode

opencode is a free, open-source, model-agnostic terminal coding agent — the same category of tool as Claude Code, but it doesn't care which model powers it.

```bash
curl -fsSL https://opencode.ai/install | bash
```

opencode reads project instructions from an `AGENTS.md` file at your project root (the direct equivalent of the `CLAUDE.md` file in the document you pasted), and supports defining custom agents as markdown files under `.opencode/agent/`, each with its own model and permissions. This maps almost exactly onto the Orchestrator/Scout/Thinker role structure you already have — see Part 4.

---

## Part 2: Get Qwen running locally (the durable free path)

```bash
curl -fsSL https://ollama.ai/install.sh | sh
ollama pull qwen2.5-coder:7b
```

Ollama runs the model on your own machine and exposes it through a local OpenAI-compatible API — no account, no key, no company that can revoke access. This is the part of the setup that's actually zero-cost forever, not zero-cost until a policy changes.

### Hardware tiers — pick the one that matches your machine

**Modest (8-16GB RAM, no dedicated GPU, or an older laptop)**
- Run `qwen2.5-coder:7b` for everything. It's capable but not a strong reasoner — lean harder on the human checkpoints from your dev-strategy doc, since this tier's judgment ceiling is real.

**Solid (16-24GB VRAM, or Apple Silicon with 24GB+ unified memory)**
- Scout: `qwen2.5-coder:7b` — fast, cheap, bounded errands
- Orchestrator: `qwen2.5-coder:14b` — the daily-driver seat
- Thinker: same 14b, invoked with a "think carefully, be adversarial" system prompt rather than a bigger model — a legitimate simplification when you can't afford a separate top-tier model

**Strong (Apple Silicon with 48-64GB+ unified memory, or a 24GB+ VRAM GPU)**
- Scout: `qwen2.5-coder:7b`
- Orchestrator: `qwen2.5-coder:14b`
- Thinker: `qwen2.5-coder:32b` (quantized) or `qwen3-coder:30b` (MoE — only ~3B active parameters per token, so it's lighter than the raw size suggests) — genuinely reserve this tier for plan review and adversarial test review, both to manage speed and because that's the actual philosophy: the expensive seat, rationed.

Pull whichever models your tier calls for the same way: `ollama pull qwen2.5-coder:14b`, etc.

---

## Part 3: OpenRouter — the actual lever for squeezing performance, used specifically for the Thinker seat

This is worth being deliberate about rather than treating as a footnote, because it maps onto your role-based design almost perfectly.

**Why it's specifically good for Thinker, not Scout or Orchestrator:** OpenRouter's free (`:free`-suffixed) models are real production-grade weights — same models paying users get, just rate-limited: a hard cap of 20 requests/minute at all times, plus a daily cap of 50/day on a $0 account or 1,000/day after a one-time, non-expiring $10 credit purchase (this unlock is permanent even if your balance later drops back to zero — it's the single best value lever available here). Your Thinker role is already supposed to be low-frequency, judgment-only, rationed by design — which means the 20/min and even the 50/day free cap are barely a constraint for it. Put Scout and Orchestrator on local Ollama (high-frequency, needs to be rate-limit-free), and point Thinker at OpenRouter to get access to genuinely bigger models than most local hardware can run — this is the actual performance squeeze, not a marginal tweak.

**Current models worth trying for the Thinker seat** (verify at openrouter.ai/models before relying on any — the free lineup rotates without warning; DeepSeek's free variants, for example, were popular through 2025 and are gone entirely as of mid-2026):
- `qwen/qwen3-next-80b-a3b-instruct:free` — much larger than anything you're likely running locally, MoE so cheaper to serve than its size implies
- `qwen/qwq-32b:free` — a reasoning-focused Qwen variant, a natural fit for adversarial plan/test review specifically
- Keep 2-3 candidates on hand, not one — see the fallback config below

**Resilience technique — never hardcode a single free model.** OpenRouter lets you pass a `models` array so a request automatically falls through to the next one if the first is rate-limited or rotated out:

```json
{
  "models": [
    "qwen/qwen3-next-80b-a3b-instruct:free",
    "qwen/qwq-32b:free",
    "qwen/qwen3-8b:free"
  ]
}
```

**Adding it to opencode** as a provider (alongside your local Ollama provider from Part 2):

```json
{
  "provider": {
    "openrouter": {
      "npm": "@ai-sdk/openai-compatible",
      "options": { "baseURL": "https://openrouter.ai/api/v1", "apiKey": "{env:OPENROUTER_API_KEY}" }
    }
  },
  "agent": {
    "thinker": { "model": "openrouter/qwen/qwen3-next-80b-a3b-instruct:free" }
  }
}
```
(Verify this exact schema against opencode's current docs at setup time — it's actively evolving, as seen with the V1/V2 instructions-field change earlier.)

**The honest cost note:** the $10 one-time credit purchase isn't technically $0, and worth naming as the small, deliberate exception it is — but it's a one-time cost for a 20x permanent increase in your daily Thinker-tier ceiling, which is a genuinely good trade if you decide to make it. Doing the math for your own sense of scale: at even 20 Thinker reviews/day, the free 50/day cap already covers you; the $10 unlock mostly matters if you start leaning on Thinker much more heavily than the rationed-by-design intent.

**Other supplementary options, lower priority:**

- **`opencode-qwen-auth`** — a third-party OpenCode plugin reverse-engineering the official Qwen Code OAuth flow. Alpha software, explicitly fragile — Alibaba could restrict third-party access at any time. Fine as an occasional extra, not a foundation.
- **Renting a GPU by the hour** (Vast.ai, RunPod) — well under a dollar/hour for mid-tier GPUs, for an occasional single heavy session. Also not literally $0 — name it as the exception it is if you use it.

---

## Part 4: Adapting your CLAUDE.md into opencode's AGENTS.md

Most of the document you pasted is already fully portable — the security rules, the brevity rules, the "never guess, verify" rules, the hard-way philosophy — none of that is Claude-specific. What needs adapting is the installation instructions and the model-assignment table, since opencode's mechanism for this is different from Claude Code's.

See the companion `AGENTS.md` file for the adapted version, ready to drop into your project root. The key mapping:

| Your role | opencode equivalent | Model tier |
|---|---|---|
| Orchestrator | The default `build` agent | Your mid-size model |
| Scout | The built-in `explore` subagent (already read-only, already bounded, already matches your "Scout" definition almost exactly) | Your smallest model |
| Thinker | A custom agent you define in `.opencode/agent/thinker.md`, invoked deliberately for plan review and test review | Your largest model, used sparingly |

The pipeline discipline (ask → think → approval gate → build → build gate → finish), the review-vs-red-team distinction, and the human-approval gate before any build starts — all of that stays exactly as written. It's a process, not a Claude-specific feature.

---

## The one thing to calibrate honestly

Open-weight Qwen models, even at the largest sizes you can realistically run locally, are behind frontier hosted models specifically at the "Thinker" judgment tasks — adversarial test review, catching subtle happy-path bias, spotting a design flaw in a plan. This doesn't mean the setup doesn't work. It means you should expect to personally do more of the adversarial reviewing yourself in the early phases (circle→airport resolution, the pure functions) rather than fully trusting a local Thinker pass — exactly the human-checkpoint discipline your dev-strategy doc already establishes, just leaned on slightly harder here. That's a calibration, not a blocker.
