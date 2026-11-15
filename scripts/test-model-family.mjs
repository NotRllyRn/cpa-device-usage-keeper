// Real local device API, synthetic uploads; no collector logs or production state.
import { createRequire } from 'node:module';
import { mkdir } from 'node:fs/promises';
import assert from 'node:assert/strict';
const { chromium } = createRequire(import.meta.url)(process.env.DEVICE_TEST_PLAYWRIGHT);
const output = process.env.DEVICE_SCREENSHOTS || '/tmp/device-model-family';
await mkdir(output, { recursive: true });
const browser = await chromium.launch({ headless: true, executablePath: process.env.DEVICE_TEST_CHROMIUM });
const day = offset => { const d = new Date(); d.setUTCDate(d.getUTCDate() + offset); return d.toISOString().slice(0, 10); };
async function select(page, label, value) {
  await page.getByLabel(label, { exact: true }).click();
  await page.getByRole('option', { name: value, exact: true }).click();
}
async function navigate(page, label) {
  const tab = page.getByRole('tab', { name: label, exact: true });
  if (await tab.isVisible()) await tab.click();
  else { await page.locator('[data-dashboard-page-trigger]').click(); await page.getByRole('menuitem', { name: label, exact: true }).click(); }
}
async function settled(page) {
  await page.waitForFunction(() => document.querySelector('main')?.textContent.includes('Refresh to include new uploads.'));
}
async function shot(page, name) {
  await settled(page);
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.waitForTimeout(350);
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true, 'no page overflow');
  await page.screenshot({ path: `${output}/${name}.png`, fullPage: true });
}
try {
  for (const theme of ['light', 'dark']) {
    const page = await browser.newPage({ viewport: { width: 1440, height: 1000 }, colorScheme: theme, reducedMotion: 'reduce' });
    const errors = [], requests = [];
    page.on('pageerror', e => errors.push(e.message));
    page.on('request', req => { if (/\/v1\/(analysis|overview|usage-boundary)\?/.test(req.url())) requests.push(new URL(req.url())); });
    await page.goto(process.env.DEVICE_TEST_URL);
    await page.getByLabel('Admin token').fill(process.env.DEVICE_TEST_TOKEN);
    await page.getByRole('button', { name: 'Connect', exact: true }).click();
    await settled(page);
    await select(page, 'Model family', 'OpenAI / Codex'); await settled(page);
    const modelTable = page.getByRole('region', { name: 'Model usage, horizontally scrollable', exact: true });
    assert.match(await modelTable.innerText(), /\[pi\] gpt-5\.4/);
    assert.match(await modelTable.innerText(), /openai\/o3-mini/);
    assert.doesNotMatch(await modelTable.innerText(), /claude|MiniMax|mystery|Unattributed/);
    assert.equal(await modelTable.getByRole('row').count(), 4, 'Pi, Hermes, custom agent models');
    for (const width of [1440, 390]) { await page.setViewportSize({ width, height: 1000 }); await shot(page, `device-${theme}-${width}-overview-openai`); }
    await navigate(page, 'Analysis'); await settled(page);
    const table = page.getByRole('region', { name: 'Token and cost breakdown', exact: true });
    assert.match(await page.locator('main').innerText(), /Total Tokens — range sum\s+5,800/);
    assert.doesNotMatch(await table.innerText(), /claude|MiniMax|mystery|Unattributed \(no model\)/);
    for (const width of [390, 1440]) { await page.setViewportSize({ width, height: 1000 }); await shot(page, `device-${theme}-${width}-analysis-openai`); }
    await select(page, 'Agent', 'pi'); await settled(page);
    assert.match(await table.innerText(), /\[pi\] gpt-5\.4/);
    assert.doesNotMatch(await table.innerText(), /openai\/o3-mini/);
    assert.match(await page.locator('main').innerText(), /Total Tokens — range sum\s+1,000/);
    await select(page, 'Agent', 'All agents'); await settled(page);
    await select(page, 'Model family', 'Anthropic / Claude'); await settled(page);
    assert.match(await page.locator('main').innerText(), /Total Tokens — range sum\s+7,000/);
    assert.doesNotMatch(await table.innerText(), /gpt|MiniMax/);
    await select(page, 'Model family', 'MiniMax'); await settled(page);
    assert.match(await page.locator('main').innerText(), /Total Tokens — range sum\s+9,000/);
    await shot(page, `device-${theme}-analysis-minimax`);
    // All time must first find the selected family's earliest MATCHING upload.
    await page.locator('[data-time-range-trigger]:visible').click();
    await page.locator('[data-time-range-mode="all"]').click(); await settled(page);
    assert.equal(requests.filter(u => u.pathname === '/v1/usage-boundary').at(-1).searchParams.get('family'), 'minimax');
    assert.equal(requests.filter(u => u.pathname === '/v1/analysis').at(-1).searchParams.get('start'), day(-2));
    await select(page, 'Model family', 'Unknown / unattributed'); await settled(page);
    assert.match(await page.locator('main').innerText(), /Total Tokens — range sum\s+2,100/);
    assert.match(await table.innerText(), /mystery/); assert.match(await table.innerText(), /Unattributed \(no model\)/);
    assert.doesNotMatch(await table.innerText(), /gpt|claude|MiniMax/);
    assert.equal(requests.filter(u => u.pathname === '/v1/analysis').at(-1).searchParams.get('start'), day(-30));
    await shot(page, `device-${theme}-analysis-unknown`);
    await select(page, 'Model family', 'All model families'); await settled(page);
    assert.match(await page.locator('main').innerText(), /Total Tokens — range sum\s+23,900/);
    // Hold a real old response. A new family must invalidate it immediately,
    // including an in-flight All time boundary, not just the chart response.
    for (const endpoint of ['analysis', 'usage-boundary']) {
      let held, response;
      await page.route(`**/v1/${endpoint}?*`, async route => {
        if (new URL(route.request().url()).searchParams.get('family') !== 'openai') return route.continue();
        response = await route.fetch(); held = route;
      });
      await select(page, 'Model family', 'OpenAI / Codex');
      await page.waitForFunction(() => !document.querySelector('[aria-label="Token and cost breakdown"]'));
      for (let i = 0; i < 300 && !held; i++) await page.waitForTimeout(10);
      assert.ok(held, `held ${endpoint}`);
      await select(page, 'Model family', 'MiniMax'); await settled(page);
      assert.match(await page.locator('main').innerText(), /Total Tokens — range sum\s+9,000/);
      await held.fulfill({ response }); await page.waitForTimeout(150);
      assert.doesNotMatch(await table.innerText(), /gpt|openai/);
      assert.match(await page.locator('main').innerText(), /Total Tokens — range sum\s+9,000/);
      await page.unroute(`**/v1/${endpoint}?*`);
    }
    // Family choice survives tab navigation, and sessions still survive reload.
    await navigate(page, 'Overview'); await settled(page);
    assert.match(await modelTable.innerText(), /MiniMax/); assert.doesNotMatch(await modelTable.innerText(), /gpt|claude/);
    const session = (await page.context().cookies()).find(c => c.name === 'device_session_local');
    assert.ok(session?.httpOnly && session.sameSite === 'Strict');
    await page.reload(); await settled(page);
    assert.equal(await page.getByLabel('Admin token').count(), 0);
    assert.deepEqual(errors, []);
    await page.close();
  }
  console.log(`PASS: real API cross-agent families, model/day/device totals, independent agent, matching All time, unknown remainder, delayed family responses, cookie reload, light/dark desktop/mobile. Screenshots: ${output}`);
} finally { await browser.close(); }
