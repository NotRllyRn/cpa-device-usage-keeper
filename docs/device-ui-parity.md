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
not shown. Daily activity uses the upstream seven-row heatmap layout. Unsupported
hour ranges are omitted; the upstream custom picker retains its 365-day limit.

Sync uses the upstream modal and matching candidate ordering. The device adapter
requires an exact catalog identity (or the established `[pi] ` prefix removal)
and all four explicit rates. Incomplete catalog entries remain unmatched rather
than becoming free. Apply is transactional and changes only base rates, retaining
stored style, multipliers and branches. Management authentication and existing
request-size bounds apply; batches are limited to 1,000 models.

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
The regular browser test exercises authenticated storage, editing and sync apply.
No production data or live catalog is used in these browser fixtures.

These tests do not establish deployed CPA plugin compatibility or replace native
host lifecycle checks before deployment. No production deployment is performed.
