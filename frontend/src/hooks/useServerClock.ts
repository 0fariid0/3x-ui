import { useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { z } from 'zod';

import { HttpUtil, IntlUtil } from '@/utils';
import { useDatepicker } from '@/hooks/useDatepicker';

const RestartStatusSchema = z.object({
  serverUnix: z.number().int(),
  timezone: z.enum(['local', 'tehran']),
  timezoneLabel: z.string(),
  offsetSeconds: z.number().int(),
  lastRestartAt: z.number().int(),
  lastTarget: z.string(),
  lastSuccess: z.boolean(),
  lastMessage: z.string(),
});

export type ScheduledRestartStatus = z.infer<typeof RestartStatusSchema>;

async function fetchRestartStatus(): Promise<ScheduledRestartStatus> {
  const msg = await HttpUtil.get('/panel/api/xray/restart-status', undefined, { silent: true });
  if (!msg.success) throw new Error(msg.msg || 'Failed to load server clock');
  const parsed = RestartStatusSchema.safeParse(msg.obj);
  if (!parsed.success) throw new Error('Malformed server clock response');
  return parsed.data;
}

function shiftedDate(unixSeconds: number, offsetSeconds: number, elapsedMs = 0): Date {
  return new Date((unixSeconds + offsetSeconds) * 1000 + elapsedMs);
}

function formatClock(date: Date): string {
  return new Intl.DateTimeFormat('en-GB', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
    timeZone: 'UTC',
  }).format(date);
}

function formatDateTime(date: Date, calendar: 'gregorian' | 'jalalian'): string {
  return IntlUtil.formatPanelDateTime(date, calendar, 0);
}


export function useServerClock() {
  const { datepicker } = useDatepicker();
  const query = useQuery({
    queryKey: ['xray', 'scheduled-restart-status'],
    queryFn: fetchRestartStatus,
    refetchInterval: 30_000,
    staleTime: 10_000,
    retry: 1,
  });
  const [, setTick] = useState(0);

  useEffect(() => {
    const timer = window.setInterval(() => setTick((value) => value + 1), 1000);
    return () => window.clearInterval(timer);
  }, []);

  useEffect(() => {
    if (query.data) IntlUtil.setPanelOffsetSeconds(query.data.offsetSeconds);
  }, [query.data]);

  const data = query.data;
  if (!data) {
    return {
      ...query,
      clockText: '--:--:--',
      dateTimeText: '',
      lastRestartText: '',
    };
  }

  const elapsedMs = Math.max(0, Date.now() - query.dataUpdatedAt);
  const current = shiftedDate(data.serverUnix, data.offsetSeconds, elapsedMs);
  const last = data.lastRestartAt > 0
    ? shiftedDate(data.lastRestartAt, data.offsetSeconds)
    : null;

  return {
    ...query,
    clockText: formatClock(current),
    dateTimeText: formatDateTime(current, datepicker),
    lastRestartText: last ? formatDateTime(last, datepicker) : '',
  };
}
