import { describe, expect, it } from 'vitest';
import { deviceCostDisplay, deviceCostText } from './costs';
import type { Overview } from './api';
const base: Overview = { total: {key:'',total_tokens:10,tokens:{input:10,output:0,cache_read:0,cache_write:0},known_cost:12,complete:true,coverage:{priced:1,missing:0,conditional:0,unattributed_tokens:0}},models:[],days:[],devices:[],breakdown:[],cells:[],agents:[],snapshots:1,collected_at:'' };
describe('cost coverage presentation',()=>{
 it('does not present an unavailable subtotal as a priced zero',()=>{
 const unknown={...base.total,known_cost:0,complete:false,coverage:{priced:0,missing:1,conditional:0,unattributed_tokens:0}};
 expect(deviceCostText(unknown)).toBe('Unknown · no priced usage');
 expect(deviceCostText({...unknown,coverage:{...unknown.coverage,priced:1}})).toBe('Partial $0.000000');
 expect(deviceCostText({...unknown,known_cost:.000001})).toBe('Partial $0.000001');
 expect(deviceCostText({...unknown,complete:true})).toBe('$0.000000');
 });
 it('shows complete cost and average',()=>{expect(deviceCostDisplay(base)).toMatchObject({label:'Estimated cost',value:'$12.00'});expect(deviceCostDisplay(base,2).value).toBe('$6.00');});
 it('shows partial subtotal and calendar-day average with real missing reasons',()=>{
 const data={...base,total:{...base.total,complete:false,coverage:{priced:1,missing:2,conditional:1,unattributed_tokens:7}}};
 expect(deviceCostDisplay(data)).toMatchObject({label:'Partial cost subtotal',value:'$12.00'});
 expect(deviceCostDisplay(data,2)).toMatchObject({label:'Partial cost / day',value:'$6.00'});
 expect(deviceCostDisplay(data).hint).toBe('Priced usage only · incomplete.');
 expect(deviceCostDisplay(data).details).toContain('2 model-day entries lack rates');
 expect(deviceCostDisplay(data).details).toContain('require request details');
 expect(deviceCostDisplay(data).details).toContain('7 tokens lack a model');
 expect(deviceCostDisplay(data,2).hint).toBe('Per calendar day, including no-upload days.');
 });
 it('keeps all-unknown distinct from explicitly free and empty',()=>{
 const free={...base,total:{...base.total,known_cost:0}};
 expect(deviceCostDisplay(free).value).toBe('$0.0000');
 expect(deviceCostDisplay({...free,total:{...free.total,complete:false,coverage:{priced:0,missing:1,conditional:0,unattributed_tokens:0}}}).value).toBe('Unknown');
 expect(deviceCostDisplay({...free,snapshots:0})).toMatchObject({value:'—',hint:'No usage uploaded for this range.'});
 });
});
