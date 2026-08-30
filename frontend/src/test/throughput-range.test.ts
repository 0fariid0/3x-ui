import { describe, expect, it } from 'vitest';

import { calculateRequestedRange } from '@/pages/index/trafficRange';

const NOW_SECONDS = Date.parse('2026-08-30T12:34:56Z') / 1000;
const PANEL_OFFSET_SECONDS = 3.5 * 60 * 60;

describe('dashboard traffic ranges', () => {
  it('applies the 00:00 option only to the 24-hour report', () => {
    const rolling24h = calculateRequestedRange('24h', false, false, NOW_SECONDS, PANEL_OFFSET_SECONDS);
    const calendarDay = calculateRequestedRange('24h', true, false, NOW_SECONDS, PANEL_OFFSET_SECONDS);
    const rolling7d = calculateRequestedRange('7d', true, false, NOW_SECONDS, PANEL_OFFSET_SECONDS);

    expect(rolling24h.from).toBe(NOW_SECONDS - 24 * 60 * 60);
    expect(calendarDay.from).toBe(Date.parse('2026-08-29T20:30:00Z') / 1000);
    expect(rolling7d.from).toBe(NOW_SECONDS - 7 * 24 * 60 * 60);
  });

  it('switches only the 30-day report to the current Jalali month', () => {
    const rolling30d = calculateRequestedRange('30d', false, false, NOW_SECONDS, PANEL_OFFSET_SECONDS);
    const jalaliMonth = calculateRequestedRange('30d', false, true, NOW_SECONDS, PANEL_OFFSET_SECONDS);

    expect(rolling30d.from).toBe(NOW_SECONDS - 30 * 24 * 60 * 60);
    expect(jalaliMonth.from).toBe(Date.parse('2026-08-22T20:30:00Z') / 1000);
    expect(jalaliMonth.to).toBe(NOW_SECONDS);
  });
});
