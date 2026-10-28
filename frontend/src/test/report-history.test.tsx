import { act, render, renderHook, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { HttpUtil, IntlUtil } from '@/utils';
import { setDatepicker, useDatepicker } from '@/hooks/useDatepicker';
import ClientHistoryDetails from '@/pages/clients/ClientHistoryDetails';

afterEach(() => vi.restoreAllMocks());

describe('live report calendar', () => {
  it('refreshes after focus, reacts to saves and rejects an older pending response', async () => {
    setDatepicker('gregorian');
    let finish: (value: { success: boolean; msg: string; obj: { datepicker: 'gregorian' } }) => void = () => {};
    vi.spyOn(HttpUtil, 'post').mockImplementationOnce(() => new Promise((resolve) => { finish = resolve; }));
    const { result } = renderHook(useDatepicker);
    act(() => window.dispatchEvent(new Event('focus')));
    act(() => setDatepicker('jalalian'));
    expect(result.current.datepicker).toBe('jalalian');
    await act(async () => finish({ success: true, msg: '', obj: { datepicker: 'gregorian' } }));
    expect(result.current.datepicker).toBe('jalalian');
    expect(IntlUtil.savedPanelCalendar()).toBe('jalalian');
    vi.spyOn(HttpUtil, 'post').mockResolvedValueOnce({ success: true, msg: '', obj: { datepicker: 'gregorian' } });
    act(() => window.dispatchEvent(new Event('focus')));
    await waitFor(() => expect(result.current.datepicker).toBe('gregorian'));
  });
});

describe('readable renewal details', () => {
  it('renders exact before/after quotas and Jalali expiry through the panel formatter', () => {
    const expiry = Date.parse('2026-10-27T12:00:00Z');
    render(<ClientHistoryDetails summary="Renewed" recordedAt={expiry - 5 * 86400000} details={JSON.stringify({ version: 1, previousTotalBytes: 10 * 1024 ** 3, totalBytes: 15 * 1024 ** 3, addedBytes: 5 * 1024 ** 3, previousExpiryTime: expiry, expiryTime: expiry + 3 * 86400000, addedMilliseconds: 3 * 86400000 })} formatDateTime={(value) => IntlUtil.formatPanelDateTime(value, 'jalalian', 12600)} />);
    expect(screen.getByText('Previous quota')).toBeTruthy();
    expect(screen.getByText('10.00 GB')).toBeTruthy();
    expect(screen.getByText('15.00 GB')).toBeTruthy();
    expect(screen.getByText('+5.00 GB')).toBeTruthy();
    expect(screen.getByText(/1405\/08\/05/)).toBeTruthy();
    expect(screen.getByText(/1405\/08\/08/)).toBeTruthy();
    expect(screen.getByText('+5.00 GB').getAttribute('dir')).toBe('ltr');
    expect(screen.queryByText(/previousTotalBytes/)).toBeNull();
    expect(screen.getByText('Previous days remaining')).toBeTruthy();
    expect(screen.getByText('5 days')).toBeTruthy();
    expect(screen.getByText('New days remaining')).toBeTruthy();
    expect(screen.getByText('8 days')).toBeTruthy();
  });

  it('keeps unlimited and delayed validity distinct from active remaining days', () => {
    render(<ClientHistoryDetails summary="Changed" recordedAt={Date.now()} details={JSON.stringify({ version: 1, previousExpiryTime: 0, expiryTime: -30 * 86400000, addedMilliseconds: null })} formatDateTime={String} />);
    expect(screen.getByText(/30 days after first connection/)).toBeTruthy();
    expect(screen.queryByText('New days remaining')).toBeNull();
  });

  it('shows remaining days and a zero duration change for quota-only renewals', () => {
    const at = Date.parse('2026-10-28T12:00:00Z');
    render(<ClientHistoryDetails summary="Quota increased" recordedAt={at} details={JSON.stringify({ version: 1, previousExpiryTime: at + 10 * 86400000, expiryTime: at + 10 * 86400000, addedMilliseconds: 0 })} formatDateTime={String} />);
    expect(screen.getAllByText('10 days')).toHaveLength(2);
    expect(screen.getByText('0 days')).toBeTruthy();
  });
});
