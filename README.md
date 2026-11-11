# CPA Device Usage Keeper

A device-usage dashboard fork of [CPA Usage Keeper](https://github.com/Willxup/cpa-usage-keeper), built around coding-agent history collected with **ccusage** rather than CPA request traffic.

**Independent project, not the official CPA Usage Keeper.** Looking for the original proxy-usage dashboard? Visit [Willxup/cpa-usage-keeper](https://github.com/Willxup/cpa-usage-keeper).

## Planned features

- Collect retained usage from multiple devices and supported coding agents.
- Explore daily and supported session/model token totals.
- Calculate estimated costs using Keeper's model pricing settings.
- Access the dashboard directly or through CPA's iframe integration.

Device reports are aggregates, not requests. Request latency, TTFT, TPS, success rates, and CPA credential/quota management are outside the initial scope. Collection will upload usage reports, not conversations or credentials.

## Status

**Early development.** The fork is based on upstream **v2.1.0**. Device ingestion is not implemented yet; the application still has upstream behavior. There is no device-ready release or installation procedure.

The target deployment is a native CPA plugin with an embedded dashboard and no separate backend service stack. This is not implemented yet. The official [Keeper embedding plugin](https://github.com/Willxup/cpa-plugin-usage-keeper) only embeds the existing standalone application; hosting our backend inside CPA requires additional integration. See the [implementation handoff](IMPLEMENTATION-HANDOFF.md).

See [project scope](DEVICE-USAGE.md) and the [upstream reference](docs/upstream/README.md) for details.

## License

[MIT](LICENSE). Based on Willxup's CPA Usage Keeper; original notices are retained.
