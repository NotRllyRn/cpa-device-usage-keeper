# Model family filter

The device toolbar keeps **Agent** (the reporting client) separate from **Model family**. Both filters apply to Overview, Analysis, daily history, rankings and All time. `family` on `/v1/overview`, `/v1/analysis` and `/v1/usage-boundary` accepts `openai`, `anthropic`, `minimax`, `unknown`, or empty (all). Authentication and existing range/work limits still apply.

This identifies names, **not the serving provider, account, endpoint, or subscription**. A Pi, Hermes or custom agent can report several families. Pricing settings do not determine family. Original model identifiers and Pi's exact-price/`[pi] ` fallback remain unchanged.

## Recognized names

Matching is case-insensitive and anchored to the whole identifier. At most one known client prefix (`[pi] `, `[hermes] `, `[codex] `, `[claude] `, `[opencode] `, `[copilot] `) is removed for classification only. One optional namespace may follow: `openai/`, `openai-codex/`, `anthropic/`, or `minimax/`. It must agree with the model family; the prefix alone never proves family. Arbitrary store labels, paths, aliases, other namespaces and conflicting prefixes are unknown.

Exact supported patterns (Go regular expressions, after those prefixes):

- OpenAI / Codex: `^(gpt-(3\.5|4o?|4\.[0-9]+|5(\.[0-9]+)?)(-(turbo|mini|nano|pro|codex|chat|latest|preview|instruct|audio|realtime|search|transcribe|tts|max|spark|[0-9]{4}-[0-9]{2}-[0-9]{2}|[0-9]{4}))*|o[134](-(mini|pro|preview|deep-research|[0-9]{4}-[0-9]{2}-[0-9]{2}))*|codex|codex-mini(-latest)?|codex-[0-9]{4}-[0-9]{2}-[0-9]{2})$`
- Anthropic / Claude: `^claude-((opus|sonnet|haiku)-[34]([.-][0-9]+)*|[34]([.-][0-9]+)*-(opus|sonnet|haiku)|(instant-)?[12](\.[0-9]+)?)(-(latest|thinking|[0-9]{8}))?$`
- MiniMax: `^minimax-m[12](\.[0-9]+)?(-(highspeed|lightning))?$`

Explicit deployment compatibility aliases also map exactly to `openai`: `gpt-5.6-luna`, `gpt-5.6-sol`, `gpt-5.6-terra`, `gpt-6-astra`, `gpt-6-luna`, `gpt-6-sol`, and `gpt-6.1-sol`. The operator confirmed these deployed names as the same OpenAI family. This is a compatibility label, not a claim that these are public model IDs or evidence of the serving provider. The same client-prefix and agreeing-namespace rules apply, including `[pi] ` variants. `gpt-5.5` already matches the standard pattern. Other aliases, such as `gpt-6-terra`, `gpt-6.1-luna` or `gpt-5.6-astra`, remain unknown; no suffix or new GPT generation is generalized from this list.

These are conservative name patterns plus explicit compatibility aliases, not a complete model registry or model-existence check. New families, other aliases, bare `M2.7`, `gpt-oss`, and names such as `my-gpt-5.4` stay unknown. No inference uses an arbitrary substring or the reporting agent's name.

## Totals and limitations

Named families sum only the matching models' disjoint uploaded input, output, cache-read and cache-write categories. They do not count the rest of a mixed snapshot. Costs and priced/missing/conditional coverage are recalculated from just those models, including missing prices honestly.

Unknown includes unrecognized models **plus the snapshot remainder**: category tokens without a model and extra total tokens without a category. Even a snapshot with only one named model cannot safely assign that remainder to its family. All retains original totals; summing the three named families and Unknown reconstructs All. The remainder appears as **Unattributed (no model)**, not a fabricated model or price, in both views. A matching zero-token model still marks an uploaded day. A zero-token snapshot with no models belongs to Unknown.

All time finds the earliest matching upload for the selected device, agent and family, independent of pricing and the current dates. No match returns an empty boundary. Family scans retain bounded time, rows and payload bytes; exceeding a limit fails rather than showing partial totals.

No collector change or client release is needed. The pinned ccusage daily `modelBreakdowns` exposes `modelName` and categories, not a serving-provider field. Its `ModelBreakdown.extra_total_tokens` is skipped in JSON; the existing upload schema likewise carries names/categories and independent snapshot totals, with no provider. Hermes may know a provider internally, but that information is not in this report contract. The server cannot reconstruct omitted attribution from it.
