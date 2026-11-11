# CPA and device testing

A Linux/amd64 test candidate is available locally; this is not a public release
or production-readiness certification. No production service has been changed.

## Build a private test bundle

From the repository (Go, C compiler and Node/npm required on the build machine):

```sh
bash scripts/build-test-bundle.sh /absolute/new/bundle-directory
```

This builds the separate Keeper-based device UI, embeds it into the native
plugin, builds the device uploader, downloads and verifies the pinned official
ccusage binary, and writes SHA256SUMS. The directory must not exist yet.
Build hosts and effective Go targets must be Linux/amd64. Transfer the bundle
privately through a trusted channel; checksums in the same directory detect
corruption, not a malicious replacement of the entire bundle.

## CPA test instance

Use CPA v8.0.63. Back up configuration and persistent storage before changing
an existing instance. Use a disposable instance first. Installing the library
or restarting production still requires approval.

Copy `device-usage-keeper.so` as a regular file into CPA's plugin directory.
Configure the following within CPA's config (adjust paths and bind interface):

```yaml
plugins:
  enabled: true
  dir: "/path/to/plugins"
  configs:
    device-usage-keeper:
      enabled: true
      priority: 0
      listen: "0.0.0.0:8788"
      data_dir: "/path/to/private/device-data"
      dashboard_url: "https://usage.example.com"
```

Use a private 0700 persistent directory owned by CPA's user. Restart CPA, route
HTTPS to port 8788, and open the HTTPS root. CPA's iframe route is
`/v0/resource/plugins/device-usage-keeper/open`. Retrieve the generated
`admin-token` privately from the data directory and enter it in the dashboard.
Keep direct HTTP access restricted; devices require an HTTPS endpoint.

To roll back: stop CPA, restore its prior config, remove only the newly added
plugin library, and restart CPA. Retain the device data directory privately;
never discard it as part of rollback. Do not hot-unload a Go native library.

## Connect test devices

In the Devices tab enroll each device separately. Transfer the bundle to each
Linux x86_64 client with a user systemd instance, then run:

```sh
DEVICE_USAGE_BUNDLE=/path/to/bundle bash /path/to/bundle/install.sh
```

Enter the HTTPS endpoint and that device's upload-only token at the private
prompts. No build or npm registration is required on clients. The first timer
run is scheduled after about two minutes; run `~/.local/bin/device-sync sync`
for an immediate sync. Use `~/.local/bin/device-sync status` to inspect pending
snapshots and last success. Use `bash /path/to/bundle/install.sh status` for
scheduler status. Bundle updates use the same environment variable with
`install.sh update`; uninstall retains private state and server history.

Check all-device totals, per-device/agent filters, UTC dates, and exact model
identifiers. Configure rates in Keeper Model Pricing Settings. Missing prices
and unallocated tokens remain unknown, not zero. Test a retry and confirm totals
do not increase; disconnect a client, sync, reconnect and verify pending uploads
clear. Revoke a throwaway device and confirm uploads are rejected. Do not use
copies of the same client history as independent devices when checking totals.

## Automated validation

- Real CPA v8.0.63 native registration, iframe routing, dashboard serving,
  shutdown and restart pass in an isolated network namespace using synthetic
  temporary state (`scripts/test-cpa-host.py`).
- Browser integration covers login, complete-range totals, disabled metrics,
  Keeper pricing tab, enrollment and logout (`scripts/test-browser.mjs`, invoked
  by optional Go `TestBrowser`).
- Installer test uses isolated HOME, synthetic binaries and mocked systemd;
  covers install, reinstall rejection, update rollback, update, uninstall and
  state retention. It does not certify a real user systemd installation.
- Upstream UI tests: 203 files / 1,727 tests passed before the final candidate.

## Upgrade boundary and remaining limits

Device code is isolated in `internal/device`, `internal/deviceclient`,
`cmd/device-plugin`, `cmd/device-sync`, and `web/src/device`. The upstream UI's
only behavior change is an optional `aggregateMode` capability on
`PricingSettings`; its default preserves upstream behavior. The device entry
point reuses Keeper's header, toolbar, themes, styles, chart wrappers and pricing
editor. It does not manufacture request-shaped data to feed upstream dashboards.

This is Keeper-based rather than a pixel-identical copy of every request view.
Device tables, filtering, and enrollment are specific to aggregate reports.
Realtime traffic, requests, latency, TTFT/TPS, credentials and quotas are unavailable.
Session-level drilldown is still unimplemented; daily totals cover the collector's
retained history without counting a second session representation.

Before production sign-off: validate the actual reverse proxy/iframe deployment,
real client discovery and totals on your platforms, real systemd installation,
backup/recovery procedure and sustained load. Supported sessions and visual
parity beyond the reused components remain work, not completed acceptance claims.
