import type { ModelPricingConfig } from '@/lib/types';
export interface Tokens { input: number; output: number; cache_read: number; cache_write: number }
export interface Aggregate { key: string; tokens: Tokens; total_tokens: number; known_cost: number; complete: boolean }
export interface Overview { breakdown: { day: string; models: Aggregate[]; devices: Aggregate[] }[]; total: Aggregate; days: Aggregate[]; models: Aggregate[]; devices: Aggregate[]; agents: string[]; collected_at: string; snapshots: number }
export interface Device { id: string; name: string; revoked: boolean; last_seen: string | null }
export interface Filters { device: string; agent: string; start: string; end: string }
// One in-memory credential per mounted application. No ambient cookie/session
// auth and no credentials in URLs, browser storage, console or exception bodies.
export class DeviceAPI {
  private controller = new AbortController();
  constructor(private token: string) {}
  close() { this.token = ''; this.controller.abort(); }
  async request<T>(path: string, method = 'GET', body?: unknown): Promise<T> {
    const response = await fetch('/v1' + path, { method, signal: this.controller.signal,
      credentials: 'omit', cache: 'no-store', redirect: 'error',
      headers: { Authorization: 'Bearer ' + this.token, 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body) });
    if (!response.ok) throw new Error(response.status === 401 ? 'Authentication expired or token invalid. Sign out and reconnect.' : `Request failed (${response.status}). Retry or narrow the date range.`);
    const value = response.status === 204 ? undefined : await response.json();
    this.controller.signal.throwIfAborted();
    return value as T;
  }
  overview(filters: Filters) { return this.request<Overview>('/overview?' + new URLSearchParams({ ...filters })); }
  devices() { return this.request<Device[]>('/devices'); }
  async prices() { return (await this.request<ModelPricingConfig[]>('/pricing/models')).map(p => ({ ...p, branches: p.branches ?? [], conditional_multipliers: p.conditional_multipliers ?? [] })); }
}
