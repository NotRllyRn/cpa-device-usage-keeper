// @vitest-environment happy-dom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import type { ChartData, ChartOptions, Plugin } from 'chart.js';
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
async function render(buckets = [bucket], aggregateMode = true, costAvailable = true) {
  await act(async () => root.render(<AnalysisTestPanel aggregateMode={aggregateMode} analysis={{
    ...emptyAnalysis, token_usage: buckets, cost_summary: { total_cost_usd: 0.5, cost_available: costAvailable },
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
  expect(group().textContent).not.toMatch(/analysis_token_average|Unattributed/);

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
  expect(group().textContent).toMatch(/analysis_token_average/);
  expect(capture.data?.datasets.at(-1)?.data).toEqual([0]);
});

it('preserves unattributed totals and never plots unavailable cost as zero or a partial total', async () => {
  await render([{ ...bucket, total_tokens: 1500 }, { ...bucket, cost_usd: 0.2, cost_available: false }], true, false);
  expect(capture.data?.datasets.at(-1)?.data).toEqual([300, 0]);
  await act(async () => buttons()[5].click());
  expect(buttons()[5].textContent).toContain('partial / unknown');
  expect(capture.data?.datasets[0].label).toContain('partial / unknown');
  expect(capture.data?.datasets[0].data).toEqual([0.5, null]);
  await render([{ ...bucket, cost_usd: 0 }]);
  expect(buttons()[5].textContent).not.toContain('partial / unknown');
  expect(capture.data?.datasets[0].label).toBe('usage_stats.total_cost');
  expect(capture.data?.datasets[0].data).toEqual([0]);
});

it('leaves upstream legends, axes, requests, and average unchanged with aggregate mode off', async () => {
  await render();
  await act(async () => buttons()[1].click());
  await render([bucket], false);
  expect(container.querySelector('[aria-label="Token chart metric"]')).toBeNull();
  expect(container.querySelector('[aria-label="Token chart legend"] button')).toBeNull();
  expect(capture.data?.datasets).toHaveLength(7);
  expect(capture.data?.datasets.map(dataset => dataset.label)).toContain('usage_stats.reasoning_tokens');
  expect(capture.data?.datasets.find(dataset => dataset.yAxisID === 'requests')?.data).toEqual([2]);
  expect(capture.options?.scales?.tokens?.display).toBeUndefined();
  expect(capture.options?.scales?.cost?.display).toBeUndefined();
  expect(capture.options?.scales?.requests?.display).toBeUndefined();
  expect(capture.options?.plugins).toHaveProperty('analysisTokenAverageLine');
  expect(capture.options?.plugins?.tooltip?.callbacks?.footer).toBeTypeOf('function');
});
