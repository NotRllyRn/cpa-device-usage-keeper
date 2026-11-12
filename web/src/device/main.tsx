import { useEffect, useState, useRef, useCallback, type CSSProperties } from 'react';
import { createRoot } from 'react-dom/client';
import { I18nextProvider } from 'react-i18next';
import i18n from '@/i18n';
import { useThemeStore } from '@/stores';
import { DashboardHeader } from '@/components/dashboard/DashboardHeader';
import { DashboardToolbar } from '@/components/dashboard/DashboardToolbar';
import { Button } from '@/components/ui/Button';
import { Select } from '@/components/ui/Select';
import { PriceCatalog } from './PriceCatalog';
import { PricingSettings } from '@/components/usage/pricing/PricingSettings';
import type { UseModelPricingDataReturn } from '@/components/usage/hooks/useModelPricingData';
import type { ModelPricingConfig } from '@/lib/types';
import { Line, Bar } from '@/components/usage/charts/UsageChart';
import '@/lib/chartjs';
import { DeviceAPI, type Overview, type Device, type Filters } from './api';
import '@/index.css';
import '@/App.css';
import '@/styles/reset.scss';
import '@/styles/variables.scss';
import '@/styles/themes.scss';
import '@/styles/layout.scss';
import '@/styles/components.scss';
import '@/styles/global.scss';
import styles from '@/pages/UsagePage.module.scss';
import './device.scss';

const emptyFilters: Filters = { device: '', agent: '', start: '', end: '' };
const message = (e: unknown) => e instanceof Error ? e.message : 'Request failed';
const number = (n: number) => new Intl.NumberFormat(undefined, { maximumFractionDigits: 0 }).format(n);
const money = (n: number) => '$' + n.toFixed(6);
type Tab = 'overview' | 'history' | 'devices' | 'settings';
const tabs: { id: Tab; label: string; href: string }[] = [
  { id: 'overview', label: 'Overview', href: '#overview' }, { id: 'history', label: 'Daily history', href: '#history' },
  { id: 'devices', label: 'Devices', href: '#devices' }, { id: 'settings', label: 'Settings', href: '#settings' },
];
function Card({ title, value, note }: { title: string; value: string; note: string }) {
  return <div className={styles.statCard} style={{ '--accent': '#6c7cff', '--accent-soft': '#6c7cff20', '--accent-border': '#6c7cff40' } as CSSProperties}>
    <div className={styles.statCardHeader}><span className={styles.statLabel}>{title}</span></div>
    <div className={styles.statValue}>{value}</div><div className={styles.statMetaRow}>{note}</div><div className={styles.statTrendPlaceholder} />
  </div>;
}
function Dashboard({ api, onLogout }: { api: DeviceAPI; onLogout: () => void }) {
  const [syncPrices, setSyncPrices] = useState(false);
  const [manualModel, setManualModel] = useState('');
  const [tab, setTab] = useState<Tab>('overview');
  const [filters, setFilters] = useState(emptyFilters);
  const [overview, setOverview] = useState<Overview | null>(null);
  const [devices, setDevices] = useState<Device[]>([]);
  const [prices, setPrices] = useState<ModelPricingConfig[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [name, setName] = useState('');
  const [enrollment, setEnrollment] = useState('');
  const [modelOptions, setModelOptions] = useState<string[]>([]);
  const request = useRef(0);
  const mutation = useRef(false);
  const refresh = useCallback(async () => {
    const id = ++request.current; setBusy(true); setError('');
    try {
      const [data, list, configs] = await Promise.all([api.overview(filters), api.devices(), api.prices()]);
      if (id !== request.current) return false;
      setOverview(data); setDevices(list); setPrices(configs);
      setModelOptions(old => [...new Set([...old, ...data.models.map(m => m.key), ...configs.map(p => p.model)])].sort());
      return true;
    } catch (e) { if (id === request.current) { setError(message(e)); setOverview(null); } return false; }
    finally { if (id === request.current) setBusy(false); }
  }, [api, filters]);
  useEffect(() => {
    void refresh();
    // This is a monotonic request generation, not a DOM ref. Invalidate every
    // pending response on filter changes and unmount.
    const sequence = request;
    return () => { sequence.current++; };
  }, [refresh]);
  const currentRefresh = useRef(refresh);
  useEffect(() => { currentRefresh.current = refresh; }, [refresh]);
  const mutate = async (operation: () => Promise<unknown>) => {
    if (mutation.current) throw new Error('Another change is still saving');
    mutation.current = true; setError('');
    try { await operation(); return await currentRefresh.current(); } finally { mutation.current = false; }
  };
  const pricing: UseModelPricingDataReturn = {
    models: prices, modelOptions, configRevision: null, loading: busy, error,
    loadPricing: refresh, refreshAfterMutation: refresh,
    saveModel: async config => ({ result: { model: config.model, config_revision: 0 }, refreshed: await mutate(() => api.request('/pricing/models', 'PUT', config)) }),
    deleteModel: async model => ({ result: { config_revision: 0 }, refreshed: await mutate(() => api.request('/pricing/models?model=' + encodeURIComponent(model), 'DELETE')) }),
  };
  const total = overview?.total;
  const filter = (key: keyof Filters, value: string) => setFilters(old => ({ ...old, [key]: value }));
  return <div className={`${styles.pageShell} ${styles.standalone}`} data-keeper-page="usage"><div className={styles.pageFrame}>
    <DashboardHeader identity="Device Usage Keeper" onLogout={onLogout} />
    <main className={styles.contentColumn}><div className={styles.container}>
      <p className="device-note">Independent Keeper fork · UTC daily aggregates · API-equivalent estimates, not subscription bills. <a href="https://github.com/Willxup/cpa-usage-keeper">Upstream</a></p>
      {error && <p role="alert" className="device-error">{error}</p>}
      <DashboardToolbar items={tabs} activeId={tab} onNavigate={setTab} onRefresh={() => void refresh()} refreshing={busy} filters={[
        <Select key="device" value={filters.device} options={[{ value: '', label: 'All devices' }, ...devices.map(d => ({ value: d.id, label: d.name + (d.revoked ? ' (revoked)' : '') }))]} onChange={v => filter('device', v)} ariaLabel="Device" />,
        <Select key="agent" value={filters.agent} options={[{ value: '', label: 'All agents' }, ...[...new Set([...(overview?.agents ?? []), filters.agent].filter(Boolean))].map(a => ({ value: a, label: a }))]} onChange={v => filter('agent', v)} ariaLabel="Agent" />,
        <label key="start" className="device-date">From <input type="date" value={filters.start} onChange={e => filter('start', e.target.value)} /></label>,
        <label key="end" className="device-date">Through <input type="date" value={filters.end} onChange={e => filter('end', e.target.value)} /></label>,
      ]} />
      <p className="device-note">{busy ? 'Loading complete range…' : overview ? `${overview.snapshots} daily snapshots · Latest collection ${new Date(overview.collected_at).toLocaleString()} · Refresh to include new uploads.` : 'No data loaded'}</p>
      {tab === 'overview' && <>
        <div className={styles.secondaryStatsGrid}>
          <Card title="Total tokens" value={total ? number(total.total_tokens) : '—'} note="All retained data in selected range" />
          <Card title="Estimated cost" value={total ? (!total.complete && total.known_cost === 0 ? 'Unknown' : money(total.known_cost)) : '—'} note={total?.complete ? 'Complete at current model rates' : 'Incomplete — known subtotal only; missing allocations/prices are unknown'} />
          <Card title="Requests / success rate" value="Unavailable" note="Daily device aggregates do not contain request outcomes" />
          <Card title="Latency / TTFT / TPS" value="Unavailable" note="No reliable request timing; no inferred performance" />
        </div>
        {overview && <div className="device-charts">
          <section className={styles.statCard}><h2>Daily tokens</h2><div className="device-chart"><Line options={{ responsive: true, maintainAspectRatio: false }} data={{ labels: overview.days.map(d => d.key), datasets: [{ label: 'Tokens (UTC)', data: overview.days.map(d => d.total_tokens), borderColor: '#6576ff', backgroundColor: '#6576ff30', fill: true, tension: 0.2 }] }} /></div></section>
          <section className={styles.statCard}><h2>Device tokens</h2><div className="device-chart"><Bar options={{ responsive: true, maintainAspectRatio: false }} data={{ labels: overview.devices.map(d => devices.find(v => v.id === d.key)?.name ?? d.key), datasets: [{ label: 'Tokens', data: overview.devices.map(d => d.total_tokens), backgroundColor: '#6576ff' }] }} /></div></section>
        </div>}
        <section className={styles.statCard}><h2>Models</h2><div className="device-table"><table><thead><tr><th>Model</th><th>Input</th><th>Output</th><th>Cache read / write</th><th>Estimated USD</th></tr></thead><tbody>{overview?.models.map(m => <tr key={m.key}><td>{m.key}</td><td>{number(m.tokens.input)}</td><td>{number(m.tokens.output)}</td><td>{number(m.tokens.cache_read)} / {number(m.tokens.cache_write)}</td><td>{m.complete ? money(m.known_cost) : `Unknown (known ${money(m.known_cost)})`}</td></tr>)}</tbody></table></div><p className="device-note">Unattributed or extra tokens remain in daily totals and are not assigned a fabricated model or cost.</p></section>
      </>}
      {tab === 'history' && <section className={styles.statCard}><h2>Daily history</h2><div className="device-table"><table><thead><tr><th>UTC day</th><th>Tokens</th><th>Input</th><th>Output</th><th>Cache read / write</th><th>Estimated USD</th></tr></thead><tbody>{overview?.days.map(d => <tr key={d.key}><td>{d.key}</td><td>{number(d.total_tokens)}</td><td>{number(d.tokens.input)}</td><td>{number(d.tokens.output)}</td><td>{number(d.tokens.cache_read)} / {number(d.tokens.cache_write)}</td><td>{d.complete ? money(d.known_cost) : `Incomplete: ${money(d.known_cost)}`}</td></tr>)}</tbody></table></div></section>}
      {tab === 'devices' && <section className={styles.statCard}><h2>Devices</h2><form onSubmit={e => { e.preventDefault(); void mutate(async () => { const result = await api.request<{ token: string }>('/devices', 'POST', { name }); setEnrollment(result.token); setName(''); }).catch(e => setError(message(e))); }}><label>Device name <input required pattern="[a-zA-Z0-9][a-zA-Z0-9._:+\-]{0,159}" value={name} onChange={e => setName(e.target.value)} /></label><Button type="submit" disabled={busy}>Enroll</Button></form>
        {enrollment && <div role="status"><p>Upload token (shown only here; enter privately into the installer):</p><output className="device-token">{enrollment}</output><Button onClick={() => setEnrollment('')}>Dismiss token</Button></div>}
        <div className="device-table"><table><thead><tr><th>Device</th><th>Last upload received</th><th>Status</th><th>Action</th></tr></thead><tbody>{devices.map(d => <tr key={d.id}><td>{d.name}</td><td>{d.last_seen ? new Date(d.last_seen).toLocaleString() : 'Never'}</td><td>{d.revoked ? 'Revoked' : 'Active'}</td><td><Button disabled={d.revoked || busy} variant="danger" onClick={() => { if (confirm('Revoke uploads for ' + d.name + '? History is retained.')) void mutate(() => api.request('/devices/' + d.id, 'DELETE')).catch(e => setError(message(e))); }}>Revoke</Button></td></tr>)}</tbody></table></div><p className="device-note">Copied histories across devices overlap. Filter old/replacement devices rather than summing both. Enrollment is capped at 1,000, including revoked devices.</p></section>}
      {tab === 'settings' && <><p className="device-note">Keeper Model Pricing Settings. Use Add for uploaded models, or Sync Prices to select and review source rates. Saving immediately refreshes historical estimates and the model breakdown. Recalculate Costs is unnecessary: aggregate costs are recalculated on every read. Request-dependent rules produce unknown daily costs.</p><label>Additional model identifier <input value={manualModel} onChange={e => setManualModel(e.target.value)} /></label><Button disabled={!manualModel.trim()} onClick={() => { setModelOptions(old => [...new Set([...old, manualModel.trim()])].sort()); setManualModel(''); }}>Add to model choices</Button><p>{modelOptions.length} model choices · {prices.length} configured. Add is unavailable only while loading or when all choices already have settings; use Edit for those models.</p><PricingSettings pricing={pricing} aggregateMode onSyncPrices={() => setSyncPrices(true)} timezone="UTC" /></>}
      {syncPrices && <PriceCatalog api={api} models={modelOptions} onClose={() => setSyncPrices(false)} onSave={pricing.saveModel} />}
      <p className="device-note">Unavailable for device aggregates: realtime traffic, request events, credentials, quotas, success rates, request speed and latency.</p>
    </div></main>
  </div></div>;
}
function App() {
  const initializeTheme = useThemeStore(s => s.initializeTheme);
  useEffect(() => initializeTheme(), [initializeTheme]);
  const [api, setAPI] = useState<DeviceAPI | null>(null);
  const [token, setToken] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  useEffect(() => () => api?.close(), [api]);
  if (api) return <Dashboard api={api} onLogout={() => { api.close(); setAPI(null); setToken(''); }} />;
  return <div className="device-login"><div className={styles.statCard}><h1>Device Usage Keeper</h1><p>Connect with the plugin admin token, not CPA credentials.</p><form onSubmit={e => { e.preventDefault(); if (busy) return; setBusy(true); setError(''); const candidate = new DeviceAPI(token); setToken(''); void candidate.devices().then(() => setAPI(candidate)).catch(e => { candidate.close(); setError(message(e)); }).finally(() => setBusy(false)); }}><label>Admin token<input type="password" autoComplete="off" value={token} onChange={e => setToken(e.target.value)} required /></label><Button type="submit" disabled={busy}>Connect</Button></form>{error && <p role="alert">{error}</p>}<p>Token stays in memory only. Use your HTTPS reverse proxy.</p></div></div>;
}
createRoot(document.getElementById('root')!).render(<I18nextProvider i18n={i18n}><App /></I18nextProvider>);
