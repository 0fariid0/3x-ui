import { useState } from 'react';
import { Alert, Button, Form, Input, Modal, Select } from 'antd';
import { useTranslation } from 'react-i18next';

import { buildWireguardConf } from '@/lib/xray/proton-outbound';
import { canImportProton, getProtonCandidates, PROTON_OUTBOUND_TAG } from '@/lib/xray/proton-routing';
import type { XraySettingsValue } from '@/hooks/useXraySetting';

interface Props {
  open: boolean;
  template: XraySettingsValue | null;
  reservedTags: string[];
  onClose: () => void;
  onAdd: (outbound: ReturnType<typeof buildWireguardConf>, legacyTag?: string) => void;
}

type Values = { legacyTag?: string; config: string };

export default function ProtonModal({ open, template, reservedTags, onClose, onAdd }: Props) {
  const { i18n } = useTranslation();
  const fa = i18n.language.toLowerCase().startsWith('fa');
  const label = (en: string, persian: string) => fa ? persian : en;
  const [form] = Form.useForm<Values>();
  const [error, setError] = useState('');

  const candidates = getProtonCandidates(template);
  function submit(values: Values) {
    if (!canImportProton(template, reservedTags, values.legacyTag)) {
      setError(label('Keep a default outbound, free the proton-free tag if used by another outbound, and choose the legacy Proton outbound if needed.', 'یک خروجی پیش‌فرض داشته باش. اگر نام proton-free با خروجی دیگری تداخل دارد، اول تداخل را برطرف کن. در صورت وجود چند Proton قدیمی، یکی را انتخاب کن.'));
      return;
    }
    try {
      if (/^\s*PrivateKey\s*=\s*\*+\s*$/im.test(values.config)) {
        setError(label('Replace the masked PrivateKey with the real key from your Proton file.', 'به‌جای ستاره‌ها، PrivateKey اصلی فایل Proton را وارد کن.'));
        return;
      }
      onAdd(buildWireguardConf(PROTON_OUTBOUND_TAG, values.config), values.legacyTag);
      onClose();
    } catch {
      setError(label('Check the WireGuard configuration fields.', 'فیلدهای فایل WireGuard را بررسی کن.'));
    }
  }

  return (
    <Modal open={open} title="Proton VPN Free · WireGuard" footer={null} onCancel={onClose}
      className="proton-config-modal" destroyOnHidden afterClose={() => setError('')}>
      <Form form={form} layout="vertical" clearOnDestroy onFinish={submit}>
        <Alert type="info" showIcon title={label('Paste your Proton VPN WireGuard .conf file', 'متن فایل WireGuard حساب Proton VPN را وارد کن')}
          description={<a href="https://protonvpn.com/support/wireguard-configurations" target="_blank" rel="noopener noreferrer">{label('Official Proton VPN guide', 'راهنمای رسمی دریافت فایل از Proton VPN')}</a>} />
        <Form.Item label={label('Outbound tag', 'نام ثابت خروجی')}>
          <Input dir="ltr" value={PROTON_OUTBOUND_TAG} readOnly />
        </Form.Item>
        {candidates.length > 1 && <Form.Item name="legacyTag" preserve={false}
          label={label('Existing Proton outbound to update', 'خروجی قبلی Proton برای به‌روزرسانی')}
          rules={[{ required: true }]}>
          <Select options={candidates.map(value => ({ value, label: value }))} />
        </Form.Item>}
        <Form.Item name="config" label="WireGuard .conf" rules={[{ required: true }]}>
          <Input.TextArea dir="ltr" rows={8} maxLength={16384} spellCheck={false} autoComplete="off" />
        </Form.Item>
        {error && <Alert type="error" showIcon title={error} />}
        <p>{label('The Proton routing row detects this outbound automatically. Save the Xray settings after editing. System routes and DNS from the file are not applied.', 'ردیف مسیریابی Proton خروجی را خودکار تشخیص می‌دهد. بعد از تغییرات، تنظیمات Xray را ذخیره کن. مسیرهای سیستم و DNS داخل فایل اعمال نمی‌شوند.')}</p>
        <Button type="primary" htmlType="submit">{candidates.length ? label('Update Proton outbound', 'به‌روزرسانی خروجی Proton') : label('Add outbound', 'افزودن خروجی')}</Button>
      </Form>
    </Modal>
  );
}
