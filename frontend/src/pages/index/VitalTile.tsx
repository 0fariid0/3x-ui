import { useMemo } from 'react';
import type { CSSProperties, ReactNode } from 'react';
import { Card, theme } from 'antd';

import { Sparkline } from '@/components/viz';
import { mean, peak } from './useOverviewHistory';

interface VitalTileProps {
  icon: ReactNode;
  label: string;
  percent: number;
  statusColor: string;
  detail: string;
  footLeft: string;
  footRight: string;
  data: number[];
  isMobile: boolean;
}

export default function VitalTile({
  icon,
  label,
  percent,
  statusColor,
  detail,
  footLeft,
  footRight,
  data,
  isMobile,
}: VitalTileProps) {
  const { token } = theme.useToken();
  const meanColor = token.colorTextTertiary;
  const referenceLines = useMemo(
    () => (data.length > 1 ? [{ y: mean(data), dash: '3 4', color: meanColor }] : []),
    [data, meanColor],
  );
  const meterStyle = {
    '--fx-meter': `${Math.min(100, Math.max(0, percent)) * 3.6}deg`,
    '--fx-meter-color': statusColor,
  } as CSSProperties;

  return (
    <Card hoverable className="ov-tile" styles={{ body: { padding: 0 } }}>
      <div className="ov-tile-top">
        <div className="ov-tile-identity">
          <span className="ov-tile-icon">{icon}</span>
          <div>
            <span className="ov-kicker">{label}</span>
            <strong>{detail}</strong>
          </div>
        </div>
        <div className="ov-meter" style={meterStyle}>
          <div><span>{percent.toFixed(0)}</span><small>%</small></div>
        </div>
      </div>

      <div className="ov-tile-chart">
        <Sparkline
          data={data}
          height={isMobile ? 54 : 70}
          strokeWidth={1.8}
          fillOpacity={0.28}
          showGrid={false}
          showMarker={false}
          valueMax={peak(data) > 0 ? null : 100}
          stroke={statusColor}
          referenceLines={referenceLines}
          yFormatter={(v) => `${v.toFixed(0)}%`}
          name1={label}
        />
      </div>

      <div className="ov-tile-foot">
        <span>{footLeft}</span>
        <span>{footRight}</span>
      </div>
    </Card>
  );
}
