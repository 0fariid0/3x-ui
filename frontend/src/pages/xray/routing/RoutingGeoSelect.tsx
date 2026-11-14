import { useState } from 'react';
import { Button, Select, Space, Tooltip } from 'antd';
import { DatabaseOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

import { GeoBrowserModal } from '@/components/geodata';
import { formatTokens, parseTokens } from '@/lib/xray/geoTokens';

interface RoutingGeoSelectProps {
  label: string;
  kind: 'ip' | 'site';
  value: string[];
  options: { label: string; value: string }[];
  onChange: (value: string[]) => void;
  disabled?: boolean;
  placeholder?: string;
  onCreate?: () => void;
  createLabel?: string;
}

export default function RoutingGeoSelect({
  label, kind, value, options, onChange, disabled, placeholder, onCreate, createLabel,
}: RoutingGeoSelectProps) {
  const { t } = useTranslation();
  const [browsing, setBrowsing] = useState(false);
  const actionLabel = disabled && onCreate && createLabel ? createLabel : t('pages.xray.geoBrowser.openTooltip');

  return (
    <>
      <Space.Compact className="routing-geo-select" block>
        <Select
          aria-label={label}
          mode="tags"
          value={value}
          options={options}
          placeholder={placeholder}
          onChange={onChange}
          disabled={disabled}
        />
        <Tooltip title={actionLabel}>
          <Button
            aria-label={`${actionLabel} — ${label}`}
            icon={<DatabaseOutlined />}
            disabled={disabled && !onCreate}
            onClick={() => disabled ? onCreate?.() : setBrowsing(true)}
          />
        </Tooltip>
      </Space.Compact>
      {browsing && !disabled && (
        <GeoBrowserModal
          open
          kind={kind}
          searchDatabase
          value={formatTokens(value)}
          onClose={() => setBrowsing(false)}
          onApply={(selection) => {
            if (selection !== formatTokens(value)) onChange(parseTokens(selection));
            setBrowsing(false);
          }}
        />
      )}
    </>
  );
}
