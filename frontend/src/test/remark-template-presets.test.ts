import { describe, expect, it } from 'vitest';

import { REMARK_TEMPLATE_PRESETS, previewRemark } from '@/lib/remark/remarkVariables';

describe('remark template presets', () => {
  it('includes the detailed per-host template and previews all variables', () => {
    const template = '{{INBOUND}}-{{EMAIL}}|📊{{TRAFFIC_LEFT}}|⏳{{DAYS_LEFT}}D';
    expect(REMARK_TEMPLATE_PRESETS).toContain(template);
    expect(previewRemark(template)).toBe('Germany-john|📊41.60GB|⏳12D');
  });
});
