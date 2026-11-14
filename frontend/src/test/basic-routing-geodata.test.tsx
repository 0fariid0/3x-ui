import { useState } from 'react';
import { fireEvent, screen } from '@testing-library/react';
import { expect, it, vi } from 'vitest';

import type { XraySettingsValue } from '@/hooks/useXraySetting';
import RoutingBasic from '@/pages/xray/routing/RoutingBasic';
import { renderWithProviders } from '@/test/test-utils';

vi.mock('@/components/geodata', () => ({
  GeoBrowserModal: ({ kind, value, searchDatabase, onApply }: {
    kind: 'ip' | 'site'; value: string; searchDatabase: boolean; onApply: (value: string) => void;
  }) => searchDatabase && (
    <button type="button" onClick={() => onApply([value, kind === 'ip' ? 'geoip:cn' : 'geosite:telegram'].filter(Boolean).join(', '))}>
      {kind}
    </button>
  ),
}));

it('opens the right database for each routing field and adds entries to its own rule', () => {
  function Harness() {
    const [settings, setSettings] = useState<XraySettingsValue | null>({
      outbounds: ['direct', 'blocked', 'warp', 'NordVPN', 'custom-vless'].map(tag => ({
        tag, protocol: tag === 'blocked' ? 'blackhole' : tag === 'custom-vless' ? 'vless' : 'freedom',
      })),
      routing: { rules: [{ type: 'field', outboundTag: 'direct', domain: ['full:www.google.com'] }] },
    });
    return <>
      <RoutingBasic templateSettings={settings} setTemplateSettings={setSettings} onShowCustomOutbound={vi.fn()} onSelectCustomOutbound={vi.fn()} />
      <pre data-testid="rules">{JSON.stringify(settings?.routing?.rules)}</pre>
    </>;
  }

  renderWithProviders(<Harness />);
  const buttons = screen.getAllByRole('button', { name: / — / });
  expect(buttons).toHaveLength(9);
  const cases = [
    ['blocked', 'ip'], ['blocked', 'site'], ['direct', 'ip'], ['direct', 'site'],
    ['IPv4', 'site'], ['warp', 'site'], ['NordVPN', 'site'], ['custom-vless', 'site'], ['custom-vless', 'ip'],
  ] as const;
  for (const [index, [outboundTag, kind]] of cases.entries()) {
    fireEvent.click(buttons[index]);
    fireEvent.click(screen.getByRole('button', { name: kind }));
    const property = kind === 'ip' ? 'ip' : 'domain';
    const values = kind === 'ip' ? ['geoip:cn'] : ['geosite:telegram'];
    if (outboundTag === 'direct' && kind === 'site') values.unshift('full:www.google.com');
    const rules = JSON.parse(screen.getByTestId('rules').textContent ?? '[]');
    expect(rules).toContainEqual(expect.objectContaining({ outboundTag, [property]: values }));
    expect(rules[0]).toEqual(expect.objectContaining({ outboundTag: 'direct', domain: expect.arrayContaining(['full:www.google.com']) }));
  }
});
