import type { UsageActivityResponse, UsageComparisonItem, UsageOverviewComparisons, UsageTimeRange, UsageCustomRange } from '@/lib/types';
import type { UsageOverviewPayload } from '@/components/usage/hooks/useUsageData';
import type { Aggregate, Device, Overview } from './api';

export function rangeDates(range: UsageTimeRange, custom?: UsageCustomRange, now = new Date()) {
  const day = (date: Date) => date.toISOString().slice(0, 10);
  if (range === 'custom' && custom?.unit === 'day') return { start: custom.start, end: custom.end };
  const end = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()));
  if (range === 'yesterday') end.setUTCDate(end.getUTCDate() - 1);
  const start = new Date(end);
  if (/^\d+d$/.test(range)) start.setUTCDate(start.getUTCDate() - Number(range.slice(0, -1)) + 1);
  return { start: day(start), end: day(end) };
}

export function dashboardData(data: Overview, devices: Device[], start: string, end: string) {
  const buckets: string[] = [];
  for (let time = Date.parse(start); time <= Date.parse(end); time += 86400000) buckets.push(new Date(time).toISOString().slice(0, 10));
  const rows = new Map((data.breakdown ?? []).map(row => [row.day, row]));
  const items = (values: Aggregate[], dimension: 'models' | 'devices'): UsageComparisonItem[] => values.map(value => ({
    key: value.key, label: dimension === 'devices' ? devices.find(device => device.id === value.key)?.name ?? value.key : value.key,
    // Structural compatibility only: aggregateMode never renders request/reasoning fields.
    requests: 0, failures: 0, reasoning_tokens: 0,
    input_tokens: value.tokens.input + value.tokens.cache_read + value.tokens.cache_write, output_tokens: value.tokens.output, cache_read_tokens: value.tokens.cache_read,
    cache_creation_tokens: value.tokens.cache_write, total_tokens: value.total_tokens, cost: value.complete ? value.known_cost : null,
    token_series: buckets.map(day => rows.get(day)?.[dimension].find(row => row.key === value.key)?.total_tokens ?? 0),
  }));
  const comparisons: UsageOverviewComparisons = { buckets, granularity: 'daily', timezone: 'UTC', models: items(data.models, 'models') };
  const total = data.total;
  const usage: UsageOverviewPayload = { usage: { total_tokens: total.total_tokens }, summary: {
    rpm: 0, tpm: 0, reasoning_tokens: 0, daily_average_requests: 0,
    input_tokens: total.tokens.input + total.tokens.cache_read + total.tokens.cache_write, cache_read_tokens: total.tokens.cache_read,
    cache_creation_tokens: total.tokens.cache_write, total_cost: total.known_cost, cost_available: total.complete,
    daily_average_tokens: total.total_tokens / Math.max(1, buckets.length), daily_average_cost: total.known_cost / Math.max(1, buckets.length), daily_average_range_days: buckets.length,
  }};
  const tokens = (value?: Aggregate) => ({ input_tokens: value ? value.tokens.input + value.tokens.cache_read + value.tokens.cache_write : 0, output_tokens: value?.tokens.output ?? 0, cache_read_tokens: value?.tokens.cache_read ?? 0, cache_creation_tokens: value?.tokens.cache_write ?? 0, reasoning_tokens: 0, total_tokens: value?.total_tokens ?? 0 });
  const activity: UsageActivityResponse = {
    window: 'month', grain: 'daily', timezone: 'UTC', total_success: 0, total_failure: 0, success_rate: 0,
    ...tokens(total), rows: 1, columns: buckets.length, bucket_seconds: 86400, window_start: start + 'T00:00:00Z', window_end: end + 'T23:59:59Z',
    blocks: buckets.map(day => ({ ...tokens(data.days.find(row => row.key === day)), start_time: day + 'T00:00:00Z', end_time: day + 'T23:59:59Z', success: 0, failure: 0, rate: 0 })),
  };
  return { comparisons, devices: items(data.devices, 'devices'), usage, activity };
}
