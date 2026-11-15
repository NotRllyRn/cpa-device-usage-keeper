// Isolated UI fixture: every response dimension is reduced from the same ledger.
export const now = new Date('2026-11-09T12:00:00Z');
export const first = '2026-02-08';
export const deviceName = 'synthetic-workstation-with-an-extremely-long-unbroken-device-identifier-123456789012345678901234567890';
export const modelName = 'gpt-5.4-synthetic-long-model-name-123456789012345678901234567890';
export const agentName = 'synthetic-agent-with-an-extremely-long-version-2026.11.1234567890';
export const devices = [{ id: 'fixture-a', name: deviceName, revoked: false, last_seen: now.toISOString() }, { id: 'fixture-b', name: 'Example laptop', revoked: false, last_seen: null }];
const tokenKeys = ['input', 'output', 'cache_read', 'cache_write'];
const coverageKeys = ['priced', 'missing', 'conditional', 'unattributed_tokens'];
function reduce(key, rows) {
  return { key, tokens: Object.fromEntries(tokenKeys.map(k => [k, rows.reduce((n, r) => n + r.tokens[k], 0)])), total_tokens: rows.reduce((n, r) => n + r.total_tokens, 0), known_cost: rows.reduce((n, r) => n + r.known_cost, 0), complete: rows.every(r => r.complete), coverage: Object.fromEntries(coverageKeys.map(k => [k, rows.reduce((n, r) => n + r.coverage[k], 0)])) };
}
export function fixture(url, state) {
  const start = url.searchParams.get('start') || first, end = url.searchParams.get('end') || '2026-11-09';
  const keys = [modelName, 'claude-synthetic-example'];
  const device = url.searchParams.get('device') || 'fixture-a';
  const factor = (device === 'fixture-b' ? 2 : 1) * (url.searchParams.get('agent') ? 3 : 1) * (url.searchParams.get('family') ? 5 : 1);
  const calendar = Math.max(1, Math.round((new Date(end) - new Date(start)) / 86400000) + 1);
  const dates = state === 'empty' ? [] : Array.from({ length: calendar }, (_, i) => new Date(+new Date(start) + i * 86400000).toISOString().slice(0, 10)).filter((_, i) => i % 7 === 0 || i === calendar - 1);
  const ledger = dates.flatMap((day, index) => keys.map((model, mi) => {
    const n = (index === 1 && state !== 'unknown') || state === 'zero' ? 0 : factor;
    const complete = ['complete', 'small', 'zero'].includes(state) || n === 0 || (state !== 'unknown' && mi === 0);
    const conditional = !complete && state !== 'unknown' && index % 2 === 0;
    return { day, device, model, tokens: { input: n * 6000000, output: n * 2000000, cache_read: n * 2000000, cache_write: n * 1000000 }, total_tokens: n * 11000000, known_cost: complete ? n * (state === 'small' ? .000001 : 13.2) : 0, complete, coverage: { priced: complete && n ? 1 : 0, missing: !complete && !conditional ? 1 : 0, conditional: conditional ? 1 : 0, unattributed_tokens: 0 } };
  }));
  const groups = (field, rows = ledger) => [...new Set(rows.map(r => r[field]))].map(key => reduce(key, rows.filter(r => r[field] === key)));
  const breakdown = dates.map(day => ({ day, models: groups('model', ledger.filter(r => r.day === day)), devices: groups('device', ledger.filter(r => r.day === day)) }));
  const cells = groups('model').map(row => ({ ...row, device, model: row.key }));
  return { total: reduce('all', ledger), days: groups('day'), models: groups('model'), devices: groups('device'), agents: [agentName, 'pi'], collected_at: now.toISOString(), snapshots: dates.length, breakdown, cells };
}
