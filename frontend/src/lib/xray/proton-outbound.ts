function ipFamily(ip: string): number {
  if (/^(\d{1,3}\.){3}\d{1,3}$/.test(ip) && ip.split('.').every(part => Number(part) <= 255)) return 4;
  if (ip.includes(':') && /^[\da-f:.]+$/i.test(ip)) {
    try { new URL(`http://[${ip}]/`); return 6; } catch { return 0; }
  }
  return 0;
}

function cidrs(value = '') {
  const list = value.split(',').map(item => item.trim());
  if (list.some(item => {
    const [ip, prefix, extra] = item.split('/');
    const family = ipFamily(ip);
    return !family || extra !== undefined || !/^\d+$/.test(prefix ?? '') || Number(prefix) > (family === 4 ? 32 : 128);
  })) throw new Error('Invalid WireGuard address or allowed IPs');
  return list;
}

export function buildWireguardConf(tag: string, config: string) {
  const sections = new Map<string, Map<string, string>>();
  let current: Map<string, string> | undefined;
  for (const raw of config.split(/\r?\n/)) {
    const line = raw.split('#')[0].trim();
    if (!line) continue;
    if (/^\[\w+\]$/.test(line)) {
      const name = line.slice(1, -1).toLowerCase();
      if (!['interface', 'peer'].includes(name) || sections.has(name)) throw new Error('Expected one Interface and one Peer');
      current = new Map();
      sections.set(name, current);
      continue;
    }
    const index = line.indexOf('=');
    if (!current || index < 1) throw new Error('Invalid WireGuard line');
    const key = line.slice(0, index).trim().toLowerCase();
    if (current.has(key)) throw new Error('Duplicate WireGuard option');
    current.set(key, line.slice(index + 1).trim());
  }
  const iface = sections.get('interface');
  const peer = sections.get('peer');
  const secretKey = iface?.get('privatekey') || '';
  const publicKey = peer?.get('publickey') || '';
  const preSharedKey = peer?.get('presharedkey');
  const keyValid = (key: string) => /^[A-Za-z0-9+/]{43}=$/.test(key);
  if (!keyValid(secretKey) || !keyValid(publicKey) || (preSharedKey !== undefined && !keyValid(preSharedKey))) {
    throw new Error('Invalid WireGuard key');
  }
  const endpoint = peer?.get('endpoint') || '';
  const match = /^(\[[\da-f:.]+\]|[a-z\d.-]+):(\d{1,5})$/i.exec(endpoint);
  if (!match || Number(match[2]) < 1 || Number(match[2]) > 65535 ||
    (match[1].startsWith('[') && ipFamily(match[1].slice(1, -1)) !== 6)) throw new Error('Invalid WireGuard endpoint');
  const integer = (raw: string | undefined, min: number, max: number) => {
    if (raw === undefined) return undefined;
    const value = Number(raw);
    if (!/^\d+$/.test(raw) || value < min || value > max) throw new Error('Invalid WireGuard numeric option');
    return value;
  };
  return {
    tag, protocol: 'wireguard',
    settings: {
      secretKey, address: cidrs(iface?.get('address')), noKernelTun: true,
      mtu: integer(iface?.get('mtu'), 576, 65535),
      peers: [{ publicKey, preSharedKey, endpoint, allowedIPs: cidrs(peer?.get('allowedips')), keepAlive: integer(peer?.get('persistentkeepalive'), 0, 65535) }],
    },
  };
}
