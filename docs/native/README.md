# Native device plugin

The plugin hosts its own HTTP listener **inside CPA**. No separate backend
process or container is needed. The user approved this design because CPA's
resource routes cannot carry upload POST bodies. No CPA source patch is used.

This is a tested local daily-usage implementation, not a published release.
Supported-session reports remain unfinished. The device UI now reuses Keeper
components, styles, charts and its full pricing editor; see [test guide](../cpa-testing.md).

## Verified sources

- Keeper stable baseline: `v2.1.0`, `41b34f12dbef209292765415b860583aad5afc09`.
  GitHub `releases/latest` was checked before implementation.
- CPA: `router-for-me/CLIProxyAPI`, `v8.0.63`,
  `6db7d4b839d86c5b24e17bbe73bcd4db18ef1b1a`.
  Native ABI 1, RPC schema 10 (`sdk/pluginabi/types.go`).
- ABI shim adapted from `Willxup/cpa-plugin-usage-keeper`,
  `2bed296d769a1e171bcbfc1c5abac3a67528b519`, `main.go`.
  Its MIT notice is preserved in [bridge-LICENSE](bridge-LICENSE).
  The official bridge is an iframe bridge, not a backend. Its v7 SDK is not
  treated as proof of v8 compatibility.
- Collector: official ccusage `20.0.43`, pinned platform archive with SHA512
  verification. Only ccusage JSON aggregates are read, not client log formats.

CPA v8.0.63 `internal/pluginhost/management.go` registers resource routes as
GET-only and forwards no body. Management POSTs require CPA management auth.
Devices never receive that credential. This listener instead separates its own
admin token from revocable per-device upload tokens.

## Configuration (not installed automatically)

Plugin configuration YAML:

```yaml
listen: "0.0.0.0:8788"
data_dir: "/var/lib/device-usage-keeper"
dashboard_url: "https://usage.example.com"
```

The default bind is `127.0.0.1:8788`. `data_dir` must be an absolute private
0700 directory writable by CPA. The HTTP listener serves the dashboard at `/`,
health at `/healthz`, and authenticated APIs below `/v1`. Point a TLS reverse
proxy at the chosen port. Restrict direct port access; the listener itself is
plain HTTP. The optional HTTPS `dashboard_url` supplies the CPA iframe target.
Without it, CPA displays a configuration notice. No upload token is placed in
an iframe URL. Bind or storage changes require a CPA restart.

On first start a random admin token is written to `data_dir/admin-token` with
0600 permissions. Retrieve it privately and enter it in the dashboard; it stays
in browser memory. Enrollment returns a separate upload-only token once.
Revocation prevents further uploads, including retries, without deleting history.
The admin token is not a CPA management token. Never commit or log these files.

## Accounting and persistence

Snapshots are keyed by authenticated device, agent, and UTC day. Higher revisions
replace; identical retries acknowledge; stale/conflicting revisions reject.
SQLite transactions include revision and revocation checks. Daily totals and
model allocations are stored separately because ccusage can leave tokens
unattributed. Those tokens remain visible and unpriced. ccusage costs are ignored.

The pricing adapter reuses Keeper's model-pricing validation and arithmetic.
Explicit complete zero rates mean free; absent rates or request-dependent rules
mean unknown. Prices are stored separately from CPA traffic settings. Current
rates apply when history is read, so changes reprice historical estimates.
Names such as `[pi] provider/model` are preserved: configure their exact prices.
Estimates are API-equivalent values, not subscription bills. No requests,
latency, TTFT, success rates, or TPS are manufactured.

History uses an insertion cursor, 200 snapshots per page. Backfills appear on
later pages; revisions to already loaded rows require Refresh. Page totals are
not a consistent point-in-time database snapshot. Copied histories on different
devices are not deduplicated. Omitted days remain; an explicit smaller/empty
snapshot replaces a day. There is no automatic deletion of retained history.

Storage uses a single SQLite connection, WAL, full synchronous commits and an
exclusive directory lock. Files are private. Stop CPA before copying the entire
data directory for a consistent backup; restore it with permissions intact.
Preserve device client state alongside server state: revision resets can conflict.
A server restore can lose acknowledgements newer than the backup; reconciliation
is not automated, so do not claim backup restoration alone restores full history.

## Bounds and lifecycle

- ABI input: 64 KiB; unknown schemas rejected before dispatch.
- HTTP JSON: 64 KiB; unknown fields rejected; 100 models per snapshot.
- Eight active handlers; header/read/write/idle deadlines; bounded history pages.
- Upload credentials verified before body decoding and again in the transaction.
- 1,000 enrolled devices including revoked devices (explicit lifetime cap).
- SQLite max page count 262144 (about 1 GiB at default page size); this is not a
  bound on WAL size or filesystem use. Monitor storage and proxy resource limits.
- Shutdown drains for up to five seconds, force-closes remaining connections,
  then closes storage and its lock. Reconfiguration never replaces a live owner.

Native code shares CPA's process. HTTP deadlines/handler limits do not establish
an OS-level memory or connection limit. A reverse proxy should enforce connection,
request-rate, and body limits. Actual CPA v8.0.63 integration now passes in an isolated local host. Sustained
load and the intended deployment still need testing before production approval.

## Local checks

```sh
(cd web && npm ci --ignore-scripts --no-audit --no-fund && npx vite build --config vite.device.config.js)
go test -race ./cmd/device-plugin ./internal/device ./internal/deviceclient ./cmd/device-sync
go vet ./cmd/device-plugin ./internal/device ./internal/deviceclient ./cmd/device-sync
CGO_ENABLED=1 go build -buildmode=c-shared -o /tmp/device-usage-keeper.so ./cmd/device-plugin
python3 scripts/test-native.py /tmp/device-usage-keeper.so
go build -o /tmp/device-sync-linux-amd64 ./cmd/device-sync
bash -n install.sh
```

Linux/amd64 only is build-tested. The ctypes smoke test uses temporary synthetic
state and the actual exported ABI: register, resource, reconfigure, shutdown and
restart. A separate isolated CPA v8.0.63 host test also passes registration, resource
routing, UI serving, shutdown and restart (`scripts/test-cpa-host.py`).
No production installation, deployment, secrets transfer, or public release has
been performed. See [device installer](../device-install.md).
