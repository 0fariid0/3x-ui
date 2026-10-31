import { describe, expect, it } from 'vitest';
import { isTunnelLocalIP, canonicalBlockIP, IPBlockMutationSchema } from '@/schemas/api/ip-blocks';
import { normalizeClientIps } from '@/lib/clients/ip-log';

describe('IP block targeting', () => {
  it('canonicalizes IPv6 and mapped IPv4 before membership comparison', () => {
    expect(canonicalBlockIP(' 2001:0DB8:0:0::1 ')).toBe('2001:db8::1');
    expect(canonicalBlockIP('::ffff:c000:0201')).toBe('192.0.2.1');
    expect(canonicalBlockIP('192.0.2.1')).toBe('192.0.2.1');
  });
  it.each(['', 'example.com', '192.0.2.0/24', '1.2.3.4;echo hi', '999.1.1.1'])('rejects non-address %s', (value) => {
    expect(canonicalBlockIP(value)).toBeNull();
  });
  it('requires an exact nonnegative integer server ID and supported operation', () => {
    const request = { nodeId: 0, ip: '192.0.2.1', action: 'ban', email: '' };
    expect(IPBlockMutationSchema.safeParse(request).success).toBe(true);
    for (const nodeId of [-1, 1.5, '0']) expect(IPBlockMutationSchema.safeParse({ ...request, nodeId }).success).toBe(false);
    expect(IPBlockMutationSchema.safeParse({ ...request, action: 'flush' }).success).toBe(false);
  });
  it('never infers server identity from an ambiguous display name', () => {
    const rows = normalizeClientIps([{ ip: '192.0.2.1', node: 'same', nodeId: 7 }, { ip: '192.0.2.2', node: 'same' }, { ip: '192.0.2.3', node: '', nodeId: 0 }]);
    expect(rows.map((row) => row.nodeId)).toEqual([7, undefined, 0]);
  });
});

describe('GRE-TUN local ranges', () => {
  it.each(['10.10.1.1', '10.20.255.2', '10.30.3.1', '10.40.0.2', '::ffff:10.30.3.1', '::ffff:a1e:301'])('recognizes %s', (ip) => {
    expect(isTunnelLocalIP(ip)).toBe(true);
  });
  it.each(['5.210.158.87', '10.0.0.1', '10.31.3.1', '192.168.1.1', '100.64.1.1', '2001:db8::1', '10.30.3.999', '10.30.3.1/24', ''])('does not exempt %s', (ip) => {
    expect(isTunnelLocalIP(ip)).toBe(false);
  });
});
