import { describe, expect, it } from 'vitest';
import { getProtonOutboundTag, PROTON_OUTBOUND_TAG, saveProtonOutbound, setProtonRules } from '@/lib/xray/proton-routing';
import type { XraySettingsValue } from '@/hooks/useXraySetting';
import { buildWireguardConf } from '@/lib/xray/proton-outbound';

const key = 'A'.repeat(43) + '=';
const imported = buildWireguardConf(PROTON_OUTBOUND_TAG, ['[Interface]', 'PrivateKey = ' + key, 'Address = 10.2.0.2/32', '[Peer]', 'PublicKey = ' + key, 'AllowedIPs = 0.0.0.0/0', 'Endpoint = 192.0.2.1:51820'].join('\n'));

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
    const savedIPRule = structuredClone(template.routing!.rules![2]);
    setProtonRules(template, 'proton-free', ['domain:example.net']);
    expect(template.routing!.rules![2]).toEqual(savedIPRule);
    setProtonRules(template, 'proton-free', []);
    expect(template.routing?.rules).toEqual([...original.routing!.rules!, savedIPRule]);
    setProtonRules(template, 'proton-free', [], []);
    expect(template).toEqual(original);
  });
});

it('detects legacy Proton, migrates routes, and updates the fixed outbound in place', () => {
  const template: XraySettingsValue = {
    outbounds: [{ tag: 'direct', protocol: 'freedom' }, { tag: 'proton-nl-223', protocol: 'wireguard' }],
    routing: { rules: [{ type: 'field', ruleTag: 'xui-proton-domain:proton-nl-223', outboundTag: 'proton-nl-223', domain: ['domain:example.org'] }] },
  };
  expect(getProtonOutboundTag(template)).toBe('proton-nl-223');
  expect(saveProtonOutbound(template, imported, [])).toBe(true);
  expect(getProtonOutboundTag(template)).toBe(PROTON_OUTBOUND_TAG);
  expect(template.routing?.rules?.[0]).toMatchObject({
    ruleTag: 'xui-proton-domain:proton-free', outboundTag: PROTON_OUTBOUND_TAG, domain: ['domain:example.org'],
  });
  expect(saveProtonOutbound(template, imported, [])).toBe(true);
  expect(template.outbounds?.map(item => item.tag)).toEqual(['direct', PROTON_OUTBOUND_TAG]);
});

it('does not select NordVPN or an unrelated WireGuard outbound and rejects a conflicting tag', () => {
  const template: XraySettingsValue = { outbounds: [
    { tag: 'nord-nl', protocol: 'wireguard' }, { tag: 'office-wg', protocol: 'wireguard' },
    { tag: PROTON_OUTBOUND_TAG, protocol: 'freedom' },
  ] };
  expect(getProtonOutboundTag(template)).toBeUndefined();
  expect(saveProtonOutbound(template, imported, [])).toBe(false);
  expect(template.outbounds?.[2].protocol).toBe('freedom');
});

it('requires a choice for multiple legacy outbounds and migrates only the selected one', () => {
  const template: XraySettingsValue = { outbounds: [
    { tag: 'direct', protocol: 'freedom' },
    { tag: 'proton-nl', protocol: 'wireguard' },
    { tag: 'proton-us', protocol: 'wireguard' },
  ] };
  expect(getProtonOutboundTag(template)).toBeUndefined();
  expect(saveProtonOutbound(template, imported, [])).toBe(false);
  expect(saveProtonOutbound(template, imported, [PROTON_OUTBOUND_TAG], 'proton-us')).toBe(false);
  expect(saveProtonOutbound(template, imported, [], 'proton-us')).toBe(true);
  expect(template.outbounds?.map(item => item.tag)).toEqual(['direct', 'proton-nl', PROTON_OUTBOUND_TAG]);
});
