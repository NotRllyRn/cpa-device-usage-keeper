import type { ModelPricingConfig } from '@/lib/types';
export interface Tokens { input: number; output: number; cache_read: number; cache_write: number }
export interface Aggregate { coverage?: { priced: number; missing: number; conditional: number; unattributed_tokens: number }; key: string; tokens: Tokens; total_tokens: number; known_cost: number; complete: boolean }
export interface Overview { cells?: (Aggregate & { device: string; model: string })[]; breakdown: { day: string; models: Aggregate[]; devices: Aggregate[] }[]; total: Aggregate; days: Aggregate[]; models: Aggregate[]; devices: Aggregate[]; agents: string[]; collected_at: string; snapshots: number }
export interface Device { id: string; name: string; revoked: boolean; last_seen: string | null }
export interface Filters { device: string; agent: string; start: string; end: string }
export class DeviceAPIError extends Error {
  constructor(public status: number) { super(status === 401 ? 'Session expired. Sign out and reconnect.' : `Request failed (${status}). Retry or narrow the date range.`); }
}
// The browser owns an HttpOnly opaque session cookie. Never retain the admin
// token in this client, browser storage, URLs, console or exception bodies.
export class DeviceAPI {
  private controller = new AbortController();
  close() { this.controller.abort(); }
  async request<T>(path: string, method = 'GET', body?: unknown): Promise<T> {
    const response = await fetch('/v1' + path, { method, signal: this.controller.signal,
      credentials: 'same-origin', cache: 'no-store', redirect: 'error',
      headers: { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body) });
    if (!response.ok) throw new DeviceAPIError(response.status);
    const value = response.status === 204 ? undefined : await response.json();
    this.controller.signal.throwIfAborted();
    return value as T;
  }
  async overview(filters: Filters, analysis = false) {
    const data = await this.request<Overview>((analysis ? '/analysis?' : '/overview?') + new URLSearchParams({ ...filters }));
    return { ...data, days: data.days ?? [], models: data.models ?? [], devices: data.devices ?? [], agents: data.agents ?? [], breakdown: data.breakdown ?? [] };
  }
  boundary({ device, agent }: Pick<Filters, 'device' | 'agent'>) { return this.request<{ earliest_day: string | null }>('/usage-boundary?' + new URLSearchParams({ device, agent })); }
  devices() { return this.request<Device[]>('/devices'); }
  async prices() { return (await this.request<ModelPricingConfig[]>('/pricing/models')).map(p => ({ ...p, branches: p.branches ?? [], conditional_multipliers: p.conditional_multipliers ?? [] })); }
}
