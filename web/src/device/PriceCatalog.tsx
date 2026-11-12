import { useState } from 'react';
import { Button } from '@/components/ui/Button';
import { Modal } from '@/components/ui/Modal';
import { ModelPricingEditor } from '@/components/usage/pricing/ModelPricingEditor';
import type { ModelPricingConfig } from '@/lib/types';
import type { DeviceAPI } from './api';
type Entry = { ProviderID: string; Model: { id: string; name: string; cost: { input: number | null; output: number | null; cache_read: number | null; cache_write: number | null } } };
type Catalog = { Source: { Name: string; URL: string }; Entries: Entry[] };
// Explicit selection, not fuzzy matching: broker aliases must never acquire a
// guessed price. Missing source categories stay blank in Keeper's editor.
export function PriceCatalog({ api, models, onSave, onClose }: { api: DeviceAPI; models: string[]; onSave: (p: ModelPricingConfig) => Promise<unknown>; onClose: () => void }) {
  const [source, setSource] = useState('litellm');
  const [catalog, setCatalog] = useState<Catalog | null>(null);
  const [model, setModel] = useState(models[0] ?? '');
  const [query, setQuery] = useState('');
  const [missingRead, setMissingRead] = useState('');
  const [missingWrite, setMissingWrite] = useState('');
  const fallback = (value: string) => value.trim() !== '' && Number.isFinite(Number(value)) && Number(value) >= 0 ? Number(value) : null;
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [draft, setDraft] = useState<ModelPricingConfig | null>(null);
  const load = async () => { setBusy(true); setError(''); setCatalog(null); try { setCatalog(await api.request<Catalog>('/pricing/catalog?source=' + source)); } catch { setError('Price source unavailable. Try the other source, or use Add to enter rates manually.'); } finally { setBusy(false); } };
  if (draft) return <ModelPricingEditor open initialConfig={draft} modelOptions={models} timezone="UTC" onClose={() => setDraft(null)} onSave={async p => { await onSave(p); onClose(); }} />;
  const entries = (catalog?.Entries ?? []).filter(e => (e.ProviderID + '/' + e.Model.id).toLowerCase().includes(query.toLowerCase()));
  return <Modal open title="Sync Prices — select and review" onClose={onClose}>
    <p>Select the uploaded model and a source price. Nothing is saved until you review and save in Keeper's editor. Missing categories must be filled explicitly; zero means free. Existing settings for the selected model will be replaced.</p>
    <label>Uploaded model <select value={model} onChange={e => setModel(e.target.value)}>{models.map(m => <option key={m}>{m}</option>)}</select></label>
    <label>Source <select value={source} disabled={busy} onChange={e => { setSource(e.target.value); setCatalog(null); }}><option value="litellm">LiteLLM</option><option value="models-dev">Models.dev</option></select></label>
    <Button disabled={busy} onClick={() => void load()}>{busy ? 'Fetching…' : 'Fetch prices'}</Button>
    <p>When the source omits a cache rate, enter your verified rate below before selecting. Do not assume an omitted price is free.</p>
    <label>Missing cache-read rate <input type="number" min="0" step="any" value={missingRead} onChange={e => setMissingRead(e.target.value)} /></label>
    <label>Missing cache-write rate <input type="number" min="0" step="any" value={missingWrite} onChange={e => setMissingWrite(e.target.value)} /></label>
    {error && <p role="alert">{error}</p>}
    {catalog && <><p>Source: {catalog.Source.Name}. USD per million tokens. Standard base rates only; provider tiers and request conditions need separate review.</p><label>Search source models <input value={query} onChange={e => setQuery(e.target.value)} /></label><p>{entries.length} matches; showing first 100. Narrow the search.</p><div className="device-table"><table><thead><tr><th>Provider / model</th><th>Input / output</th><th>Cache read / write</th><th>Review</th></tr></thead><tbody>{entries.slice(0, 100).map((e, i) => <tr key={i}><td>{e.ProviderID} / {e.Model.id}</td><td>{e.Model.cost.input ?? 'Unknown'} / {e.Model.cost.output ?? 'Unknown'}</td><td>{e.Model.cost.cache_read ?? 'Unknown'} / {e.Model.cost.cache_write ?? 'Unknown'}</td><td><Button disabled={!model} onClick={() => { const c=e.Model.cost; const read=c.cache_read ?? fallback(missingRead); const write=c.cache_write ?? fallback(missingWrite); if(c.input===null || c.output===null || read===null || write===null) { setError('This source has missing rates. Fill missing cache rates or use Add for manual input/output rates.'); return; } setDraft({ model, pricing_style: e.Model.id.includes('claude') ? 'claude' : 'openai', base_prices: {input:c.input,output:c.output,cache_read:read,cache_write:write}, model_multiplier: 1, branches: [], conditional_multipliers: [] }); }}>Select</Button></td></tr>)}</tbody></table></div></>}
  </Modal>;
}
