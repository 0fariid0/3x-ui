const hiddenEventKinds = new Set(['subscription_links', 'external_links']);

export function visibleClientEvents<T extends { kind: string }>(events: readonly T[]): T[] {
  return events.filter((event) => !hiddenEventKinds.has(event.kind));
}

export function remainingDaysAtChange(expiry: number, recordedAt: number): number | null {
  if (!Number.isFinite(expiry) || !Number.isFinite(recordedAt) || expiry <= 0 || recordedAt <= 0) return null;
  return Math.max(0, Math.ceil((expiry - recordedAt) / 86400000));
}
