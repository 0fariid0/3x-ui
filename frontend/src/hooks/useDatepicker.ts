import { useEffect, useState } from 'react';
import { HttpUtil, IntlUtil } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import { DefaultsPayloadSchema } from '@/schemas/defaults';

type Calendar = 'gregorian' | 'jalalian';

let cachedValue: Calendar = IntlUtil.savedPanelCalendar();
let fetched = false;
let pending: Promise<void> | null = null;
let revision = 0;
const listeners = new Set<(value: Calendar) => void>();

function notify(value: Calendar) {
  listeners.forEach((fn) => fn(value));
}

async function loadOnce(): Promise<void> {
  if (fetched) return;
  if (pending) {
    await pending;
    return;
  }
  const requestedRevision = revision;
  pending = (async () => {
    try {
      const msg = await HttpUtil.post('/panel/api/setting/defaultSettings');
      if (msg?.success) {
        const validated = parseMsg(msg, DefaultsPayloadSchema, 'setting/defaultSettings');
        const calendar = validated.obj?.datepicker;
        if (calendar && revision === requestedRevision) setDatepicker(calendar);
      }
    } catch (_) {
      fetched = false;
    } finally {
      pending = null;
    }
  })();
  await pending;
}

export function setDatepicker(value: Calendar) {
  revision += 1;
  fetched = true;
  cachedValue = value || 'gregorian';
  IntlUtil.setPanelCalendar(cachedValue);
  notify(cachedValue);
}

export function useDatepicker() {
  const [datepicker, setLocal] = useState<Calendar>(cachedValue);

  useEffect(() => {
    listeners.add(setLocal);
    void loadOnce();
    const refresh = () => {
      fetched = false;
      void loadOnce();
    };
    window.addEventListener('focus', refresh);
    return () => {
      listeners.delete(setLocal);
      window.removeEventListener('focus', refresh);
    };
  }, []);

  return { datepicker };
}
