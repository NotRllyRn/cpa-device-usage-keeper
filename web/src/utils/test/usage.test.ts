import { describe, expect, it } from 'vitest';
import { calculateCacheReadRate, calculateDisplayInputTokens, formatUsd, resolveUsageFilterWindow } from '@/utils/usage';

describe('formatUsd small-value opt-in', () => {
  it('preserves upstream formatting by default', () => {
    expect(formatUsd(0.000003)).toBe('$0.0000');
    expect(formatUsd(0)).toBe('$0.0000');
  });

  it('distinguishes small positive values from zero and from each other', () => {
    expect([0, 0.000001, 0.000002].map(value => formatUsd(value, true))).toEqual(['$0.0000', '$0.000001', '$0.000002']);
    expect(formatUsd(0.00000000003, true)).toBe('$0.00000000003');
  });

  it('does not change normal currency values when opted in', () => {
    for (const value of [0, 0.0001, 0.25, 1, 12345]) expect(formatUsd(value, true)).toBe(formatUsd(value));
  });
});

describe('resolveUsageFilterWindow', () => {
  it('resolves today from local day start through the refresh anchor', () => {
    const nowMs = Date.parse('2026-04-23T12:34:56.000Z');
    const expectedStart = new Date(nowMs);
    expectedStart.setHours(0, 0, 0, 0);

    const window = resolveUsageFilterWindow(null, 'today', { nowMs });

    expect(window).toEqual({
      startMs: expectedStart.getTime(),
      endMs: nowMs,
      windowMinutes: Math.max((nowMs - expectedStart.getTime()) / 60000, 1),
    });
  });

  it('resolves yesterday as the previous local day boundary', () => {
    const nowMs = Date.parse('2026-04-23T12:34:56.000Z');
    const expectedStart = new Date(nowMs);
    expectedStart.setHours(0, 0, 0, 0);
    expectedStart.setDate(expectedStart.getDate() - 1);
    const expectedEnd = new Date(expectedStart);
    expectedEnd.setDate(expectedEnd.getDate() + 1);
    expectedEnd.setMilliseconds(expectedEnd.getMilliseconds() - 1);

    const window = resolveUsageFilterWindow(null, 'yesterday', { nowMs });

    expect(window).toEqual({
      startMs: expectedStart.getTime(),
      endMs: expectedEnd.getTime(),
      windowMinutes: 24 * 60,
    });
  });

  it('resolves 30d as a rolling thirty-day window', () => {
    const nowMs = Date.parse('2026-04-23T12:34:56.000Z');

    const window = resolveUsageFilterWindow(null, '30d', { nowMs });

    expect(window).toEqual({
      startMs: nowMs - 30 * 24 * 60 * 60 * 1000,
      endMs: nowMs,
      windowMinutes: 30 * 24 * 60,
    });
  });
});

describe('calculateCacheReadRate', () => {
  it('uses normalized input tokens as the denominator', () => {
    expect(calculateCacheReadRate({ inputTokens: 1000, cacheReadTokens: 250 })).toBe(25);
  });

  it('does not apply provider-specific token math in the frontend', () => {
    expect(calculateCacheReadRate({ inputTokens: 400, cacheReadTokens: 600 })).toBe(150);
  });

  it('returns null when there is no cacheable input', () => {
    expect(calculateCacheReadRate({ inputTokens: 0, cacheReadTokens: 0 })).toBeNull();
  });
});

describe('calculateDisplayInputTokens', () => {
  it('subtracts both cache read and cache write tokens from normalized input', () => {
    expect(calculateDisplayInputTokens({
      inputTokens: 1000,
      cacheReadTokens: 250,
      cacheCreationTokens: 100,
    })).toBe(650);
  });
});
