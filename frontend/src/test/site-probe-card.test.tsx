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
        direct: { status: 'ok', delay: 28 }, routed: { status: 'timeout' } },
    ],
  };
  const get = vi.spyOn(HttpUtil, 'get').mockResolvedValue({ success: true, obj: response, msg: '' } as never);
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
    expect(within(card).getByText('Timeout')).toBeTruthy();
    expect(within(card).queryByText('YouTube')).toBeNull();
    fireEvent.click(within(card).getByRole('button', { name: 'Run checks' }));
    const dialog = await screen.findByRole('dialog');
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
