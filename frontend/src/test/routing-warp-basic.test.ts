import { describe, expect, it } from 'vitest';

import type { XraySettingsValue } from '@/hooks/useXraySetting';
import { ruleGetter, ruleSetter } from '@/pages/xray/basics/helpers';

describe('WARP basic routing', () => {
  it('stores and removes selected service domains on the warp outbound rule', () => {
    const tt = {
      outbounds: [{ tag: 'warp', protocol: 'wireguard' }],
      routing: { rules: [] },
    } as XraySettingsValue;

    ruleSetter(tt, 'warp', 'domain', ['geosite:google', 'geosite:openai']);
    expect(ruleGetter(tt, 'warp', 'domain')).toEqual(['geosite:google', 'geosite:openai']);

    ruleSetter(tt, 'warp', 'domain', []);
    expect(ruleGetter(tt, 'warp', 'domain')).toEqual([]);
  });
});
