import { createRequire } from 'node:module';
import assert from 'node:assert/strict';
const { chromium } = createRequire(import.meta.url)(process.env.DEVICE_TEST_PLAYWRIGHT);
const browser = await chromium.launch({headless:true,executablePath:process.env.DEVICE_TEST_CHROMIUM});
const aggregate = key => ({key,tokens:{input:1,output:0,cache_read:0,cache_write:0},total_tokens:1,known_cost:0,complete:true});
const fixture = (dimension, count, name='fresh-model') => {
  const models = Array.from({length:dimension==='models'?count:1},(_,i)=>aggregate(count===1?name:`model-${i}`));
  const devices = Array.from({length:dimension==='devices'?count:1},(_,i)=>aggregate(`device-${i}`));
  return {total:aggregate('all'),models,devices,days:[aggregate('2026-11-09')],breakdown:[{day:'2026-11-09',models,devices}],cells:[],agents:['pi','codex'],snapshots:1,collected_at:'2026-11-09T12:00:00Z'};
};
async function navigate(page,label) {
  const tab=page.getByRole('tab',{name:label,exact:true});
  if(await tab.isVisible()) await tab.click();
  else {await page.locator('[data-dashboard-page-trigger]').click();await page.getByRole('menuitem',{name:label,exact:true}).click();}
}
async function select(page,label,value) {await page.getByLabel(label,{exact:true}).click();await page.getByRole('option',{name:value,exact:true}).click();}
try {
 for(const dimension of ['models','devices']) for(const failure of ['server','adapter']) {
  const page=await browser.newPage();const errors=[];page.on('pageerror',error=>errors.push(error.message));
  await page.clock.install({time:new Date('2026-11-09T12:00:00Z')});
  let mode='limit',held;
  const waitHeld=async()=>{for(let i=0;i<300&&!held;i++)await page.waitForTimeout(10);assert.ok(held,'delayed request arrived');};
  await page.route('**/v1/**',async route=>{
   const url=new URL(route.request().url());
   if(url.pathname==='/v1/session')return route.fulfill({json:{authenticated:true}});
   if(url.pathname==='/v1/devices')return route.fulfill({json:[{id:'device-0',name:'Laptop',revoked:false}]});
   if(url.pathname==='/v1/pricing/models')return route.fulfill({json:[]});
   if(url.pathname==='/v1/overview')return route.fulfill({json:fixture(dimension,101,'overview-model')});
   if(url.pathname==='/v1/analysis') {
    if(mode==='hold') {held=route;return;}
    if(mode==='limit'&&!url.searchParams.get('device'))return failure==='server'?route.fulfill({status:503,json:{error:'narrow the range'}}):route.fulfill({json:fixture(dimension,101)});
    return route.fulfill({json:fixture(dimension,1,url.searchParams.get('agent')==='codex'?'latest-model':'fresh-model')});
   }
   throw Error('Unexpected request '+url.pathname);
  });
  await page.goto(process.env.DEVICE_TEST_URL);
  await page.getByRole('heading',{name:'Models',exact:true}).waitFor();
  await page.getByRole('region',{name:'Model usage, horizontally scrollable'}).getByRole('row').nth(1).waitFor();
  await navigate(page,'Analysis');
  const alert=page.getByRole('alert');await alert.waitFor();
  assert.match(await alert.innerText(),failure==='server'?/503.*narrow/i:/100 models or devices/i);
  assert.equal(await page.getByRole('region',{name:'Token and cost breakdown',exact:true}).count(),0);
  assert.equal(await page.locator('main').count(),1);
  await select(page,'Device','Laptop');
  const table=page.getByRole('region',{name:'Token and cost breakdown',exact:true});
  await table.getByText('fresh-model',{exact:true}).waitFor();assert.equal(await alert.count(),0);
  // A delayed Analysis response must not populate a newly selected Overview.
  mode='hold';await select(page,'Agent','pi');
  await page.waitForFunction(()=>!document.querySelector('[aria-label="Token and cost breakdown"]'));
  await waitHeld();
  const old=held;held=undefined;
  await navigate(page,'Overview');
  await page.getByRole('region',{name:'Model usage, horizontally scrollable'}).getByRole('row').nth(1).waitFor();
  await old.fulfill({json:fixture(dimension,1,'stale-tab-model')});
  mode='success';await navigate(page,'Analysis');await table.getByText('fresh-model',{exact:true}).waitFor();
  // A delayed old filter response must not replace the newest filter result.
  mode='hold';await select(page,'Agent','All agents');
  await waitHeld();
  const oldFilter=held;mode='success';
  await select(page,'Agent','codex');await table.getByText('latest-model',{exact:true}).waitFor();
  await oldFilter.fulfill({json:fixture(dimension,1,'stale-filter-model')});
  await page.waitForTimeout(150);
  assert.doesNotMatch(await page.locator('main').innerText(),/stale-tab-model|stale-filter-model/);
  await table.getByText('latest-model',{exact:true}).waitFor();assert.deepEqual(errors,[]);
  console.log(`PASS ${dimension}=101: ${failure} limit, recovery, delayed tab and filter responses`);
  await page.close();
 }
} finally {await browser.close();}
