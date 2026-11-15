// @vitest-environment happy-dom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import type { ChartData, ChartOptions, Plugin, ScriptableContext, ScriptableLineSegmentContext, TooltipItem, TooltipModel } from 'chart.js';
import type { AnalysisTokenUsageBucket } from '@/lib/types';
import { AnalysisTestPanel, emptyAnalysis } from './analysisFixtures';

globalThis.IS_REACT_ACT_ENVIRONMENT = true;
const capture = vi.hoisted(() => ({ data: null as ChartData<'bar'> | null, options: null as ChartOptions<'bar'> | null }));
vi.mock('react-chartjs-2', () => ({
  Bar: ({ data, options, plugins }: { data: ChartData<'bar'>; options: ChartOptions<'bar'>; plugins: Plugin<'bar'>[] }) => {
    if (plugins.some(plugin => plugin.id === 'analysis-token-average-line')) {
      capture.data = data;
      capture.options = options;
    }
    return null;
  },
  Doughnut: () => null, Scatter: () => null,
}));
vi.mock('react-i18next', () => ({ initReactI18next: { type: '3rdParty', init() {} }, useTranslation: () => ({ t: (key: string) => key }) }));

const bucket: AnalysisTokenUsageBucket = {
  bucket: '2026-05-28T01:00:00Z', input_tokens: 1000, output_tokens: 200,
  cache_read_tokens: 100, cache_creation_tokens: 50, reasoning_tokens: 0,
  total_tokens: 1200, requests: 2, cost_usd: 0.5, cost_available: true,
};
let container: HTMLDivElement;
let root: Root;
const group = () => container.querySelector('[aria-label="Token chart metric"]')!;
const buttons = () => Array.from(group().querySelectorAll('button'));
const rangeSummary = () => container.querySelector('dl')!;
async function render(buckets = [bucket], aggregateMode = true, costAvailable = true, isMobile = false) {
  await act(async () => root.render(<AnalysisTestPanel aggregateMode={aggregateMode} isMobile={isMobile} analysis={{
    ...emptyAnalysis, granularity: 'daily', token_usage: buckets, cost_summary: { total_cost_usd: buckets.reduce((sum, row) => sum + row.cost_usd, 0), cost_available: costAvailable },
  }} />));
}
beforeEach(() => {
  container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  capture.data = null;
  capture.options = null;
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
});

it.each([
  [1, 'input_tokens', 850, 1700],
  [2, 'output_tokens', 200, 400],
  [3, 'cache_read_tokens', 100, 200],
  [4, 'cache_creation_tokens', 50, 100],
  [5, 'total_cost', 0.5, 1],
] as const)('selects metric %s exclusively and keeps it current after data updates', async (index, label, value, updatedValue) => {
  await render();
  expect(group().getAttribute('role')).toBe('group');
  expect(buttons()).toHaveLength(6);
  expect(buttons()[0].getAttribute('aria-pressed')).toBe('true');
  expect(capture.data?.datasets).toHaveLength(6);
  await act(async () => buttons()[index].click());
  expect(buttons().filter(button => button.getAttribute('aria-pressed') === 'true')).toEqual([buttons()[index]]);
  expect(capture.data?.datasets).toHaveLength(1);
  expect(capture.data?.datasets[0].label).toBe(`usage_stats.${label}`);
  expect(capture.data?.datasets[0].data).toEqual([value]);
  expect(capture.options?.scales?.tokens?.display).toBe(index !== 5);
  expect(capture.options?.scales?.cost?.display).toBe(index === 5);
  expect(capture.options?.scales?.requests?.display).toBe(false);
  expect(capture.options?.plugins).not.toHaveProperty('analysisTokenAverageLine');
  expect(capture.options?.plugins?.tooltip?.callbacks?.footer).toBeUndefined();
  expect(group().textContent).not.toMatch(/Avg \/ uploaded day|Unattributed/);

  await render([{ ...bucket, input_tokens: 2000, output_tokens: 400, cache_read_tokens: 200, cache_creation_tokens: 100, total_tokens: 2400, cost_usd: 1 }]);
  expect(buttons()[index].getAttribute('aria-pressed')).toBe('true');
  expect(capture.data?.datasets[0].data).toEqual([updatedValue]);
  await render([]);
  expect(container.textContent).toContain('usage_stats.no_data');
  await render();
  expect(buttons()[index].getAttribute('aria-pressed')).toBe('true');
  expect(capture.data?.datasets[0].data).toEqual([value]);

  await act(async () => buttons()[0].click());
  expect(buttons()[0].getAttribute('aria-pressed')).toBe('true');
  expect(capture.data?.datasets).toHaveLength(6);
  expect(capture.options?.scales?.tokens?.display).toBe(true);
  expect(capture.options?.scales?.cost?.display).toBe(true);
  expect(capture.options?.plugins).toHaveProperty('analysisTokenAverageLine');
  expect(capture.options?.plugins?.tooltip?.callbacks?.footer).toBeTypeOf('function');
  expect(group().textContent).toContain('Avg / uploaded day');
  expect(capture.data?.datasets.at(-1)?.data).toEqual([0]);
});

it('plots complete and partial known costs, including free/partial zero, but not unpriced or unattributed-only days', async () => {
  const states = [
    { ...bucket, total_tokens: 1500, cost_state: 'complete' as const },
    { ...bucket, cost_usd: 0.2, cost_available: false, cost_state: 'partial' as const },
    { ...bucket, cost_usd: 0, cost_available: false, cost_state: 'unknown' as const },
    { ...bucket, cost_usd: 0, cost_state: 'complete' as const },
    { ...bucket, cost_usd: 0, cost_available: false, cost_state: 'partial' as const },
    { ...bucket, input_tokens: 0, output_tokens: 0, cache_read_tokens: 0, cache_creation_tokens: 0, cost_usd: 0, cost_available: false, cost_state: 'unknown' as const },
  ].map((row, index) => ({ ...row, bucket: `2026-05-${String(20 + index).padStart(2, '0')}T00:00:00Z` }));
  await render(states, true, false);
  expect(capture.data?.datasets.at(-1)?.data).toEqual([300, 0, 0, 0, 0, 1200]);
  expect(capture.data?.datasets.find(dataset => dataset.yAxisID === 'cost')?.data).toEqual([0.5, 0.2, null, 0, 0, null]);
  await act(async () => buttons()[5].click());
  expect(buttons()[5].textContent).toContain('partial / unknown');
  expect(capture.data?.datasets[0].label).toContain('partial / unknown');
  expect(capture.data?.datasets[0].data).toEqual([0.5, 0.2, null, 0, 0, null]);
  const cost = capture.data?.datasets[0] as unknown as {
    spanGaps: boolean;
    pointStyle: (context: ScriptableContext<'line'>) => string;
    segment: { borderDash: (context: ScriptableLineSegmentContext) => number[] | undefined };
  };
  expect(cost.spanGaps).toBe(false);
  expect(cost.pointStyle({ dataIndex: 0 } as ScriptableContext<'line'>)).toBe('circle');
  expect(cost.pointStyle({ dataIndex: 1 } as ScriptableContext<'line'>)).toBe('triangle');
  expect(cost.segment.borderDash({ p0DataIndex: 0, p1DataIndex: 1 } as ScriptableLineSegmentContext)).toEqual([5, 4]);
  expect(cost.segment.borderDash({ p0DataIndex: 3, p1DataIndex: 3 } as ScriptableLineSegmentContext)).toBeUndefined();
  const tooltip = capture.options?.plugins?.tooltip?.callbacks?.label;
  expect(tooltip?.call({} as TooltipModel<'bar'>, { dataset: capture.data!.datasets[0], dataIndex: 1, parsed: { y: .2 } } as TooltipItem<'bar'>)).toBe('Known cost subtotal — incomplete: $0.2000');
  expect(tooltip?.call({} as TooltipModel<'bar'>, { dataset: capture.data!.datasets[0], dataIndex: 0, parsed: { y: .5 } } as TooltipItem<'bar'>)).toBe('usage_stats.total_cost: $0.5000');
  await render([{ ...bucket, cost_usd: 0 }]);
  expect(buttons()[5].textContent).not.toContain('partial / unknown');
  expect(capture.data?.datasets[0].label).toBe('usage_stats.total_cost');
  expect(capture.data?.datasets[0].data).toEqual([0]);
});

it.each([false, true])('sums the active multiday range, not gaps or a viewport (mobile %s)', async (isMobile) => {
  const days = [bucket, { ...bucket, bucket: '2026-05-31T01:00:00Z', total_tokens: 1500, cost_usd: .2, cost_available: false, cost_state: 'partial' as const }];
  await render(days, true, false, isMobile);
  expect(rangeSummary().textContent).toContain('usage_stats.total_tokens — range sum2,700');
  expect(rangeSummary().textContent).toContain('usage_stats.total_cost — range sumUnknown (known $0.7000)');
  expect(capture.data?.datasets[0].data).toEqual([850, null, null, 850]);
  expect(capture.data?.datasets.find(dataset => dataset.yAxisID === 'cost')?.data).toEqual([.5, null, null, .2]);
  expect(capture.options?.plugins).toHaveProperty('analysisTokenAverageLine.value', 1350);
  for (const [index, label, sum] of [[1, 'input_tokens', '1,700'], [2, 'output_tokens', '400'], [3, 'cache_read_tokens', '200'], [4, 'cache_creation_tokens', '100']] as const) {
    await act(async () => buttons()[index].click());
    expect(rangeSummary().textContent).toContain(`usage_stats.${label} — range sum${sum}`);
    expect(capture.data?.datasets).toHaveLength(1);
  }
  await act(async () => buttons()[5].click());
  expect(capture.data?.datasets[0].data).toEqual([.5, null, null, .2]);
  expect(rangeSummary().textContent).toContain('Unknown (known $0.7000)');
  await act(async () => buttons()[1].click());
  await render([bucket], true, true, isMobile);
  expect(rangeSummary().textContent).toContain('usage_stats.input_tokens — range sum850');
  expect(rangeSummary().textContent).toContain('$0.50');
  expect(capture.data?.datasets[0].data).toEqual([850]);
});

it('leaves upstream legends, axes, requests, and average unchanged with aggregate mode off', async () => {
  await render();
  await act(async () => buttons()[1].click());
  await render([bucket], false);
  expect(container.querySelector('[aria-label="Token chart metric"]')).toBeNull();
  expect(container.querySelector('[aria-label="Token chart legend"] button')).toBeNull();
  expect(rangeSummary().textContent).not.toContain('range sum');
  expect(capture.data?.datasets).toHaveLength(7);
  expect(capture.data?.datasets.map(dataset => dataset.label)).toContain('usage_stats.reasoning_tokens');
  expect(capture.data?.datasets.find(dataset => dataset.yAxisID === 'requests')?.data).toEqual([2]);
  expect(capture.options?.scales?.tokens?.display).toBeUndefined();
  expect(capture.options?.scales?.cost?.display).toBeUndefined();
  expect(capture.options?.scales?.requests?.display).toBeUndefined();
  expect(capture.options?.plugins).toHaveProperty('analysisTokenAverageLine');
  expect(capture.options?.plugins?.tooltip?.callbacks?.footer).toBeTypeOf('function');
});
