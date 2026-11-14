import { useTranslation } from 'react-i18next';
import RoutingGeoSelect from './RoutingGeoSelect';
import type { XraySettingsValue, SetTemplate } from '@/hooks/useXraySetting';
import { getProtonOutboundTag, setProtonRules } from '@/lib/xray/proton-routing';
import { SettingListItem } from '@/components/ui';
import { SERVICES_OPTIONS } from '../basics/constants';

export default function ProtonRouting({ templateSettings, setTemplateSettings, onShowProton }: {
  templateSettings: XraySettingsValue | null;
  setTemplateSettings: SetTemplate;
  onShowProton?: () => void;
}) {
  const { i18n } = useTranslation();
  const fa = i18n.language?.startsWith('fa');
  const rules = templateSettings?.routing?.rules ?? [];
  const tag = getProtonOutboundTag(templateSettings);
  const domains = rules.find(rule => rule.outboundTag === tag && rule.ruleTag?.startsWith('xui-proton-domain:'))?.domain ?? [];

  function setDomains(outboundTag: string, nextDomains: string[]) {
    setTemplateSettings(prev => {
      if (!prev) return prev;
      const next = JSON.parse(JSON.stringify(prev)) as XraySettingsValue;
      setProtonRules(next, outboundTag, nextDomains);
      return next;
    });
  }

  return (
    <SettingListItem title="Proton VPN" paddings="small"
      description={fa ? 'سرویس‌های انتخاب‌شده از مسیر Proton VPN عبور می‌کنند.' : 'Route selected services through Proton VPN.'}
      control={
        <RoutingGeoSelect
          key={tag ?? ''}
          label="Proton VPN"
          kind="site"
          value={domains}
          options={SERVICES_OPTIONS}
          disabled={!tag}
          placeholder={fa ? 'ابتدا خروجی Proton VPN را بساز' : 'Create the Proton VPN outbound first'}
          onChange={next => { if (tag) setDomains(tag, next); }}
          onCreate={onShowProton}
          createLabel={fa ? 'ساخت خروجی Proton' : 'Create Proton outbound'}
        />
      }
    />
  );
}
