import { useState } from 'react';
import { Button, Select } from 'antd';
import { useTranslation } from 'react-i18next';
import { GeoTokenInput } from '@/components/geodata';
import { parseTokens } from '@/lib/xray/geoTokens';
import type { XraySettingsValue, SetTemplate } from '@/hooks/useXraySetting';
import { setProtonRules } from '@/lib/xray/proton-routing';
import { SettingListItem } from '@/components/ui';

function ProtonRuleFields({ domain, ip, onApply }: { domain: string; ip: string; onApply: (domains: string[], ips: string[]) => void }) {
  const { i18n } = useTranslation();
  const fa = i18n.language?.startsWith('fa');
  const [domains, setDomains] = useState(domain);
  const [ips, setIPs] = useState(ip);
  return <>
    <label>{fa ? 'دامنه‌ها و دسته‌های Geosite' : 'Domains and Geosite categories'}</label>
    <GeoTokenInput searchDatabase kind="domain" value={domains} onChange={setDomains} placeholder="example.com, geosite:google" />
    <label>{fa ? 'IP، شبکه و دسته‌های GeoIP' : 'IPs, networks and GeoIP categories'}</label>
    <GeoTokenInput searchDatabase kind="ip" value={ips} onChange={setIPs} placeholder="geoip:ir, 203.0.113.0/24" />
    <Button type="primary" disabled={domains === domain && ips === ip} onClick={() => onApply(parseTokens(domains), parseTokens(ips))}>
      {fa ? 'اعمال قانون Proton' : 'Apply Proton rule'}
    </Button>
  </>;
}

export default function ProtonRouting({ templateSettings, setTemplateSettings, onShowProton }: {
  templateSettings: XraySettingsValue | null;
  setTemplateSettings: SetTemplate;
  onShowProton?: () => void;
}) {
  const { i18n } = useTranslation();
  const fa = i18n.language?.startsWith('fa');
  const [selection, setSelection] = useState<string>();
  const tags = (templateSettings?.outbounds ?? []).flatMap(outbound => outbound.protocol === 'wireguard' && outbound.tag ? [outbound.tag] : []);
  const tag = selection && tags.includes(selection) ? selection : tags.find(value => value.startsWith('proton'));
  const existing = (kind: 'domain' | 'ip') => (templateSettings?.routing?.rules ?? []).find(rule => rule.ruleTag?.startsWith(`xui-proton-${kind}:`) && rule.outboundTag === tag)?.[kind]?.join(', ') ?? '';
  const domain = existing('domain');
  const ip = existing('ip');
  return <SettingListItem title="Proton VPN" paddings="small"
    description={fa ? 'پس از اعمال، تنظیمات Xray را ذخیره کن. اگر قانونی بالاتر همان ترافیک را می‌گیرد، اولویت Proton را در تب قوانین جابه‌جا کن.' : 'Save Xray settings after applying. If an earlier rule matches the same traffic, move the Proton rule up in the Rules tab.'}
    control={<div className="proton-routing-fields">
      <Select
        aria-label={fa ? 'خروجی Proton' : 'Proton outbound'}
        value={tag}
        onChange={setSelection}
        options={tags.map(value => ({ value, label: value }))}
        placeholder={fa ? 'خروجی WireGuard پروتون را انتخاب کن' : 'Select the Proton WireGuard outbound'}
        style={{ width: '100%' }}
      />
      {tag && <ProtonRuleFields key={JSON.stringify([tag, domain, ip])} domain={domain} ip={ip}
        onApply={(domains, ips) => setTemplateSettings(prev => {
          if (!prev) return prev;
          const next = JSON.parse(JSON.stringify(prev)) as XraySettingsValue;
          setProtonRules(next, tag, domains, ips);
          return next;
        })} />}
      <Button onClick={onShowProton} disabled={!onShowProton}>{fa ? 'ساخت خروجی از فایل Proton' : 'Import Proton configuration'}</Button>
    </div>}
  />;
}
