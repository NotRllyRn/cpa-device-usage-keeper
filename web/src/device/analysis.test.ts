import { describe, expect, it } from 'vitest';
import { analysisData } from './analysis';
import type { Aggregate, Overview } from './api';
const value: Aggregate = { key: 'model', tokens: { input: 100, output: 20, cache_read: 10, cache_write: 5 }, total_tokens: 142, known_cost: .01, complete: false };
const data: Overview = { total: value, models: [value], devices: [{...value,key:'d'}], days: [{...value,key:'2026-01-02'}], breakdown: [{ day:'2026-01-02',models:[value],devices:[{...value,key:'d'}] }],cells:[{...value,device:'d',model:'model'}],agents:['pi'],snapshots:1,collected_at:'' };
describe('device analysis adapter',()=>{
 it('preserves exact totals, unknown pricing and inclusive input without invented days',()=>{
  const result=analysisData(data,[{id:'d',name:'Laptop',revoked:false,last_seen:null}],'2026-01-01','2026-01-03');
  expect(result.token_usage).toHaveLength(1);
  expect(result.token_usage[0]).toMatchObject({input_tokens:115,output_tokens:20,total_tokens:142,cost_usd:.01,cost_available:false});
  expect(result.api_key_composition[0].label).toBe('Laptop');
  expect(result.model_usage?.series[0].total_tokens).toEqual([142]);
  expect(result.device_usage.series[0]).toMatchObject({model:'Laptop',total_tokens:[142]});
  expect(result.heatmap.cells[0]).toMatchObject({api_key:'d',model:'model',cost_available:false});
  expect(result.model_efficiency).toEqual([]);
 });
 it('retains zero, unattributed and empty results distinctly',()=>{
  const unknown={...value,key:'',total_tokens:7};
  const result=analysisData({...data,models:[unknown]},[],'2026-01-02','2026-01-02');
  expect(result.model_composition[0].label).toBe('Unattributed (no model)');
  const zero={...value,total_tokens:0,known_cost:0,complete:true};
  expect(analysisData({...data,total:zero},[],'2026-01-02','2026-01-02').cost_summary).toEqual({total_cost_usd:0,cost_available:true});
  expect(analysisData({...data,models:[],days:[],cells:[],devices:[]},[],'2026-01-02','2026-01-02').token_usage).toEqual([]);
 });
 it('keeps devices with identical display names separate in trends',()=>{
  const devices=[{id:'d',name:'Laptop',revoked:false,last_seen:null},{id:'e',name:'Laptop',revoked:false,last_seen:null}];
  const result=analysisData({...data,devices:[{...value,key:'d'},{...value,key:'e'}]},devices,'2026-01-02','2026-01-02');
  expect(result.device_usage.series.map(s=>s.model)).toEqual(['Laptop (d)','Laptop (e)']);
 });
 it('rejects excessive work instead of silently taking a top N',()=>{
  expect(()=>analysisData({...data,models:Array(101).fill(value)},[],'2026-01-02','2026-01-02')).toThrow(/100 models/);
  expect(()=>analysisData(data,[],'2000-01-01','2026-01-02')).toThrow(/3660/);
 });
});
