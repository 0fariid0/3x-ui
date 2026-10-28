import { describe, expect, it } from 'vitest';
import { remainingDaysAtChange, visibleClientEvents } from '@/lib/clients/history';

describe('focused client renewal history', () => {
  it('hides only the two link-setting kinds without mutating stored responses', () => {
    const events = ['renewed', 'subscription_links', 'traffic_reset', 'external_links', 'limits_changed', 'created'].map((kind, id) => ({ kind, id }));
    expect(visibleClientEvents(events).map((event) => event.kind)).toEqual(['renewed', 'traffic_reset', 'limits_changed', 'created']);
    expect(events).toHaveLength(6);
    expect(visibleClientEvents([{ kind: 'subscription_links' }, { kind: 'external_links' }])).toEqual([]);
  });

  it('measures remaining days from the event time, including expired and first-use cases', () => {
    const at = Date.parse('2026-10-28T12:00:00Z');
    expect(remainingDaysAtChange(at + 5 * 86400000, at)).toBe(5);
    expect(remainingDaysAtChange(at + 35 * 86400000, at)).toBe(35);
    expect(remainingDaysAtChange(at + 1, at)).toBe(1);
    expect(remainingDaysAtChange(at - 1, at)).toBe(0);
    expect(remainingDaysAtChange(0, at)).toBeNull();
    expect(remainingDaysAtChange(-30 * 86400000, at)).toBeNull();
    expect(remainingDaysAtChange(at, 0)).toBeNull();
  });
});
