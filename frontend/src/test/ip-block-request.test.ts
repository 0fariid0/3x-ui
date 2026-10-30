import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { http, HttpResponse } from 'msw';
import { changeIPBlock } from '@/api/queries/useIPBlocks';
import { setupHttp } from '@/api/http-init';
import { server } from './msw/server';
const origin = 'http://localhost';
beforeEach(() => {
  vi.stubGlobal('document', { querySelector: () => null });
  window.X_UI_BASE_PATH = origin;
  setupHttp();
  server.use(http.get(`${origin}/csrf-token`, () => HttpResponse.json({ success: true, obj: 'csrf-test' })));
});
afterEach(() => { vi.unstubAllGlobals(); delete window.X_UI_BASE_PATH; });
describe('IP mutation HTTP contract', () => {
  it.each(['ban', 'unban'] as const)('sends %s as JSON through the real HTTP helper', async (action) => {
    let body: unknown;
    let contentType: string | null = null;
    let token: string | null = null;
    server.use(http.post(`${origin}/panel/api/server/ipBlocks/change`, async ({ request }) => {
      contentType = request.headers.get('Content-Type');
      token = request.headers.get('X-CSRF-Token');
      body = await request.json();
      return HttpResponse.json({ success: true, obj: { available: true, reason: '', jail: '3x-ipl', banSeconds: 1800, blocks: [], auditWarning: false } });
    }));
    await changeIPBlock({ nodeId: 7, ip: '2001:0db8::1', action, email: 'کاربر ۱۲' });
    expect(contentType).toBe('application/json');
    expect(token).toBe('csrf-test');
    expect(body).toEqual({ nodeId: 7, ip: '2001:db8::1', action, email: 'کاربر ۱۲' });
  });
  it('preserves server errors', async () => {
    server.use(http.post(`${origin}/panel/api/server/ipBlocks/change`, () => HttpResponse.json({ success: false, msg: 'Fail2ban jail unavailable' })));
    await expect(changeIPBlock({ nodeId: 0, ip: '192.0.2.1', action: 'ban', email: '' })).rejects.toThrow('Fail2ban jail unavailable');
  });
});
