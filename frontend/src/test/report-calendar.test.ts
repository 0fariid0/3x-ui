import { afterEach, describe, expect, it, vi } from 'vitest';

import { IntlUtil } from '@/utils';
import { normalizeTranslationPlaceholders } from '@/i18n/placeholders';
import { parseClientEventDetails } from '@/schemas/api/client-event';
import { normalizeClientIps } from '@/lib/clients/ip-log';

afterEach(() => vi.unstubAllGlobals());

describe('report calendar preference', () => {
  it('applies Jalali to default report formatters and preserves explicit Gregorian', () => {
    vi.stubGlobal('document', { cookie: 'lang=en-US' });
    const values = new Map<string, string>();
    vi.stubGlobal('localStorage', {
      getItem: (key: string) => values.get(key) ?? null,
      setItem: (key: string, value: string) => values.set(key, value),
    });
    IntlUtil.setPanelCalendar('jalalian');
    const timestamp = Date.parse('2026-10-27T21:00:00Z');
    expect(IntlUtil.formatDate(timestamp)).toContain('1405');
    expect(IntlUtil.formatPanelDate(timestamp)).toContain('1405/08/05');
    expect(IntlUtil.formatPanelDate(timestamp, 'jalalian', 12600)).toContain('1405/08/06');
    expect(IntlUtil.formatPanelDayKey('2026-10-27')).toContain('1405/08/05');
    expect(IntlUtil.formatDate(timestamp, 'gregorian')).toContain('2026');
    expect(IntlUtil.formatDate(timestamp / 1000)).toBe(IntlUtil.formatDate(timestamp));
    IntlUtil.setPanelCalendar('gregorian');
    expect(IntlUtil.formatDate(timestamp)).toContain('2026');
  });

  it('handles unavailable storage and invalid dates', () => {
    vi.stubGlobal('localStorage', { getItem: () => { throw new Error('disabled'); } });
    expect(IntlUtil.savedPanelCalendar()).toBe('gregorian');
    expect(IntlUtil.formatDate('invalid')).toBe('');
    expect(IntlUtil.formatPanelDateTime(null)).toBe('');
  });
});

describe('report translations and history details', () => {
  it('preserves legacy IP responses and accepts absolute timestamps for calendar rendering', () => {
    expect(normalizeClientIps([{ ip: '1.1.1.1', time: '2026-10-27 12:00:00', node: '', timestamp: 1793102400000 }]))
      .toEqual([{ ip: '1.1.1.1', time: '2026-10-27 12:00:00', node: '', timestamp: 1793102400000 }]);
    expect(normalizeClientIps(['1.1.1.1 (old time)'])).toEqual([{ ip: '1.1.1.1 (old time)', time: '', node: '' }]);
  });
  it('normalizes legacy interpolation while preserving subscription remark variables', () => {
    expect(normalizeTranslationPlaceholders({ text: '{{hours}} hours / {days} days', remark: '{{EMAIL}} {{TOTAL}}', nested: ['{{count}}'] }))
      .toEqual({ text: '{hours} hours / {days} days', remark: '{{EMAIL}} {{TOTAL}}', nested: ['{count}'] });
  });

  it('validates detailed history and strips unrelated sensitive payload fields', () => {
    expect(parseClientEventDetails(JSON.stringify({ version: 1, previousTotalBytes: 1024, totalBytes: 2048, addedBytes: 1024, previousExpiryTime: 0, expiryTime: 0, addedMilliseconds: null, password: 'secret' })))
      .toEqual({ version: 1, previousTotalBytes: 1024, totalBytes: 2048, addedBytes: 1024, previousExpiryTime: 0, expiryTime: 0, addedMilliseconds: null });
    expect(parseClientEventDetails('invalid')).toBeNull();
    expect(parseClientEventDetails('{"totalBytes":"invalid"}')).toBeNull();
  });
});
