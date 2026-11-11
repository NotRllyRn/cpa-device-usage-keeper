# Native hosting proof and upload blocker

This is a local, non-production proof, not the device product. It exports the
native C ABI and serves a public static status page. It starts no server,
workers, database, or CPA capture. Shutdown has nothing to release. No device
data or credentials are exposed. The full Keeper dashboard is not embedded yet.

## Verified sources

- Keeper stable baseline: `v2.1.0`, `41b34f12dbef209292765415b860583aad5afc09`.
  GitHub `releases/latest` was checked again before implementation.
- CPA: `router-for-me/CLIProxyAPI`, `v8.0.63`,
  `6db7d4b839d86c5b24e17bbe73bcd4db18ef1b1a`.
  `sdk/pluginabi/types.go`: native ABI 1, RPC schema 10.
- ABI shim adapted from `Willxup/cpa-plugin-usage-keeper`,
  `2bed296d769a1e171bcbfc1c5abac3a67528b519`, `main.go`.
  Its MIT notice is preserved in [bridge-LICENSE](bridge-LICENSE).
  That bridge imports the v7 SDK; this proof checks the v8 wire contract
  explicitly rather than treating the bridge as a v8 backend.

The adapter rejects non-10 schemas and bounds incoming ABI payloads to 64 KiB
before copying C memory. This does not bound allocations already made by CPA.
It is intentionally isolated from upstream application code.

## Blocking upload decision

In CPA v8.0.63:

- `sdk/pluginapi/types.go:1591-1629` describes resource and management routes.
- `internal/pluginhost/management.go:170-187` registers resources as GET only.
- `internal/pluginhost/management.go:297-323` rejects other resource methods
  and forwards headers/query but **no body**.
- `internal/api/server_management.go:255-291` applies CPA management
  authentication before dispatching plugin management routes.
- `internal/pluginhost/management.go:245-272` reads management bodies before
  plugin invocation. Plugin validation cannot protect this allocation itself.
- `internal/pluginhost/client_guard.go` drains active calls before native
  shutdown; any future plugin-owned background work must also stop explicitly.

Therefore ordinary upload-only authenticated POST ingestion cannot be registered
on the existing resource API. Giving devices management credentials, encoding
snapshots in GET URLs, or abusing model execution routes is not acceptable.

Decision needed before ingestion implementation:

1. **Preferred for one existing CPA port:** a small CPA host change supporting
   method-aware resource routes and a bounded body before ABI serialization.
   The plugin must authenticate upload tokens itself. This requires maintaining
   and deploying a CPA patch, not only a Keeper fork.
2. **No CPA source patch:** a plugin-owned bounded HTTP listener inside the CPA
   process, with persistent local SQLite storage. This is still a native plugin,
   not another service stack, but needs a port/TLS or reverse-proxy route and
   explicit lifecycle management. Confirm this deployment tradeoff first.

Persistent storage, TLS exposure, concurrency limits, host runtime loading, and
full dashboard integration are not yet verified. Nothing was deployed.

## Local checks

```sh
go test -race ./cmd/device-plugin
CGO_ENABLED=1 go build -buildmode=c-shared -o /tmp/device-usage-proof.so ./cmd/device-plugin
```

The proof is Linux/amd64 build-tested only. Do not install it as a finished
plugin. Production support and release artifacts are not claimed.
