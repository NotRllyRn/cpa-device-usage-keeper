import { createRequire } from 'node:module';
import assert from 'node:assert/strict';
const require = createRequire(import.meta.url);
const { chromium } = require(process.env.DEVICE_TEST_PLAYWRIGHT);
const browser = await chromium.launch({ headless: true, executablePath: process.env.DEVICE_TEST_CHROMIUM || undefined });
try {
  const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.goto(process.env.DEVICE_TEST_URL);
  await page.getByLabel('Admin token').fill(process.env.DEVICE_TEST_TOKEN);
  await page.getByRole('button', { name: 'Connect', exact: true }).click();
  await page.getByText('130', { exact: true }).first().waitFor();
  assert.match(await page.locator('body').innerText(), /Known/);
  assert.match(await page.locator('body').innerText(), /Unavailable/);
  // Keeper toolbar starts collapsed. Open its page selector to navigate.
  async function navigate(label) {
    const direct = page.getByRole('tab', { name: label, exact: true });
    if (await direct.isVisible().catch(() => false)) { await direct.click(); return; }
    await page.locator('[data-dashboard-page-trigger]').click();
    await page.getByRole('menuitem', { name: label, exact: true }).click();
  }
  await navigate('Settings');
  await page.getByRole('heading', { name: /Model Pric/i }).waitFor();
  assert.equal(await page.getByRole('button', { name: /Sync Prices/i }).isDisabled(), false);
  await page.getByRole('button', {name:'Add',exact:true}).click();
  const dialog=page.getByRole('dialog');
  await dialog.waitFor();
  const inputs=dialog.locator('input[type="number"]');
  assert.equal(await inputs.count(),5);
  for(let i=0;i<5;i++) await inputs.nth(i).fill('1');
  await dialog.getByRole('button',{name:'Save',exact:true}).click();
  await dialog.waitFor({state:'hidden'});
  await navigate('Overview');
  await page.getByText('$0.000130',{exact:true}).first().waitFor();
  await navigate('Settings');
  await page.getByRole('button',{name:/Sync Prices/i}).click();
  await page.getByRole('dialog',{name:'Sync Prices',exact:true}).waitFor();
  await page.route('**/v1/pricing/sync?*', route => route.fulfill({json:{source:'models-dev',matches:[{model:'model-test',matched_model:'model-test',provider:'Synthetic',pricing_style:'openai',base_prices:{input:2,output:2,cache_read:2,cache_write:0}}],unmatched_models:[]}}));
  await page.getByRole('button',{name:'Fetch',exact:true}).click();
  const row = page.locator('[data-sync-model="model-test"]');
  await row.waitFor();
  await row.getByRole('checkbox').uncheck();
  assert.equal(await page.getByRole('button',{name:/Apply selected prices/}).isDisabled(),true);
  await row.getByRole('checkbox').check();
  await page.getByRole('button',{name:/Apply selected prices/}).click();
  await page.getByRole('dialog').waitFor({state:'hidden'});
  await navigate('Overview');
  await page.getByText('$0.000260',{exact:true}).first().waitFor();
  await navigate('Devices');
  await page.getByRole('cell', { name: 'test-laptop', exact: true }).waitFor();
  await page.getByLabel('Device name', { exact: true }).fill('test-desktop');
  await page.getByRole('button', { name: 'Enroll', exact: true }).click();
  await page.getByRole('cell', { name: 'test-desktop', exact: true }).waitFor();
  assert.match(await page.locator('output').innerText(), /^[a-f0-9]{64}$/);
  await page.getByRole('button', { name: 'Dismiss token' }).click();
  await page.getByRole('button', { name: /more actions/i }).click();
  await page.getByRole('menuitem', { name: /logout|log out|sign out/i }).click();
  await page.getByRole('button', { name: 'Connect', exact: true }).waitFor();
  assert.equal(await page.locator('output').count(), 0);
  assert.deepEqual(errors, []);
  console.log('PASS: browser login, aggregate totals, disabled metrics, Keeper pricing/settings, enrollment, logout');
} finally { await browser.close(); }
