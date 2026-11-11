# Implementation handoff

## Task and workspace

Implement CPA Device Usage Keeper in this repository. This is independent work continued directly with the user in a new T3 thread, not a delegated task. Do not report to or ask the originating thread to manage this work.

Read applicable AGENTS.md instructions, including /root/.config/pi-t3/agent/AGENTS.md. Use simple, modular changes, generic examples, concise commits authored with the configured NotRllyRn identity. Review diffs and run relevant checks. Do not deploy or modify production without explicit approval and backups. No credentials or personal conversation contents belong in git.

- Local checkout: /root/code/cpa-device-usage-keeper
- Public fork: NotRllyRn/cpa-device-usage-keeper
- Branch/default branch: device-usage
- Upstream: Willxup/cpa-usage-keeper
- Baseline: v2.1.0, commit 41b34f12dbef209292765415b860583aad5afc09
- GitHub releases/latest verified on 2026-11-11: v2.1.0, published 2026-11-10, draft=false, prerelease=false. Work from this stable baseline, not upstream main.
- So far only documentation changed; no device implementation exists. Original README preserved byte-for-byte in docs/upstream/README.v2.1.0.md. MIT notices retained.

## Latest requirements override earlier proposals

Implement directly as a native CPA plugin, serving the Keeper-derived UI through CPA's dashboard iframe integration, WITHOUT a separate production service/container stack. The earlier standalone-backend recommendation is superseded. Verify the host's actual plugin ABI/routing/lifecycle before choosing the smallest implementation. If a hard constraint prevents this, explain it to the user in this thread; do not silently substitute a standalone server.

Keep changes to upstream as localized and minimal as practical for future stable-release comparisons. Preserve upstream structure and reuse UI/pricing code rather than broad rewrites. Minimal diff is a goal, not permission to misrepresent aggregate reports as request events. Native plugin integration is real work and cannot be promised to take only a few lines.

## User wording retained from the originating conversation

These are exact excerpts relevant to implementation, not a full chat transcript:

- "I think to make things easier, I will strip out everything specific to CPA usage capture traffic capture and make a generic function plug in that is actually my own adapter that connects to the custom backend that is capturing all my devices and their usages from all the different clients."
- "for 2H, lets let keeper handle those model calculations based upon the model pricing settings menu that it should use to show the user how much it costs."
- "why can't we just use ccusage and use it's data instead of creating our custom adapters ? that way we don't have to support so many different providers and leave it up to ccusage to define that information?"
- "that's a fine tradeoff."
- "cpa-device-usage-keeper"
- "please emphasize to make it as many minimal changes as possible to make future updates easy to perform and update and support."
- "I don't really feel like registering a new npm package."
- "also make sure to tell it to make sure to implement it directly as a CPA plugin so that I don't have to run another production stack just for this service fork."

The user wants a new independent implementation thread on Pi provider, GPT-6-Astra, medium thinking, and will continue there. Do not send progress back to the original thread.

## Agreed product scope

- Full retained device usage history, including before CPA was installed and direct-to-provider traffic.
- Use ccusage's parsers, not custom Pi/Codex/OpenCode parsers. Pin a tested collector version and validate its schema.
- Collect daily and supported session/model reports. Include device identity, agent, model, date, tokens, and collection freshness. Do not count daily and session representations twice.
- Treat cumulative reports as revisioned snapshots with idempotent replacement, not additive events. Re-uploading or retrying must not inflate totals. Handle outages, pending acknowledgements, concurrent runs, and shrinking history deliberately.
- Upload numerical reports/necessary identifiers, not conversation logs, prompts, credentials, tool arguments, or raw errors. Consider local path privacy in session reports.
- Device enrollment and revocable upload-only tokens; never distribute CPA management credentials to devices. Validate sizes, numbers, dates, ownership, revisions, and TLS. Protect local state permissions.
- Keeper's Model Pricing Settings owns estimated cost calculation. Normalize token categories to its conventions. Do not trust ccusage costs as authoritative or implement a separate live-pricing system initially. Existing Sync Prices and historical recalculation may be reused as appropriate. Missing rates/conditions are unknown, not free. Subscription usage is API-equivalent estimated cost, not actual billed spend.
- Hide/disable CPA ingestion, credential/quota management, request counts/failure rates, latency, TTFT, request TPS, and exact minute-level throughput when unavailable. Do not fabricate requests from daily totals. Session duration averages are not model TPS.
- Keep device totals distinct from any proxy traffic; combining them double-counts overlapping usage. Initial product replaces CPA capture with device ingestion, not a blended sum.
- Use Keeper's visual style and useful charts. Brand as an independent fork and link clearly to upstream. README stays short and honest about implemented versus planned behavior.

## Installation requirement

Provide an easy one-command device installation path, preferably a hosted install.sh usable with curl piped into bash, plus a download-and-inspect alternative. No new npm package registration. Existing upstream npm artifacts can be used for ccusage if needed, but end users should not need to build code. Prefer versioned release binaries, explicit supported OS/architectures, pinned collector versions and verified artifact integrity, private enrollment prompts (no secrets in command lines), durable local state, scheduler setup, status/update/uninstall commands, and no unnecessary root requirements. Document unsupported platforms rather than claiming untested support. Do not execute the installer on this device or production as a side effect of developing it.

## Official repositories: avoid earlier confusion

1. Willxup/cpa-usage-keeper: standalone backend/database/dashboard; this repository's GitHub fork parent.
2. Willxup/cpa-plugin-usage-keeper: official CPA menu/iframe bridge to that backend, NOT itself the usage backend.
3. rainoffallingstar/CPA-Usage-Keeper-Plugin: separate native implementation inspired by Keeper, NOT the official bridge and NOT installed as the user's Keeper bridge. It may offer architecture references, subject to license/compatibility review; do not assume its SDK compatibility.

A GitHub fork has one parent. If official companion source is incorporated, track provenance/license and upstream independently. Native integration requires backend execution, not merely embedding a URL.

Read-only production findings: dockge-02 runs CLIProxyAPI v8.0.63 and standalone Keeper v2.1.0; installed official bridge is keeper-v0.1.1.so. Its configuration names Willxup/cpa-plugin-usage-keeper. The separate rainoffallingstar source inspected used CPA v7 SDK and c-shared Go build with SQLite and management/resource routes; verify against v8 rather than copying assumptions. Native plugins share the proxy process: bound ingestion/resource usage, avoid proxy-blocking DB work, and implement clean startup/shutdown and persistent storage. Device upload authentication must be enforced even if resource routes lack host admin authentication.

## Useful local references (read-only unless specifically authorized)

- /root/code/codex-broker/docs/usage-sync.md
- /root/code/codex-broker/internal/usagesync/
- /root/code/codex-broker/internal/usagereport/
- Existing uploader executable: /root/.codex-broker-usage-native/ccusage (20.0.43).
- Its private config.json contains credentials: do not copy or print it.
- Public collector config ccusage.json uses pi.defaults.piPath with both /root/.pi/agent/sessions and /root/.config/pi-t3/agent/sessions. Explicit paths were necessary; PI_CODING_AGENT_DIR alone did not fix discovery for the tested collector.
- Existing uploader scans all history and uploads changed days. Study its design without bringing production state into tests.

Read-only commands already verified (substitute executable/config paths):
ccusage daily --by-agent --json --timezone UTC --offline --config PATH
ccusage session --by-agent --json --offline --config PATH
ccusage pi session --json --offline --config PATH
ccusage codex session --json --offline --config PATH

Observed unified session report: top-level session/totals, rows with agent, period, metadata, modelBreakdowns, modelsUsed, token categories, totalCost. Pi-specific sessions include sessionId, firstActivity, lastActivity, projectPath, model breakdowns. Codex-specific sessions include sessionId, lastActivity, directory, sessionFile, models, reasoningOutputTokens. These expose local paths; sanitize them before upload. They do not expose the raw logs' request timing/outcomes.

Underlying Pi/Codex logs contain richer data, but the user explicitly accepted losing that detail to avoid maintaining client parsers. Do not expand back into custom readers/live hooks without asking. Historical repeated response IDs were observed, so use ccusage accounting and test idempotency rather than blindly summing original logs.

## Initial implementation sequence and acceptance

1. Inspect pinned upstream architecture and official CPA v8 plugin SDK. Establish feasible native entry point and resource/auth boundaries; keep versioned provenance.
2. Implement minimal native host lifecycle and embedded UI serving; test locally, not in production.
3. Add isolated aggregate device report storage, enrollment, upload validation/idempotency, and Keeper pricing integration.
4. Adapt necessary UI/filter endpoints; hide incompatible CPA/request features without unnecessary churn.
5. Add collector/uploader and one-command release installer, offline retry and scheduling, status/update/uninstall.
6. Test token normalization, day/session non-double-counting, stale/conflicting revisions, replay, ownership/revocation, pricing unknowns, historical replacement, no secrets in logs, installer behavior and plugin lifecycle. Build supported release artifacts and run relevant upstream checks.
7. Review final diff, document actual support/limits, and keep commits concise. No production deployment or public release without user approval; pushing completed repository changes follows existing project guidance.
