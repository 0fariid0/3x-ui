import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Empty, Modal, Select, Spin, Tabs, Tag, theme } from 'antd';
import {
  AlertOutlined,
  AppstoreOutlined,
  AreaChartOutlined,
  GlobalOutlined,
  HistoryOutlined,
} from '@ant-design/icons';

import { HttpUtil, IntlUtil, SizeFormatter } from '@/utils';
import { Sparkline } from '@/components/viz';
import { useDatepicker } from '@/hooks/useDatepicker';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import './ClientUsageModal.css';

interface ApiMsg<T> { success?: boolean; obj?: T; }
interface DailyUsage { day: string; up: number; down: number; total: number; }
interface HourlyUsage { hour: number; up: number; down: number; total: number; bytes: number; }
interface ReportIP { id: number; ip: string; firstSeen: number; lastSeen: number; seenCount: number; }
interface ReportApp { id: number; appName: string; version?: string; os?: string; userAgent?: string; format?: string; requestCount?: number; firstSeen?: number; lastSeen?: number; }
interface ReportHost { id: number; inboundId: number; remark?: string; address?: string; port?: number; lastSeen?: number; }
interface ReportEvent { id: number; kind: string; summary: string; details?: string; createdAt: number; }
interface ReportAnomaly {
  id: number;
  kind: string;
  action: string;
  status: string;
  details?: string;
  observedBytesPerMin?: number;
  thresholdBytesPerMin?: number;
  ipCount?: number;
  createdAt: number;
  resolvedAt?: number;
}

export interface ClientInsightReport {
  email: string;
  days: number;
  lastOnline: number;
  recentIpCount: number;
  recentIps: ReportIP[];
  apps: ReportApp[];
  hosts: ReportHost[];
  dailyUsage: DailyUsage[];
  hourlyUsage: HourlyUsage[];
  totalUp: number;
  totalDown: number;
  totalUsage: number;
  averageDaily: number;
  peakDay?: string;
  peakDayBytes: number;
  peakHour: number;
  peakHourBytes: number;
  peakMinuteBytes: number;
  latestMinuteBytes: number;
  activeDays: number;
  activeMinutes: number;
  firstDataAt: number;
  lastDataAt: number;
  events: ReportEvent[];
  anomalies: ReportAnomaly[];
}

interface ClientUsageModalProps {
  open: boolean;
  email: string | null;
  initialDays?: number;
  onClose: () => void;
}

const RANGE_OPTIONS = [
  { value: 1, label: '24h' },
  { value: 7, label: '7d' },
  { value: 14, label: '14d' },
  { value: 30, label: '30d' },
  { value: 60, label: '60d' },
  { value: 90, label: '90d' },
  { value: 180, label: '180d' },
  { value: 365, label: '365d' },
];

function dayLabel(day: string, days: number): string {
  if (!day) return '';
  if (days > 90) return day.slice(0, 7);
  if (days > 30) return day.slice(5);
  return day.slice(5);
}

export default function ClientUsageModal({ open, email, initialDays = 7, onClose }: ClientUsageModalProps) {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  const { datepicker } = useDatepicker();
  const { isMobile } = useMediaQuery();
  const [days, setDays] = useState(initialDays);
  const [activeTab, setActiveTab] = useState('usage');
  const [report, setReport] = useState<ClientInsightReport | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (open) {
      setDays(initialDays);
      setActiveTab('usage');
    }
  }, [open, initialDays, email]);

  const load = useCallback(async () => {
    if (!open || !email) return;
    setLoading(true);
    try {
      const msg = await HttpUtil.get(
        `/panel/api/clients/report/${encodeURIComponent(email)}?days=${days}`,
        undefined,
        { silent: true },
      ) as ApiMsg<ClientInsightReport>;
      setReport(msg?.success && msg.obj ? msg.obj : null);
    } catch (_) {
      setReport(null);
    } finally {
      setLoading(false);
    }
  }, [open, email, days]);

  useEffect(() => { void load(); }, [load]);

  const dateLabel = (ts?: number) => (!ts || ts <= 0 ? '—' : IntlUtil.formatDate(ts, datepicker));
  const daily = report?.dailyUsage ?? [];
  const hourly = report?.hourlyUsage ?? [];
  const useHourlyMainChart = days === 1;
  const labels = useMemo(
    () => useHourlyMainChart
      ? hourly.map((row) => `${String(row.hour).padStart(2, '0')}:00`)
      : daily.map((row) => dayLabel(row.day, days)),
    [daily, hourly, days, useHourlyMainChart],
  );
  const up = useMemo(
    () => useHourlyMainChart ? hourly.map((row) => row.up || 0) : daily.map((row) => row.up),
    [daily, hourly, useHourlyMainChart],
  );
  const down = useMemo(
    () => useHourlyMainChart ? hourly.map((row) => row.down || 0) : daily.map((row) => row.down),
    [daily, hourly, useHourlyMainChart],
  );
  const maxHourly = useMemo(() => Math.max(1, ...hourly.map((row) => row.total || row.bytes || 0)), [hourly]);

  const anomalyName = (kind: string) => {
    const key = `pages.clients.anomalyKinds.${kind}`;
    const value = t(key);
    return value === key ? kind : value;
  };
  const eventName = (event: ReportEvent) => {
    const key = `pages.clients.eventKinds.${event.kind}`;
    const value = t(key);
    return value === key ? event.summary : value;
  };

  const usagePanel = report ? (
    <div className="client-usage-panel">
      <div className="client-usage-summary">
        <div><span>{t('pages.clients.usageTotal')}</span><strong>{SizeFormatter.sizeFormat(report.totalUsage || 0)}</strong></div>
        <div><span>{t('pages.clients.usageAverageDaily')}</span><strong>{SizeFormatter.sizeFormat(report.averageDaily || 0)}</strong></div>
        <div><span>{t('pages.clients.usagePeakDay')}</span><strong>{report.peakDay || '—'}</strong><small>{SizeFormatter.sizeFormat(report.peakDayBytes || 0)}</small></div>
        <div><span>{t('pages.clients.usagePeakMinute')}</span><strong>{SizeFormatter.sizeFormat(report.peakMinuteBytes || 0)}/min</strong></div>
        <div><span>{t('pages.clients.activeDays')}</span><strong>{report.activeDays}/{report.days}</strong></div>
        <div><span>{t('pages.clients.recentIpCount')}</span><strong>{report.recentIpCount}</strong></div>
      </div>

      <div className="client-usage-chart-shell">
        <div className="client-usage-chart-head">
          <div>
            <div className="client-usage-chart-title">{t('pages.clients.usageChart')}</div>
            <div className="client-usage-chart-sub">
              {t('pages.clients.usageChartRange', { days: report.days })} · {dateLabel(report.firstDataAt)} — {dateLabel(report.lastDataAt)}
            </div>
          </div>
          <div className="client-usage-legend">
            <span><i style={{ background: token.colorPrimary }} />{t('pages.index.upload')} <b>{SizeFormatter.sizeFormat(report.totalUp || 0)}</b></span>
            <span><i style={{ background: token.colorTextTertiary }} />{t('pages.index.download')} <b>{SizeFormatter.sizeFormat(report.totalDown || 0)}</b></span>
          </div>
        </div>
        <Sparkline
          data={up}
          data2={down}
          labels={labels}
          height={isMobile ? 220 : 310}
          maxPoints={400}
          stroke={token.colorPrimary}
          stroke2={token.colorTextTertiary}
          name1={t('pages.index.upload')}
          name2={t('pages.index.download')}
          valueMax={null}
          showAxes
          showGrid
          showTooltip
          showLegend={false}
          fillOpacity={0.18}
          tickCountX={isMobile ? 4 : 7}
          yFormatter={SizeFormatter.sizeFormat}
          tooltipFormatter={SizeFormatter.sizeFormat}
        />
        <div className="client-usage-chart-foot">
          <div><span>{t('pages.clients.latestMinute')}</span><strong>{SizeFormatter.sizeFormat(report.latestMinuteBytes || 0)}/min</strong></div>
          <div><span>{t('pages.clients.activeMinutes')}</span><strong>{report.activeMinutes.toLocaleString()}</strong></div>
          <div><span>{t('lastOnline')}</span><strong>{dateLabel(report.lastOnline)}</strong></div>
        </div>
      </div>

      <div className="client-usage-split">
        <section className="client-usage-detail-card">
          <div className="client-usage-section-title">{t('pages.clients.hourlyPattern')}</div>
          <div className="client-hour-bars">
            {(report.hourlyUsage ?? []).map((row) => (
              <div className="client-hour-bar-wrap" key={row.hour} title={`${String(row.hour).padStart(2, '0')}:00 — ${SizeFormatter.sizeFormat(row.total || row.bytes || 0)}`}>
                <div className="client-hour-bar" style={{ height: `${Math.max(2, ((row.total || row.bytes || 0) / maxHourly) * 100)}%` }} />
                <span>{String(row.hour).padStart(2, '0')}</span>
              </div>
            ))}
          </div>
        </section>
        <section className="client-usage-detail-card">
          <div className="client-usage-section-title">{t('pages.clients.rangeBreakdown')}</div>
          <div className="client-usage-breakdown">
            <div><span>{t('pages.index.upload')}</span><strong>{SizeFormatter.sizeFormat(report.totalUp || 0)}</strong></div>
            <div><span>{t('pages.index.download')}</span><strong>{SizeFormatter.sizeFormat(report.totalDown || 0)}</strong></div>
            <div><span>{t('pages.clients.peakUsageHour')}</span><strong>{String(report.peakHour).padStart(2, '0')}:00</strong></div>
            <div><span>{t('pages.clients.peakHourUsage')}</span><strong>{SizeFormatter.sizeFormat(report.peakHourBytes || 0)}</strong></div>
          </div>
        </section>
      </div>
    </div>
  ) : null;

  const tabItems = report ? [
    { key: 'usage', label: <span><AreaChartOutlined /> {t('pages.clients.usageTab')}</span>, children: usagePanel },
    {
      key: 'ips', label: <span><GlobalOutlined /> {t('pages.clients.recentIps')}</span>, children: (
        <div className="client-usage-list">
          {report.recentIps.length === 0 ? <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} /> : report.recentIps.map((ip) => (
            <div className="client-usage-list-row" key={ip.id}>
              <div><strong className="client-usage-mono">{ip.ip}</strong><small>{t('pages.clients.firstSeen')}: {dateLabel(ip.firstSeen)}</small></div>
              <div><span>{t('pages.clients.seenCount')}: {ip.seenCount}</span><small>{t('pages.clients.lastSeen')}: {dateLabel(ip.lastSeen)}</small></div>
            </div>
          ))}
        </div>
      ),
    },
    {
      key: 'apps', label: <span><AppstoreOutlined /> {t('pages.clients.appsAndOs')}</span>, children: (
        <div className="client-usage-list">
          {report.apps.length === 0 ? <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} /> : report.apps.map((app) => (
            <div className="client-usage-list-row" key={app.id}>
              <div><strong>{app.appName}{app.version ? ` ${app.version}` : ''}</strong><small>{app.userAgent || '—'}</small></div>
              <div><Tag>{app.os || t('unknown')}</Tag><small>{dateLabel(app.lastSeen)}</small></div>
            </div>
          ))}
        </div>
      ),
    },
    {
      key: 'hosts', label: <span><GlobalOutlined /> {t('pages.clients.usedHosts')}</span>, children: (
        <div className="client-usage-list">
          {report.hosts.length === 0 ? <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} /> : report.hosts.map((host) => (
            <div className="client-usage-list-row" key={host.id}>
              <div><strong>{host.remark || `Inbound ${host.inboundId}`}</strong><small>Inbound #{host.inboundId}</small></div>
              <div><code>{host.address || '—'}{host.port ? `:${host.port}` : ''}</code><small>{dateLabel(host.lastSeen)}</small></div>
            </div>
          ))}
        </div>
      ),
    },
    {
      key: 'anomalies', label: <span><AlertOutlined /> {t('pages.clients.anomalyHistory')}</span>, children: (
        <div className="client-usage-list">
          {report.anomalies.length === 0 ? <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} /> : report.anomalies.map((item) => (
            <div className="client-usage-list-row" key={item.id}>
              <div><strong>{anomalyName(item.kind)}</strong><small>{item.details || item.action}</small></div>
              <div><Tag color={item.status === 'resolved' ? 'green' : item.status === 'acted' ? 'red' : 'orange'}>{item.status}</Tag><small>{dateLabel(item.createdAt)}</small></div>
            </div>
          ))}
        </div>
      ),
    },
    {
      key: 'history', label: <span><HistoryOutlined /> {t('pages.clients.changeHistory')}</span>, children: (
        <div className="client-usage-list">
          {report.events.length === 0 ? <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} /> : report.events.map((event) => (
            <div className="client-usage-list-row" key={event.id}>
              <div><strong>{eventName(event)}</strong><small>{event.details || event.summary}</small></div>
              <div><Tag>{event.kind}</Tag><small>{dateLabel(event.createdAt)}</small></div>
            </div>
          ))}
        </div>
      ),
    },
  ] : [];

  return (
    <Modal
      open={open}
      footer={null}
      width={isMobile ? '96vw' : 1120}
      onCancel={onClose}
      destroyOnHidden
      title={(
        <div className="client-usage-modal-title">
          <div><AreaChartOutlined /> {t('pages.clients.usageDetails')} {email ? `— ${email}` : ''}</div>
          <Select
            value={days}
            size="small"
            className="client-usage-range"
            options={RANGE_OPTIONS}
            onChange={setDays}
          />
        </div>
      )}
    >
      <Spin spinning={loading}>
        {!loading && !report ? (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('pages.clients.noReportData')} />
        ) : report ? (
          <Tabs activeKey={activeTab} onChange={setActiveTab} items={tabItems} className="client-usage-tabs" />
        ) : <div className="client-usage-loading-placeholder" />}
      </Spin>
    </Modal>
  );
}
