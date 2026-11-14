import { expect, it } from 'vitest';
import type { XraySettingsValue } from '@/hooks/useXraySetting';
import { customRouteValues, isQuickRoutingVisible, setCustomRoute, setQuickRoutingVisible } from '@/lib/xray/custom-outbound-routing';
import { propagateOutboundTagRename } from '@/pages/xray/basics/helpers';
import { applyOutboundDeletion } from '@/pages/xray/reference-cleanup';

it('adds independent routes per outbound without touching saved blocks and legacy Proton rules', () => {
  const template: XraySettingsValue = {
    outbounds: [{ tag: 'vless-edge', protocol: 'vless' }, { tag: 'ss-edge', protocol: 'shadowsocks' }],
    routing: { rules: [
      { type: 'field', outboundTag: 'blocked', ip: ['geoip:private'] },
      { type: 'field', outboundTag: 'proton-free', ruleTag: 'xui-proton-domain:proton-free', domain: ['geosite:google'] },
    ] },
  };
  const original = structuredClone(template.routing?.rules);
  setQuickRoutingVisible(template, 'vless-edge', true);
  setCustomRoute(template, 'vless-edge', 'domain', ['geosite:telegram', 'geosite:telegram']);
  setCustomRoute(template, 'vless-edge', 'ip', ['geoip:cn']);
  setCustomRoute(template, 'ss-edge', 'domain', ['full:example.com']);
  expect(template.routing?.rules?.slice(0, 2)).toEqual(original);
  expect(customRouteValues(template, 'vless-edge', 'domain')).toEqual(['geosite:telegram']);
  expect(customRouteValues(template, 'ss-edge', 'domain')).toEqual(['full:example.com']);

  const rules = template.routing?.rules ?? [];
  rules[2].enabled = false;
  setCustomRoute(template, 'vless-edge', 'domain', ['geosite:youtube']);
  expect(template.routing?.rules?.[2]).toMatchObject({ enabled: false, domain: ['geosite:youtube'] });
  setCustomRoute(template, 'vless-edge', 'domain', []);
  expect(customRouteValues(template, 'vless-edge', 'domain')).toEqual([]);
  expect(customRouteValues(template, 'vless-edge', 'ip')).toEqual(['geoip:cn']);

  setQuickRoutingVisible(template, 'vless-edge', false);
  expect(isQuickRoutingVisible(template, 'vless-edge')).toBe(false);
  expect(customRouteValues(template, 'vless-edge', 'ip')).toEqual(['geoip:cn']);
  setQuickRoutingVisible(template, 'vless-edge', true);
  template.outbounds![0].tag = 'moved';
  propagateOutboundTagRename(template, 'vless-edge', 'moved');
  expect(customRouteValues(template, 'moved', 'ip')).toEqual(['geoip:cn']);
  expect(template.routing?.rules?.[2].ruleTag).toBe('xui-custom-outbound-ip:moved');
  expect(isQuickRoutingVisible(template, 'moved')).toBe(true);
  applyOutboundDeletion(template, 0);
  expect(isQuickRoutingVisible(template, 'moved')).toBe(false);
  expect(template.routing?.rules?.slice(0, 2)).toEqual(original);
  expect(customRouteValues(template, 'moved', 'ip')).toEqual([]);
  expect(customRouteValues(template, 'ss-edge', 'domain')).toEqual(['full:example.com']);
});
