# CPA Device Usage Keeper

A device-usage dashboard fork of [CPA Usage Keeper](https://github.com/Willxup/cpa-usage-keeper), built around coding-agent history collected with **ccusage** rather than CPA request traffic.

**Independent project, not the official CPA Usage Keeper.** Looking for the original proxy-usage dashboard? Visit [Willxup/cpa-usage-keeper](https://github.com/Willxup/cpa-usage-keeper).

## Implemented locally

- Native CPA plugin with a Keeper-based embedded dashboard and its own configurable HTTP port.
- Authenticated, revisioned daily snapshots from multiple devices using pinned ccusage.
- Durable upload retries, revocable device tokens, and retained daily/model history.
- Estimated costs using Keeper's pricing validation and arithmetic.
- Linux x86_64 uploader and private test-bundle installer (no release published yet).

Device reports are aggregates, not requests. Request latency, TTFT, TPS, success rates, and CPA credential/quota management are outside the initial scope. Collection uploads allowlisted aggregates and model identifiers, not conversation logs or provider credentials.

## Status

**In development, not production-ready.** Based on stable upstream **v2.1.0**.
The native target is `cmd/device-plugin`; the original upstream application remains
unchanged for easy comparisons and is not the device backend. No separate service
stack is needed. Point your TLS reverse proxy at the plugin's configured port.

Start with [CPA and device testing](docs/cpa-testing.md),
[native configuration](docs/native/README.md), and [device installation](docs/device-install.md).
Real CPA v8.0.63 lifecycle and browser checks pass locally. Keeper components,
charts and pricing editor are reused with minimal upstream changes. Session
reports, full visual parity, real-device/systemd validation and production load
and recovery checks remain unfinished. There is no public release or production installation.
The official [Keeper embedding plugin](https://github.com/Willxup/cpa-plugin-usage-keeper)
is an iframe bridge, not this backend. See the [implementation handoff](IMPLEMENTATION-HANDOFF.md).

See [project scope](DEVICE-USAGE.md) and the [upstream reference](docs/upstream/README.md) for details.

## License

[MIT](LICENSE). Based on Willxup's CPA Usage Keeper; original notices are retained.
