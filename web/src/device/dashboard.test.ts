import { describe, expect, it } from 'vitest';
import { rangeDates } from './dashboard';
describe('UTC aggregate ranges', () => {
 const now = new Date('2026-01-01T23:59:00Z');
 it('uses inclusive days across year boundaries', () => {
  expect(rangeDates('7d', undefined, now)).toEqual({start:'2025-12-26',end:'2026-01-01'});
  expect(rangeDates('yesterday', undefined, now)).toEqual({start:'2025-12-31',end:'2025-12-31'});
  expect(rangeDates('today', undefined, now)).toEqual({start:'2026-01-01',end:'2026-01-01'});
 });
});
