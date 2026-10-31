import { beforeEach, expect, it, vi } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import IPBlockTab from '@/pages/settings/IPBlockTab';
import { HttpUtil } from '@/utils';
import { renderWithProviders, chooseSelectOption } from './test-utils';

const policy = { enabled: true, banSeconds: 1800, maxRetry: 1, findSeconds: 32, scanSeconds: 10, liveSeconds: 120, historySeconds: 1800, disconnect: false, exemptIPs: '' };
beforeEach(() => {
  vi.restoreAllMocks();
  vi.spyOn(HttpUtil, 'get').mockImplementation(async (url) => url.includes('/nodes/list') ? { success: true, msg: '', obj: [] } : url.includes('/overview') ? {success:true,msg:'',obj:{checkedAt:Date.now(),health:{checkedAt:0,lastSuccess:0,available:false,stale:true,reason:''},checks:{},events:[],history:[]}} : { success: true, msg: '', obj: { settings: policy, available: true, reason: '' } });
  vi.spyOn(HttpUtil, 'post').mockImplementation(async (_url, value) => ({ success: true, msg: '', obj: { settings: value, available: true, reason: '' } }));
});
it('sends changed ban duration as JSON to the selected local server', async () => {
  renderWithProviders(<IPBlockTab />);
  const duration = await screen.findByLabelText('Ban duration (seconds)');
  fireEvent.change(duration, { target: { value: '600' } });
  fireEvent.click(screen.getByRole('button', { name: 'Save IP block settings' }));
  await waitFor(() => expect(HttpUtil.post).toHaveBeenCalledWith('/panel/api/server/ipBlocks/settings?nodeId=0', expect.objectContaining({ banSeconds: 600, enabled: true, disconnect: false }), { headers: { 'Content-Type': 'application/json' } }));
});
it('preserves edits and displays the server error when application fails', async () => {
  vi.spyOn(HttpUtil, 'post').mockResolvedValue({ success: false, msg: 'permission denied', obj: null });
  renderWithProviders(<IPBlockTab />);
  const duration = await screen.findByLabelText('Ban duration (seconds)');
  fireEvent.change(duration, { target: { value: '900' } });
  fireEvent.click(screen.getByRole('button', { name: 'Save IP block settings' }));
  await screen.findByText('permission denied');
  expect((duration as HTMLInputElement).value).toBe('900');
  expect((screen.getByRole('button', { name: 'Save IP block settings' }) as HTMLButtonElement).disabled).toBe(false);
});
it('does not offer writable defaults when the live settings request fails', async () => {
  vi.spyOn(HttpUtil, 'get').mockResolvedValue({ success: false, msg: 'node offline', obj: null });
  renderWithProviders(<IPBlockTab />);
  await screen.findByText('node offline');
  expect(screen.queryByRole('button', { name: 'Save IP block settings' })).toBeNull();
  expect(vi.mocked(HttpUtil.post).mock.calls.some(([url]) => url.includes('/ipBlocks/settings'))).toBe(false);
});

it('saves the explicitly selected account/IP engine', async () => {
 renderWithProviders(<IPBlockTab />);
 await screen.findByLabelText('Enforcement method');
 chooseSelectOption('enforcementScope', 'Account + IP (Fara core)');
 fireEvent.click(screen.getByRole('button', {name:'Save IP block settings'}));
 await waitFor(()=>expect(HttpUtil.post).toHaveBeenCalledWith('/panel/api/server/ipBlocks/settings?nodeId=0',expect.objectContaining({enforcementScope:'client'}),expect.anything()));
});
it('serializes a named temporary exemption without exposing JSON editing',async()=>{
 renderWithProviders(<IPBlockTab />);
 fireEvent.click(await screen.findByText('Add exemption'));
 fireEvent.change(screen.getByLabelText('Name'),{target:{value:'office'}});
 fireEvent.change(screen.getByLabelText('IP or CIDR'),{target:{value:'192.0.2.0/24'}});
 fireEvent.change(screen.getByLabelText('Lifetime (minutes, 0 = permanent)'),{target:{value:'10'}});
 fireEvent.click(screen.getByText('Save IP block settings').closest('button')!);
 await waitFor(()=>expect(vi.mocked(HttpUtil.post).mock.calls.some(([url])=>url.includes('/ipBlocks/settings'))).toBe(true));
 const call=vi.mocked(HttpUtil.post).mock.calls.find(([url])=>url.includes('/ipBlocks/settings'))!;
 const payload=call[1] as {namedExemptions:string};const rules=JSON.parse(payload.namedExemptions);
 expect(rules[0]).toMatchObject({name:'office',address:'192.0.2.0/24'});
 expect(rules[0].expiresAt).toBeGreaterThan(Date.now()+9*60000);
}, 15000);
