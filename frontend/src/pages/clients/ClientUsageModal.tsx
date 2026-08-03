import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Empty, message, Modal, Popconfirm, Select, Spin, Tabs, Tag, theme } from 'antd';
import {
  AlertOutlined,
  AppstoreOutlined,
  AreaChartOutlined,
  GlobalOutlined,
  DeleteOutlined,
  HistoryOutlined,
} from '@ant-design/icons';

import { HttpUtil, SizeFormatter } from '@/utils';
import { Sparkline } from '@/components/viz';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { usePanelDateTime } from '@/hooks/usePanelDateTime';
import './ClientUsageModal.css';

interface ApiMsg<T> { success?: boolean; msg?: string; obj?: T; }
interface DailyUsage { day: string; up: number; down: number; total: number; }
interface HourlyUsage { hour: number; up: number; down: number; total: number; bytes: number; }
interface TimelineUsage { bucketStart: number; up: number; down: number; total: number; }
interface ReportIP { id: number; ip: string; firstSeen: number; lastSeen: number; seenCount: number; }
interface ReportApp { id: number; appName: string; version?: string; os?: string; userAgent?: string; format?: string; requestCount?: number; firstSeen?: number; lastSeen?: number; }
interface ReportHost { id: number; inboundId: number; remark?: string; address?: string; port?: number; lastSeen?: number; }
interface DestinationSummary { service: string; owner?: string; connections: number; destinations: number; lastSeen: number; active: boolean; activeDestinations: number; }
interface DestinationItem { key: string; service: string; owner?: string; domain?: string; ip?: string; port?: number; protocol?: string; confidence: string; connections: number; firstSeen: number; lastSeen: number; active: boolean; }
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
  hours: number;
  rangeStart: number;
  rangeEnd: number;
  lastOnline: number;
  recentIpCount: number;
  recentIps: ReportIP[];
  apps: ReportApp[];
  hosts: ReportHost[];
  dailyUsage: DailyUsage[];
  hourlyUsage: HourlyUsage[];
  timelineUsage: TimelineUsage[];
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
  destinationTracking: boolean;
  destinationSummaries: DestinationSummary[];
  destinations: DestinationItem[];
}

interface ClientUsageModalProps {
  open: boolean;
  email: string | null;
  initialDays?: number;
  onClose: () => void;
}

type RangeValue = '12h' | '24h' | '7d' | '14d' | '30d' | '60d' | '90d' | '180d' | '365d';

const RANGE_OPTIONS: { value: RangeValue; label: string }[] = [
  { value: '12h', label: '12h' },
  { value: '24h', label: '24h' },
  { value: '7d', label: '7d' },
  { value: '14d', label: '14d' },
  { value: '30d', label: '30d' },
  { value: '60d', label: '60d' },
  { value: '90d', label: '90d' },
  { value: '180d', label: '180d' },
  { value: '365d', label: '365d' },
];

function initialRange(initialDays: number): RangeValue {
  if (initialDays === 1) return '24h';
  const value = `${initialDays}d` as RangeValue;
  return RANGE_OPTIONS.some((option) => option.value === value) ? value : '7d';
}

function rangeQuery(range: RangeValue): string {
  return range.endsWith('h')
    ? `hours=${Number.parseInt(range, 10)}`
    : `days=${Number.parseInt(range, 10)}`;
}

export default function ClientUsageModal({ open, email, initialDays = 7, onClose }: ClientUsageModalProps) {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  const panelDateTime = usePanelDateTime();
  const { isMobile } = useMediaQuery();
  const [range, setRange] = useState<RangeValue>(() => initialRange(initialDays));
  const [activeTab, setActiveTab] = useState('usage');
  const [report, setReport] = useState<ClientInsightReport | null>(null);
  const [loading, setLoading] = useState(false);
  const [resettingDestinations, setResettingDestinations] = useState(false);

  useEffect(() => {
    if (open) {
      setRange(initialRange(initialDays));
      setActiveTab('usage');
    }
  }, [open, initialDays, email]);

  const load = useCallback(async (background = false) => {
    if (!open || !email) return;
    if (!background) setLoading(true);
    try {
      const msg = await HttpUtil.get(
        `/panel/api/clients/report/${encodeURIComponent(email)}?${rangeQuery(range)}`,
        undefined,
        { silent: true },
      ) as ApiMsg<ClientInsightReport>;
      if (msg?.success && msg.obj) {
        setReport(msg.obj);
      } else if (!background) {
        setReport(null);
      }
    } catch (_) {
      if (!background) setReport(null);
    } finally {
      if (!background) setLoading(false);
    }
  }, [open, email, range]);

  useEffect(() => { void load(); }, [load]);

  useEffect(() => {
    if (!open || !email || activeTab !== 'destinations' || !report?.destinationTracking) return undefined;
    const timer = window.setInterval(() => { void load(true); }, 15_000);
    return () => window.clearInterval(timer);
  }, [activeTab, email, load, open, report?.destinationTracking]);

  const clearDestinations = useCallback(async () => {
    if (!email || resettingDestinations) return;
    setResettingDestinations(true);
    try {
      const msg = await HttpUtil.post(
        `/panel/api/clients/clearDestinations/${encodeURIComponent(email)}`,
        undefined,
        { silent: true },
      ) as ApiMsg<unknown>;
      if (!msg?.success) {
        throw new Error(msg?.msg || t('pages.clients.destinationResetFailed'));
      }
      message.success(msg.msg || t('pages.clients.destinationResetSuccess'));
      await load();
    } catch (error) {
      message.error(error instanceof Error ? error.message : t('pages.clients.destinationResetFailed'));
    } finally {
      setResettingDestinations(false);
    }
  }, [email, load, resettingDestinations, t]);

  const dateLabel = (ts?: number) => (!ts || ts <= 0 ? '—' : panelDateTime.formatDateTime(ts));
  const daily = report?.dailyUsage ?? [];
  const hourly = report?.hourlyUsage ?? [];
  const timeline = report?.timelineUsage ?? [];
  const useRollingHours = (report?.hours ?? 0) > 0;
  const labels = useMemo(
    () => useRollingHours
      ? timeline.map((row) => panelDateTime.formatTime(row.bucketStart))
      : daily.map((row) => panelDateTime.formatDayKey(row.day)),
    [daily, timeline, panelDateTime, useRollingHours],
  );
  const up = useMemo(
    () => useRollingHours ? timeline.map((row) => row.up || 0) : daily.map((row) => row.up || 0),
    [daily, timeline, useRollingHours],
  );
  const down = useMemo(
    () => useRollingHours ? timeline.map((row) => row.down || 0) : daily.map((row) => row.down || 0),
    [daily, timeline, useRollingHours],
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

  const chartRangeText = report
    ? report.hours > 0
      ? t('pages.clients.usageChartHours', { hours: report.hours })
      : t('pages.clients.usageChartRange', { days: report.days })
    : '';

  const usagePanel = report ? (
    <div className="client-usage-panel">
      <div className="client-usage-summary">
        <div><span>{t('pages.clients.usageTotal')}</span><strong>{SizeFormatter.sizeFormat(report.totalUsage || 0)}</strong></div>
        <div><span>{t('pages.clients.usageAverageDaily')}</span><strong>{SizeFormatter.sizeFormat(report.averageDaily || 0)}</strong></div>
        <div>
          <span>{t('pages.clients.usagePeakDay')}</span>
          <strong>{report.peakDay ? panelDateTime.formatDayKey(report.peakDay) : '—'}</strong>
          <small>{SizeFormatter.sizeFormat(report.peakDayBytes || 0)}</small>
        </div>
        <div><span>{t('pages.clients.usagePeakMinute')}</span><strong>{SizeFormatter.sizeFormat(report.peakMinuteBytes || 0)}/min</strong></div>
        <div><span>{t('pages.clients.activeMinutes')}</span><strong>{report.activeMinutes.toLocaleString()}</strong></div>
        <div><span>{t('pages.clients.recentIpCount')}</span><strong>{report.recentIpCount}</strong></div>
      </div>

      <div className="client-usage-chart-shell">
        <div className="client-usage-chart-head">
          <div>
            <div className="client-usage-chart-title">{t('pages.clients.usageChart')}</div>
            <div className="client-usage-chart-sub">
              {chartRangeText} · {dateLabel(report.rangeStart || report.firstDataAt)} — {dateLabel(report.rangeEnd || report.lastDataAt)}
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
          tickCountX={isMobile ? 4 : Math.min(8, Math.max(5, labels.length))}
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
            {hourly.map((row) => (
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
      key: 'destinations', label: <span><GlobalOutlined /> {t('pages.clients.destinationsTab')}</span>, children: (
        <div className="client-destination-panel">
          <div className="client-destination-toolbar">
            <div className="client-destination-privacy-note">{t('pages.clients.destinationPrivacyNote')}</div>
            <Popconfirm
              title={t('pages.clients.destinationResetConfirmTitle')}
              description={t('pages.clients.destinationResetConfirmDesc')}
              okText={t('confirm')}
              cancelText={t('cancel')}
              okButtonProps={{ danger: true, loading: resettingDestinations }}
              onConfirm={clearDestinations}
            >
              <Button
                danger
                size="small"
                icon={<DeleteOutlined />}
                loading={resettingDestinations}
                disabled={report.destinations.length === 0}
              >
                {t('pages.clients.destinationReset')}
              </Button>
            </Popconfirm>
          </div>
          {!report.destinationTracking ? (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('pages.clients.destinationTrackingDisabled')} />
          ) : report.destinations.length === 0 ? (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('pages.clients.destinationTrackingEmpty')} />
          ) : (
            <>
              <div className="client-destination-summary-grid">
                {report.destinationSummaries.map((item) => (
                  <div className={`client-destination-summary-card${item.active ? ' is-active' : ''}`} key={item.service}>
                    <div>
                      <strong className="client-destination-service-name">
                        {item.active ? <i className="client-destination-live-dot" aria-label={t('pages.clients.destinationActive')} /> : null}
                        {item.service || t('pages.clients.destinationServiceOther')}
                      </strong>
                      <small>{item.owner || '—'}{item.active ? ` · ${item.activeDestinations} ${t('pages.clients.destinationActiveCount')}` : ''}</small>
                    </div>
                    <div><b>{item.connections.toLocaleString()}</b><small>{t('pages.clients.destinationConnections')}</small></div>
                    <div><b>{item.destinations}</b><small>{t('pages.clients.destinationCount')}</small></div>
                    <div><small>{t('pages.clients.destinationLastSeen')}</small><span>{dateLabel(item.lastSeen)}</span></div>
                  </div>
                ))}
              </div>
              <div className="client-usage-list">
                {report.destinations.map((item) => (
                  <div className={`client-usage-list-row client-destination-row${item.active ? ' is-active' : ''}`} key={item.key}>
                    <div>
                      <strong className="client-destination-service-name">
                        {item.active ? <i className="client-destination-live-dot" aria-label={t('pages.clients.destinationActive')} /> : null}
                        {item.service || t('pages.clients.destinationServiceOther')}
                      </strong>
                      <small className="client-usage-mono">{item.domain || item.ip || '—'}{item.port ? `:${item.port}` : ''}</small>
                    </div>
                    <div>
                      <span>{item.active ? <Tag color="green">{t('pages.clients.destinationActive')}</Tag> : null}{item.connections.toLocaleString()} {t('pages.clients.destinationConnections')}</span>
                      <small>{item.owner || '—'} · {item.confidence === 'domain' ? t('pages.clients.destinationConfidenceDomain') : item.confidence === 'network' ? t('pages.clients.destinationConfidenceNetwork') : t('pages.clients.destinationConfidenceIp')} · {dateLabel(item.lastSeen)}</small>
                    </div>
                  </div>
                ))}
              </div>
            </>
          )}
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
          <Select<RangeValue>
            value={range}
            size="small"
            className="client-usage-range"
            options={RANGE_OPTIONS}
            onChange={setRange}
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
