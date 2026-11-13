import { useState } from 'react';
import { Alert, Button, Form, Input, Modal } from 'antd';
import { useTranslation } from 'react-i18next';

import { buildWireguardConf } from '@/lib/xray/proton-outbound';

interface Props {
  open: boolean;
  tags: string[];
  hasDefault: boolean;
  onClose: () => void;
  onAdd: (outbound: Record<string, unknown>) => void;
}

type Values = { tag: string; config: string };

export default function ProtonModal({ open, tags, hasDefault, onClose, onAdd }: Props) {
  const { i18n } = useTranslation();
  const fa = i18n.language.toLowerCase().startsWith('fa');
  const label = (en: string, persian: string) => fa ? persian : en;
  const [form] = Form.useForm<Values>();
  const [error, setError] = useState('');

  function submit(values: Values) {
    const tag = values.tag.trim();
    if (!hasDefault || !tag || tags.includes(tag) || tag.startsWith('nord-') || tag.startsWith('NordVPN')) {
      setError(label('Keep an existing default outbound and use a unique tag.', 'یک خروجی پیش‌فرض داشته باش و نام یکتایی برای خروجی جدید انتخاب کن.'));
      return;
    }
    try {
      if (/^\s*PrivateKey\s*=\s*\*+\s*$/im.test(values.config)) {
        setError(label('Replace the masked PrivateKey with the real key from your Proton file.', 'به‌جای ستاره‌ها، PrivateKey اصلی فایل Proton را وارد کن.'));
        return;
      }
      onAdd(buildWireguardConf(tag, values.config));
      onClose();
    } catch {
      setError(label('Check the WireGuard configuration fields.', 'فیلدهای فایل WireGuard را بررسی کن.'));
    }
  }

  return (
    <Modal open={open} title="Proton VPN Free · WireGuard" footer={null} onCancel={onClose}
      className="proton-config-modal" destroyOnHidden afterClose={() => setError('')}>
      <Form form={form} layout="vertical" initialValues={{ tag: 'proton-free' }} clearOnDestroy onFinish={submit}>
        <Alert type="info" showIcon title={label('Paste your Proton VPN WireGuard .conf file', 'متن فایل WireGuard حساب Proton VPN را وارد کن')}
          description={<a href="https://protonvpn.com/support/wireguard-configurations" target="_blank" rel="noopener noreferrer">{label('Official Proton VPN guide', 'راهنمای رسمی دریافت فایل از Proton VPN')}</a>} />
        <Form.Item name="tag" label={label('Outbound tag', 'نام خروجی')} rules={[{ required: true }]}>
          <Input dir="ltr" maxLength={100} />
        </Form.Item>
        <Form.Item name="config" label="WireGuard .conf" rules={[{ required: true }]}>
          <Input.TextArea dir="ltr" rows={8} maxLength={16384} spellCheck={false} autoComplete="off" />
        </Form.Item>
        {error && <Alert type="error" showIcon title={error} />}
        <p>{label('Select the new tag in a routing rule, then save the Xray settings. System routes and DNS from the file are not applied.', 'نام خروجی جدید را در قانون مسیریابی انتخاب کن، سپس تنظیمات Xray را ذخیره کن. مسیرهای سیستم و DNS داخل فایل اعمال نمی‌شوند.')}</p>
        <Button type="primary" htmlType="submit">{label('Add outbound', 'افزودن خروجی')}</Button>
      </Form>
    </Modal>
  );
}
