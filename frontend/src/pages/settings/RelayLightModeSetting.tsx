import { Alert, Modal, Switch } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { SettingListItem } from '@/components/ui';
import { HttpUtil } from '@/utils';

const endpoint = '/panel/api/setting/relayLightMode';
type RelayMode = { enabled: boolean; restarting?: boolean };

export default function RelayLightModeSetting() {
  const { t } = useTranslation();
  const [target, setTarget] = useState<boolean | null>(null);
  const [saving, setSaving] = useState(false);
  const [restarting, setRestarting] = useState(false);
  const mode = useQuery({
    queryKey: ['settings', 'relayLightMode'],
    queryFn: async () => {
      const response = await HttpUtil.get<RelayMode>(endpoint, undefined, { silent: true });
      if (!response.success || typeof response.obj?.enabled !== 'boolean') throw new Error('relay mode unavailable');
      return response.obj;
    },
  });

  async function apply() {
    if (target === null || saving) return;
    setSaving(true);
    try {
      const response = await HttpUtil.post<RelayMode>(endpoint, { enabled: target, confirmed: true });
      if (!response.success || !response.obj) return;
      setTarget(null);
      if (response.obj.restarting) {
        setRestarting(true);
        window.setTimeout(() => window.location.reload(), 6500);
      } else {
        void mode.refetch();
      }
    } finally {
      setSaving(false);
    }
  }

  return (
    <>
      <SettingListItem paddings="small" title={t('pages.settings.relayLightMode')}
        description={t('pages.settings.relayLightModeDesc')}>
        <Switch aria-label={t('pages.settings.relayLightMode')} checked={mode.data?.enabled ?? false}
          loading={mode.isPending || saving || restarting}
          disabled={!mode.data || restarting} onChange={setTarget} />
      </SettingListItem>
      {restarting && <Alert type="info" showIcon title={t('pages.settings.restartPanelSuccess')} />}
      {mode.isError && <Alert type="error" showIcon title={t('somethingWentWrong')} />}
      <Modal open={target !== null} title={t('pages.settings.relayLightMode')}
        onCancel={() => { if (!saving) setTarget(null); }}
        onOk={() => void apply()} confirmLoading={saving}
        okText={t('confirm')} cancelText={t('cancel')} okButtonProps={{ danger: target === true }}>
        <Alert type="warning" showIcon
          title={t(target ? 'pages.settings.relayLightModeEnableWarning' : 'pages.settings.relayLightModeDisableWarning')} />
      </Modal>
    </>
  );
}
