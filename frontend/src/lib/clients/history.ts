const hiddenEventKinds = new Set(['subscription_links', 'external_links']);

export function visibleClientEvents<T extends { kind: string; details?: string }>(events: readonly T[]): T[] {
  return events.filter((event) => !hiddenEventKinds.has(event.kind) && (event.kind !== 'updated' || hasServiceDetails(event.details)));
}

export function remainingDaysAtChange(expiry: number, recordedAt: number): number | null {
  if (!Number.isFinite(expiry) || !Number.isFinite(recordedAt) || expiry <= 0 || recordedAt <= 0) return null;
  return Math.max(0, Math.ceil((expiry - recordedAt) / 86400000));
}

export function isIPActivity(event: {kind: string}): boolean {
  return event.kind.startsWith('ip_');
}

export function isServiceChange(event: {kind: string; details?: string}): boolean {
  return !isIPActivity(event);
}

function hasServiceDetails(raw?: string): boolean {
  try {
    const d = JSON.parse(raw || '{}');
    if (!d || typeof d !== 'object' || Array.isArray(d)) return false;
    return ['totalGB','totalBytes','previousTotalBytes','expiryTime','previousExpiryTime','addedBytes','addedMilliseconds','addBytes','addDays'].some(key => typeof d[key] === 'number' && Number.isFinite(d[key]))
      || (Array.isArray(d.inboundIds) && d.inboundIds.some((id: unknown) => typeof id === 'number'));
  } catch { return false; }
}
