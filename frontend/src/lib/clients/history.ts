const hiddenEventKinds = new Set(['subscription_links', 'external_links']);

export function visibleClientEvents<T extends { kind: string }>(events: readonly T[]): T[] {
  return events.filter((event) => !hiddenEventKinds.has(event.kind));
}

export function remainingDaysAtChange(expiry: number, recordedAt: number): number | null {
  if (!Number.isFinite(expiry) || !Number.isFinite(recordedAt) || expiry <= 0 || recordedAt <= 0) return null;
  return Math.max(0, Math.ceil((expiry - recordedAt) / 86400000));
}

export function isServiceChange(event: {kind: string; details?: string}): boolean {
  if (['limits_changed','activated','renewed','attached','detached'].includes(event.kind)) return true;
  if (!['updated','bulk_adjusted'].includes(event.kind)) return false;
  try {
    const d = JSON.parse(event.details || '{}');
    return (typeof d.previousTotalBytes === 'number' && typeof d.totalBytes === 'number' && d.previousTotalBytes !== d.totalBytes)
      || (typeof d.previousExpiryTime === 'number' && typeof d.expiryTime === 'number' && d.previousExpiryTime !== d.expiryTime);
  } catch { return false; }
}
