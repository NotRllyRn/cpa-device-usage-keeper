import { test } from 'node:test';
import assert from 'node:assert/strict';
import { fixture } from './device-ui-fixture.mjs';
const metrics = row => [row.total_tokens, ...Object.values(row.tokens), row.known_cost, ...Object.values(row.coverage)];
function conserves(expected, rows) {
  metrics(expected).forEach((n, index) => assert.ok(Math.abs(n - rows.reduce((sum, r) => sum + metrics(r)[index], 0)) < 1e-7, `metric ${index} must conserve`));
}
for (const state of ['partial', 'complete', 'unknown', 'zero', 'small', 'empty']) {
  for (const query of ['', '?start=2026-11-01&end=2026-11-09', '?device=fixture-b&agent=pi&family=unknown']) {
    test(`${state} ${query || 'all time'} conserves across every response dimension`, () => {
      const data = fixture(new URL('http://fixture/v1/analysis' + query), state);
      for (const field of ['days', 'models', 'devices', 'cells']) conserves(data.total, data[field]);
      assert.equal(new Set(data.cells.map(r => `${r.device}/${r.model}`)).size, data.cells.length, 'one matrix cell per device/model, not per day');
      for (const model of data.models) conserves(model, data.cells.filter(c => c.model === model.key));
      for (const device of data.devices) conserves(device, data.cells.filter(c => c.device === device.key));
      for (const day of data.days) {
        const breakdown = data.breakdown.find(b => b.day === day.key);
        conserves(day, breakdown.models); conserves(day, breakdown.devices);
      }
      assert.equal(data.total.total_tokens, Object.values(data.total.tokens).reduce((n, v) => n + v, 0));
      assert.ok(data.total.coverage.unattributed_tokens <= data.total.total_tokens);
    });
  }
}
test('reviewed complete and unknown matrices match rankings, not the first daily cell', () => {
  for (const [state, total] of [['complete', 880000000], ['unknown', 902000000]]) {
    const data = fixture(new URL('http://fixture/v1/analysis'), state);
    assert.equal(data.total.total_tokens, total);
    assert.deepEqual(data.cells.map(c => c.total_tokens), data.models.map(m => m.total_tokens));
    assert.equal(data.cells[0].total_tokens, total / 2);
  }
});
