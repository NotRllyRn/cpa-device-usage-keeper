import { afterEach, describe, expect, it, vi } from 'vitest';
import { DeviceAPI, DeviceAPIError } from './api';

afterEach(() => vi.unstubAllGlobals());
it('uses same-origin cookies without retaining or sending bearer credentials', async () => {
  const fetch = vi.fn().mockResolvedValue({ok:true,status:204});
  vi.stubGlobal('fetch',fetch);
  const api = new DeviceAPI();
  await api.request('/session','POST',{token:'synthetic-login-only'});
  await api.devices();
  const request = fetch.mock.calls[1][1];
  expect(request.credentials).toBe('same-origin');
  expect(request.headers).toEqual({'Content-Type':'application/json'});
  expect(request.body).toBeUndefined();
  expect(JSON.stringify(api)).not.toContain('synthetic-login-only');
  await api.request('/session/logout','POST');
  expect(fetch.mock.calls[2][0]).toBe('/v1/session/logout');
  expect(fetch.mock.calls[2][1].method).toBe('POST');
  api.close();
  expect(request.signal.aborted).toBe(true);
});
it('exposes only status errors, never server response or login credentials', async () => {
  vi.stubGlobal('fetch',vi.fn().mockResolvedValue({ok:false,status:401}));
  await expect(new DeviceAPI().request('/session')).rejects.toMatchObject({status:401});
  expect(new DeviceAPIError(401).message).toBe('Session expired. Sign out and reconnect.');
});
describe('device family API scope', () => {
  it('sends independent family and agent filters to both views and the All time boundary', async () => {
    const urls: URL[] = [];
    vi.stubGlobal('fetch', vi.fn(async (path: string) => {
      urls.push(new URL(path, 'https://example.invalid'));
      return { ok: true, status: 200, json: async () => ({ earliest_day: '2026-01-02' }) };
    }));
    const api = new DeviceAPI();
    const filters = { device: 'laptop', agent: 'custom-worker', family: 'openai', start: '2026-01-02', end: '2026-01-03' };
    await api.overview(filters);
    await api.overview(filters, true);
    await api.boundary(filters);
    expect(urls.map(u => u.pathname)).toEqual(['/v1/overview', '/v1/analysis', '/v1/usage-boundary']);
    for (const url of urls) {
      expect(url.searchParams.get('family')).toBe('openai');
      expect(url.searchParams.get('agent')).toBe('custom-worker');
      expect(url.searchParams.get('device')).toBe('laptop');
    }
    expect(urls[2].searchParams.has('start')).toBe(false);
    api.close();
  });
  it('keeps unknown and all distinct without deriving a serving provider', async () => {
    const paths: string[] = [];
    vi.stubGlobal('fetch', vi.fn(async (path: string) => {
      paths.push(path);
      return { ok: true, status: 200, json: async () => ({}) };
    }));
    const api = new DeviceAPI();
    for (const family of ['unknown', '']) await api.boundary({ device: '', agent: '', family });
    expect(paths).toEqual(['/v1/usage-boundary?device=&agent=&family=unknown', '/v1/usage-boundary?device=&agent=&family=']);
    api.close();
  });
});
