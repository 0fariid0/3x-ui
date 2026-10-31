import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Alert, Button, Col, Form, Input, InputNumber, Row, Modal, Select, Space, Spin, Switch, Typography, message } from 'antd';
import { z } from 'zod';
import { HttpUtil } from '@/utils';
import { NodeListSchema } from '@/schemas/node';
import { keys } from '@/api/queryKeys';
import ClientIpManagerModal from '@/pages/clients/ClientIpManagerModal';

const PolicySchema = z.object({
  exemptIPs: z.string(), enabled: z.boolean(), banSeconds: z.number().int(), findSeconds: z.number().int(),
  maxRetry: z.number().int(), scanSeconds: z.number().int(), liveSeconds: z.number().int(),
  historySeconds: z.number().int(), disconnect: z.boolean(),
});
const StatusSchema = z.object({ settings: PolicySchema, available: z.boolean(), reason: z.string() });
type Policy = z.infer<typeof PolicySchema>;

function PolicyForm({ nodeId, initial, available, onDirty }: { nodeId: number; initial: Policy; available: boolean; onDirty: (value: boolean) => void }) {
  const { t } = useTranslation();
  const text = (key: string) => t(`pages.settings.ipBlocking.${key}`);
  const [form] = Form.useForm<Policy>();
  const [busy, setBusy] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [messageApi, contextHolder] = message.useMessage();
  const client = useQueryClient();
  const banSeconds = Form.useWatch('banSeconds', form);
  const permanent = banSeconds === -1;
  async function save(values: Policy) {
    setBusy(true);
    try {
      const response = await HttpUtil.post(`/panel/api/server/ipBlocks/settings?nodeId=${nodeId}`, values, { headers: { 'Content-Type': 'application/json' } });
      if (!response?.success) throw new Error(response?.msg || text('failed'));
      const result = StatusSchema.parse(response.obj);
      form.setFieldsValue(result.settings);
      setDirty(false); onDirty(false);
      messageApi.success(text('saved'));
      await client.invalidateQueries({ queryKey: ['ip-block-settings', nodeId] });
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : text('failed'));
    } finally { setBusy(false); }
  }
  const field = (name: keyof Policy, min: number, max: number, step = 1) => (
    <Col xs={24} sm={12} lg={8} key={name}>
      <Form.Item name={name} label={text(name)} rules={[{ required: true }, { type: 'integer', min, max }]}>
        <InputNumber min={min} max={max} step={step} style={{ width: '100%' }} />
      </Form.Item>
    </Col>
  );
  return <>
    {contextHolder}
    <Form form={form} layout="vertical" initialValues={initial} onFinish={save} onValuesChange={() => { setDirty(true); onDirty(true); }} disabled={!available || busy}>
      <Row gutter={16}>
        <Col xs={24} sm={12}><Form.Item name="enabled" label={text('enabled')} valuePropName="checked"><Switch /></Form.Item></Col>
        <Col xs={24} sm={12}><Form.Item label={text('permanent')}><Switch checked={permanent} onChange={(checked) => { form.setFieldValue('banSeconds', checked ? -1 : 1800); setDirty(true); onDirty(true); }} /></Form.Item></Col>
        <Col xs={24} sm={12} lg={8}><Form.Item name="banSeconds" label={text('banSeconds')} rules={[{ required: true }, { type: 'integer', min: permanent ? -1 : 1, max: 31536000 }]}><InputNumber disabled={permanent || !available || busy} min={permanent ? -1 : 1} max={31536000} style={{ width: '100%' }} /></Form.Item></Col>
        {field('maxRetry', 1, 100)}
        {field('findSeconds', 1, 86400)}
        {field('scanSeconds', 10, 300, 10)}
        {field('liveSeconds', 10, 600)}
        {field('historySeconds', 10, 86400)}
        <Col span={24}><Form.Item name="exemptIPs" label={text('exemptIPs')} extra={text('exemptHelp')}><Input.TextArea rows={3} maxLength={8192} dir="ltr" /></Form.Item></Col>
        <Col span={24}><Form.Item name="disconnect" label={text('disconnect')} extra={text('disconnectHelp')} valuePropName="checked"><Switch /></Form.Item></Col>
      </Row>
      <Typography.Paragraph type="secondary">{text('timingHelp')}</Typography.Paragraph>
      <Space><Button htmlType="submit" type="primary" loading={busy} disabled={!available || !dirty || busy}>{text('save')}</Button><Button disabled={busy || !dirty} onClick={() => { form.resetFields(); setDirty(false); onDirty(false); }}>{text('reset')}</Button></Space>
    </Form>
  </>;
}

export default function IPBlockTab() {
  const { t } = useTranslation();
  const text = (key: string) => t(`pages.settings.ipBlocking.${key}`);
  const [nodeId, setNodeId] = useState(0);
  const [dirty, setDirty] = useState(false);
  const [modal, modalContext] = Modal.useModal();
  function guarded(action: () => void) {
    if (dirty) modal.confirm({ title: text('unsaved'), onOk: () => { setDirty(false); action(); } });
    else action();
  }
  const [managerOpen, setManagerOpen] = useState(false);
  const nodes = useQuery({ queryKey: keys.nodes.list(), queryFn: async () => {
    const response = await HttpUtil.get('/panel/api/nodes/list');
    if (!response?.success) throw new Error(response?.msg || text('failed'));
    return NodeListSchema.parse(response.obj);
  } });
  const query = useQuery({ queryKey: ['ip-block-settings', nodeId], refetchOnWindowFocus: false, queryFn: async () => {
    const response = await HttpUtil.get(`/panel/api/server/ipBlocks/settings?nodeId=${nodeId}`, undefined, { silent: true });
    if (!response?.success) throw new Error(response?.msg || text('failed'));
    return StatusSchema.parse(response.obj);
  } });
  return <Space orientation="vertical" size="middle" style={{ width: '100%' }}>
    {modalContext}<Typography.Title level={4}>{text('title')}</Typography.Title>
    <Space wrap>
      <Select aria-label={text('server')} value={nodeId} style={{ minWidth: 200 }} onChange={(id) => guarded(() => setNodeId(id))} options={[{ value: 0, label: t('pages.clients.ipManager.localServer') }, ...(nodes.data ?? []).filter(n => n.enable).map(n => ({ value: n.id, label: n.name }))]} />
      <Button loading={query.isFetching} onClick={() => guarded(() => { void query.refetch(); })}>{text('refresh')}</Button>
      <Button onClick={() => setManagerOpen(true)}>{text('manage')}</Button>
    </Space>
    <Alert type="info" showIcon title={text('policy')} description={text('scope')} />
    {query.isError && <Alert type="error" showIcon title={text('failed')} description={query.error.message} />}
    <Spin spinning={query.isFetching}>
      {query.data && !query.isError && <>
        {!query.data.available && <Alert type="warning" title={text('unavailable')} description={query.data.reason} />}
        <PolicyForm key={`${nodeId}:${query.dataUpdatedAt}`} nodeId={nodeId} initial={query.data.settings} available={query.data.available} onDirty={setDirty} />
      </>}
    </Spin>
    {managerOpen && <ClientIpManagerModal open initialNodeId={nodeId} email="" ips={[]} loading={false} clearing={false} onRefresh={() => query.refetch().then(() => undefined)} onClear={() => undefined} onClose={() => setManagerOpen(false)} />}
  </Space>;
}
