import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { AlertOutlined, AreaChartOutlined, ReloadOutlined } from '@ant-design/icons';
import { Button, Card, Empty, Select, Spin, Tag, Tooltip } from 'antd';

import { HttpUtil, SizeFormatter } from '@/utils';
import { usePanelDateTime } from '@/hooks/usePanelDateTime';

interface ApiMsg<T> { success?: boolean; obj?: T; }

export interface ClientUsageAlert {
  email: string;
  totalUp: number;
  totalDown: number;
  totalUsage: number;
  averageDaily: number;
  peakMinuteBytes: number;
  activeMinutes: number;
  recentIpCount: number;
  lastOnline: number;
  anomalyCount: number;
  lastAnomalyKind?: string;
  lastAnomalyStatus?: string;
  severity: 'info' | 'warning' | 'critical' | string;
  quotaBytes: number;
  usagePercent: number;
}

interface ClientUsageAlertsResponse {
  days: number;
  generatedAt: number;
  items: ClientUsageAlert[];
}

interface UsageAlertsCardProps {
  onOpenClient: (email: string) => void;
}

const RANGE_OPTIONS = [
  { value: 1, label: '24h' },
  { value: 7, label: '7d' },
  { value: 30, label: '30d' },
  { value: 90, label: '90d' },
  { value: 180, label: '180d' },
  { value: 365, label: '365d' },
];

export default function UsageAlertsCard({ onOpenClient }: UsageAlertsCardProps) {
  const { t, i18n } = useTranslation();
  const panelDateTime = usePanelDateTime();
  const [days, setDays] = useState(1);
  const [items, setItems] = useState<ClientUsageAlert[]>([]);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const msg = await HttpUtil.get(
        `/panel/api/clients/usageAlerts?days=${days}&limit=6`,
        undefined,
        { silent: true },
      ) as ApiMsg<ClientUsageAlertsResponse>;
      setItems(msg?.success && msg.obj?.items ? msg.obj.items : []);
    } catch (_) {
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, [days]);

  useEffect(() => { void load(); }, [load]);

  const tagFor = (item: ClientUsageAlert) => {
    if (item.severity === 'critical') return <Tag color="red">{t('pages.index.usageAlertCritical')}</Tag>;
    if (item.severity === 'warning') return <Tag color="orange">{t('pages.index.usageAlertWarning')}</Tag>;
    return <Tag color="blue">{t('pages.index.usageAlertHigh')}</Tag>;
  };

  return (
    <Card hoverable styles={{ body: { padding: 0 } }} className="ov-usage-alerts" dir={i18n.dir()}>
      <div className="ov-usage-alerts-head">
        <div className="ov-usage-alerts-title">
          <div className="ov-kicker ov-kicker-icon"><AlertOutlined /> {t('pages.index.usageAlerts')}</div>
          <div className="ov-sub">{t('pages.index.usageAlertsSub')}</div>
        </div>
        <div className="ov-usage-alerts-controls">
          <Select size="small" value={days} options={RANGE_OPTIONS} onChange={setDays} />
          <Tooltip title={t('refresh')}>
            <Button type="text" size="small" icon={<ReloadOutlined />} onClick={() => void load()} />
          </Tooltip>
        </div>
      </div>

      <Spin spinning={loading}>
        <div className="ov-usage-alerts-list">
          {!loading && items.length === 0 ? (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('pages.index.noUsageAlerts')} />
          ) : items.map((item, index) => (
            <button type="button" className="ov-usage-alert-row" key={item.email} onClick={() => onOpenClient(item.email)}>
              <span className="ov-usage-rank">{index + 1}</span>
              <span className="ov-usage-main">
                <span className="ov-usage-user-line">
                  <strong><bdi dir="ltr">{item.email}</bdi></strong>
                  {tagFor(item)}
                </span>
                <span className="ov-usage-meta">
                  <span className="ov-usage-meta-item">
                    <span>{t('pages.clients.recentIpCount')}:</span>
                    <bdi dir="ltr">{item.recentIpCount}</bdi>
                  </span>
                  <span className="ov-usage-meta-item">
                    <span>{t('pages.clients.usagePeakMinute')}:</span>
                    <bdi dir="ltr">{SizeFormatter.sizeFormat(item.peakMinuteBytes || 0)}/min</bdi>
                  </span>
                  {item.anomalyCount > 0 && (
                    <span className="ov-usage-meta-item"><bdi>{t('pages.index.usageAlertCount', { count: item.anomalyCount })}</bdi></span>
                  )}
                  {item.lastOnline > 0 && (
                    <span className="ov-usage-meta-item">
                      <span>{t('lastOnline')}:</span>
                      <bdi dir="ltr">{panelDateTime.formatDateTime(item.lastOnline)}</bdi>
                    </span>
                  )}
                </span>
              </span>
              <span className="ov-usage-values">
                <strong><bdi dir="ltr">{SizeFormatter.sizeFormat(item.totalUsage || 0)}</bdi></strong>
                <small><bdi dir="ltr">{SizeFormatter.sizeFormat(item.averageDaily || 0)}</bdi>/{t('pages.index.day')}</small>
                <AreaChartOutlined />
              </span>
            </button>
          ))}
        </div>
      </Spin>
    </Card>
  );
}
