import { describe, expect, it } from 'vitest';
import { buildWireguardConf } from '@/lib/xray/proton-outbound';
import { WireguardOutboundSettingsSchema } from '@/schemas/protocols/outbound/wireguard';

const key = 'A'.repeat(43) + '=';
const config = `[Interface]
PrivateKey = ${key}
Address = 10.2.0.2/32, fd00::2/128
DNS = 10.2.0.1
PostUp = ignored
[Peer]
PublicKey = ${key}
AllowedIPs = 0.0.0.0/0, ::/0
Endpoint = [2001:db8::1]:51820
PersistentKeepalive = 25
`;

describe('Proton VPN WireGuard outbound', () => {
  it('imports a WireGuard conf including IPv6 and does not apply DNS or shell hooks', () => {
    const result = buildWireguardConf('proton-free', config.replaceAll('\n', '\r\n'));
    const settings = WireguardOutboundSettingsSchema.parse(result.settings);
    expect(settings.address).toEqual(['10.2.0.2/32', 'fd00::2/128']);
    expect(settings.peers[0]).toMatchObject({ endpoint: '[2001:db8::1]:51820', keepAlive: 25, allowedIPs: ['0.0.0.0/0', '::/0'] });
    expect(settings.noKernelTun).toBe(true);
    expect(Object.keys(result.settings)).not.toContain('DNS');
    expect(Object.keys(result.settings)).not.toContain('PostUp');
  });
  it('accepts Proton comments, DNS, both interface families and only the active endpoint', () => {
    const sample = config.replace('Endpoint = [2001:db8::1]:51820', 'Endpoint = 185.107.56.49:51820\n# Endpoint = [2a02:6ea0:e00a:2242::10]:51820')
      .replace('DNS = 10.2.0.1', 'DNS = 10.2.0.1, 2a07:b944::2:1')
      .replace('[Peer]', '[Peer]\n# NL-FREE#125');
    expect(buildWireguardConf('proton-free', sample).settings.peers[0].endpoint).toBe('185.107.56.49:51820');
    expect(() => buildWireguardConf('proton-free', sample.replace(`PrivateKey = ${key}`, 'PrivateKey = *****'))).toThrow();
  });
  it('rejects a missing private key, multiple peers and invalid addresses/ports', () => {
    for (const input of [
      config.replace('PrivateKey', 'WrongKey'),
      config + '[Peer]\n',
      config.replace('10.2.0.2/32', '10.2.0.2/33'),
      config.replace(':51820', ':65536'),
    ]) expect(() => buildWireguardConf('proton', input)).toThrow();
  });
});
