import { IntlUtil } from '@/utils';

export type TrafficRangeKey = '24h' | '7d' | '30d';

const RANGE_CONFIG: Record<TrafficRangeKey, { seconds: number; bucket: number }> = {
  '24h': { seconds: 24 * 60 * 60, bucket: 30 * 60 },
  '7d': { seconds: 7 * 24 * 60 * 60, bucket: 3 * 60 * 60 },
  '30d': { seconds: 30 * 24 * 60 * 60, bucket: 12 * 60 * 60 },
};

function jalaliMonthStart(to: number, offset: number): number {
  const panelMidnight = new Date((to + offset) * 1000);
  panelMidnight.setUTCHours(0, 0, 0, 0);
  const dayFormatter = new Intl.DateTimeFormat('en-US-u-ca-persian-nu-latn', {
    day: 'numeric',
    timeZone: 'UTC',
  });

  // A Jalali month has at most 31 days. Searching panel-local midnights keeps
  // this independent of the administrator browser timezone and avoids any
  // Gregorian/Jalali conversion ambiguity around day boundaries.
  for (let daysBack = 0; daysBack < 31; daysBack += 1) {
    const candidate = new Date(panelMidnight);
    candidate.setUTCDate(candidate.getUTCDate() - daysBack);
    const day = Number(dayFormatter.formatToParts(candidate).find((part) => part.type === 'day')?.value);
    if (day === 1) return Math.floor(candidate.getTime() / 1000) - offset;
  }

  // Modern browsers used by the panel support the Persian calendar. Keep a
  // bounded fallback for unusual Intl builds instead of issuing an invalid API
  // range.
  return to - 30 * 24 * 60 * 60;
}

export function calculateRequestedRange(
  key: TrafficRangeKey,
  fromMidnight: boolean,
  useJalaliMonth: boolean,
  to: number,
  offset: number,
) {
  if (key === '30d' && useJalaliMonth) {
    return { from: jalaliMonthStart(to, offset), to, bucket: RANGE_CONFIG['30d'].bucket };
  }

  const config = RANGE_CONFIG[key];
  if (key !== '24h' || !fromMidnight) {
    return { from: to - config.seconds, to, bucket: config.bucket };
  }

  // Align midnight to the panel clock, not the administrator browser clock.
  const panelDate = new Date((to + offset) * 1000);
  panelDate.setUTCHours(0, 0, 0, 0);
  return { from: Math.floor(panelDate.getTime() / 1000) - offset, to, bucket: config.bucket };
}

export function requestedRange(key: TrafficRangeKey, fromMidnight: boolean, useJalaliMonth: boolean) {
  return calculateRequestedRange(
    key,
    fromMidnight,
    useJalaliMonth,
    Math.floor(Date.now() / 1000),
    IntlUtil.savedPanelOffsetSeconds(),
  );
}
