import { useTranslation } from 'react-i18next';

import { SizeFormatter } from '@/utils';
import { parseClientEventDetails } from '@/schemas/api/client-event';
import { remainingDaysAtChange } from '@/lib/clients/history';

interface ClientHistoryDetailsProps {
  details?: string;
  summary: string;
  formatDateTime: (value: number) => string;
  recordedAt: number;
}

export default function ClientHistoryDetails({ details, summary, formatDateTime, recordedAt }: ClientHistoryDetailsProps) {
  const { t, i18n } = useTranslation();
  const data = parseClientEventDetails(details);
  const number = (value: number) => new Intl.NumberFormat(i18n.language, { maximumFractionDigits: 2 }).format(value);
  const size = (value: number) => value <= 0 ? t('unlimited') : SizeFormatter.sizeFormat(value);
  const expiry = (value: number) => value === 0 ? t('unlimited') : value < 0
    ? `${number(Math.abs(value) / 86400000)} ${t('pages.clients.historyDetails.firstUseDays')}`
    : formatDateTime(value);
  const lines: { label: string; value: string }[] = [];
  const add = (key: string, value: string) => lines.push({ label: t(`pages.clients.historyDetails.${key}`), value });
  const addRemainingDays = (key: string, value: number | undefined) => {
    if (value === undefined) return;
    const remaining = remainingDaysAtChange(value, recordedAt);
    if (remaining !== null) add(key, `${number(remaining)} ${t('days')}`);
    else if (value === 0) add(key, t('unlimited'));
  };

  if (data) {
    if (data.previousTotalBytes !== undefined) add('previousQuota', size(data.previousTotalBytes));
    const total = data.totalBytes ?? data.totalGB;
    if (total !== undefined) add('quota', size(total));
    if (data.addedBytes !== undefined && (data.previousTotalBytes ?? 0) > 0 && (total ?? 0) > 0) {
      add('quotaChange', `${data.addedBytes > 0 ? '+' : data.addedBytes < 0 ? '−' : ''}${SizeFormatter.sizeFormat(Math.abs(data.addedBytes))}`);
    }
    if (data.previousExpiryTime !== undefined) add('previousExpiry', expiry(data.previousExpiryTime));
    if (data.expiryTime !== undefined) add('expiry', expiry(data.expiryTime));
    addRemainingDays('previousRemainingDays', data.previousExpiryTime);
    addRemainingDays('remainingDays', data.expiryTime);
    if (data.addedMilliseconds != null) {
      add('daysChange', `${data.addedMilliseconds > 0 ? '+' : data.addedMilliseconds < 0 ? '−' : ''}${number(Math.abs(data.addedMilliseconds) / 86400000)} ${t('days')}`);
    }
    if (data.previousUsedBytes !== undefined) add('previousUsage', SizeFormatter.sizeFormat(data.previousUsedBytes));
    if (data.usedBytes !== undefined) add('usage', SizeFormatter.sizeFormat(data.usedBytes));
    if (data.version !== 1 && data.addBytes) add('requestedBytes', SizeFormatter.sizeFormat(data.addBytes));
    if (data.version !== 1 && data.addDays) add('requestedDays', number(data.addDays));
  }

  return lines.length > 0 ? (
    <div className="client-history-details">
      {lines.map((line) => (
        <div key={line.label}><span>{line.label}</span><bdi dir="ltr">{line.value}</bdi></div>
      ))}
    </div>
  ) : <small dir="auto">{summary}</small>;
}
