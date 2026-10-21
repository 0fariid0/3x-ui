import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ArrowDownOutlined, ArrowUpOutlined, DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import { Button, Modal, Select, Space, Typography, message } from 'antd';

import type { HostRecord } from '@/api/queries/useHostsQuery';
import type { InboundOption } from '@/schemas/client';
import type { DisplayHostPriority, SubscriptionDisplayHost } from '@/schemas/api/host';

interface DisplayPriorityModalProps {
  open: boolean;
  displayHost?: SubscriptionDisplayHost;
  hosts: HostRecord[];
  inboundOptions: InboundOption[];
  save: (payload: SubscriptionDisplayHost) => Promise<{ success?: boolean; msg?: string } | undefined>;
  onOpenChange: (open: boolean) => void;
}

const encodePriority = (priority: DisplayHostPriority) => JSON.stringify(priority);

export default function DisplayPriorityModal({
  open,
  displayHost,
  hosts,
  inboundOptions,
  save,
  onOpenChange,
}: DisplayPriorityModalProps) {
  const { t } = useTranslation();
  const [messageApi, contextHolder] = message.useMessage();
  const [rows, setRows] = useState<DisplayHostPriority[]>([]);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (open) setRows(displayHost?.priorities ?? []);
  }, [open, displayHost?.priorities]);

  const options = useMemo(() => inboundOptions.flatMap((inbound) => {
    const inboundLabel = `${inbound.remark || inbound.tag || `#${inbound.id}`} · ${inbound.protocol || ''}`;
    const result = [{
      value: encodePriority({ inboundId: inbound.id, hostGroupId: '', address: '' }),
      label: `${inboundLabel} — ${t('pages.hosts.displayHost.defaultAddress')}`,
    }];
    for (const host of hosts) {
      if (!host.inboundIds.includes(inbound.id)) continue;
      for (const address of host.hosts) {
        result.push({
          value: encodePriority({ inboundId: inbound.id, hostGroupId: host.groupId, address }),
          label: `${inboundLabel} — ${host.remark || address} (${address})`,
        });
      }
    }
    return result;
  }), [hosts, inboundOptions, t]);

  const updateRow = (index: number, value: string) => {
    setRows((current) => current.map((row, rowIndex) => rowIndex === index ? JSON.parse(value) as DisplayHostPriority : row));
  };

  const moveRow = (index: number, offset: number) => {
    setRows((current) => {
      const target = index + offset;
      if (target < 0 || target >= current.length) return current;
      const next = [...current];
      [next[index], next[target]] = [next[target], next[index]];
      return next;
    });
  };

  const submit = async () => {
    if (!displayHost) return;
    setSaving(true);
    try {
      const response = await save({ ...displayHost, priorities: rows });
      if (response?.success) {
        messageApi.success(t('pages.hosts.displayHost.prioritySaved'));
        onOpenChange(false);
      }
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal
      open={open}
      title={t('pages.hosts.displayHost.priorityTitle')}
      okText={t('save')}
      cancelText={t('cancel')}
      confirmLoading={saving}
      onOk={submit}
      onCancel={() => onOpenChange(false)}
      width={760}
    >
      {contextHolder}
      <Typography.Paragraph type="secondary">
        {t('pages.hosts.displayHost.priorityHint')}
      </Typography.Paragraph>
      <Space orientation="vertical" style={{ width: '100%' }} size="middle">
        {rows.map((row, index) => (
          <Space key={`${index}-${encodePriority(row)}`} style={{ width: '100%' }} align="center">
            <Typography.Text style={{ minWidth: 28 }}>{index + 1}</Typography.Text>
            <Select
              showSearch
              style={{ minWidth: 470, flex: 1 }}
              value={encodePriority(row)}
              options={options}
              optionFilterProp="label"
              onChange={(value) => updateRow(index, value)}
            />
            <Button icon={<ArrowUpOutlined />} disabled={index === 0} onClick={() => moveRow(index, -1)} />
            <Button icon={<ArrowDownOutlined />} disabled={index === rows.length - 1} onClick={() => moveRow(index, 1)} />
            <Button danger icon={<DeleteOutlined />} onClick={() => setRows((current) => current.filter((_, rowIndex) => rowIndex !== index))} />
          </Space>
        ))}
        <Button
          block
          icon={<PlusOutlined />}
          disabled={options.length === 0}
          onClick={() => setRows((current) => [...current, JSON.parse(options[0].value) as DisplayHostPriority])}
        >
          {t('pages.hosts.displayHost.addPriority')}
        </Button>
        <Typography.Text type="secondary">
          {t('pages.hosts.displayHost.priorityCount', { count: rows.length })}
        </Typography.Text>
      </Space>
    </Modal>
  );
}
