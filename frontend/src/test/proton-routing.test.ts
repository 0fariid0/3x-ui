import { describe, expect, it } from 'vitest';
import { setProtonRules } from '@/lib/xray/proton-routing';
import type { XraySettingsValue } from '@/hooks/useXraySetting';

describe('Proton basic routing', () => {
  it('adds separate OR-style domain/IP rules without changing default or existing rules', () => {
    const template: XraySettingsValue = {
      outbounds: [{ tag: 'direct', protocol: 'freedom' }, { tag: 'proton-free', protocol: 'wireguard' }],
      routing: { rules: [{ type: 'field', outboundTag: 'NordVPN', domain: ['example.org'] }] },
    };
    const original = structuredClone(template);
    setProtonRules(template, 'proton-free', ['geosite:google', 'geosite:google'], ['geoip:ir']);
    expect(template.outbounds).toEqual(original.outbounds);
    expect(template.routing?.rules).toEqual([
      ...original.routing!.rules!,
      { type: 'field', outboundTag: 'proton-free', ruleTag: 'xui-proton-domain:proton-free', domain: ['geosite:google'] },
      { type: 'field', outboundTag: 'proton-free', ruleTag: 'xui-proton-ip:proton-free', ip: ['geoip:ir'] },
    ]);
    setProtonRules(template, 'proton-free', [], []);
    expect(template).toEqual(original);
  });
});
