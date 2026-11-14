// Matching synthetic data through both compiled Keeper entry points.
import { createRequire } from 'node:module';
import { createServer } from 'node:http';
import { readFile, mkdir } from 'node:fs/promises';
import { resolve, extname } from 'node:path';
import assert from 'node:assert/strict';
const { chromium } = createRequire(import.meta.url)(process.env.DEVICE_TEST_PLAYWRIGHT);
const output=process.env.DEVICE_SCREENSHOTS || '/tmp/device-analysis-parity';
await mkdir(output,{recursive:true});
const root=resolve(import.meta.dirname,'../web/dist');
const server=createServer(async(req,res)=>{try{const path=new URL(req.url,'http://localhost').pathname;const file=path.startsWith('/assets/')?resolve(root,'.'+path):resolve(root,'index.html');if(!file.startsWith(root+'/'))throw Error();res.setHeader('Content-Type',{'.js':'text/javascript','.css':'text/css','.html':'text/html'}[extname(file)]||'application/octet-stream');res.end(await readFile(file));}catch{res.statusCode=404;res.end();}});
await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
const upstream=`http://127.0.0.1:${server.address().port}`;
const agg=(key,n,complete=true)=>({key,tokens:{input:600*n,output:200*n,cache_read:150*n,cache_write:50*n},total_tokens:1000*n,known_cost:complete?.02*n:0,complete,coverage:{priced:complete?1:0,missing:complete?0:1,conditional:0,unattributed_tokens:0}});
const models=[agg('[pi] gpt-example',21),agg('claude-example',14,false)];
const total={...agg('all',35,false),known_cost:.42,coverage:{priced:7,missing:7,conditional:0,unattributed_tokens:0}};
const days=Array.from({length:7},(_,i)=>({...agg(`2026-11-${String(i+3).padStart(2,'0')}`,5,false),known_cost:.06,coverage:{priced:1,missing:1,conditional:0,unattributed_tokens:0}}));
const fixture={total,days,models,devices:[{...total,key:'laptop'}],agents:['pi','codex'],collected_at:'2026-11-09T12:00:00Z',snapshots:7,
 breakdown:days.map(d=>({day:d.key,models:[agg(models[0].key,3),agg(models[1].key,2,false)],devices:[{...d,key:'laptop'}]})),
 cells:models.map(m=>({...m,device:'laptop',model:m.key}))};
const fields=v=>({input_tokens:v.tokens.input+v.tokens.cache_read+v.tokens.cache_write,output_tokens:v.tokens.output,cache_read_tokens:v.tokens.cache_read,cache_creation_tokens:v.tokens.cache_write,reasoning_tokens:0,total_tokens:v.total_tokens,requests:0,cost_usd:v.known_cost,cost_available:v.complete});
const composition=(rows)=>rows.map(v=>({...fields(v),key:v.key,label:v.key==='laptop'?'Example laptop':v.key,percent:v.total_tokens/total.total_tokens*100}));
const analysis={granularity:'daily',timezone:'UTC',range_start:'2026-11-03T00:00:00Z',range_end:'2026-11-10T00:00:00Z',token_usage:days.map(v=>({...fields(v),bucket:v.key+'T00:00:00Z'})),cost_summary:{total_cost_usd:total.known_cost,cost_available:false},model_usage:{buckets:days.map(v=>v.key+'T00:00:00Z'),series:models.map((m,i)=>({model:m.key,total_tokens:days.map(()=>i===0?3000:2000),requests:[]}))},api_key_composition:composition(fixture.devices),model_composition:composition(models),credentials_composition:[],ai_provider_composition:[],model_efficiency:[],heatmap:{api_keys:['laptop'],api_key_labels:{laptop:'Example laptop'},models:models.map(m=>m.key),cells:models.map(m=>({...fields(m),api_key:'laptop',model:m.key,intensity:1}))}};
async function navigate(page,label){const tab=page.getByRole('tab',{name:label,exact:true});if(await tab.isVisible())await tab.click();else{await page.locator('[data-dashboard-page-trigger]').click();await page.getByRole('menuitem',{name:label,exact:true}).click();}}
async function shot(page,path){await page.evaluate(()=>window.scrollTo(0,0));await page.waitForTimeout(300);await page.screenshot({path,fullPage:true});}
const browser=await chromium.launch({headless:true,executablePath:process.env.DEVICE_TEST_CHROMIUM});
try{
for(const theme of ['light','dark'])for(const target of ['upstream','device']){
 const page=await browser.newPage({viewport:{width:1440,height:1000},colorScheme:theme});const errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.clock.install({time:new Date('2026-11-09T12:00:00Z')});
 let empty=false,zero=false,pricingMode='partial';const requests=[];
 await page.route('**/api/v1/**',async route=>{const path=new URL(route.request().url()).pathname.replace('/api/v1','');await route.fulfill({json:{'/startup/status':{phase:'ready',message:'',progress:null},'/auth/session':{authenticated:true,role:'admin'},'/usage/analysis':analysis,'/usage/analysis/latency':{supported:false,points:[],density:[]},'/usage/overview':{usage:{total_tokens:35000},summary:{},timezone:'UTC'},'/usage/overview/comparisons':{buckets:[],models:[]},'/usage/activity':{blocks:[]},'/pricing/models':{models:[]},'/status':{timezone:'UTC'},'/cpa/api-keys/options':{items:[]}}[path]??{}});});
 await page.route('**/v1/**',async route=>{if(route.request().url().includes('/api/v1/'))return route.fallback();const url=new URL(route.request().url());requests.push(url);let data={'/v1/session':{authenticated:true},'/v1/devices':[{id:'laptop',name:'Example laptop',revoked:false}],'/v1/pricing/models':[], '/v1/usage-boundary':{earliest_day:'2026-11-03'}}[url.pathname];
 if(['/v1/overview','/v1/analysis'].includes(url.pathname))data=empty?{...fixture,total:agg('all',0),days:[],models:[],devices:[],breakdown:[],cells:[],snapshots:0}:zero?{...fixture,total:agg('all',0),days:[agg('2026-11-09',0)],models:[agg('free-model',0)],devices:[agg('laptop',0)],breakdown:[{day:'2026-11-09',models:[agg('free-model',0)],devices:[agg('laptop',0)]}],cells:[{...agg('free-model',0),device:'laptop',model:'free-model'}]}:fixture;
 if(data===fixture && url.searchParams.get('agent')==='pi')data={...fixture,total:models[0],models:[models[0]],devices:[{...models[0],key:'laptop'}],days:days.map(d=>agg(d.key,3)),breakdown:days.map(d=>({day:d.key,models:[agg(models[0].key,3)],devices:[agg('laptop',3)]})),cells:[fixture.cells[0]]};
 if(data && ['complete','unknown'].includes(pricingMode) && ['/v1/overview','/v1/analysis'].includes(url.pathname))data=JSON.parse(JSON.stringify(data),(key,value)=>key==='complete'?pricingMode==='complete':key==='known_cost'&&pricingMode==='unknown'?0:key==='coverage'?{priced:pricingMode==='complete'?1:0,missing:pricingMode==='unknown'?1:0,conditional:0,unattributed_tokens:0}:value);
 if(data!==undefined)return route.fulfill({json:data});throw Error('Unexpected endpoint '+url.pathname);});
 await page.goto(target==='device'?process.env.DEVICE_TEST_URL:upstream);await page.waitForTimeout(1000);
 if(target==='device') { const text=await page.locator('main').innerText();assert.match(text,/Partial cost subtotal/);assert.match(text,/Partial cost \/ day/);assert.doesNotMatch(text,/Set pricing to calculate cost|Known \$/);await shot(page,`${output}/device-${theme}-1440-overview-partial.png`); }
 await navigate(page,'Analysis');
 if(target==='upstream'){await page.locator('[data-time-range-trigger]:visible').click();await page.locator('[data-time-range-mode="day"]').click();await page.keyboard.press('Escape');}
 await page.getByRole('heading',{name:'Token Usage Over Time',exact:true}).waitFor();await page.waitForTimeout(1300);
 for(const width of [1440,390]){await page.setViewportSize({width,height:1000});await page.waitForTimeout(600);assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true,`${target} overflow ${width}`);await shot(page,`${output}/${target}-${theme}-${width}-analysis.png`);}
 if(target==='device'){
  const body=await page.locator('main').innerText();assert.match(body,/Unknown \(known/);assert.doesNotMatch(body,/Requests\s+0|RPM\s+0|TPM\s+0|TTFT\s+0/);
  await page.getByRole('tablist',{name:'Usage Distribution'}).getByRole('tab',{name:'Model',exact:true}).click();const ranking=page.getByRole('list').first().getByRole('button').first();await ranking.click();assert.equal(await ranking.getAttribute('aria-pressed'),'true');await ranking.click();assert.equal(await ranking.getAttribute('aria-pressed'),'false');
  await page.getByRole('tablist',{name:'Trend dimension'}).getByRole('tab',{name:'Device',exact:true}).click();await page.getByRole('heading',{name:'Top Devices'}).waitFor();await page.getByRole('list',{name:'Device ranking'}).getByRole('button',{name:/Example laptop/}).click();await shot(page,`${output}/device-${theme}-device-trend.png`);
  await page.getByRole('tablist',{name:'Usage Distribution'}).getByRole('tab',{name:'Device',exact:true}).click();await page.getByRole('list').first().getByRole('button',{name:/Example laptop/}).waitFor();
  await page.getByLabel('Breakdown dimension').click();await page.getByRole('option',{name:'Device',exact:true}).click();assert.match(await page.getByRole('region',{name:'Token and cost breakdown',exact:true}).innerText(),/Example laptop/);
  await page.getByLabel('Device',{exact:true}).click();await page.getByRole('option',{name:'Example laptop',exact:true}).click();await page.waitForTimeout(300);assert.equal(requests.filter(u=>u.pathname==='/v1/analysis').at(-1).searchParams.get('device'),'laptop');
  await page.getByLabel('Agent',{exact:true}).click();await page.getByRole('option',{name:'pi',exact:true}).click();await page.waitForTimeout(300);assert.equal(requests.filter(u=>u.pathname==='/v1/analysis').at(-1).searchParams.get('agent'),'pi');assert.doesNotMatch(await page.getByRole('region',{name:'Token and cost breakdown',exact:true}).innerText(),/claude-example/);assert.match(await page.locator('main').innerText(),/21.00K/);
  await page.locator('[data-time-range-trigger]:visible').click();await page.locator('[data-time-range-mode="all"]').click();await page.waitForTimeout(300);assert.equal(requests.filter(u=>u.pathname==='/v1/analysis').at(-1).searchParams.get('start'),'2026-11-03');
  zero=true;await page.locator('[data-dashboard-refresh]').click();await page.getByText('free-model',{exact:true}).first().waitFor();assert.doesNotMatch(await page.getByRole('region',{name:'Token and cost breakdown',exact:true}).innerText(),/Unknown/);await shot(page,`${output}/device-${theme}-zero.png`);
  empty=true;await page.locator('[data-dashboard-refresh]').click();await page.waitForTimeout(300);assert.equal(await page.getByRole('region',{name:'Token and cost breakdown',exact:true}).getByRole('row').count(),1);await shot(page,`${output}/device-${theme}-empty.png`);
  await navigate(page,'Overview');await page.waitForTimeout(300);assert.match(await page.locator('main').innerText(),/No usage uploaded for this range/);
  empty=false;zero=true;await page.locator('[data-dashboard-refresh]').click();await page.waitForTimeout(300);assert.doesNotMatch(await page.locator('main').innerText(),/Unknown|Set pricing to calculate cost/);
  zero=false;pricingMode='unknown';await page.locator('[data-dashboard-refresh]').click();await page.waitForTimeout(300);assert.match(await page.locator('main').innerText(),/Unknown/);assert.doesNotMatch(await page.locator('main').innerText(),/Set pricing to calculate cost/);await shot(page,`${output}/device-${theme}-390-overview-unknown.png`);
  pricingMode='complete';await page.locator('[data-dashboard-refresh]').click();await page.waitForTimeout(300);assert.match(await page.locator('main').innerText(),/All reported usage priced/);assert.doesNotMatch(await page.locator('main').innerText(),/Partial cost|Unknown|Set pricing to calculate cost/);await shot(page,`${output}/device-${theme}-390-overview-complete.png`);
 }
 assert.deepEqual(errors,[]);await page.close();
}
console.log('Analysis parity, filters, composition/ranking selection, table dimensions, zero/empty and mobile checks passed. Screenshots: '+output);
}finally{await browser.close();await new Promise(resolve=>server.close(resolve));}
