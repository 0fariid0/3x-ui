import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Card, Checkbox, Select, theme } from 'antd';
import { ArrowDownOutlined, ArrowUpOutlined } from '@ant-design/icons';

import { HttpUtil, SizeFormatter, TimeFormatter } from '@/utils';
import { Sparkline } from '@/components/viz';
import type { Status } from '@/models/status';
import { requestedRange, type TrafficRangeKey } from './trafficRange';
import { mean, peak } from './useOverviewHistory';

interface ThroughputCardProps {
  status: Status;
  up: number[];
  down: number[];
  labels: string[];
  isMobile: boolean;
}

type RangeKey = 'live' | TrafficRangeKey;

interface TrafficPoint {
  t: number;
  v: number;
}

interface TrafficRange {
  from: number;
  to: number;
  sent: number;
  recv: number;
  up: TrafficPoint[];
  down: TrafficPoint[];
}

const EMPTY_TRAFFIC: TrafficRange = { from: 0, to: 0, sent: 0, recv: 0, up: [], down: [] };

function alignedSeries(report: TrafficRange) {
  const times = Array.from(new Set([...report.up, ...report.down].map((p) => p.t))).sort((a, b) => a - b);
  const upByTime = new Map(report.up.map((p) => [p.t, p.v]));
  const downByTime = new Map(report.down.map((p) => [p.t, p.v]));
  return {
    up: times.map((t) => upByTime.get(t) ?? 0),
    down: times.map((t) => downByTime.get(t) ?? 0),
    labels: times.map(TimeFormatter.formatClock),
  };
}

export default function ThroughputCard({ status, up, down, labels, isMobile }: ThroughputCardProps) {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  const accent = token.colorPrimary;
  const downColor = token.colorTextTertiary;

  const [rangeKey, setRangeKey] = useState<RangeKey>('live');
  const [fromMidnight, setFromMidnight] = useState(false);
  const [useJalaliMonth, setUseJalaliMonth] = useState(false);
  const [traffic, setTraffic] = useState<TrafficRange>(EMPTY_TRAFFIC);
  const [loading, setLoading] = useState(false);

  const loadTraffic = useCallback(async (signal?: AbortSignal) => {
    if (rangeKey === 'live') return;
    const range = requestedRange(rangeKey, fromMidnight, useJalaliMonth);
    setLoading(true);
    try {
      const msg = await HttpUtil.get<TrafficRange>('/panel/api/server/trafficHistory', range, { signal, silent: true });
      if (msg?.success && msg.obj) setTraffic(msg.obj);
    } finally {
      if (!signal?.aborted) setLoading(false);
    }
  }, [rangeKey, fromMidnight, useJalaliMonth]);

  useEffect(() => {
    if (rangeKey === 'live') return undefined;
    const controller = new AbortController();
    void loadTraffic(controller.signal);
    const timer = window.setInterval(() => void loadTraffic(controller.signal), 30_000);
    return () => {
      controller.abort();
      window.clearInterval(timer);
    };
  }, [loadTraffic, rangeKey]);

  const selected = useMemo(() => alignedSeries(traffic), [traffic]);
  const isLive = rangeKey === 'live';
  const reportSeconds = Math.max(1, traffic.to - traffic.from);
  const avgUp = isLive ? mean(up) : traffic.sent / reportSeconds;
  const avgDown = isLive ? mean(down) : traffic.recv / reportSeconds;
  const chartUp = isLive ? up : selected.up;
  const chartDown = isLive ? down : selected.down;
  const chartLabels = isLive ? labels : selected.labels;
  const sent = isLive ? status.netTraffic.sent : traffic.sent;
  const recv = isLive ? status.netTraffic.recv : traffic.recv;

  const referenceLines = useMemo(
    () => [
      { y: status.netIO.down, color: downColor, dash: '2 4' },
      { y: status.netIO.up, color: accent, dash: '2 4' },
    ],
    [status.netIO.up, status.netIO.down, accent, downColor],
  );

  return (
    <Card hoverable styles={{ body: { padding: 0 } }}>
      <div className="ov-wide-head">
        <div>
          <div className="ov-kicker">{t('pages.index.overallSpeed')}</div>
          <div className="ov-sub">
            {`${t('pages.index.throughputSub')} · ${t('pages.index.peak')} ${SizeFormatter.speedFormat(peak(chartDown))}`}
          </div>
        </div>
        <div className="ov-traffic-range">
          <Select<RangeKey>
            size="small"
            value={rangeKey}
            loading={loading}
            aria-label={t('pages.index.trafficRange')}
            onChange={(value) => {
              setLoading(false);
              if (value !== 'live') setTraffic(EMPTY_TRAFFIC);
              setRangeKey(value);
            }}
            options={[
              { value: 'live', label: t('pages.index.trafficLive') },
              { value: '24h', label: t('pages.index.last24Hours') },
              { value: '7d', label: t('pages.index.last7Days') },
              { value: '30d', label: t('pages.index.last30Days') },
            ]}
          />
          {rangeKey === '24h' && (
            <Checkbox checked={fromMidnight} onChange={(event) => setFromMidnight(event.target.checked)}>
              {t('pages.index.fromMidnight')}
            </Checkbox>
          )}
          {rangeKey === '30d' && (
            <Checkbox checked={useJalaliMonth} onChange={(event) => setUseJalaliMonth(event.target.checked)}>
              {t('pages.index.jalaliCalendar')}
            </Checkbox>
          )}
        </div>
        <div className="ov-wide-legend">
          <div className="ov-legend-label">
            <ArrowUpOutlined style={{ color: accent }} />
            {t('pages.index.upload')}
            <span className="ov-legend-num">{SizeFormatter.speedFormat(status.netIO.up)}</span>
          </div>
          <div className="ov-legend-label">
            <ArrowDownOutlined style={{ color: downColor }} />
            {t('pages.index.download')}
            <span className="ov-legend-num">{SizeFormatter.speedFormat(status.netIO.down)}</span>
          </div>
        </div>
      </div>

      <div className="ov-wide-chart">
        <Sparkline
          data={chartUp}
          data2={chartDown}
          labels={chartLabels}
          height={isMobile ? 140 : 186}
          strokeWidth={1.75}
          fillOpacity={0.24}
          showTooltip
          showLegend={false}
          valueMax={null}
          stroke={accent}
          stroke2={downColor}
          name1={t('pages.index.upload')}
          name2={t('pages.index.download')}
          yFormatter={SizeFormatter.speedFormat}
          referenceLines={referenceLines}
        />
      </div>

      <div className="ov-wide-foot">
        <div>
          <div className="ov-kicker">{t('pages.index.sent')}</div>
          <div className="ov-foot-value">{SizeFormatter.sizeFormat(sent)}</div>
        </div>
        <span className="ov-foot-sep" />
        <div>
          <div className="ov-kicker">{t('pages.index.received')}</div>
          <div className="ov-foot-value">{SizeFormatter.sizeFormat(recv)}</div>
        </div>
        <span className="ov-foot-sep" />
        <div>
          <div className="ov-kicker">{t('pages.index.avgWindow')}</div>
          <div className="ov-foot-value">
            <span className="ov-foot-part">{`↑ ${SizeFormatter.speedFormat(avgUp)}`}</span>{' '}
            <span className="ov-foot-part">{`↓ ${SizeFormatter.speedFormat(avgDown)}`}</span>
          </div>
        </div>
      </div>
    </Card>
  );
}
