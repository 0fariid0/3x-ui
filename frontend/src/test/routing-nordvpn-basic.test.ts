import { describe, expect, it } from 'vitest';

import type { XraySettingsValue } from '@/hooks/useXraySetting';
import {
  getNordOutboundIndex,
  getNordOutboundTag,
  NORDVPN_OUTBOUND_TAG,
  propagateOutboundTagRename,
  ruleGetter,
  ruleSetter,
} from '@/pages/xray/basics/helpers';

function template(tag: string): XraySettingsValue {
  return {
    outbounds: [{ tag, protocol: 'wireguard' }],
    routing: { rules: [] },
  } as XraySettingsValue;
}

describe('NordVPN basic routing', () => {
  it('uses the stable NordVPN outbound tag', () => {
    const tt = template(NORDVPN_OUTBOUND_TAG);
    expect(getNordOutboundIndex(tt)).toBe(0);
    expect(getNordOutboundTag(tt)).toBe('NordVPN');

    ruleSetter(tt, 'NordVPN', 'domain', ['geosite:google', 'geosite:openai']);
    expect(ruleGetter(tt, 'NordVPN', 'domain')).toEqual(['geosite:google', 'geosite:openai']);
  });

  it('recognizes legacy nord-* tags and migrates their routing rules', () => {
    const tt = template('nord-us1234.nordvpn.com');
    ruleSetter(tt, 'nord-us1234.nordvpn.com', 'domain', ['geosite:netflix']);

    expect(getNordOutboundIndex(tt)).toBe(0);
    expect(getNordOutboundTag(tt)).toBe('nord-us1234.nordvpn.com');

    propagateOutboundTagRename(tt, 'nord-us1234.nordvpn.com', 'NordVPN');
    expect(ruleGetter(tt, 'NordVPN', 'domain')).toEqual(['geosite:netflix']);
  });
});
