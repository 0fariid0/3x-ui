import { beforeEach, expect, it, vi } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import IPBlockTab from '@/pages/settings/IPBlockTab';
import { HttpUtil } from '@/utils';
import { renderWithProviders } from './test-utils';

const policy = { enabled: true, banSeconds: 1800, maxRetry: 1, findSeconds: 32, scanSeconds: 10, liveSeconds: 120, historySeconds: 1800, disconnect: false, exemptIPs: '' };
beforeEach(() => {
  vi.restoreAllMocks();
  vi.spyOn(HttpUtil, 'get').mockImplementation(async (url) => url.includes('/nodes/list') ? { success: true, msg: '', obj: [] } : { success: true, msg: '', obj: { settings: policy, available: true, reason: '' } });
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
  expect(HttpUtil.post).not.toHaveBeenCalled();
});
