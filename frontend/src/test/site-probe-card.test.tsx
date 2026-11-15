import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import SiteProbeCard from '@/pages/index/SiteProbeCard';
import { HttpUtil } from '@/utils';
import { renderWithProviders } from './test-utils';

it('shows only favorites in the overview and sends the saved order and manual run request', async () => {
  const targets = [
    { name: 'Google', url: 'https://google.com/', starred: true },
    { name: 'YouTube', url: 'https://youtube.com/', starred: false },
  ];
  const response = {
    targets, running: false, results: [
      { url: targets[0].url, at: 1787000000000,
        ping: { status: 'ok', delay: 28 } },
    ],
  };
  const get = vi.spyOn(HttpUtil, 'get').mockImplementation(async path =>
    ({ success: true, obj: path.endsWith('/outboundOptions') ? [] : response, msg: '' }) as never);
  const post = vi.spyOn(HttpUtil, 'post').mockImplementation(async (path, data) => ({
    success: true, msg: '', obj: path.endsWith('/save')
      ? { ...response, ...(data as object) }
      : { ...response, running: true },
  }) as never);
  try {
    renderWithProviders(<SiteProbeCard />);
    const card = screen.getByRole('region', { name: 'Site connectivity' });
    await within(card).findByText('Google');
    expect(within(card).getByText('28 ms')).toBeTruthy();
    expect(within(card).getAllByText(/\d+ ms/)).toHaveLength(1);
    expect(within(card).queryByText('Direct')).toBeNull();
    expect(within(card).queryByText('Xray route')).toBeNull();
    expect(within(card).queryByText('YouTube')).toBeNull();
    fireEvent.click(within(card).getByRole('button', { name: 'Site check settings' }));
    const dialog = await screen.findByRole('dialog');
    expect(post.mock.calls.filter(([path]) => path.includes('/siteProbes/'))).toHaveLength(0);
    expect(within(dialog).getByText('YouTube')).toBeTruthy();
    fireEvent.click(within(dialog).getByRole('button', { name: 'Move up: YouTube' }));
    await waitFor(() => expect(post).toHaveBeenCalledWith(
      expect.stringContaining('/save'), { targets: [targets[1], targets[0]] },
      expect.objectContaining({ headers: { 'Content-Type': 'application/json' } }),
    ));
    fireEvent.click(within(dialog).getByRole('button', { name: 'Test site: Google' }));
    await waitFor(() => expect(post).toHaveBeenCalledWith(
      expect.stringContaining('/run'), { url: targets[0].url }, expect.any(Object),
    ));
  } finally {
    get.mockRestore();
    post.mockRestore();
  }
});

it('starts a test of the full saved list from the card without opening settings', async () => {
  const view = { targets: [{ name: 'Google', url: 'https://google.com/', starred: true },
    { name: 'YouTube', url: 'https://youtube.com/', starred: false }], results: [], running: false };
  const get = vi.spyOn(HttpUtil, 'get').mockResolvedValue({ success: true, obj: view } as never);
  const post = vi.spyOn(HttpUtil, 'post').mockResolvedValue({
    success: true, obj: { ...view, running: true },
  } as never);
  try {
    renderWithProviders(<SiteProbeCard />);
    const card = screen.getByRole('region', { name: 'Site connectivity' });
    await within(card).findByText('Google');
    const run = within(card).getByRole('button', { name: 'Test all' });
    fireEvent.click(run);
    await waitFor(() => expect(post.mock.calls.filter(([path]) => path.includes('/siteProbes/'))).toEqual([
      [expect.stringContaining('/run'), { url: '' }, expect.any(Object)],
    ]));
    expect(screen.queryByRole('dialog')).toBeNull();
    await waitFor(() => expect(card.getAttribute('aria-busy')).toBe('true'));
    expect(run.hasAttribute('disabled')).toBe(true);
  } finally {
    get.mockRestore();
    post.mockRestore();
  }
});

it('saves an outbound mode and tests an outbound-only dashboard with the same main button', async () => {
  const view = {
    targets: [], results: [], running: false,
    outboundTargets: [{ tag: 'VLESS Berlin', mode: 'tcp' }],
    outboundResults: [{ tag: 'VLESS Berlin', mode: 'tcp', ping: { status: 'ok', delay: 43 } }],
  };
  const get = vi.spyOn(HttpUtil, 'get').mockImplementation(async path =>
    ({ success: true, obj: path.endsWith('/outboundOptions') ? [{ tag: 'VLESS Berlin', protocol: 'vless' }] : view }) as never);
  const post = vi.spyOn(HttpUtil, 'post').mockImplementation(async (path, data) => ({
    success: true,
    obj: path.endsWith('/outbounds')
      ? { ...view, outboundTargets: (data as { targets: unknown[] }).targets, outboundResults: [] }
      : { ...view, running: true },
  }) as never);
  try {
    renderWithProviders(<SiteProbeCard />);
    const card = screen.getByRole('region', { name: 'Site connectivity' });
    await within(card).findByText('VLESS Berlin');
    expect(within(card).getByText('43 ms').closest('.site-probe-chip--outbound')).toBeTruthy();
    expect(within(card).queryByText('Site connectivity')).toBeNull();
    fireEvent.click(within(card).getByRole('button', { name: 'Site check settings' }));
    const dialog = await screen.findByRole('dialog');
    fireEvent.click(within(dialog).getByRole('radio', { name: 'HTTP' }));
    await waitFor(() => expect(post).toHaveBeenCalledWith(
      expect.stringContaining('/outbounds'), { targets: [{ tag: 'VLESS Berlin', mode: 'http' }] }, expect.any(Object),
    ));
    await waitFor(() => expect(within(card).queryByText('43 ms')).toBeNull());
    fireEvent.click(within(dialog).getByRole('button', { name: /close/i }));
    fireEvent.click(within(card).getByRole('button', { name: 'Test all' }));
    await waitFor(() => expect(post).toHaveBeenCalledWith(
      expect.stringContaining('/run'), { url: '' }, expect.any(Object),
    ));
  } finally {
    get.mockRestore();
    post.mockRestore();
  }
});
