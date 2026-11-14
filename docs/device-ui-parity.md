# Device dashboard adapters

The device entry point imports Keeper's StatCards/DailyAverageCard,
TokenActivityCard, UsageComparisonCharts, TimeRangeControl/CustomRangePanel,
PricingSettings and CompletePricingSyncModal. Optional aggregate capabilities
leave the default upstream behavior unchanged. No request events are created.

`web/src/device/dashboard.ts` converts daily totals into the shared view contracts.
Dates are UTC; missing days contribute zero to the selected calendar-day average.
Token Trends uses actual daily model/device totals, not inferred hourly traffic.
Model distribution excludes unattributed tokens; overall totals retain them.
Request health, request counts, throughput, latency and reasoning breakdowns are
not shown. Daily activity reuses the upstream heatmap with compact cells: up to seven days
form one keyboard-accessible row; longer ranges use weekly columns. Unsupported
hour ranges are omitted. The upstream custom picker keeps its default 365-day
limit; the device entry opts into 3,660 days and year-bearing custom labels.

All time is an optional action in the shared range menu. It resolves the earliest
stored usage day for the selected device and agent through today UTC. The
admin-authenticated `GET /v1/usage-boundary?device=…&agent=…` reads only `MIN(day)`,
with a three-second context deadline and boundary indexes, independent of the
current date range. Empty history uses today and displays an explanation. The
intent survives filter changes, refreshes and UTC midnight; refresh (or returning
to the tab) discovers late older uploads. Selecting another range clears it.
The resolved dates are shown as a custom range, without clipping to 365 days.

Device rendering explicitly rejects more than 3,660 calendar days or 100,000
dense model/device chart points; no partial totals are displayed. The existing
server snapshot, breakdown-cell and query-time limits also remain in force.
For larger histories, select a narrower custom range or filters. These are
resource limits, not a claim that arbitrary lifetime histories can be rendered.

Sync uses the upstream modal and matching candidate ordering. The device adapter
requires an exact catalog identity (or the established `[pi] ` prefix removal)
and all four explicit rates. Incomplete catalog entries remain unmatched rather
than becoming free. Apply is transactional and changes only base rates, retaining
stored style, multipliers and branches. Management authentication and existing
request-size bounds apply; batches are limited to 1,000 models. Sync discovers
identifiers with a dedicated context-bounded distinct query, not a historical
usage aggregation. Shared-view input includes cache reads/writes for correct
cache percentages; the model table retains exclusive input counts. Relative
ranges resolve at every refresh and login; custom dates remain fixed.

## Analysis

Analysis imports Keeper's `AnalysisPanel`, chart builders, ranking/selection,
composition and heatmap directly. Its optional aggregate mode hides requests,
reasoning, RPM/TPM, latency, model speed and request-based efficiency; it relabels
API Key as Device and offers model/device trend selection. Credentials and
provider composition are omitted because uploads have neither dimension.
The device-only breakdown table exposes exclusive input, output, cache read,
cache write, unattributed category tokens, total tokens and estimated cost.

`GET /v1/analysis` uses the same session authentication, filters, consistent
SQLite snapshot and Keeper pricing as Overview, adding a device/model matrix.
Unassigned tokens receive an explicit unattributed row, never an invented model.
Only uploaded dates appear in Analysis charts; the chart average is per uploaded
day, unlike Overview's selected-calendar-day average. No hourly samples or
request totals are inferred. Fixed prices and Pi fallback are supported;
conditional prices still require unavailable request details.

Both aggregates return priced/missing/conditional model-day entry counts and
unattributed token counts. Overview and Analysis display a normal-sized amount
under **Estimated cost** or **Partial cost subtotal**, with missing reasons.
Overview's daily average divides that same subtotal by selected calendar days,
explicitly labeling partial coverage. All-unknown costs remain unknown; explicit
free rates remain zero; no uploads remain empty. Incomplete daily costs are gaps
in the cost line, not zero-cost points. Shared upstream defaults are unchanged.

The query rejects more than 100,000 snapshots or 256 MiB of snapshot payloads,
with an eight-second deadline and existing 100,000 breakdown-cell limit.
Analysis additionally rejects more than 100 models/devices or 100,000 dense
chart points. The browser retains the 3,660-day limit. Limits return errors,
never silently clipped totals; filter by device/agent or shorten the range.

## Reproduce local checks

```sh
npm --prefix web run typecheck
npm --prefix web test
npm --prefix web run build
(cd web && npx vite build --config vite.device.config.js)
go vet ./...
go test -race ./internal/device ./internal/service ./internal/pricing/...
DEVICE_TEST_PLAYWRIGHT=/path/to/node_modules/playwright \
DEVICE_TEST_CHROMIUM=/path/to/chrome \
go test ./internal/device -run TestBrowser -v -count=1
```

Repeat the browser command with `DEVICE_COMPARE_UI=1` to compare both compiled
entry points, using synthetic fixtures, at 1440×1000 in light and dark themes.
`DEVICE_SCREENSHOTS` defaults to `/tmp/device-ui-parity`. The comparison checks
legend toggling, device grouping, range capability controls and sync previews.
Add `DEVICE_ALL_TIME=1` to the comparison command to exercise All time via
keyboard, multi-year dates, device/agent changes, late uploads, midnight,
empty history, explicit range rejection and mobile overflow. Screenshot output
also includes mobile menus and multi-year custom panels in both themes.
The regular browser test exercises authenticated storage, editing and sync apply.
No production data or live catalog is used in these browser fixtures.

Use `DEVICE_ANALYSIS_UI=1` instead of `DEVICE_COMPARE_UI` for matching synthetic
Analysis fixtures in both compiled entry points, at 1440×1000 and 390×1000 in
light and dark themes. Output defaults to `/tmp/device-analysis-parity`.
This checks composition/ranking selection, model/device trends and tables,
device/agent/date/all-time filters, zero/empty states, partial/unknown/complete
cost presentation and mobile overflow. Upstream-only unsupported sections remain
visible in upstream screenshots and are intentionally absent in device views.

Use `DEVICE_ANALYSIS_SCOPE=1` for the navigation regression: Overview with 101
models or devices → Analysis, server/adapter limit errors, narrowing filters to
recover, and delayed responses during tab/filter changes. Loaded views are tied
to their endpoint/query scope; adapters run inside the request error handler,
not during rendering.

These tests do not establish deployed CPA plugin compatibility or replace native
host lifecycle checks before deployment. No production deployment is performed.
