import { describe, expect, it } from 'vitest';
import { remainingDaysAtChange, visibleClientEvents } from '@/lib/clients/history';

describe('focused client renewal history', () => {
  it('hides generic settings and link-setting events without mutating stored responses', () => {
    const events = ['renewed', 'subscription_links', 'traffic_reset', 'external_links', 'limits_changed', 'created', 'updated'].map((kind, id) => ({ kind, id }));
    expect(visibleClientEvents(events).map((event) => event.kind)).toEqual(['renewed', 'traffic_reset', 'limits_changed', 'created']);
    expect(events).toHaveLength(7);
    expect(visibleClientEvents([{ kind: 'subscription_links' }, { kind: 'external_links' }, {kind:'updated'}])).toEqual([]);
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

import { isServiceChange } from '@/lib/clients/history';
it('separates IP and unchanged settings from quota, expiry and inbound changes',()=>{
 for(const kind of ['ip_ban','ip_unban','ip_limit_decision','ip_blocked','ip_unblocked','ip_history_cleared']) expect(isServiceChange({kind})).toBe(false);
 for(const kind of ['renewed','limits_changed','attached','detached','updated','created','enabled','disabled','traffic_reset','traffic_adjusted']) expect(isServiceChange({kind})).toBe(true);
 expect(isServiceChange({kind:'updated',details:JSON.stringify({previousTotalBytes:0,totalBytes:0,previousExpiryTime:0,expiryTime:0})})).toBe(true);
 expect(isServiceChange({kind:'updated',details:JSON.stringify({previousTotalBytes:10,totalBytes:20})})).toBe(true);
});
