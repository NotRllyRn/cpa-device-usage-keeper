import { useEffect, useState, useRef, useCallback } from 'react';
import { createRoot } from 'react-dom/client';
import { I18nextProvider } from 'react-i18next';
import i18n from '@/i18n';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { useThemeStore } from '@/stores';
import { DashboardHeader } from '@/components/dashboard/DashboardHeader';
import { DashboardToolbar } from '@/components/dashboard/DashboardToolbar';
import { Button } from '@/components/ui/Button';
import { Select } from '@/components/ui/Select';
import { CompletePricingSyncModal } from '@/components/usage/pricing/CompletePricingSyncModal';
import { PricingSettings } from '@/components/usage/pricing/PricingSettings';
import type { UseModelPricingDataReturn } from '@/components/usage/hooks/useModelPricingData';
import type { ModelPricingConfig } from '@/lib/types';
import { UsageComparisonCharts } from '@/components/usage/UsageComparisonCharts';
import { TokenActivityCard } from '@/components/usage/TokenActivityCard';
import { AnalysisPanel } from '@/components/usage/analysis/AnalysisPanel';
import { analysisData } from './analysis';
import { AnalysisDetails } from './AnalysisDetails';
import { deviceCostDisplay, deviceCostText } from './costs';
import { StatCards } from '@/components/usage/StatCards';
import { TimeRangeControl } from '@/components/usage/TimeRangeControl';
import type { UsageTimeRange, UsageCustomRange } from '@/lib/types';
import { dashboardData, rangeDates, allTimeDates, validateDashboardRange, MAX_DEVICE_RANGE_DAYS, formatDeviceDay } from './dashboard';
import '@/lib/chartjs';
import { DeviceAPI, DeviceAPIError, type Overview, type Device, type Filters } from './api';
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

const emptyFilters: Filters = { device: '', agent: '', family: '', ...rangeDates('7d') };
const message = (e: unknown) => e instanceof Error ? e.message : 'Request failed';
const number = (n: number) => new Intl.NumberFormat(undefined, { maximumFractionDigits: 0 }).format(n);
type Tab = 'overview' | 'analysis' | 'history' | 'devices' | 'settings';
const tabs: { id: Tab; label: string; href: string }[] = [
  { id: 'overview', label: 'Overview', href: '#overview' }, { id: 'analysis', label: 'Analysis', href: '#analysis' }, { id: 'history', label: 'Daily history', href: '#history' },
  { id: 'devices', label: 'Devices', href: '#devices' }, { id: 'settings', label: 'Settings', href: '#settings' },
];
function Dashboard({ api, onLogout }: { api: DeviceAPI; onLogout: () => void }) {
  const isMobile = useMediaQuery('(max-width: 768px)');
  const isDark = useThemeStore(s => s.resolvedTheme === 'dark');
  const [range, setRange] = useState<UsageTimeRange>('7d');
  const [customRange, setCustomRange] = useState<UsageCustomRange>();
  const [allTime, setAllTime] = useState(false);
  const [allTimeRange, setAllTimeRange] = useState<UsageCustomRange>();
  const [emptyHistory, setEmptyHistory] = useState(false);
  const [syncPrices, setSyncPrices] = useState(false);
  const [manualModel, setManualModel] = useState('');
  const [tab, setTab] = useState<Tab>('overview');
  const [filters, setFilters] = useState(() => ({ ...emptyFilters, ...rangeDates('7d') }));
  const scope = JSON.stringify([tab, filters, range, customRange, allTime]);
  const [loaded, setLoaded] = useState<{ scope: string; data: Overview; dashboard: ReturnType<typeof dashboardData>; analysis: ReturnType<typeof analysisData> | null } | null>(null);
  const [loadError, setLoadError] = useState<{ scope: string; message: string } | null>(null);
  const [devices, setDevices] = useState<Device[]>([]);
  const [prices, setPrices] = useState<ModelPricingConfig[]>([]);
  const [pending, setBusy] = useState(false);
  const busy = pending || (loaded?.scope !== scope && loadError?.scope !== scope);
  const current = !busy && loaded?.scope === scope ? loaded : null;
  const overview = current?.data ?? null;
  const [error, setError] = useState('');
  const [name, setName] = useState('');
  const [enrollment, setEnrollment] = useState('');
  const [modelOptions, setModelOptions] = useState<string[]>([]);
  const request = useRef(0);
  const mutation = useRef(false);
  const refresh = useCallback(async () => {
    const id = ++request.current; setBusy(true); setError(''); setLoadError(null);
    try {
      const boundary = allTime ? await api.boundary(filters) : null;
      if (id !== request.current) return false;
      const dates = allTime ? allTimeDates(boundary!.earliest_day) : rangeDates(range, customRange);
      setEmptyHistory(allTime && boundary!.earliest_day === null);
      if (allTime) setAllTimeRange({ unit: 'day', ...dates });
      validateDashboardRange(dates.start, dates.end);
      const [data, list, configs] = await Promise.all([api.overview({ ...filters, ...dates }, tab === 'analysis'), api.devices(), api.prices()]);
      if (id !== request.current) return false;
      validateDashboardRange(dates.start, dates.end, data.models.length + data.devices.length);
      // Adapt only the completed response for this endpoint/query, inside the
      // request error handler. Never adapt previous Overview data during navigation.
      const dashboard = dashboardData(data, list, dates.start, dates.end);
      const analysis = tab === 'analysis' ? analysisData(data, list, dates.start, dates.end) : null;
      setLoaded({ scope, data, dashboard, analysis }); setDevices(list); setPrices(configs);
      setModelOptions(old => [...new Set([...old, ...data.models.map(m => m.key).filter(Boolean), ...configs.map(p => p.model)])].sort());
      return true;
    } catch (e) { if (id === request.current) { setLoadError({ scope, message: message(e) }); setLoaded(null); } return false; }
    finally { if (id === request.current) setBusy(false); }
  }, [api, filters, range, customRange, allTime, tab, scope]);
  useEffect(() => {
    void refresh();
    // This is a monotonic request generation, not a DOM ref. Invalidate every
    // pending response on filter changes and unmount.
    const sequence = request;
    return () => { sequence.current++; };
  }, [refresh]);
  const currentRefresh = useRef(refresh);
  useEffect(() => { currentRefresh.current = refresh; }, [refresh]);
  useEffect(() => {
    if (!allTime) return;
    let timer: ReturnType<typeof setTimeout>;
    const schedule = () => {
      const now = Date.now();
      timer = setTimeout(() => { void currentRefresh.current(); schedule(); }, 86400000 - now % 86400000 + 50);
    };
    const onVisible = () => { if (document.visibilityState === 'visible') void currentRefresh.current(); };
    schedule();
    document.addEventListener('visibilitychange', onVisible);
    return () => { clearTimeout(timer); document.removeEventListener('visibilitychange', onVisible); };
  }, [allTime]);
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
  const analysis = current?.analysis ?? null;
  const dashboard = current?.dashboard ?? null;
  const filter = (key: keyof Filters, value: string) => setFilters(old => ({ ...old, [key]: value }));
  return <div className={`${styles.pageShell} ${styles.standalone}`} data-keeper-page="usage" data-device-dashboard><div className={styles.pageFrame}>
    <DashboardHeader identity="Device Usage Keeper" onLogout={onLogout} />
    <main className={styles.contentColumn}><div className={styles.container}>
      {loadError?.scope === scope && <p role="alert" className="device-error">{loadError.message}</p>}
      {error && <p role="alert" className="device-error">{error}</p>}
      <DashboardToolbar items={tabs} activeId={tab} onNavigate={setTab} onRefresh={() => void refresh()} refreshing={busy} filters={[
        <Select key="device" dropdownClassName="device-filter-options" dropdownMinWidth={320} value={filters.device} options={[{ value: '', label: 'All devices' }, ...devices.map(d => ({ value: d.id, label: d.name + (d.revoked ? ' (revoked)' : '') }))]} onChange={v => filter('device', v)} ariaLabel="Device" ariaDescribedBy="device-filter-value" dataAttributes={{ 'data-selected-value': filters.device }} renderValue={option => <span id="device-filter-value" className="device-filter-value" title={option?.label}>{option?.label}</span>} />,
        <Select key="agent" dropdownClassName="device-filter-options" dropdownMinWidth={320} value={filters.agent} options={[{ value: '', label: 'All agents' }, ...[...new Set([...(loaded?.data.agents ?? []), filters.agent].filter(Boolean))].map(a => ({ value: a, label: a }))]} onChange={v => filter('agent', v)} ariaLabel="Agent" ariaDescribedBy="agent-filter-value" dataAttributes={{ 'data-selected-value': filters.agent }} renderValue={option => <span id="agent-filter-value" className="device-filter-value" title={option?.label}>{option?.label}</span>} />,
        <Select key="family" dropdownClassName="device-filter-options" dropdownMinWidth={320} value={filters.family} options={[{ value: '', label: 'All model families' }, { value: 'openai', label: 'OpenAI / Codex' }, { value: 'anthropic', label: 'Anthropic / Claude' }, { value: 'minimax', label: 'MiniMax' }, { value: 'unknown', label: 'Unknown / unattributed' }]} onChange={v => filter('family', v)} ariaLabel="Model family" ariaDescribedBy="family-filter-value" dataAttributes={{ 'data-selected-value': filters.family }} renderValue={option => <span id="family-filter-value" className="device-filter-value" title={option?.label}>{option?.label}</span>} />,
        <TimeRangeControl key="range" dailyOnly value={allTime ? 'custom' : range} customRange={allTime ? allTimeRange : customRange} maxCustomDayRangeDays={MAX_DEVICE_RANGE_DAYS} showCustomRangeYear allTime={{ active: allTime, onSelect: () => { setAllTimeRange(undefined); setAllTime(true); if (allTime) void refresh(); } }} timeZone="UTC" ariaLabel="Time range" onChange={(value, custom) => { setAllTime(false); setRange(value); setCustomRange(custom); setFilters(old => ({ ...old, ...rangeDates(value, custom) })); }} />,
      ]} />
      <div className="device-context">
        <div className="device-metadata" role="status">{busy ? 'Loading complete range…' : overview ? <><span>{number(overview.snapshots)} snapshots · UTC</span>{overview.snapshots > 0 && <span>Collected <time dateTime={overview.collected_at}>{new Date(overview.collected_at).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })}</time></span>}</> : 'No data loaded'}</div>
        <div className="device-context-actions">
          <details className="device-help"><summary>About this view</summary><div>
            <p>Independent Keeper fork · UTC daily aggregates. API-equivalent estimates, not subscription bills. <a href="https://github.com/Willxup/cpa-usage-keeper">Upstream</a></p>
            <p>Model family uses recognized model names, not the serving provider. Unknown names and unattributed totals are separate; agents are independent.</p>
            <p>Refresh to include new uploads. Missing uploads are unknown usage, not confirmed zero. Overview averages use all calendar days, including no-upload days; Analysis averages use uploaded days only.</p>
          </div></details>
          {(filters.device || filters.agent || filters.family) && <Button variant="secondary" onClick={() => setFilters(old => ({ ...old, device: '', agent: '', family: '' }))}>Reset filters</Button>}
          {overview && !overview.total.complete && <Button variant="secondary" onClick={() => setTab('settings')}>Configure model prices</Button>}
        </div>
        {allTime && emptyHistory && <p role="status" className="device-note">No history for these filters. All time shows today (UTC) until uploads arrive.</p>}
      </div>
      {tab === 'overview' && <>
        <StatCards costDisplay={overview ? deviceCostDisplay(overview) : undefined} dailyAverageCostDisplay={overview && dashboard ? deviceCostDisplay(overview, dashboard.calendarDays) : undefined} aggregateMode usage={dashboard?.usage ?? null} dailyAverageUsage={dashboard?.usage ?? null} reserveDailyAverage loading={busy || !overview} sparklines={{ requests: null, tokens: null, rpm: null, tpm: null, cacheReadRate: null, cost: null }} />
        {dashboard ? <>
        <TokenActivityCard aggregateMode observedBuckets={dashboard.observedBuckets} activity={dashboard.activity} loading={busy} requestIdentity={JSON.stringify(filters)} />
        {dashboard && <details className="device-note device-coverage" data-device-coverage><summary>{dashboard.observedDays.toLocaleString()} of {dashboard.calendarDays.toLocaleString()} days have uploads · About gaps</summary><p>{dashboard.zeroDays} uploaded days report zero tokens. {(dashboard.calendarDays - dashboard.observedDays).toLocaleString()} days have no uploads, not confirmed zero usage. Dashed activity cells mean no upload; filled gray cells mean uploaded zero. Focus or tap a cell for its UTC date and coverage. Scroll the grid horizontally to reach later dates. Trends leave unreported days as gaps. Calendar-day averages include all selected days.</p></details>}
        <UsageComparisonCharts aggregateMode comparisons={dashboard?.comparisons} deviceComparisons={dashboard?.devices} observedBuckets={dashboard?.observedBuckets} formatBucketLabel={formatDeviceDay} loading={busy} isDark={isDark} isMobile={isMobile} />
        <section className={styles.statCard}><h2>Models</h2><p id="device-model-scroll" className="device-note">Scroll horizontally for all token and cost columns. Keyboard: focus the table region, then use ← / →.</p><div className="device-table" role="region" aria-label="Model usage, horizontally scrollable" aria-describedby="device-model-scroll" tabIndex={0}><table><thead><tr><th>Model</th><th>Input</th><th>Output</th><th>Cache read / write</th><th>Estimated USD</th></tr></thead><tbody>{overview?.models.map(m => <tr key={m.key}><td>{m.key || 'Unattributed (no model)'}</td><td>{number(m.tokens.input)}</td><td>{number(m.tokens.output)}</td><td>{number(m.tokens.cache_read)} / {number(m.tokens.cache_write)}</td><td>{deviceCostText(m)}</td></tr>)}</tbody></table></div><p className="device-note">Unattributed or extra tokens stay in the all-families and unknown selections, not a named family or fabricated cost. Named families include only uploaded model token categories.</p></section>
        </> : <section className={styles.statCard} data-device-overview-state={busy ? 'loading' : 'unavailable'} role="status"><h2>Usage details</h2><p className="device-note">{busy ? 'Loading usage details…' : 'Usage details unavailable. Refresh to retry.'}</p></section>}
      </>}
      {tab === 'analysis' && <>
        <details className="device-note device-coverage"><summary>Daily token and cost analysis · About coverage</summary><p>Only uploaded dates carry values; calendar gaps are not confirmed zero usage. The average is per uploaded day. Triangular points and dashed segments show known cost subtotals for incomplete days; days with no priced usage have no cost point. Unattributed includes tokens without a model or category. Select a narrower range if analysis exceeds 100 models/devices or 100,000 chart points.</p><p>Not available from daily reports: latency diagnostics, model speed, request-based model efficiency, credentials and provider composition.</p></details>
        <AnalysisPanel costDisplay={overview ? deviceCostDisplay(overview) : undefined} key={JSON.stringify([filters, range, customRange, allTime])} aggregateMode compositionDimensions={['api_key', 'model']} analysis={analysis} deviceUsage={analysis?.device_usage} loading={busy} isDark={isDark} isMobile={isMobile} />
        {!busy && overview && <AnalysisDetails data={overview} devices={devices} />}
      </>}
      {tab === 'history' && <section className={styles.statCard}><h2>Daily history</h2><p id="device-history-scroll" className="device-note">Scroll horizontally for all token and cost columns. Focus the table, then use ← / →.</p><div className="device-table" role="region" aria-label="Daily history, horizontally scrollable" aria-describedby="device-history-scroll" tabIndex={0}><table><thead><tr><th>UTC day</th><th>Tokens</th><th>Input</th><th>Output</th><th>Cache read / write</th><th>Estimated USD</th></tr></thead><tbody>{overview?.days.map(d => <tr key={d.key}><td>{d.key}</td><td>{number(d.total_tokens)}</td><td>{number(d.tokens.input)}</td><td>{number(d.tokens.output)}</td><td>{number(d.tokens.cache_read)} / {number(d.tokens.cache_write)}</td><td>{deviceCostText(d)}</td></tr>)}</tbody></table></div></section>}
      {tab === 'devices' && <section className={styles.statCard}><h2>Devices</h2><form onSubmit={e => { e.preventDefault(); void mutate(async () => { const result = await api.request<{ token: string }>('/devices', 'POST', { name }); setEnrollment(result.token); setName(''); }).catch(e => setError(message(e))); }}><label>Device name <input required pattern="[a-zA-Z0-9][a-zA-Z0-9._:+\-]{0,159}" value={name} onChange={e => setName(e.target.value)} /></label><Button type="submit" disabled={busy}>Enroll</Button></form>
        {enrollment && <div role="status"><p>Upload token (shown only here; enter privately into the installer):</p><output className="device-token">{enrollment}</output><Button onClick={() => setEnrollment('')}>Dismiss token</Button></div>}
        <p id="device-list-scroll" className="device-note">Scroll horizontally for upload status and actions. Focus the table region, then use ← / →.</p><div className="device-table" role="region" aria-label="Devices, horizontally scrollable" aria-describedby="device-list-scroll" tabIndex={0}><table><thead><tr><th>Device</th><th>Last upload received</th><th>Status</th><th>Action</th></tr></thead><tbody>{devices.map(d => <tr key={d.id}><td>{d.name}</td><td>{d.last_seen ? new Date(d.last_seen).toLocaleString() : 'Never'}</td><td>{d.revoked ? 'Revoked' : 'Active'}</td><td><Button disabled={d.revoked || busy} variant="danger" onClick={() => { if (confirm('Revoke uploads for ' + d.name + '? History is retained.')) void mutate(() => api.request('/devices/' + d.id, 'DELETE')).catch(e => setError(message(e))); }}>Revoke</Button></td></tr>)}</tbody></table></div><p className="device-note">Copied histories across devices overlap. Filter old/replacement devices rather than summing both. Enrollment is capped at 1,000, including revoked devices.</p></section>}
      {tab === 'settings' && <><p className="device-note">Keeper Model Pricing Settings. Use Add for uploaded models, or Sync Prices to select and review source rates. Saving immediately refreshes historical estimates and the model breakdown. Recalculate Costs is unnecessary: aggregate costs are recalculated on every read. Request-dependent rules produce unknown daily costs.</p><label>Additional model identifier <input value={manualModel} onChange={e => setManualModel(e.target.value)} /></label><Button disabled={!manualModel.trim()} onClick={() => { setModelOptions(old => [...new Set([...old, manualModel.trim()])].sort()); setManualModel(''); }}>Add to model choices</Button><p>{modelOptions.length} model choices · {prices.length} configured. Add is unavailable only while loading or when all choices already have settings; use Edit for those models.</p><PricingSettings pricing={pricing} aggregateMode onSyncPrices={() => setSyncPrices(true)} timezone="UTC" /></>}
      <CompletePricingSyncModal open={syncPrices} models={prices} onClose={() => setSyncPrices(false)} onRefreshPricing={refresh} onNotice={setError} transport={{ fetch: source => api.request('/pricing/sync?source=' + source), apply: input => api.request('/pricing/sync/apply', 'POST', input), models: async () => ({ models: await api.prices(), config_revision: 0 }) }} />
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
  const [checking, setChecking] = useState(true);
  useEffect(() => {
    let active = true;
    const candidate = new DeviceAPI();
    void candidate.request('/session').then(() => { if (active) setAPI(candidate); })
      .catch(e => { candidate.close(); if (active && !(e instanceof DeviceAPIError && e.status === 401)) setError(message(e)); })
      .finally(() => { if (active) setChecking(false); });
    return () => { active = false; candidate.close(); };
  }, []);
  useEffect(() => () => api?.close(), [api]);
  if (checking) return <div className="device-login" role="status">Checking your session…</div>;
  if (api) return <>{error && <p className="device-error" role="alert">{error}</p>}<Dashboard api={api} onLogout={() => {
    if (busy) return;
    setBusy(true); setError('');
    void api.request('/session/logout', 'POST').then(() => { api.close(); setAPI(null); setToken(''); })
      .catch(e => setError(message(e))).finally(() => setBusy(false));
  }} /></>;
  return <div className="device-login"><div className={styles.statCard}><h1>Device Usage Keeper</h1><p>Connect with the plugin admin token, not CPA credentials.</p><form onSubmit={e => { e.preventDefault(); if (busy) return; setBusy(true); setError(''); const candidate = new DeviceAPI(); const credential = token; setToken(''); void candidate.request('/session', 'POST', {token: credential}).then(() => setAPI(candidate)).catch(e => { candidate.close(); setError(message(e)); }).finally(() => setBusy(false)); }}><label>Admin token<input type="password" autoComplete="off" value={token} onChange={e => setToken(e.target.value)} required /></label><Button type="submit" disabled={busy}>Connect</Button></form>{error && <p role="alert">{error}</p>}<p>Stay signed in for 30 days on this browser. Only an HttpOnly session cookie is saved—not your admin token. Sign out on shared devices.</p></div></div>;
}
createRoot(document.getElementById('root')!).render(<I18nextProvider i18n={i18n}><App /></I18nextProvider>);
