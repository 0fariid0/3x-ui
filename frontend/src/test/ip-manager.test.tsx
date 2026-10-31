import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { HttpUtil } from '@/utils';
import ClientIpManagerModal from '@/pages/clients/ClientIpManagerModal';
import { renderWithProviders } from './test-utils';

const mock = vi.hoisted(() => ({ mutateAsync: vi.fn(), error: false, data: { available: true, banSeconds: 1800, blocks: [{ ip: '192.0.2.99', source: 'automatic', email: 'other', bannedAt: 1, expiresAt: 2, permanent: false }] } }));
vi.mock('@/hooks/usePanelDateTime', () => ({ usePanelDateTime: () => ({ formatDateTime: () => '1405/08/08 12:00' }) }));
vi.mock('@/api/queries/useIPBlocks', () => ({ useIPBlocks: () => ({
  query: { data: mock.data, isError: mock.error, error: new Error('offline'), isFetching: false, refetch: vi.fn() },
  mutation: { mutateAsync: mock.mutateAsync, isPending: false, isError: false, reset: vi.fn() },
}) }));

function show() {
  renderWithProviders(<ClientIpManagerModal open email="client" ips={[{ ip: '192.0.2.1', node: '', nodeId: 0, time: '' }, { ip: '192.0.2.2', node: 'remote', nodeId: 9, time: '' }]} loading={false} clearing={false} onRefresh={vi.fn()} onClear={vi.fn()} onClose={vi.fn()} />);
}

beforeEach(() => { vi.spyOn(HttpUtil, 'get').mockImplementation(async <T,>(url: string) => ({ success: true, msg: '', obj: (url.includes('/ipBlocks/clients?') ? { ip: new URL(url, 'http://localhost').searchParams.get('ip'), clients: [{ email: 'client', lastSeen: 1 }, { email: url.includes('nodeId=9') ? 'remote-peer' : 'shared-peer', lastSeen: 2 }], windowSeconds: 1800, checkedAt: 3 } : []) as T })); mock.error = false; mock.mutateAsync.mockReset().mockResolvedValue({ auditWarning: false }); });

describe('IP manager', () => {
  it('lists automatic blocks that are absent from the client report and confirms unblock', async () => {
    show();
    fireEvent.click(screen.getByText(/All active server blocks/));
    expect(screen.getByText('192.0.2.99')).toBeTruthy();
    expect(screen.getByText('Automatic IP limit')).toBeTruthy();
    fireEvent.click(screen.getByText('Unblock').closest('button')!);
    expect(mock.mutateAsync).not.toHaveBeenCalled();
    const dialog = screen.getByText('Unblock this IP?').closest('[role=dialog]') as HTMLElement;
    expect(within(dialog).getAllByText('This server').length).toBeGreaterThan(0);
    await waitFor(() => expect(within(dialog).getByText('shared-peer')).toBeTruthy());
    fireEvent.click(within(dialog).getByText('Unblock').closest('button')!);
    await waitFor(() => expect(mock.mutateAsync).toHaveBeenCalledWith(expect.objectContaining({ nodeId: 0, ip: '192.0.2.99', action: 'unban', email: 'other' })));
  });
  it('does not allow a remote observation to act on the local server', () => {
    show();
    const row = screen.getByText('192.0.2.2').closest('tr')!;
    expect((within(row).getByText('Block').closest('button') as HTMLButtonElement).disabled).toBe(true);
    const localRow = screen.getByText('192.0.2.1').closest('tr')!;
    expect((within(localRow).getByText('Block').closest('button') as HTMLButtonElement).disabled).toBe(false);
  });
  it('disables actions when live status fails instead of reporting unblocked', () => {
    mock.error = true;
    show();
    const row = screen.getByText('192.0.2.1').closest('tr')!;
    expect(within(row).getByText('Unknown')).toBeTruthy();
    expect((within(row).getByText('Block').closest('button') as HTMLButtonElement).disabled).toBe(true);
  });
  it('looks up a remote IP by exact node ID', async () => {
    show();
    const row = screen.getByText('192.0.2.2').closest('tr')!;
    fireEvent.click(within(row).getByText('Show users').closest('button')!);
    await waitFor(() => expect(screen.getByText('remote-peer')).toBeTruthy());
    expect(HttpUtil.get).toHaveBeenCalledWith('/panel/api/server/ipBlocks/clients?nodeId=9&ip=192.0.2.2', undefined, { silent: true });
    expect(mock.mutateAsync).not.toHaveBeenCalled();
  });
  it('does not present a lookup failure as absence of other users', async () => {
    vi.spyOn(HttpUtil, 'get').mockResolvedValue({ success: false, msg: 'node offline', obj: null });
    show();
    const row = screen.getByText('192.0.2.1').closest('tr')!;
    fireEvent.click(within(row).getByText('Show users').closest('button')!);
    await waitFor(() => expect(screen.getByText('Could not check users sharing this IP. Their absence is not confirmed.')).toBeTruthy());
    expect(screen.queryByText('No recent users were found in this server’s IP report.')).toBeNull();
  });

  it('keeps tunnel IPs in a compact strip without actions or client slots', () => {
    renderWithProviders(<ClientIpManagerModal open email="client" ips={[{ ip: '10.30.3.1', node: '', nodeId: 0, time: '' }, { ip: '5.210.158.87', node: '', nodeId: 0, time: '' }]} loading={false} clearing={false} onRefresh={vi.fn()} onClear={vi.fn()} onClose={vi.fn()} />);
    const address = screen.getByText('10.30.3.1');
    const strip = address.closest('.ip-manager-local')!;
    expect(strip).not.toBeNull();
    expect(strip.querySelector('button')).toBeNull();
    expect(address.closest('tr')).toBeNull();
    expect(screen.getByText('Client IPs (1)')).toBeTruthy();
    const row = screen.getByText('5.210.158.87').closest('tr')!;
    expect(within(row).getByText('Block')).toBeTruthy();
  });

});
