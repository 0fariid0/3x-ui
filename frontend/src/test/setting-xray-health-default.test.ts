import { describe, expect, it } from 'vitest';

import { AllSetting } from '@/models/setting';
import { AllSettingSchema } from '@/schemas/setting';

describe('Xray health settings in full panel settings', () => {
  it('defaults the health monitor to disabled', () => {
    expect(new AllSetting().xrayHealthEnable).toBe(false);
  });

  it('preserves health settings loaded from the backend', () => {
    const setting = new AllSetting({
      xrayHealthEnable: true,
      xrayHealthFailureThreshold: 4,
      xrayHealthRestartCooldown: 10,
      xrayHealthMaxRestarts: 5,
      xrayHealthWindowMinutes: 60,
    });

    expect(setting.xrayHealthEnable).toBe(true);
    expect(setting.xrayHealthFailureThreshold).toBe(4);
    expect(setting.xrayHealthRestartCooldown).toBe(10);
    expect(setting.xrayHealthMaxRestarts).toBe(5);
    expect(setting.xrayHealthWindowMinutes).toBe(60);
  });

  it('accepts the health fields in a settings save payload', () => {
    const result = AllSettingSchema.safeParse(new AllSetting());
    expect(result.success).toBe(true);
  });
});
