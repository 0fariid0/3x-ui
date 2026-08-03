import { useMemo } from 'react';

import { IntlUtil } from '@/utils';
import { useDatepicker } from '@/hooks/useDatepicker';
import { useServerClock } from '@/hooks/useServerClock';

export function usePanelDateTime() {
  const { datepicker } = useDatepicker();
  const serverClock = useServerClock();
  const offsetSeconds = serverClock.data?.offsetSeconds ?? 0;

  return useMemo(() => ({
    calendar: datepicker,
    offsetSeconds,
    timezoneLabel: serverClock.data?.timezoneLabel || '',
    formatDateTime: (value: string | number | Date | null | undefined) => (
      IntlUtil.formatPanelDateTime(value, datepicker, offsetSeconds)
    ),
    formatDate: (value: string | number | Date | null | undefined) => (
      IntlUtil.formatPanelDate(value, datepicker, offsetSeconds)
    ),
    formatTime: (value: string | number | Date | null | undefined, withSeconds = false) => (
      IntlUtil.formatPanelTime(value, offsetSeconds, withSeconds)
    ),
    formatDayKey: (day: string) => IntlUtil.formatPanelDayKey(day, datepicker),
  }), [datepicker, offsetSeconds, serverClock.data?.timezoneLabel]);
}
