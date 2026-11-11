# Device usage project

This fork starts from CPA Usage Keeper v2.1.0. It is intended to display retained coding-agent usage collected from devices, separately from CPA proxy traffic.

## Initial scope

- Use pinned ccusage reports instead of maintaining individual agent-log parsers.
- Collect daily and supported session/model token breakdowns with device identity.
- Treat reports as revisioned snapshots, not individual requests. Repeated uploads must not add the same usage again.
- Use Keeper model pricing settings for estimated costs after normalizing token categories. Do not import collector cost estimates as authoritative prices.
- Keep original model identifiers and distinguish unavailable pricing conditions from known zero rates.
- Hide CPA credentials, quotas, request latency, TTFT, request TPS, and request success rates when unavailable from device reports.
- Prefer CPA dashboard iframe access, with a standalone access path when feasible.
- Never upload conversations, credentials, or raw session logs.

## Upstream and deployment

This repository forks [Willxup/cpa-usage-keeper](https://github.com/Willxup/cpa-usage-keeper), the standalone backend and dashboard.

Its official companion, [Willxup/cpa-plugin-usage-keeper](https://github.com/Willxup/cpa-plugin-usage-keeper), adds a CPA management dashboard entry that embeds the standalone application. It does not store usage or replace the backend. These are connected components, not competing installations.

The similarly named `rainoffallingstar/CPA-Usage-Keeper-Plugin` is a separate native implementation. It is not the official companion, is not our upstream, and is not the basis for this project.

The latest implementation requirement is a native CPA plugin that hosts device ingestion, storage, pricing, and the embedded dashboard without a separate service stack. This supersedes the earlier standalone-backend plan. Verify CPA v8 plugin ABI, lifecycle, storage, resource routing, and upload-only authentication before implementation. The official companion currently embeds a separate service; it does not already supply this native backend. Track provenance and licenses for any reused companion code. Only the companion's small native C ABI shim has been adapted for the local hosting proof, with its MIT notice and pinned provenance retained in docs/native/. See IMPLEMENTATION-HANDOFF.md for the full implementation brief.

A GitHub fork has one parent. Packaging both components in this repository later would not make it a GitHub fork of both projects. An iframe does not eliminate the need for a running backend. Replacing an existing Keeper deployment could avoid an additional container, but requires explicit deployment approval and a data migration/backup plan.

Keep upstream code and MIT notices intact. A local native ABI/static-page proof is included; device ingestion is not implemented. See [native hosting findings](docs/native/README.md) for the upload-routing blocker and options.

## Git workflow

- `origin`: the public device-usage fork.
- `upstream`: Willxup/cpa-usage-keeper.
- `device-usage`: development branch based on v2.1.0.

Keep changes localized, review final diffs, and run relevant checks before publishing implementation changes.
