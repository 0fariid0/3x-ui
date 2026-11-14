import { Button, Select, Space } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { SettingListItem } from '@/components/ui';
import type { XraySettingsValue, SetTemplate } from '@/hooks/useXraySetting';
import { customRouteValues, setCustomRoute } from '@/lib/xray/custom-outbound-routing';
import { isBalancerLoopbackTag } from '../balancers/balancer-loopback';
import { IPS_OPTIONS, SERVICES_OPTIONS } from '../basics/constants';
import { getNordOutboundTag } from '../basics/helpers';
import RoutingGeoSelect from './RoutingGeoSelect';

export default function CustomOutboundRouting({
  templateSettings, setTemplateSettings, onShowCustomOutbound, customOutboundTag, onSelectCustomOutbound,
}: {
  templateSettings: XraySettingsValue | null;
  setTemplateSettings: SetTemplate;
  onShowCustomOutbound: () => void;
  customOutboundTag?: string;
  onSelectCustomOutbound: (tag: string) => void;
}) {
  const { i18n } = useTranslation();
  const fa = i18n.language?.startsWith('fa');
  const title = fa ? 'خروجی سفارشی' : 'Custom outbound';
  const add = fa ? 'افزودن خروجی سفارشی' : 'Add custom outbound';
  const domain = fa ? 'دامنه‌ها و Geosite' : 'Domains and Geosite';
  const ip = fa ? 'آدرس‌های IP و GeoIP' : 'IP addresses and GeoIP';
  const nordTag = getNordOutboundTag(templateSettings);
  const options = (templateSettings?.outbounds ?? []).flatMap(({ tag, protocol }) => tag
    && !isBalancerLoopbackTag(tag) && tag !== nordTag && !['direct', 'blocked', 'IPv4', 'warp'].includes(tag)
    ? [{ value: tag, label: protocol ? `${tag} · ${protocol}` : tag }]
    : []);
  const tag = options.find(option => option.value === customOutboundTag)?.value
    ?? options.find(({ value }) => customRouteValues(templateSettings, value, 'domain').length
      || customRouteValues(templateSettings, value, 'ip').length)?.value
    ?? options[0]?.value;

  function update(kind: 'domain' | 'ip', values: string[]) {
    if (!tag) return;
    setTemplateSettings(prev => {
      if (!prev) return prev;
      const next = JSON.parse(JSON.stringify(prev)) as XraySettingsValue;
      setCustomRoute(next, tag, kind, values);
      return next;
    });
  }

  return (
    <SettingListItem title={title} paddings="small"
      description={fa
        ? 'خروجی دلخواه را انتخاب کن و دامنه‌ها یا IPها را از این مسیر عبور بده.'
        : 'Choose an outbound for the selected domains or IP addresses.'}
      control={
        <Space orientation="vertical" size={6} style={{ width: '100%', minWidth: 0 }}>
          <Space.Compact className="routing-geo-select" block>
            <Select aria-label={title} showSearch={{ optionFilterProp: 'label' }} value={tag} options={options} onChange={onSelectCustomOutbound}
              placeholder={fa ? 'ابتدا یک خروجی سفارشی بساز' : 'Create a custom outbound first'} />
            <Button onClick={onShowCustomOutbound} icon={<PlusOutlined />} title={add} aria-label={add} />
          </Space.Compact>
          {tag && (
            <>
              <RoutingGeoSelect key={`${tag}:domain`} label={domain} kind="site" placeholder={domain}
                value={customRouteValues(templateSettings, tag, 'domain')} options={SERVICES_OPTIONS}
                onChange={values => update('domain', values)} />
              <RoutingGeoSelect key={`${tag}:ip`} label={ip} kind="ip" placeholder={ip}
                value={customRouteValues(templateSettings, tag, 'ip')} options={IPS_OPTIONS}
                onChange={values => update('ip', values)} />
            </>
          )}
        </Space>
      }
    />
  );
}
