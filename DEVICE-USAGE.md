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

## Deployment decision pending

An iframe presents a web application; it does not host its backend. Choose between a standalone backend and a CPA-native plugin after verifying the deployed host's plugin ABI, resource routing, upload authentication, storage, and lifecycle.

The independent rainoffallingstar/CPA-Usage-Keeper-Plugin project is a native shared-library implementation inspired by Keeper, not merely an iframe wrapper for this upstream application. Reusing that architecture requires compatibility and security review.

Keep the initial upstream code and MIT notices intact until this deployment decision is made. No device ingestion or plugin implementation is included yet.

## Git workflow

- `origin`: the public device-usage fork.
- `upstream`: Willxup/cpa-usage-keeper.
- `device-usage`: development branch based on v2.1.0.

Keep changes localized, review final diffs, and run relevant checks before publishing implementation changes.
