// Local synthetic UI regression harness. Never connects to a running Keeper.
// DEVICE_TEST_PLAYWRIGHT=/path/to/playwright node scripts/fuzz-device-ui.mjs
// Optional: DEVICE_TEST_CHROMIUM, DEVICE_UI_OUTPUT, DEVICE_UI_PHASE=baseline|final,
// DEVICE_UI_MODE=screens|fuzz|all, DEVICE_UI_SEEDS=173,947,2027, DEVICE_UI_ACTIONS=120.
import { createRequire } from 'node:module';
import { createServer } from 'node:http';
import { readFile, writeFile, mkdir } from 'node:fs/promises';
import { resolve, extname } from 'node:path';
import assert from 'node:assert/strict';
const require = createRequire(import.meta.url);
const { chromium } = require(process.env.DEVICE_TEST_PLAYWRIGHT || 'playwright');
const root = resolve(import.meta.dirname, '../web/device-dist');
const phase = process.env.DEVICE_UI_PHASE || 'final';
const output = resolve(process.env.DEVICE_UI_OUTPUT || `/tmp/device-ui-polish/${phase}`);
const mode = process.env.DEVICE_UI_MODE || 'all';
const widths = [320, 390, 768, 1024, 1440, 1920];
const seeds = (process.env.DEVICE_UI_SEEDS || '173,947,2027').split(',').map(Number);
const actions = Number(process.env.DEVICE_UI_ACTIONS || 120);
await mkdir(output, { recursive: true });
const report = { phase, fixture: 'Synthetic only: 275 calendar days; sparse uploads including reported zero; long identifiers; large, small, zero, partial and unavailable costs', screenshots: [], coverage: {}, seeds: [], failures: [], blockedNetwork: [], syntheticSaves: 0, untested: ['Real server/auth/enrollment/revocation', 'Real pricing mutations/sync source network', 'Real browser zoom (CSS zoom + reduced viewport proxy only)', 'Other browsers, OSes, physical touch hardware, screen readers', 'Exhaustive state permutations'] };
const count = key => { report.coverage[key] = (report.coverage[key] || 0) + 1; };
const now = new Date('2026-11-09T12:00:00Z');
const first = '2026-02-08';
const deviceName = 'synthetic-workstation-with-an-extremely-long-unbroken-device-identifier-123456789012345678901234567890';
const modelName = 'gpt-5.4-synthetic-long-model-name-123456789012345678901234567890';
const agentName = 'synthetic-agent-with-an-extremely-long-version-2026.11.1234567890';
const devices = [{ id: 'fixture-a', name: deviceName, revoked: false, last_seen: now.toISOString() }, { id: 'fixture-b', name: 'Example laptop', revoked: false, last_seen: null }];
function aggregate(key, n = 1) {
  return { key, tokens: { input: n * 6000000, output: n * 2000000, cache_read: n * 2000000, cache_write: n * 1000000 }, total_tokens: n * 11000000, known_cost: n * 1234.56789, complete: false, coverage: { priced: n, missing: n * 12345, conditional: n * 9876, unattributed_tokens: n * 123456789 } };
}
function fixture(url, state) {
  const start = url.searchParams.get('start') || first, end = url.searchParams.get('end') || '2026-11-09';
  const empty = state === 'empty';
  const keys = [modelName, 'claude-synthetic-example'];
  const device = url.searchParams.get('device') || 'fixture-a';
  const factor = (device === 'fixture-b' ? 2 : 1) * (url.searchParams.get('agent') ? 3 : 1) * (url.searchParams.get('family') ? 5 : 1);
  const calendar = Math.max(1, Math.round((new Date(end) - new Date(start)) / 86400000) + 1);
  const dates = Array.from({ length: calendar }, (_, i) => new Date(+new Date(start) + i * 86400000).toISOString().slice(0, 10)).filter((_, i) => i % 7 === 0 || i === calendar - 1);
  const reported = dates.map((day, index) => ({ day, n: (index === 1 && state !== 'unknown') || state === 'zero' ? 0 : factor }));
  const sum = reported.reduce((total, day) => total + day.n, 0);
  function make(key, n) {
    const row = aggregate(key, n);
    if (['complete', 'small', 'zero'].includes(state) || n === 0) { row.complete = true; row.coverage = { priced: n ? 1 : 0, missing: 0, conditional: 0, unattributed_tokens: 0 }; }
    if (state === 'unknown') { row.known_cost = 0; row.coverage.priced = 0; row.complete = n === 0; }
    if (state === 'small') row.known_cost = n * .000001;
    if (state === 'zero') row.known_cost = 0;
    return row;
  }
  const breakdown = empty ? [] : reported.map(({ day, n }) => ({ day, models: keys.map(key => make(key, n)), devices: [make(device, n * 2)] }));
  return { total: make('all', empty ? 0 : sum * 2), days: empty ? [] : reported.map(({ day, n }) => make(day, n * 2)), models: empty ? [] : keys.map(key => make(key, sum)), devices: empty ? [] : [make(device, sum * 2)], agents: [agentName, 'pi'], collected_at: now.toISOString(), snapshots: empty ? 0 : dates.length, breakdown, cells: breakdown.flatMap(day => day.models.map(m => ({ ...m, key: day.day, device, model: m.key }))) };
}
const prices = [{ model: modelName, pricing_style: 'openai', base_prices: { input: 1, output: 3, cache_read: .1, cache_write: 1 }, model_multiplier: 1, conditional_multipliers: [], branches: [] }];
const server = createServer(async (req, res) => {
  try {
    assert.equal(req.method, 'GET', 'Static server accepts GET only');
    const path = new URL(req.url, 'http://localhost').pathname;
    assert.ok(!path.startsWith('/v1/'), 'Every API request must be intercepted');
    const file = path.startsWith('/assets/') ? resolve(root, '.' + path) : resolve(root, 'device.html');
    assert.ok(file.startsWith(root + '/'));
    res.setHeader('Content-Type', { '.js': 'text/javascript', '.css': 'text/css', '.html': 'text/html', '.svg': 'image/svg+xml' }[extname(file)] || 'application/octet-stream');
    res.end(await readFile(file));
  } catch (e) { res.statusCode = 500; res.end(String(e)); }
});
await new Promise(done => server.listen(0, '127.0.0.1', done));
const origin = `http://127.0.0.1:${server.address().port}`;
const browser = await chromium.launch({ headless: true, executablePath: process.env.DEVICE_TEST_CHROMIUM || undefined });
async function makePage(width = 1440, theme = 'light', collapsed = false) {
  const context = await browser.newContext({ viewport: { width, height: 1000 }, colorScheme: theme, reducedMotion: 'reduce', serviceWorkers: 'block' });
  const page = await context.newPage();
  const state = { mode: 'partial', delay: 0, requests: [], errors: [] };
  page.on('pageerror', e => state.errors.push(e.message));
  await page.clock.setFixedTime(now);
  await page.addInitScript(({ collapsed }) => {
    localStorage.setItem('keeper-dashboard-toolbar-collapsed', String(collapsed));
    window.__uiRejections = [];
    window.addEventListener('unhandledrejection', e => window.__uiRejections.push(String(e.reason)));
  }, { collapsed });
  await page.route('**/*', async route => {
    const url = new URL(route.request().url());
    if (url.origin !== origin) { report.blockedNetwork.push(url.origin + url.pathname); return route.abort(); }
    if (!url.pathname.startsWith('/v1/')) return route.continue();
    const method = route.request().method();
    if (method !== 'GET') {
      assert.ok(method === 'PUT' && url.pathname === '/v1/pricing/models', 'Only isolated synthetic pricing saves are allowed');
      const config = route.request().postDataJSON();
      assert.equal(config.model, modelName);
      prices[0] = config; report.syntheticSaves++;
      return route.fulfill({ json: { model: config.model } });
    }
    state.requests.push(url.pathname + url.search);
    const snapshot = state.mode;
    if (state.delay) await new Promise(done => setTimeout(done, state.delay));
    if (url.pathname === '/v1/session') return route.fulfill({ json: { authenticated: true } });
    if (url.pathname === '/v1/usage-boundary') return route.fulfill({ json: { earliest_day: snapshot === 'empty' ? null : first } });
    if (url.pathname === '/v1/devices') return route.fulfill({ json: devices });
    if (url.pathname === '/v1/pricing/models') return route.fulfill({ json: prices });
    if (url.pathname === '/v1/overview' || url.pathname === '/v1/analysis') return snapshot === 'error' ? route.fulfill({ status: 503, json: {} }) : route.fulfill({ json: fixture(url, snapshot) });
    throw new Error('Unexpected synthetic endpoint: ' + url.pathname);
  });
  await page.goto(origin);
  await settled(page);
  return { page, state, context };
}
async function settled(page) {
  await page.waitForFunction(() => { const refresh = document.querySelector('[data-dashboard-refresh]'); return refresh && refresh.getAttribute('aria-busy') === 'false'; });
  await page.waitForTimeout(80);
}
async function navigate(page, label) {
  await page.evaluate(() => window.scrollTo(0, 0));
  const tab = page.getByRole('tab', { name: label, exact: true });
  if (await tab.isVisible()) await tab.click();
  else { await page.locator('[data-dashboard-page-trigger]').click(); await page.getByRole('menuitem', { name: label, exact: true }).click(); }
  await settled(page); count('tab:' + label);
}
async function select(page, label, index) {
  await page.evaluate(() => window.scrollTo(0, 0));
  const trigger = page.getByRole('button', { name: label, exact: true });
  await trigger.click();
  await invariantPanels(page);
  const options = page.getByRole('option');
  await options.first().waitFor();
  const optionCount = await options.count();
  if (index >= optionCount) { count('filter:' + label + ':unavailable-in-current-data'); index = 0; }
  const option = options.nth(index);
  const text = await option.innerText();
  await option.click(); await settled(page);
  assert.ok((await trigger.innerText()).includes(text.trim()), `Selected ${label} remains current`);
  count('filter:' + label + ':' + index);
}
// Query correctness is checked against the selected labels in invariant(), not
// just HTTP response order. The fixture gives each filter a distinct token total.
const pageStates = new WeakMap();
function stateRequest(page) { return pageStates.get(page)?.requests.filter(s => /^\/v1\/(overview|analysis)\?/.test(s)).at(-1) || '/'; }
async function chooseRange(page, kind) {
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.locator('[data-time-range-trigger]:visible').click();
  await invariantPanels(page);
  const panel = page.locator('[data-device-range]');
  if (kind === 'all') await panel.locator('[data-time-range-mode="all"]').click();
  else if (kind === 'custom') {
    await panel.locator('[data-time-range-mode="custom"]').click();
    await panel.locator('[data-custom-endpoint="start"]').click();
    const month = await panel.locator('[data-custom-calendar-month]').getAttribute('data-custom-calendar-month');
    await panel.locator(`[data-custom-day="${month}-01"]`).click();
    const endMonth = await panel.locator('[data-custom-calendar-month]').getAttribute('data-custom-calendar-month');
    await panel.locator(`[data-custom-day^="${endMonth}"]:not(:disabled)`).last().click();
    await panel.locator('[data-custom-range-apply]').click();
  } else if (kind === 'days') {
    await panel.locator('[data-time-range-mode="day"]').click();
    await page.keyboard.press('Escape');
  } else {
    await panel.locator(`[data-time-range-mode="${kind}"]`).click();
    await page.keyboard.press('Escape');
  }
  await panel.waitFor({ state: 'hidden' });
  await settled(page); count('range:' + kind);
}
async function invariantPanels(page) {
  for (const el of await page.locator('[role="dialog"]:visible,[role="listbox"]:visible,[data-dashboard-page-menu]:visible').all()) {
    const box = await el.boundingBox(); const size = page.viewportSize();
    assert.ok(box && box.x >= -1 && box.y >= -1 && box.x + box.width <= size.width + 1 && box.y + box.height <= size.height + 1, `Panel outside viewport: ${JSON.stringify(box)} / ${JSON.stringify(size)}`);
  }
}
async function invariant(page, state) {
  assert.deepEqual(state.errors, [], 'No page errors');
  assert.deepEqual(await page.evaluate(() => window.__uiRejections), [], 'No unhandled rejections');
  const geometry = await page.evaluate(() => ({ width: innerWidth, scroll: document.documentElement.scrollWidth, toolbar: document.querySelector('[data-dashboard-toolbar]').getBoundingClientRect().height }));
  assert.ok(geometry.scroll <= geometry.width + 1, 'Page horizontal overflow: ' + JSON.stringify(geometry));
  assert.ok(geometry.toolbar <= (geometry.width < 600 ? 210 : 160), 'Toolbar too tall: ' + JSON.stringify(geometry));
  await invariantPanels(page);
  for (const control of await page.locator('[data-dashboard-toolbar] button:visible,[data-dashboard-toolbar] [role="combobox"]:visible').all()) {
    assert.ok((await control.getAttribute('aria-label')) || (await control.innerText()).trim(), 'Toolbar control needs an accessible name');
  }
  const request = stateRequest(page);
  if (request !== '/' && state.mode !== 'error') {
    const url = new URL('http://local' + request);
    for (const [label, param, values] of [['Device', 'device', ['', 'fixture-a', 'fixture-b']], ['Agent', 'agent', ['', agentName, 'pi']], ['Model family', 'family', ['', 'openai', 'anthropic', 'minimax', 'unknown']]]) {
      const trigger = page.getByRole('button', { name: label, exact: true });
      const selected = await trigger.getAttribute('data-selected-value');
      if (selected !== null) assert.equal(url.searchParams.get(param), selected, 'Request uses current filter');
      assert.ok(values.includes(url.searchParams.get(param)), 'Known filter value');
    }
    const tokenCard = page.locator('[data-device-stat="tokens"][data-token-total]');
    if (await tokenCard.count()) assert.equal(await tokenCard.getAttribute('data-token-total'), String(fixture(url, state.mode).total.total_tokens), 'No stale data');
  }
  count('invariant:passed');
}
async function screenshot(page, name, meta) {
  const path = resolve(output, name + '.png');
  await page.screenshot({ path, fullPage: meta.fullPage || false });
  report.screenshots.push({ path, ...meta });
}
async function pricingValidation(page, state) {
  state.mode = 'partial'; state.delay = 0;
  await page.locator('[data-dashboard-refresh]').click(); await settled(page);
  await navigate(page, 'Settings');
  await page.getByRole('button', { name: 'Edit', exact: true }).first().click();
  const dialog = page.getByRole('dialog');
  await invariantPanels(page);
  const input = dialog.locator('[data-pricing-field="base_prices.input"]');
  await input.fill('-1'); await dialog.getByRole('button', { name: 'Save', exact: true }).click();
  assert.equal(await input.getAttribute('aria-invalid'), 'true', 'Invalid price rejected');
  assert.ok(await input.evaluate(el => el === document.activeElement), 'Invalid price receives focus');
  count('pricing:invalid-negative-rejected');
  await input.fill('1.25');
  await dialog.getByRole('button', { name: 'Save', exact: true }).click();
  await dialog.waitFor({ state: 'hidden' }); await settled(page);
  assert.equal(prices[0].base_prices.input, 1.25, 'Saved only to isolated synthetic fixture');
  count('pricing:synthetic-save');
}
function random(seed) { return () => { seed |= 0; seed = seed + 0x6D2B79F5 | 0; let t = Math.imul(seed ^ seed >>> 15, 1 | seed); t = t + Math.imul(t ^ t >>> 7, 61 | t) ^ t; return ((t ^ t >>> 14) >>> 0) / 4294967296; }; }
try {
  if (mode !== 'fuzz') {
    for (const theme of ['light', 'dark']) for (const width of widths) for (const collapsed of [false, true]) {
      const { page, state, context } = await makePage(width, theme, collapsed); pageStates.set(page, state);
      await select(page, 'Device', 1); await select(page, 'Agent', 1); await select(page, 'Model family', 4);
      for (const range of ['7d', 'all']) {
        if (range === 'all') await chooseRange(page, 'all');
        const meta = { theme, width, collapsed, range, tab: 'Overview' };
        await screenshot(page, `overview-${theme}-${width}-${collapsed ? 'collapsed' : 'expanded'}-${range}`, meta);
        // Baseline deliberately records failures without stopping the comparison.
        try { await invariant(page, state); } catch (e) { report.failures.push({ ...meta, error: e.message }); }
      }
      await context.close();
    }
    for (const theme of ['light', 'dark']) {
      const { page, state, context } = await makePage(390, theme); pageStates.set(page, state);
      for (const tab of ['Analysis', 'Daily history', 'Devices', 'Settings']) {
        await navigate(page, tab); await screenshot(page, `${tab.replaceAll(' ', '-')}-${theme}-390`, { theme, width: 390, tab });
        try { await invariant(page, state); } catch (e) { report.failures.push({ tab, theme, error: e.message }); }
      }
      await context.close();
    }
    if (phase !== 'baseline') for (const theme of ['light', 'dark']) for (const width of [390, 1024]) {
      const { page, state, context } = await makePage(width, theme); pageStates.set(page, state);
      await chooseRange(page, 'all');
      for (const data of ['partial', 'complete', 'unknown', 'zero', 'small', 'empty', 'error']) {
        state.mode = data;
        await page.locator('[data-dashboard-refresh]').click(); await settled(page);
        for (const tab of ['Overview', 'Analysis', 'Daily history', 'Devices', 'Settings']) {
          await navigate(page, tab);
          const meta = { theme, width, tab, data, range: 'all' };
          await screenshot(page, `state-${data}-${tab.replaceAll(' ', '-')}-${theme}-${width}`, meta);
          await invariant(page, state); count(`matrix:${data}:${tab}:${theme}:${width}`);
          if (tab === 'Analysis' && !['empty', 'error'].includes(data)) {
            await page.getByRole('heading', { name: 'Device × Model', exact: true }).scrollIntoViewIfNeeded();
            await page.evaluate(() => window.scrollBy(0, -document.querySelector('[data-dashboard-toolbar]').getBoundingClientRect().height - 24));
            await screenshot(page, `matrix-${data}-${theme}-${width}`, { ...meta, section: 'Device × Model' });
            const grid = page.getByRole('region', { name: 'Device by model usage, horizontally scrollable' });
            await grid.focus(); await page.keyboard.press('ArrowRight'); await page.waitForTimeout(120);
            assert.ok(await grid.evaluate(el => el.scrollWidth <= el.clientWidth || el.scrollLeft > 0), 'Matrix scrolls with the keyboard');
            count('matrix:keyboard-scroll-verified');
            await grid.evaluate(el => { el.scrollLeft = el.scrollWidth; });
            assert.ok(await grid.evaluate(el => el.scrollWidth <= el.clientWidth || el.scrollLeft > 0), 'Matrix totals reachable');
            const label = grid.locator('[class*="heatmapKeyColumn"]').first();
            assert.ok(await label.evaluate(el => Math.abs(el.getBoundingClientRect().left - el.closest('[role="region"]').getBoundingClientRect().left) <= 1), 'Matrix labels remain sticky while scrolling');
            count('matrix:sticky-labels-verified');
            await screenshot(page, `matrix-totals-${data}-${theme}-${width}`, { ...meta, section: 'Device × Model scrolled to totals' });
            await page.evaluate(() => window.scrollTo(0, 0));
          }
        }
      }
      state.mode = 'partial'; await page.locator('[data-dashboard-refresh]').click(); await settled(page);
      await navigate(page, 'Overview');
      await page.locator('[data-device-coverage]').evaluate(el => { el.open = true; });
      await page.locator('[data-device-coverage]').scrollIntoViewIfNeeded();
      await screenshot(page, `coverage-${theme}-${width}`, { theme, width, tab: 'Overview', section: 'Gap coverage' });
      await navigate(page, 'Overview');
      if (width === 390) {
        await page.locator('[data-dashboard-page-trigger]').click();
        await screenshot(page, `nav-open-${theme}-${width}`, { theme, width, section: 'All five destinations' });
        await invariantPanels(page); await page.keyboard.press('Escape');
      }
      await page.locator('[data-time-range-trigger]:visible').click();
      await screenshot(page, `range-open-${theme}-${width}`, { theme, width, section: 'All-time dates in range menu' });
      await invariantPanels(page); await page.keyboard.press('Escape');
      await select(page, 'Device', 1);
      await page.getByRole('button', { name: 'Device', exact: true }).click();
      await screenshot(page, `filter-open-${theme}-${width}`, { theme, width, section: 'Long device options' });
      await invariantPanels(page); await page.keyboard.press('Escape');
      await page.getByRole('button', { name: 'Reset filters', exact: true }).click(); await settled(page);
      assert.equal(await page.getByRole('button', { name: 'Device', exact: true }).getAttribute('data-selected-value'), ''); count('filters:reset');
      await pricingValidation(page, state);
      await screenshot(page, `pricing-saved-${theme}-${width}`, { theme, width, section: 'Settings after synthetic save' });
      await context.close();
    }
    const { page, state, context } = await makePage(1440); pageStates.set(page, state);
    await page.evaluate(() => { document.documentElement.style.zoom = '2'; }); await page.setViewportSize({ width: 1440, height: 1000 });
    await screenshot(page, 'zoom-200-light-1440', { zoom: 'CSS 200%', width: 1440 });
    await context.close();
  }
  if (mode !== 'screens') for (const seed of seeds) {
    const rng = random(seed), pick = a => a[Math.floor(rng() * a.length)];
    const { page, state, context } = await makePage(); pageStates.set(page, state);
    const sequence = []; const entry = { seed, planned: actions, completed: 0, sequence, failures: [] }; report.seeds.push(entry);
    // Each seed has a guaranteed prefix. Randomness explores transitions, not
    // whether the key states happen to be covered at all.
    const prefix = [ ...['Overview', 'Analysis', 'Daily history', 'Devices', 'Settings'].map(value => ({ kind: 'tab', value })), ...['all', 'custom', 'today', 'yesterday', 'days'].map(value => ({ kind: 'range', value })), ...[['Device', 3], ['Agent', 3], ['Model family', 5]].flatMap(([value, length]) => Array.from({ length }, (_, index) => ({ kind: 'filter', value, index }))), ...['loading', 'error', 'empty', 'partial', 'complete', 'unknown', 'zero', 'small', 'race'].map(value => ({ kind: 'data', value })), { kind: 'keyboard' }, { kind: 'pricing-dialog' }, { kind: 'pricing-validation' }, ...widths.map(value => ({ kind: 'resize', value })), { kind: 'theme', value: 'dark' }, { kind: 'theme', value: 'light' } ];
    for (let index = 0; index < actions; index++) {
      const action = prefix[index] || pick([{ kind: 'tab', value: pick(['Overview', 'Analysis', 'Daily history', 'Devices', 'Settings']) }, { kind: 'range', value: pick(['all', 'custom', 'today', 'yesterday', 'days']) }, { kind: 'filter', value: pick(['Device', 'Agent', 'Model family']), index: Math.floor(rng() * 3) }, { kind: 'resize', value: pick(widths) }, { kind: 'theme', value: pick(['light', 'dark']) }, { kind: 'collapse' }, { kind: 'keyboard' }, { kind: 'data', value: pick(['loading', 'error', 'empty', 'partial', 'complete', 'unknown', 'zero', 'small', 'race']) }, { kind: 'pricing-dialog' }]);
      sequence.push(action);
      try {
        await page.keyboard.press('Escape');
        if (action.kind === 'tab') await navigate(page, action.value);
        if (action.kind === 'range') await chooseRange(page, action.value);
        if (action.kind === 'filter') await select(page, action.value, action.index);
        if (action.kind === 'resize') { await page.setViewportSize({ width: action.value, height: 1000 }); await page.waitForTimeout(100); count('width:' + action.value); }
        if (action.kind === 'theme') { await page.emulateMedia({ colorScheme: action.value }); count('theme:' + action.value); }
        if (action.kind === 'collapse') { const button = page.locator('[data-dashboard-collapse]'); if (await button.isVisible()) { await button.click(); await page.waitForTimeout(100); count('collapse:toggled'); } else count('collapse:responsive-hidden'); }
        if (action.kind === 'keyboard') {
          const trigger = page.getByRole('button', { name: 'Device', exact: true });
          await trigger.focus(); await page.keyboard.press('ArrowDown'); await invariantPanels(page); await page.keyboard.press('Escape');
          assert.ok(await trigger.evaluate(el => el === document.activeElement), 'Escape restores filter focus');
          await page.keyboard.press('Tab'); count('keyboard:filter-escape-tab');
        }
        if (action.kind === 'pricing-validation') await pricingValidation(page, state);
        if (action.kind === 'pricing-dialog') {
          await navigate(page, 'Settings'); await page.getByRole('button', { name: /Sync Prices/i }).click();
          await invariantPanels(page);
          const dialog = page.getByRole('dialog'); assert.ok(await dialog.evaluate(el => el.contains(document.activeElement)), 'Dialog contains focus');
          await page.keyboard.press('Escape'); await dialog.waitFor({ state: 'hidden' }); count('dialog:pricing-open-close');
        }
        if (action.kind === 'data') {
          await navigate(page, 'Overview');
          state.mode = ['loading', 'race'].includes(action.value) ? 'partial' : action.value;
          state.delay = action.value === 'loading' || action.value === 'race' ? 300 : 0;
          await page.locator('[data-dashboard-refresh]').click();
          if (state.delay) { await page.locator('[data-dashboard-refresh][aria-busy="true"]').waitFor(); count('loading:observed'); }
          if (action.value === 'race') {
            // Supersede the slow Overview response before it resolves.
            await navigate(page, 'Analysis'); count('race:superseded-overview');
          }
          await settled(page); state.delay = 0;
          if (state.mode === 'error') { await page.getByRole('alert').waitFor(); count('error:observed'); }
          count('data:' + action.value);
        }
        await settled(page); await invariant(page, state); entry.completed++;
      } catch (e) {
        const path = resolve(output, `failure-${seed}-${index}.png`); await page.screenshot({ path, fullPage: true }).catch(() => {});
        const failure = { index, action, error: e.stack, screenshot: path }; entry.failures.push(failure); report.failures.push({ seed, ...failure });
        // A seed stops at its first failure; replay uses the same prefix/PRNG.
        break;
      }
    }
    await writeFile(resolve(output, `seed-${seed}.json`), JSON.stringify(entry, null, 2));
    await context.close();
  }
} catch (e) { report.failures.push({ fatal: e.stack }); }
finally {
  await writeFile(resolve(output, 'report.json'), JSON.stringify(report, null, 2));
  await browser.close(); await new Promise(done => server.close(done));
}
console.log(JSON.stringify({ phase, output, screenshots: report.screenshots.length, seeds: report.seeds.map(({ seed, completed, planned, failures }) => ({ seed, completed, planned, failures: failures.length })), failures: report.failures.length, coverage: report.coverage }, null, 2));
if (report.failures.length && phase !== 'baseline') process.exitCode = 1;
