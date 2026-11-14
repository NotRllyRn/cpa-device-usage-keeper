import type { AnalysisResponse, AnalysisModelUsagePayload } from '@/lib/types';
import type { Aggregate, Device, Overview } from './api';
import { validateDashboardRange } from './dashboard';

export function analysisData(data: Overview, devices: Device[], start: string, end: string): AnalysisResponse & { device_usage: AnalysisModelUsagePayload } {
  validateDashboardRange(start, end, data.models.length + data.devices.length);
  if (data.models.length > 100 || data.devices.length > 100) throw new Error('Analysis exceeds 100 models or devices. Narrow the filters; no partial totals are shown.');
  const modelLabel = (key: string) => key || 'Unattributed (no model)';
  // Keeper input is inclusive; the aggregate display removes caches for its input bar.
  // Request/reasoning fields are structural only, never rendered in aggregate mode.
  const metrics = (v: Aggregate) => ({ input_tokens: v.tokens.input + v.tokens.cache_read + v.tokens.cache_write,
    output_tokens: v.tokens.output, cache_read_tokens: v.tokens.cache_read, cache_creation_tokens: v.tokens.cache_write,
    reasoning_tokens: 0, requests: 0, total_tokens: v.total_tokens, cost_usd: v.known_cost, cost_available: v.complete });
  const names = Object.fromEntries(devices.map(d => [d.id, devices.filter(other => other.name === d.name).length > 1 ? `${d.name} (${d.id})` : d.name]));
  const composition = (values: Aggregate[], label: (key: string) => string) => [...values].sort((a,b) => b.total_tokens-a.total_tokens || a.key.localeCompare(b.key)).map(v => ({
    ...metrics(v), key: v.key, label: label(v.key), percent: data.total.total_tokens ? v.total_tokens/data.total.total_tokens*100 : 0,
  }));
  const dayDevices = new Map(data.breakdown.map(day => [day.day, new Map(day.devices.map(v => [v.key, v.total_tokens]))]));
  const dayModels = new Map(data.breakdown.map(day => [day.day, new Map(day.models.map(v => [v.key, v.total_tokens]))]));
  // Only uploaded dates carry values; the chart may insert null calendar gaps.
  const buckets = data.days.map(day => day.key + 'T00:00:00Z');
  return {
    granularity: 'daily', timezone: 'UTC', range_start: start + 'T00:00:00Z', range_end: end + 'T23:59:59Z',
    token_usage: data.days.map(day => ({ bucket: day.key + 'T00:00:00Z', ...metrics(day),
      cost_state: day.complete ? 'complete' : (day.coverage?.priced ?? 0) > 0 ? 'partial' : 'unknown' })),
    model_usage: { buckets, series: data.models.map(model => ({ model: modelLabel(model.key), requests: [], total_tokens: data.days.map(day => dayModels.get(day.key)?.get(model.key) ?? 0) })) },
    device_usage: { buckets, series: data.devices.map(device => ({ model: names[device.key] || device.key, requests: [], total_tokens: data.days.map(day => dayDevices.get(day.key)?.get(device.key) ?? 0) })) },
    api_key_composition: composition(data.devices, key => names[key] || key), model_composition: composition(data.models, modelLabel),
    credentials_composition: [], ai_provider_composition: [], model_efficiency: [],
    cost_summary: { total_cost_usd: data.total.known_cost, cost_available: data.total.complete },
    heatmap: { api_keys: data.devices.map(d => d.key), api_key_labels: names, models: data.models.map(m => modelLabel(m.key)),
      cells: (data.cells ?? []).map(cell => ({ ...metrics(cell), api_key: cell.device, model: modelLabel(cell.model), intensity: 0 })) },
  };
}
