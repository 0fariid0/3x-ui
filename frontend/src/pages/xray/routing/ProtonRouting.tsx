import { useState } from 'react';
import { Button, Select, Space, Tooltip } from 'antd';
import { DatabaseOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { GeoBrowserModal } from '@/components/geodata';
import { formatTokens, parseTokens } from '@/lib/xray/geoTokens';
import type { XraySettingsValue, SetTemplate } from '@/hooks/useXraySetting';
import { setProtonRules } from '@/lib/xray/proton-routing';
import { SettingListItem } from '@/components/ui';
import { SERVICES_OPTIONS } from '../basics/constants';

export default function ProtonRouting({ templateSettings, setTemplateSettings, onShowProton }: {
  templateSettings: XraySettingsValue | null;
  setTemplateSettings: SetTemplate;
  onShowProton?: () => void;
}) {
  const { i18n, t } = useTranslation();
  const fa = i18n.language?.startsWith('fa');
  const [browsingTag, setBrowsingTag] = useState<string>();
  const [selection, setSelection] = useState<string>();
  const rules = templateSettings?.routing?.rules ?? [];
  const tags = (templateSettings?.outbounds ?? []).flatMap(outbound =>
    outbound.protocol === 'wireguard' && outbound.tag && !/^(nord|warp)/i.test(outbound.tag) ? [outbound.tag] : [],
  );
  const tag = selection && tags.includes(selection) ? selection
    : tags.find(value => value.startsWith('proton'))
      ?? tags.find(value => rules.some(rule => rule.outboundTag === value && rule.ruleTag?.startsWith('xui-proton-')));
  const domains = rules.find(rule => rule.outboundTag === tag && rule.ruleTag?.startsWith('xui-proton-domain:'))?.domain ?? [];

  function setDomains(outboundTag: string, nextDomains: string[]) {
    setTemplateSettings(prev => {
      if (!prev) return prev;
      const next = JSON.parse(JSON.stringify(prev)) as XraySettingsValue;
      setProtonRules(next, outboundTag, nextDomains);
      return next;
    });
  }

  return <>
    <SettingListItem title="Proton VPN" paddings="small"
      description={fa ? 'سرویس‌های انتخاب‌شده از مسیر Proton VPN عبور می‌کنند.' : 'Route selected services through Proton VPN.'}
      control={<Space.Compact className="proton-routing-control" style={{ width: '100%' }}>
        <Select
          aria-label="Proton VPN"
          mode="tags"
          value={domains}
          options={SERVICES_OPTIONS}
          style={{ width: '100%', minWidth: 0 }}
          disabled={!tags.length}
          placeholder={fa ? 'ابتدا خروجی Proton VPN را بساز یا انتخاب کن' : 'Create or select a Proton VPN outbound'}
          onChange={next => { if (tag) setDomains(tag, next); }}
          popupRender={menu => <>
            {tag && menu}
            {(!tag || tags.length > 1) && <div className="proton-outbound-picker">
              <Select
                aria-label={fa ? 'خروجی Proton' : 'Proton outbound'}
                value={tag}
                onChange={setSelection}
                options={tags.map(value => ({ value, label: value }))}
                placeholder={fa ? 'انتخاب خروجی WireGuard' : 'Select WireGuard outbound'}
                style={{ width: '100%' }}
              />
            </div>}
          </>}
        />
        <Tooltip title={tag ? t('pages.xray.geoBrowser.openTooltip') : fa ? 'ساخت خروجی Proton' : 'Create Proton outbound'}>
          <Button aria-label={tag ? t('pages.xray.geoBrowser.openTooltip') : fa ? 'ساخت خروجی Proton' : 'Create Proton outbound'}
            icon={<DatabaseOutlined />} disabled={!tag && !onShowProton}
            onClick={() => tag ? setBrowsingTag(tag) : onShowProton?.()} />
        </Tooltip>
      </Space.Compact>}
    />
    {browsingTag && <GeoBrowserModal open kind="site" searchDatabase
      value={formatTokens(rules.find(rule => rule.outboundTag === browsingTag && rule.ruleTag?.startsWith('xui-proton-domain:'))?.domain ?? [])}
      onClose={() => setBrowsingTag(undefined)}
      onApply={value => { setDomains(browsingTag, parseTokens(value)); setBrowsingTag(undefined); }}
    />}
  </>;
}
