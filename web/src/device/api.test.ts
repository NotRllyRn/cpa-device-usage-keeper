import { afterEach, expect, it, vi } from 'vitest';
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
