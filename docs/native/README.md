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
session_origin: "https://usage.example.com"
```

The default bind is `127.0.0.1:8788`. `data_dir` must be an absolute private
0700 directory writable by CPA. The HTTP listener serves the dashboard at `/`,
health at `/healthz`, and authenticated APIs below `/v1`. Point a TLS reverse
proxy at the chosen port. Restrict direct port access; the listener itself is
plain HTTP. The optional HTTPS `dashboard_url` supplies the CPA iframe target.
Without it, CPA displays a configuration notice. No upload token is placed in
an iframe URL. Bind, storage or session-origin changes require a CPA restart.

On first start a random admin token is written to `data_dir/admin-token` with
0600 permissions. Retrieve it privately and enter it in the dashboard. Login
exchanges it for a random 30-day HttpOnly session cookie; the admin token is never
stored in a cookie or browser storage. Enrollment returns a separate upload-only token once.
Revocation prevents further uploads, including retries, without deleting history.
The admin token is not a CPA management token. Never commit or log these files.

## Browser sessions

Set `session_origin` to the exact public HTTPS origin (scheme, host and optional
port; no path or trailing slash). Without it, browser login is disabled; existing
bearer API clients remain supported. Cookies are Secure even when the listener
uses HTTP behind your TLS proxy. Forwarded headers are not trusted. Keep the
backend private and proxy the dashboard and `/v1` under the same origin.

Sessions expire 30 days after login (not sliding), survive CPA restarts, and are
stored only as SHA-256 digests in SQLite. Sign out revokes the current session.
Changing `admin-token` **and restarting CPA** invalidates all old sessions. At
most 128 sessions are retained; a new login evicts the oldest when full. Expired
and old-token records are removed during login. Login has a global budget of 10
attempts per minute, including successes; restart resets that in-memory budget.
Keep proxy rate limits as a second layer. Backups contain session hashes: restoring
an old database can restore old revocation state; rotate the admin token on restore
if all previous sessions must be invalidated.

Cookies use host-only `__Host-`, HttpOnly, Path=/ and SameSite=Strict. All cookie
mutations, including login and POST logout, require the exact configured Origin;
missing, `null`, cross-site and foreign origins are rejected. Bearer clients do
not need Origin. Session cookies never authorize device uploads. Startup checks
the session before displaying the login form. The prior adapter used memory-only
tokens, so there is no stored token to migrate or preserve.

CPA and the dashboard should use HTTPS sibling hosts on the same registrable
site. Cross-site iframes cannot use Strict cookies: open the dashboard directly
instead. Browser third-party-cookie policies may impose further restrictions.
There is no SameSite=None exception. For local tests only, an explicitly configured
`http://127.0.0.1:PORT` (or localhost) origin is allowed with a loopback-only
listener and loopback peer. That uses a separate non-Secure local cookie; never
expose that test listener through a public proxy.

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
