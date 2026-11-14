// Isolated, synthetic comparison of the two compiled entry points. No live
// pricing requests or usage fixtures are read. Run through device TestBrowser.
import { createRequire } from 'node:module';
import { createServer } from 'node:http';
import { readFile, mkdir } from 'node:fs/promises';
import { resolve, extname } from 'node:path';
import assert from 'node:assert/strict';
const require = createRequire(import.meta.url);
const { chromium } = require(process.env.DEVICE_TEST_PLAYWRIGHT);
const root = resolve(import.meta.dirname, '../web/dist');
const output = process.env.DEVICE_SCREENSHOTS || '/tmp/device-ui-parity';
await mkdir(output, { recursive: true });
const server = createServer(async (req, res) => {
  const path = new URL(req.url, 'http://localhost').pathname;
  try {
    const file = path.startsWith('/assets/') ? resolve(root, '.' + path) : resolve(root, 'index.html');
    if (!file.startsWith(root + '/')) throw new Error('invalid path');
    res.setHeader('Content-Type', {'.js':'text/javascript','.css':'text/css','.html':'text/html','.svg':'image/svg+xml'}[extname(file)] || 'application/octet-stream');
    res.end(await readFile(file));
  } catch { res.statusCode = 404; res.end(); }
});
await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
const upstreamURL = `http://127.0.0.1:${server.address().port}`;
const day = i => `2026-11-${String(i + 3).padStart(2,'0')}`;
const aggregate = (key, n) => ({ key, tokens:{input:n*6,output:n*2,cache_read:n*2,cache_write:0}, total_tokens:n*10, known_cost:n*.00002,complete:true });
const sum = (key, rows) => rows.reduce((a,b) => ({key,tokens:Object.fromEntries(Object.keys(a.tokens).map(k=>[k,a.tokens[k]+b.tokens[k]])),total_tokens:a.total_tokens+b.total_tokens,known_cost:a.known_cost+b.known_cost,complete:true}),aggregate(key,0));
const breakdown = Array.from({length:7},(_,i)=>({day:day(i),models:[aggregate('gpt-example',30+i*9),aggregate('claude-example',20+i*6)],devices:[aggregate('synthetic-device',50+i*15)]}));
const days = breakdown.map(row=>sum(row.day,row.models));
const models = ['gpt-example','claude-example'].map((key,index)=>sum(key,breakdown.map(row=>row.models[index])));
const total = sum('all',days);
const overview = {total,days,models,devices:[{...total,key:'synthetic-device'}],agents:['pi'],collected_at:'2026-11-09T12:00:00Z',snapshots:7,breakdown};
const prices = models.map(row=>({model:row.key,pricing_style:'openai',base_prices:{input:1,output:3,cache_read:.1,cache_write:0},model_multiplier:1,conditional_multipliers:[],branches:[]}));
const preview = {source:'models-dev',matches:prices.map(p=>({model:p.model,matched_model:p.model,provider:'Synthetic catalog',pricing_style:'openai',base_prices:{input:2,output:4,cache_read:.2,cache_write:0}})),unmatched_models:['unknown-example']};
const comparisons = {buckets:days.map(row=>row.key),granularity:'daily',timezone:'UTC',models:models.map((row,index)=>({key:row.key,label:row.key,requests:0,failures:0,input_tokens:row.tokens.input,output_tokens:row.tokens.output,cache_read_tokens:row.tokens.cache_read,cache_creation_tokens:0,reasoning_tokens:0,total_tokens:row.total_tokens,cost:row.known_cost,token_series:breakdown.map(day=>day.models[index].total_tokens)})),api_keys:[],credentials:[],ai_providers:[]};
const tokenFields = row=>({input_tokens:row.tokens.input,output_tokens:row.tokens.output,cache_read_tokens:row.tokens.cache_read,cache_creation_tokens:0,reasoning_tokens:0,total_tokens:row.total_tokens});
const activity = {window:'month',grain:'daily',timezone:'UTC',total_success:0,total_failure:0,success_rate:0,...tokenFields(total),rows:1,columns:7,bucket_seconds:86400,window_start:day(0)+'T00:00:00Z',window_end:day(6)+'T23:59:59Z',blocks:days.map(row=>({...tokenFields(row),start_time:row.key+'T00:00:00Z',end_time:row.key+'T23:59:59Z',success:0,failure:0,rate:0}))};
const upstreamOverview = {usage:{total_requests:0,success_count:0,failure_count:0,total_tokens:total.total_tokens},timezone:'UTC',summary:{rpm:0,tpm:0,...tokenFields(total),total_cost:total.known_cost,cost_available:true,daily_average_requests:0,daily_average_tokens:total.total_tokens/7,daily_average_cost:total.known_cost/7,daily_average_range_days:7}};
async function navigate(page,label) {
  const tab=page.getByRole('tab',{name:label,exact:true});
  if(await tab.isVisible().catch(()=>false)) return tab.click();
  await page.locator('[data-dashboard-page-trigger]').click();
  await page.getByRole('menuitem',{name:label,exact:true}).click();
}
const browser = await chromium.launch({headless:true,executablePath:process.env.DEVICE_TEST_CHROMIUM});
try {
 for(const theme of ['light','dark']) for(const target of ['upstream','device']) {
  const page=await browser.newPage({viewport:{width:1440,height:1000},colorScheme:theme});
  let boundaryDay = '2024-01-01';
  let boundaryEmpty = false;
  const errors=[]; page.on('pageerror',error=>errors.push(error.message));
  await page.clock.install({time:new Date('2026-11-09T12:00:00Z')});
  await page.addInitScript(() => localStorage.setItem('cli-proxy-usage-time-range-v1', '7d'));
  await page.route('**/api/v1/**',async route=>{
   const path=new URL(route.request().url()).pathname.replace('/api/v1','');
   const data={
    '/startup/status':{phase:'ready',message:'',progress:null},
    '/auth/session':{authenticated:true,role:'admin'}, '/usage/overview':upstreamOverview,
    '/usage/overview/comparisons':comparisons,'/usage/activity':activity,
    '/pricing/models':{models:prices,config_revision:0},'/pricing/model-options':{models:models.map(row=>row.key)},
    '/pricing/sync/fetch':preview,'/pricing/recalculations/current':{task:null},
    '/status':{timezone:'UTC',running:true},'/version':{version:'synthetic'},
    '/cpa/api-keys/options':{items:[]},'/cpa/api-keys':{items:[]},'/auth/sessions':{items:[]},
   }[path] ?? {};
   await route.fulfill({json:data});
  });
  await page.route('**/v1/**',async route=>{
   if(route.request().url().includes('/api/v1/')) return route.fallback();
   const url=new URL(route.request().url());
   const path=url.pathname;
   if(path==='/v1/usage-boundary') {
     assert.equal(url.searchParams.has('start'),false);
     return route.fulfill({json:{earliest_day:boundaryEmpty ? null : url.searchParams.get('device') ? '2025-01-01' : url.searchParams.get('agent') ? '2024-06-01' : boundaryDay}});
   }
   if(path==='/v1/overview' && boundaryEmpty) return route.fulfill({json:{...overview,total:aggregate('all',0),days:null,models:null,devices:null,agents:null,breakdown:null,snapshots:0}});
   const data={'/v1/overview':overview,'/v1/devices':[{id:'synthetic-device',name:'Example laptop',revoked:false,last_seen:'2026-11-09T12:00:00Z'}],'/v1/pricing/models':prices,'/v1/pricing/sync':preview}[path];
   if(data) return route.fulfill({json:data});
   return route.fallback();
  });
  await page.goto(target==='upstream'?upstreamURL:process.env.DEVICE_TEST_URL);
  if(target==='device') {
   await page.getByLabel('Admin token').fill('synthetic-only');
   await page.getByRole('button',{name:'Connect',exact:true}).click();
  }
  await page.locator('[data-comparison="models"]').waitFor().catch(async error => { console.log(target, await page.locator('body').innerText(), errors); await page.screenshot({path:`${output}/failure.png`}); throw error; });
  await page.waitForTimeout(1200);
  await page.screenshot({path:`${output}/${target}-${theme}-overview.png`,fullPage:true});
  await page.locator('[data-comparison="models"]').locator('..').locator('..').screenshot({path:`${output}/${target}-${theme}-distribution.png`});
  const entry=page.locator('[data-comparison-entry]').first();
  await entry.click(); assert.equal(await entry.getAttribute('aria-pressed'),'false'); await entry.click();
  if(target==='device') {
    assert.equal(await page.locator('[data-dimension="credentials"]').count(),0);
    await page.getByRole('button',{name:'Devices',exact:true}).click();
    await page.locator('[data-comparison-entry="synthetic-device"]').waitFor();
    await page.getByRole('button',{name:'Models',exact:true}).click();
  }
  await page.locator('[data-time-range-trigger="desktop"]').click();
  if(target==='device') assert.equal(await page.locator('[data-time-range-mode="hour"]').count(),0);
  await page.locator('[data-time-range-mode="day"]').click();
  await page.screenshot({path:`${output}/${target}-${theme}-days.png`});
  await page.locator('[data-time-range-mode="custom"]').click();
  await page.screenshot({path:`${output}/${target}-${theme}-range.png`});
  await page.locator('[data-custom-endpoint="start"]').click();
  await page.screenshot({path:`${output}/${target}-${theme}-calendar.png`});
  await page.keyboard.press('Escape'); await page.mouse.click(1300,950);
  await navigate(page,target==='upstream'?'Models':'Settings');
  await page.getByRole('button',{name:'Sync Prices',exact:true}).click();
  await page.getByRole('dialog',{name:'Sync Prices',exact:true}).waitFor();
  await page.getByRole('button',{name:'Fetch',exact:true}).click();
  await page.locator('[data-sync-model]').first().waitFor();
  await page.screenshot({path:`${output}/${target}-${theme}-sync.png`});
  if(target==='device') {
    await page.getByRole('dialog').getByRole('button',{name:'Cancel',exact:true}).click();
    await navigate(page,'Overview');
    assert.equal(await page.locator('[data-activity-index]').count(),7);
    const firstDay = page.locator('[data-activity-index="0"]');
    await firstDay.focus(); await page.keyboard.press('ArrowRight');
    assert.equal(await page.locator('[data-activity-index="1"]').evaluate(el=>el===document.activeElement),true);
    await page.keyboard.press('Escape');
    await page.locator('[data-dashboard-refresh]').focus();
    for(const width of [390,768]) {
      await page.setViewportSize({width,height:1000});
      await page.waitForTimeout(400);
      assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth <= window.innerWidth),true,`overflow at ${width}`);
      await page.screenshot({path:`${output}/device-${theme}-${width}-overview.png`,fullPage:true});
      await page.locator('[data-time-range-trigger]:visible').click();
      await page.screenshot({path:`${output}/device-${theme}-${width}-menu.png`,fullPage:true});
      await page.keyboard.press('Escape');
    }
  }
  if(target==='device') {
    await page.clock.setSystemTime(new Date('2026-11-10T00:01:00Z'));
    const nextRequest=page.waitForRequest(req=>req.url().includes('/v1/overview?'));
    await page.locator('[data-dashboard-refresh]').click();
    const params=new URL((await nextRequest).url()).searchParams;
    assert.equal(params.get('start'),'2026-11-04');
    assert.equal(params.get('end'),'2026-11-10');
    // A fresh login resolves relative dates now, not at JS module evaluation.
    await page.reload();
    await page.getByLabel('Admin token').fill('synthetic-only');
    const loginRequest=page.waitForRequest(req=>req.url().includes('/v1/overview?'));
    await page.getByRole('button',{name:'Connect',exact:true}).click();
    assert.equal(new URL((await loginRequest).url()).searchParams.get('end'),'2026-11-10');
  }
  if(target==='device' && process.env.DEVICE_ALL_TIME) {
    const trigger=()=>page.locator('[data-time-range-trigger]:visible');
    const expectDates=async (action,start,end='2026-11-10')=>{
      const requested=page.waitForRequest(req=>req.url().includes('/v1/overview?') && new URL(req.url()).searchParams.get('start')===start);
      await action();
      const params=new URL((await requested).url()).searchParams;
      assert.equal(params.get('start'),start); assert.equal(params.get('end'),end);
      await page.getByText('Loading complete range…',{exact:true}).waitFor({state:'hidden'});
    };
    await trigger().click();
    await page.locator('[data-time-range-mode="all"]').focus();
    await expectDates(()=>page.keyboard.press('Enter'),'2024-01-01');
    assert.match(await trigger().innerText(),/2024/);
    for(const width of [1440,390]) {
      await page.setViewportSize({width,height:1000}); await page.waitForTimeout(500);
      await page.screenshot({path:`${output}/device-${theme}-${width}-all-time.png`,fullPage:true});
      assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true,`all time overflow ${width}: ${await page.evaluate(()=>JSON.stringify([...document.querySelectorAll('body *')].filter(el=>el.getBoundingClientRect().right>innerWidth+2).slice(-15).map(el=>({tag:el.tagName,cls:el.className,right:el.getBoundingClientRect().right}))))}`);
      await trigger().click();
      if(process.env.DEVICE_RANGE_POLISH) {
        await page.locator('[data-all-time-summary]').waitFor();
        assert.equal(await page.locator('[data-custom-endpoint]').count(),0);
        await page.waitForTimeout(400);
        const panel=page.getByRole('dialog',{name:'Time range',exact:true});
        const background=await panel.evaluate(el=>getComputedStyle(el).backgroundColor);
        assert.match(background,/^rgb\(/,`opaque range panel: ${background}`);
        for(const box of await page.locator('[data-time-range-mode]').evaluateAll(nodes=>nodes.map(el=>({width:el.getBoundingClientRect().width,height:el.getBoundingClientRect().height})))) {
          assert.ok(box.width>=44 && box.height>=44,'preset target at least 44px');
        }
        if(width===390) {
          const overlay=await page.locator('.modal-overlay').boundingBox();
          assert.ok(overlay.y<=0 && overlay.height>=1000,'overlay covers viewport');
          const y=await page.evaluate(()=>scrollY);
          await page.mouse.move(5,700); await page.mouse.wheel(0,500); await page.waitForTimeout(100);
          assert.equal(await page.evaluate(()=>scrollY),y,'background stays scroll locked');
        }
        await page.screenshot({path:`${output}/device-${theme}-${width}-all-time-menu.png`});
      }
      await page.locator('[data-time-range-mode="custom"]').click();
      assert.match(await page.locator('[data-custom-endpoint="start"]').innerText(),/Jan 1, 2024/);
      await page.screenshot({path:`${output}/device-${theme}-${width}-all-time-range.png`});
      if(process.env.DEVICE_RANGE_POLISH) {
        assert.equal(await page.locator('[data-time-range-mode="custom"]').getAttribute('aria-pressed'),'true');
        assert.doesNotMatch(await trigger().innerText(),/All time/);
        await expectDates(()=>page.locator('[data-time-range-mode="all"]').click(),'2024-01-01');
      } else await page.keyboard.press('Escape');
    }
    await page.getByRole('button',{name:'Device',exact:true}).click();
    await expectDates(()=>page.getByRole('option',{name:'Example laptop',exact:true}).click(),'2025-01-01');
    await page.getByRole('button',{name:'Device',exact:true}).click();
    await expectDates(()=>page.getByRole('option',{name:'All devices',exact:true}).click(),'2024-01-01');
    await page.getByRole('button',{name:'Agent',exact:true}).click();
    await expectDates(()=>page.getByRole('option',{name:'pi',exact:true}).click(),'2024-06-01');
    await page.getByRole('button',{name:'Agent',exact:true}).click();
    await expectDates(()=>page.getByRole('option',{name:'All agents',exact:true}).click(),'2024-01-01');
    boundaryDay='2023-01-01';
    await expectDates(()=>page.locator('[data-dashboard-refresh]').click(),'2023-01-01');
    await expectDates(()=>page.clock.fastForward(86400000),'2023-01-01','2026-11-11');
    boundaryEmpty=true;
    await expectDates(()=>page.locator('[data-dashboard-refresh]').click(),'2026-11-11','2026-11-11');
    await page.getByText('No usage history for these filters.',{exact:false}).waitFor();
    boundaryEmpty=false; boundaryDay='1900-01-01';
    await page.locator('[data-dashboard-refresh]').click();
    await page.getByRole('alert').filter({hasText:'Range exceeds 3660 days'}).waitFor();
    assert.equal(await page.locator('[data-activity-index]').count(),0);
  }
  assert.deepEqual(errors,[],`${target} ${theme} page errors`);
  await page.close();
 }
 console.log(`PASS: synthetic upstream/device comparison and native component interactions; screenshots ${output}`);
} finally { await browser.close(); await new Promise(resolve=>server.close(resolve)); }
