import { describe, expect, it } from 'vitest';
import { dashboardData, rangeDates } from './dashboard';
import type { Aggregate, Overview } from './api';
describe('shared aggregate contracts', () => {
 it('includes both cache categories in shared input without changing stored counts', () => {
  const total: Aggregate = {key:'example', tokens:{input:100,output:50,cache_read:900,cache_write:200},total_tokens:1250,known_cost:1,complete:true};
  const data: Overview = {total,models:[total],days:[],devices:[],agents:[],snapshots:1,collected_at:'',breakdown:[]};
  const view=dashboardData(data,[],'2026-01-01','2026-01-01');
  expect(view.usage.summary?.input_tokens).toBe(1200);
  expect(view.activity.input_tokens).toBe(1200);
  expect(view.comparisons.models[0].input_tokens).toBe(1200);
  expect(view.comparisons.models[0].cache_read_tokens / view.comparisons.models[0].input_tokens).toBe(.75);
  expect(total.tokens.input).toBe(100);
 });
});
describe('UTC aggregate ranges', () => {
 const now = new Date('2026-01-01T23:59:00Z');
 it('refreshes relative dates after midnight while preserving custom days', () => {
  expect(rangeDates('today', undefined, new Date('2026-01-02T00:01:00Z'))).toEqual({start:'2026-01-02',end:'2026-01-02'});
  expect(rangeDates('custom', {unit:'day',start:'2025-12-01',end:'2025-12-02'},now)).toEqual({start:'2025-12-01',end:'2025-12-02'});
 });
 it('uses inclusive days across year boundaries', () => {
  expect(rangeDates('7d', undefined, now)).toEqual({start:'2025-12-26',end:'2026-01-01'});
  expect(rangeDates('yesterday', undefined, now)).toEqual({start:'2025-12-31',end:'2025-12-31'});
  expect(rangeDates('today', undefined, now)).toEqual({start:'2026-01-01',end:'2026-01-01'});
 });
});
