import { useTranslation } from 'react-i18next';
import { SettingListItem } from '@/components/ui';
import type { XraySettingsValue, SetTemplate } from '@/hooks/useXraySetting';
import { customRouteValues, isQuickRoutingVisible, setCustomRoute } from '@/lib/xray/custom-outbound-routing';
import { isBalancerLoopbackTag } from '../balancers/balancer-loopback';
import { SERVICES_OPTIONS } from '../basics/constants';
import { getNordOutboundTag } from '../basics/helpers';
import RoutingGeoSelect from './RoutingGeoSelect';

export default function CustomOutboundRouting({ templateSettings, setTemplateSettings }: {
  templateSettings: XraySettingsValue | null;
  setTemplateSettings: SetTemplate;
}) {
  const { i18n } = useTranslation();
  const fa = i18n.language?.startsWith('fa');
  const nordTag = getNordOutboundTag(templateSettings);
  const tags = new Set((templateSettings?.outbounds ?? []).flatMap(({ tag }) =>
    tag && isQuickRoutingVisible(templateSettings, tag)
      && !isBalancerLoopbackTag(tag) && tag !== nordTag && !['direct', 'blocked', 'IPv4', 'warp'].includes(tag)
      ? [tag] : [],
  ));

  return [...tags].map(tag => (
    <SettingListItem key={tag} title={<bdi>{tag}</bdi>} paddings="small"
      description={fa ? 'سرویس‌های انتخاب‌شده از این خروجی عبور می‌کنند.' : 'Route selected services through this outbound.'}
      control={
        <RoutingGeoSelect label={tag} kind="site"
          value={customRouteValues(templateSettings, tag, 'domain')} options={SERVICES_OPTIONS}
          onChange={values => setTemplateSettings(prev => {
            if (!prev) return prev;
            const next = JSON.parse(JSON.stringify(prev)) as XraySettingsValue;
            setCustomRoute(next, tag, 'domain', values);
            return next;
          })}
        />
      }
    />
  ));
}
