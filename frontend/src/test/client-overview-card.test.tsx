import { fireEvent, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, expect, it, vi } from 'vitest';

import ClientOverviewCard from '@/pages/index/ClientOverviewCard';
import { renderWithProviders } from '@/test/test-utils';
import { HttpUtil, Msg } from '@/utils';

afterEach(() => vi.restoreAllMocks());

it('shows global counts, handles an unavailable summary, and retries without showing false zeros', async () => {
  const get = vi.spyOn(HttpUtil, 'get')
    .mockResolvedValueOnce(new Msg(false, 'Unavailable', null))
    .mockResolvedValueOnce(new Msg(true, '', {
      items: [{ email: 'the-only-row-on-this-page' }],
      summary: { total: 760, onlineCount: 120, active: 275, expiringCount: 0,
        depletedCount: 483, deactiveCount: 2, online: [], depleted: [], expiring: [], deactive: [] },
    }));
  const { container } = renderWithProviders(<MemoryRouter><ClientOverviewCard /></MemoryRouter>);
  const counts = () => [...container.querySelectorAll('dd')].map((item) => item.textContent);
  const retry = await screen.findByRole('button');
  expect(counts()).toEqual(Array(6).fill('—'));
  expect(screen.getByRole('link').getAttribute('href')).toBe('/clients');

  fireEvent.click(retry);
  await screen.findByText('760', { exact: true });
  expect(counts()).toEqual(['760', '120', '275', '0', '483', '2']);
  expect(get.mock.calls[1]?.slice(0, 2)).toEqual([
    '/panel/api/clients/list/paged', { page: 1, pageSize: 1 },
  ]);
});
